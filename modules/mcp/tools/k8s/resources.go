package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
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

func (c *Controller) dynamicClients(resolved *resolvedCluster) (dynamic.Interface, meta.RESTMapper, error) {
	dyn, err := dynamic.NewForConfig(resolved.REST)
	if err != nil {
		return nil, nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(resolved.REST)
	if err != nil {
		return nil, nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))
	return dyn, mapper, nil
}

func parseAPIVersionKind(apiVersion, kind string) (schema.GroupVersionKind, error) {
	apiVersion = strings.TrimSpace(apiVersion)
	kind = strings.TrimSpace(kind)
	if apiVersion == "" || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion and kind are required")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid apiVersion %q: %w", apiVersion, err)
	}
	return gv.WithKind(kind), nil
}

func (c *Controller) resolveMapping(resolved *resolvedCluster, apiVersion, kind string) (dynamic.Interface, *meta.RESTMapping, error) {
	gvk, err := parseAPIVersionKind(apiVersion, kind)
	if err != nil {
		return nil, nil, err
	}
	dyn, mapper, err := c.dynamicClients(resolved)
	if err != nil {
		return nil, nil, err
	}
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %s %s: %w", apiVersion, kind, err)
	}
	return dyn, mapping, nil
}

type ResourcesListInput struct {
	ClusterRef
	APIVersion    string `json:"apiVersion" jsonschema:"required apiVersion e.g. v1, apps/v1"`
	Kind          string `json:"kind" jsonschema:"required kind e.g. Pod, Deployment"`
	Namespace     string `json:"namespace,omitempty" jsonschema:"optional namespace; omit for all namespaces (namespaced resources)"`
	LabelSelector string `json:"labelSelector,omitempty"`
	FieldSelector string `json:"fieldSelector,omitempty"`
}

type ResourceRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

type ResourcesListOutput struct {
	KubeconfigID string        `json:"kubeconfig_id"`
	Context      string        `json:"context"`
	Count        int           `json:"count"`
	Items        []ResourceRef `json:"items"`
}

func (c *Controller) ResourcesListTool(ctx context.Context, _ *mcp.CallToolRequest, input ResourcesListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	dyn, mapping, err := c.resolveMapping(resolved, input.APIVersion, input.Kind)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	opts := metav1.ListOptions{
		LabelSelector: strings.TrimSpace(input.LabelSelector),
		FieldSelector: strings.TrimSpace(input.FieldSelector),
	}
	var list *unstructured.UnstructuredList
	ns := strings.TrimSpace(input.Namespace)
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		list, err = dyn.Resource(mapping.Resource).Namespace(ns).List(runCtx, opts)
	} else {
		list, err = dyn.Resource(mapping.Resource).List(runCtx, opts)
	}
	if err != nil {
		return nil, nil, err
	}
	items := make([]ResourceRef, 0, len(list.Items))
	for _, obj := range list.Items {
		items = append(items, ResourceRef{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		})
	}
	return nil, ResourcesListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Count:        len(items),
		Items:        items,
	}, nil
}

type ResourcesGetInput struct {
	ClusterRef
	APIVersion string `json:"apiVersion" jsonschema:"required"`
	Kind       string `json:"kind" jsonschema:"required"`
	Name       string `json:"name" jsonschema:"required"`
	Namespace  string `json:"namespace,omitempty"`
}

type ResourcesGetOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	YAML         string `json:"yaml"`
}

func (c *Controller) ResourcesGetTool(ctx context.Context, _ *mcp.CallToolRequest, input ResourcesGetInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	dyn, mapping, err := c.resolveMapping(resolved, input.APIVersion, input.Kind)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	var obj *unstructured.Unstructured
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := defaultNS(resolved, input.Namespace)
		obj, err = dyn.Resource(mapping.Resource).Namespace(ns).Get(runCtx, name, metav1.GetOptions{})
	} else {
		obj, err = dyn.Resource(mapping.Resource).Get(runCtx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, nil, err
	}
	y, err := yaml.Marshal(obj.Object)
	if err != nil {
		return nil, nil, err
	}
	return nil, ResourcesGetOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		YAML:         string(y),
	}, nil
}

type ResourcesCreateOrUpdateInput struct {
	ClusterRef
	Resource string `json:"resource" jsonschema:"required complete YAML or JSON of the Kubernetes resource"`
}

type ResourcesCreateOrUpdateOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	APIVersion   string `json:"apiVersion"`
	Kind         string `json:"kind"`
	Namespace    string `json:"namespace,omitempty"`
	Name         string `json:"name"`
	YAML         string `json:"yaml"`
}

func (c *Controller) ResourcesCreateOrUpdateTool(ctx context.Context, _ *mcp.CallToolRequest, input ResourcesCreateOrUpdateInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	content := strings.TrimSpace(input.Resource)
	if content == "" {
		return nil, nil, fmt.Errorf("resource YAML/JSON is required")
	}
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(content), &obj.Object); err != nil {
		return nil, nil, fmt.Errorf("invalid yaml/json: %w", err)
	}
	if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
		return nil, nil, fmt.Errorf("resource must include apiVersion and kind")
	}
	if strings.TrimSpace(obj.GetName()) == "" {
		return nil, nil, fmt.Errorf("resource must include metadata.name")
	}
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "selfLink")

	dyn, mapping, err := c.resolveMapping(resolved, obj.GetAPIVersion(), obj.GetKind())
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	var resource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := strings.TrimSpace(obj.GetNamespace())
		if ns == "" {
			ns = defaultNS(resolved, "")
			obj.SetNamespace(ns)
		}
		resource = dyn.Resource(mapping.Resource).Namespace(ns)
	} else {
		resource = dyn.Resource(mapping.Resource)
	}
	applied, err := resource.Apply(runCtx, obj.GetName(), &obj, metav1.ApplyOptions{
		FieldManager: "containerws-mcp",
		Force:        true,
	})
	if err != nil {
		return nil, nil, err
	}
	y, err := yaml.Marshal(applied.Object)
	if err != nil {
		y = []byte(content)
	}
	return nil, ResourcesCreateOrUpdateOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		APIVersion:   applied.GetAPIVersion(),
		Kind:         applied.GetKind(),
		Namespace:    applied.GetNamespace(),
		Name:         applied.GetName(),
		YAML:         string(y),
	}, nil
}

type ResourcesDeleteInput struct {
	ClusterRef
	APIVersion         string `json:"apiVersion" jsonschema:"required"`
	Kind               string `json:"kind" jsonschema:"required"`
	Name               string `json:"name" jsonschema:"required"`
	Namespace          string `json:"namespace,omitempty"`
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds,omitempty"`
}

type ResourcesDeleteOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Deleted      string `json:"deleted"`
}

func (c *Controller) ResourcesDeleteTool(ctx context.Context, _ *mcp.CallToolRequest, input ResourcesDeleteInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	dyn, mapping, err := c.resolveMapping(resolved, input.APIVersion, input.Kind)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	opts := metav1.DeleteOptions{}
	if input.GracePeriodSeconds != nil {
		opts.GracePeriodSeconds = input.GracePeriodSeconds
	}
	var deleted string
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := defaultNS(resolved, input.Namespace)
		err = dyn.Resource(mapping.Resource).Namespace(ns).Delete(runCtx, name, opts)
		deleted = fmt.Sprintf("%s/%s/%s", input.Kind, ns, name)
	} else {
		err = dyn.Resource(mapping.Resource).Delete(runCtx, name, opts)
		deleted = fmt.Sprintf("%s/%s", input.Kind, name)
	}
	if err != nil {
		return nil, nil, err
	}
	return nil, ResourcesDeleteOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Deleted:      deleted,
	}, nil
}

type ResourcesScaleInput struct {
	ClusterRef
	APIVersion string `json:"apiVersion" jsonschema:"required e.g. apps/v1"`
	Kind       string `json:"kind" jsonschema:"required e.g. Deployment, StatefulSet"`
	Name       string `json:"name" jsonschema:"required"`
	Namespace  string `json:"namespace,omitempty"`
	Scale      *int32 `json:"scale,omitempty" jsonschema:"optional new replica count; omit to only read current scale"`
}

type ResourcesScaleOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Replicas     int32  `json:"replicas"`
	Updated      bool   `json:"updated"`
}

func (c *Controller) ResourcesScaleTool(ctx context.Context, _ *mcp.CallToolRequest, input ResourcesScaleInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	name := strings.TrimSpace(input.Name)
	kind := strings.TrimSpace(input.Kind)
	if name == "" || kind == "" {
		return nil, nil, fmt.Errorf("kind and name are required")
	}
	ns := defaultNS(resolved, input.Namespace)
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	// Prefer typed scale subresource for common kinds.
	var scale *autoscalingv1.Scale
	updated := false
	switch strings.ToLower(kind) {
	case "deployment":
		if input.Scale != nil {
			scale, err = resolved.Client.AppsV1().Deployments(ns).UpdateScale(runCtx, name, &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       autoscalingv1.ScaleSpec{Replicas: *input.Scale},
			}, metav1.UpdateOptions{})
			updated = true
		} else {
			scale, err = resolved.Client.AppsV1().Deployments(ns).GetScale(runCtx, name, metav1.GetOptions{})
		}
	case "statefulset":
		if input.Scale != nil {
			scale, err = resolved.Client.AppsV1().StatefulSets(ns).UpdateScale(runCtx, name, &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       autoscalingv1.ScaleSpec{Replicas: *input.Scale},
			}, metav1.UpdateOptions{})
			updated = true
		} else {
			scale, err = resolved.Client.AppsV1().StatefulSets(ns).GetScale(runCtx, name, metav1.GetOptions{})
		}
	case "replicaset":
		if input.Scale != nil {
			scale, err = resolved.Client.AppsV1().ReplicaSets(ns).UpdateScale(runCtx, name, &autoscalingv1.Scale{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec:       autoscalingv1.ScaleSpec{Replicas: *input.Scale},
			}, metav1.UpdateOptions{})
			updated = true
		} else {
			scale, err = resolved.Client.AppsV1().ReplicaSets(ns).GetScale(runCtx, name, metav1.GetOptions{})
		}
	default:
		return nil, nil, fmt.Errorf("resources_scale supports Deployment, StatefulSet, ReplicaSet (got %q); use resources_create_or_update for others", kind)
	}
	if err != nil {
		return nil, nil, err
	}
	return nil, ResourcesScaleOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Namespace:    ns,
		Name:         name,
		Kind:         kind,
		Replicas:     scale.Spec.Replicas,
		Updated:      updated,
	}, nil
}
