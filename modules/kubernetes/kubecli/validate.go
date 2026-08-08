package kubecli

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ManifestRef is a durable identity used to look up live cluster objects.
type ManifestRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	ClusterScoped bool `json:"cluster_scoped,omitempty"`
}

// ManifestAnalysis summarizes a multi-doc YAML stream.
type ManifestAnalysis struct {
	Namespace string        `json:"namespace"`
	Resources []ManifestRef `json:"resources"`
}

// well-known cluster-scoped kinds (no metadata.namespace).
var clusterScopedKinds = map[string]struct{}{
	"Namespace": {}, "Node": {}, "PersistentVolume": {},
	"ClusterRole": {}, "ClusterRoleBinding": {},
	"StorageClass": {}, "CSIDriver": {}, "CSINode": {},
	"PriorityClass": {}, "RuntimeClass": {}, "IngressClass": {},
	"CustomResourceDefinition": {}, "MutatingWebhookConfiguration": {},
	"ValidatingWebhookConfiguration": {}, "APIService": {},
	"ClusterIssuer": {}, "VolumeAttachment": {},
}

func IsClusterScopedKind(kind string) bool {
	_, ok := clusterScopedKinds[strings.TrimSpace(kind)]
	return ok
}

// AnalyzeManifests parses YAML docs, enforces a single shared namespace for
// namespaced resources, and returns resource refs for live cluster lookups.
func AnalyzeManifests(content string) (*ManifestAnalysis, error) {
	docs := SplitYAMLDocuments(content)
	if len(docs) == 0 {
		return nil, fmt.Errorf("yaml content is empty")
	}

	var sharedNS string
	nsSet := false
	refs := make([]ManifestRef, 0, len(docs))

	for i, doc := range docs {
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
			return nil, fmt.Errorf("document %d: invalid yaml: %w", i+1, err)
		}
		if obj.Object == nil {
			return nil, fmt.Errorf("document %d: empty document", i+1)
		}
		kind := obj.GetKind()
		apiVersion := obj.GetAPIVersion()
		name := strings.TrimSpace(obj.GetName())
		if kind == "" || apiVersion == "" {
			return nil, fmt.Errorf("document %d: apiVersion and kind are required", i+1)
		}
		if name == "" {
			return nil, fmt.Errorf("document %d (%s): metadata.name is required", i+1, kind)
		}

		clusterScoped := IsClusterScopedKind(kind)
		ns := strings.TrimSpace(obj.GetNamespace())

		if clusterScoped {
			if ns != "" {
				return nil, fmt.Errorf("document %d (%s/%s): cluster-scoped resources must not set metadata.namespace", i+1, kind, name)
			}
			refs = append(refs, ManifestRef{
				APIVersion:    apiVersion,
				Kind:          kind,
				Name:          name,
				ClusterScoped: true,
			})
			continue
		}

		if ns != "" {
			if !nsSet {
				sharedNS = ns
				nsSet = true
			} else if ns != sharedNS {
				return nil, fmt.Errorf(
					"all namespaced resources must use the same namespace; found %q and %q (document %d: %s/%s)",
					sharedNS, ns, i+1, kind, name,
				)
			}
		}

		refs = append(refs, ManifestRef{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			Namespace:  ns,
		})
	}

	return &ManifestAnalysis{Namespace: sharedNS, Resources: refs}, nil
}

// RewriteManifestNamespace sets metadata.namespace on every namespaced document.
// Cluster-scoped kinds are left unchanged. Returns rewritten YAML.
func RewriteManifestNamespace(content, namespace string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "", fmt.Errorf("namespace is required to rewrite manifests")
	}
	docs := SplitYAMLDocuments(content)
	if len(docs) == 0 {
		return "", fmt.Errorf("yaml content is empty")
	}

	out := make([]string, 0, len(docs))
	for i, doc := range docs {
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
			return "", fmt.Errorf("document %d: invalid yaml: %w", i+1, err)
		}
		if obj.Object == nil {
			continue
		}
		kind := obj.GetKind()
		if !IsClusterScopedKind(kind) {
			obj.SetNamespace(namespace)
		}
		b, err := yaml.Marshal(obj.Object)
		if err != nil {
			return "", fmt.Errorf("document %d: marshal: %w", i+1, err)
		}
		out = append(out, strings.TrimSpace(string(b)))
	}
	return strings.Join(out, "\n---\n"), nil
}

// EnsureUniformNamespace validates YAML and optionally rewrites to targetNS.
// If targetNS is empty, keeps the shared namespace discovered in the YAML (must be set on at least one doc, or provided).
func EnsureUniformNamespace(content, targetNS string) (rewritten string, ns string, analysis *ManifestAnalysis, err error) {
	analysis, err = AnalyzeManifests(content)
	if err != nil {
		return "", "", nil, err
	}
	targetNS = strings.TrimSpace(targetNS)
	ns = strings.TrimSpace(analysis.Namespace)
	if targetNS != "" {
		ns = targetNS
	}
	if ns == "" {
		// All namespaced docs omitted namespace — require an explicit target.
		hasNamespaced := false
		for _, r := range analysis.Resources {
			if !r.ClusterScoped {
				hasNamespaced = true
				break
			}
		}
		if hasNamespaced {
			return "", "", analysis, fmt.Errorf("choose a namespace, or set metadata.namespace on the manifests")
		}
		return content, "", analysis, nil
	}

	rewritten, err = RewriteManifestNamespace(content, ns)
	if err != nil {
		return "", "", analysis, err
	}
	// Re-analyze rewritten content for accurate refs/namespace.
	analysis, err = AnalyzeManifests(rewritten)
	if err != nil {
		return "", "", nil, err
	}
	analysis.Namespace = ns
	for i := range analysis.Resources {
		if !analysis.Resources[i].ClusterScoped {
			analysis.Resources[i].Namespace = ns
		}
	}
	return rewritten, ns, analysis, nil
}
