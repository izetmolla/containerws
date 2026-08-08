package kubecli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/config"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

// ApplyExpect constrains which object a pasted YAML may update.
type ApplyExpect struct {
	APIVersion string // optional, e.g. "v1" or "apps/v1"
	Kind       string // required, e.g. "Pod"
	Namespace  string // empty for cluster-scoped
	Name       string // required
}

// ApplyResult is the applied object summary.
type ApplyResult struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	YAML       string `json:"yaml"`
}

// ApplyYAML server-side applies YAML to the cluster, enforcing expect identity.
func ApplyYAML(app *config.AppClients, content string, expect ApplyExpect) (*ApplyResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("yaml content is empty")
	}
	expect.Kind = strings.TrimSpace(expect.Kind)
	expect.Name = strings.TrimSpace(expect.Name)
	expect.Namespace = strings.TrimSpace(expect.Namespace)
	expect.APIVersion = strings.TrimSpace(expect.APIVersion)
	if expect.Kind == "" || expect.Name == "" {
		return nil, fmt.Errorf("expected kind and name are required")
	}

	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(content), &obj.Object); err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}
	if obj.Object == nil {
		return nil, fmt.Errorf("invalid yaml: empty document")
	}

	gvk := obj.GroupVersionKind()
	if gvk.Kind == "" {
		return nil, fmt.Errorf("yaml must include kind")
	}
	if gvk.Version == "" && obj.GetAPIVersion() == "" {
		return nil, fmt.Errorf("yaml must include apiVersion")
	}
	if !strings.EqualFold(gvk.Kind, expect.Kind) {
		return nil, fmt.Errorf("kind mismatch: yaml has %q, expected %q", gvk.Kind, expect.Kind)
	}
	if expect.APIVersion != "" && obj.GetAPIVersion() != expect.APIVersion {
		return nil, fmt.Errorf("apiVersion mismatch: yaml has %q, expected %q", obj.GetAPIVersion(), expect.APIVersion)
	}
	if name := strings.TrimSpace(obj.GetName()); name == "" {
		obj.SetName(expect.Name)
	} else if name != expect.Name {
		return nil, fmt.Errorf("name mismatch: yaml has %q, expected %q", name, expect.Name)
	}
	if expect.Namespace != "" {
		ns := strings.TrimSpace(obj.GetNamespace())
		if ns == "" {
			obj.SetNamespace(expect.Namespace)
		} else if ns != expect.Namespace {
			return nil, fmt.Errorf("namespace mismatch: yaml has %q, expected %q", ns, expect.Namespace)
		}
	}

	// Drop server-managed fields so SSA can proceed cleanly.
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "selfLink")

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
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("resolve resource mapping: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var resource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := strings.TrimSpace(obj.GetNamespace())
		if ns == "" {
			ns = expect.Namespace
		}
		if ns == "" {
			return nil, fmt.Errorf("namespace is required for this resource")
		}
		resource = dyn.Resource(mapping.Resource).Namespace(ns)
	} else {
		resource = dyn.Resource(mapping.Resource)
	}

	applied, err := resource.Apply(ctx, expect.Name, &obj, metav1.ApplyOptions{
		FieldManager: "containerws",
		Force:        true,
	})
	if err != nil {
		return nil, fmt.Errorf("apply failed: %w", err)
	}

	y, err := ToYAML(applied)
	if err != nil {
		y = content
	}
	return &ApplyResult{
		APIVersion: applied.GetAPIVersion(),
		Kind:       applied.GetKind(),
		Namespace:  applied.GetNamespace(),
		Name:       applied.GetName(),
		YAML:       y,
	}, nil
}
