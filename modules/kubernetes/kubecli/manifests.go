package kubecli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

// ManifestApplyOptions controls multi-document apply behavior.
type ManifestApplyOptions struct {
	// DefaultNamespace is used when a namespaced object omits metadata.namespace.
	DefaultNamespace string
	// DryRun validates and simulates apply without persisting.
	DryRun bool
}

// ManifestResult is one applied (or dry-run) document.
type ManifestResult struct {
	Index      int    `json:"index"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	Action     string `json:"action"` // created | configured | unchanged | dry-run
	Error      string `json:"error,omitempty"`
}

// ManifestApplySummary aggregates multi-document apply results.
type ManifestApplySummary struct {
	DryRun  bool             `json:"dry_run"`
	Total   int              `json:"total"`
	Applied int              `json:"applied"`
	Failed  int              `json:"failed"`
	Results []ManifestResult `json:"results"`
}

// SplitYAMLDocuments splits a multi-doc YAML stream on document markers.
func SplitYAMLDocuments(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	raw := strings.Split(content, "\n---")
	out := make([]string, 0, len(raw))
	for i, part := range raw {
		doc := part
		if i > 0 {
			// Restore leading content after --- (may include trailing yaml directive text).
			doc = strings.TrimPrefix(doc, "\n")
			if strings.HasPrefix(strings.TrimSpace(doc), "{") {
				// keep as-is
			}
			// Drop optional YAML document end / directives on the separator line remainder.
			if nl := strings.IndexByte(doc, '\n'); nl >= 0 {
				first := strings.TrimSpace(doc[:nl])
				if first == "" || strings.HasPrefix(first, "#") || first == "..." {
					doc = doc[nl+1:]
				} else if !strings.Contains(first, ":") && !strings.HasPrefix(first, "-") {
					// separator had trailing junk like "--- foo"; keep body after first line if junk-only
					doc = doc
				}
			}
		}
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "..." {
			continue
		}
		// Skip pure comments
		onlyComments := true
		for _, line := range strings.Split(doc, "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			onlyComments = false
			break
		}
		if onlyComments {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func stripManagedFields(obj *unstructured.Unstructured) {
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "selfLink")
}

// ApplyManifests server-side applies one or more YAML documents (any kinds).
func ApplyManifests(app *config.AppClients, content string, opts ManifestApplyOptions) (*ManifestApplySummary, error) {
	docs := SplitYAMLDocuments(content)
	if len(docs) == 0 {
		return nil, fmt.Errorf("yaml content is empty")
	}

	restCfg, _, err := RestConfig(app)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))

	summary := &ManifestApplySummary{
		DryRun:  opts.DryRun,
		Total:   len(docs),
		Results: make([]ManifestResult, 0, len(docs)),
	}

	timeout := 60 * time.Second
	if opts.DryRun {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	defaultNS := strings.TrimSpace(opts.DefaultNamespace)

	for i, doc := range docs {
		res := ManifestResult{Index: i + 1}
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
			res.Error = fmt.Sprintf("invalid yaml: %v", err)
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		if obj.Object == nil {
			res.Error = "empty document"
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}

		gvk := obj.GroupVersionKind()
		res.APIVersion = obj.GetAPIVersion()
		res.Kind = gvk.Kind
		res.Name = obj.GetName()
		res.Namespace = obj.GetNamespace()

		if gvk.Kind == "" || obj.GetAPIVersion() == "" {
			res.Error = "apiVersion and kind are required"
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		if strings.TrimSpace(obj.GetName()) == "" {
			res.Error = "metadata.name is required"
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}

		stripManagedFields(&obj)

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			res.Error = fmt.Sprintf("resolve resource: %v", err)
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}

		var resource dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := strings.TrimSpace(obj.GetNamespace())
			if ns == "" {
				ns = defaultNS
				if ns != "" {
					obj.SetNamespace(ns)
				}
			}
			if ns == "" {
				res.Error = "namespace is required for this resource (set metadata.namespace or choose a default namespace)"
				summary.Failed++
				summary.Results = append(summary.Results, res)
				continue
			}
			res.Namespace = ns
			resource = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			resource = dyn.Resource(mapping.Resource)
		}

		applyOpts := metav1.ApplyOptions{
			FieldManager: "containerws",
			Force:        true,
		}
		if opts.DryRun {
			applyOpts.DryRun = []string{metav1.DryRunAll}
		}

		// Detect create vs update for friendlier action labels (best-effort).
		action := "configured"
		if _, getErr := resource.Get(ctx, obj.GetName(), metav1.GetOptions{}); getErr != nil {
			action = "created"
		}

		applied, err := resource.Apply(ctx, obj.GetName(), &obj, applyOpts)
		if err != nil {
			res.Error = err.Error()
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		if opts.DryRun {
			action = "dry-run"
		}
		res.Action = action
		res.APIVersion = applied.GetAPIVersion()
		res.Kind = applied.GetKind()
		res.Name = applied.GetName()
		res.Namespace = applied.GetNamespace()
		summary.Applied++
		summary.Results = append(summary.Results, res)
	}

	return summary, nil
}

// DeleteManifests deletes cluster objects described by a multi-doc YAML stream
// (reverse document order for safer teardown).
func DeleteManifests(app *config.AppClients, content string, defaultNamespace string) (*ManifestApplySummary, error) {
	docs := SplitYAMLDocuments(content)
	if len(docs) == 0 {
		return nil, fmt.Errorf("yaml content is empty")
	}
	refs := make([]ManifestRef, 0, len(docs))
	for i := len(docs) - 1; i >= 0; i-- {
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(docs[i]), &obj.Object); err != nil || obj.Object == nil {
			continue
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" || strings.TrimSpace(obj.GetName()) == "" {
			continue
		}
		ref := ManifestRef{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
			Namespace:  obj.GetNamespace(),
		}
		refs = append(refs, ref)
	}
	return DeleteRefs(app, refs, defaultNamespace)
}

// DeleteRefs deletes cluster objects by stored identity refs.
func DeleteRefs(app *config.AppClients, refs []ManifestRef, defaultNamespace string) (*ManifestApplySummary, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("no resources to delete")
	}
	restCfg, _, err := RestConfig(app)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))

	summary := &ManifestApplySummary{
		Total:   len(refs),
		Results: make([]ManifestResult, 0, len(refs)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defaultNS := strings.TrimSpace(defaultNamespace)
	propagation := metav1.DeletePropagationBackground

	for i, ref := range refs {
		res := ManifestResult{
			Index:      i + 1,
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			Namespace:  ref.Namespace,
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			res.Error = err.Error()
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		mapping, err := mapper.RESTMapping(gv.WithKind(ref.Kind).GroupKind(), gv.Version)
		if err != nil {
			res.Error = fmt.Sprintf("resolve resource: %v", err)
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		var resource dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := strings.TrimSpace(ref.Namespace)
			if ns == "" {
				ns = defaultNS
			}
			if ns == "" {
				res.Error = "namespace is required"
				summary.Failed++
				summary.Results = append(summary.Results, res)
				continue
			}
			res.Namespace = ns
			resource = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			resource = dyn.Resource(mapping.Resource)
		}
		err = resource.Delete(ctx, ref.Name, metav1.DeleteOptions{PropagationPolicy: &propagation})
		if err != nil {
			if apierrors.IsNotFound(err) {
				res.Action = "missing"
				summary.Applied++
				summary.Results = append(summary.Results, res)
				continue
			}
			res.Error = err.Error()
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		res.Action = "deleted"
		summary.Applied++
		summary.Results = append(summary.Results, res)
	}
	return summary, nil
}
