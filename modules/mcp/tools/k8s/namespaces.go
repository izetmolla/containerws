package k8s

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type NamespacesListInput struct {
	ClusterRef
	FieldSelector string `json:"fieldSelector,omitempty" jsonschema:"optional Kubernetes field selector"`
}

type NamespaceItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"created_at"`
}

type NamespacesListOutput struct {
	KubeconfigID string          `json:"kubeconfig_id"`
	Context      string          `json:"context"`
	Cluster      string          `json:"cluster,omitempty"`
	Count        int             `json:"count"`
	Items        []NamespaceItem `json:"items"`
}

func (c *Controller) NamespacesListTool(ctx context.Context, _ *mcp.CallToolRequest, input NamespacesListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	list, err := resolved.Client.CoreV1().Namespaces().List(runCtx, metav1.ListOptions{
		FieldSelector: strings.TrimSpace(input.FieldSelector),
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]NamespaceItem, 0, len(list.Items))
	for _, ns := range list.Items {
		items = append(items, NamespaceItem{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    ns.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return nil, NamespacesListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Cluster:      resolved.Cluster,
		Count:        len(items),
		Items:        items,
	}, nil
}

type ProjectsListInput struct {
	ClusterRef
}

type ProjectsListOutput struct {
	KubeconfigID string          `json:"kubeconfig_id"`
	Context      string          `json:"context"`
	Source       string          `json:"source"`
	Count        int             `json:"count"`
	Items        []NamespaceItem `json:"items"`
}

func (c *Controller) ProjectsListTool(ctx context.Context, _ *mcp.CallToolRequest, input ProjectsListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	// Try OpenShift Project API first.
	if dyn, _, err := c.dynamicClients(resolved); err == nil {
		gvr := schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projects"}
		if ul, listErr := dyn.Resource(gvr).List(runCtx, metav1.ListOptions{}); listErr == nil {
			items := make([]NamespaceItem, 0, len(ul.Items))
			for _, obj := range ul.Items {
				phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
				items = append(items, NamespaceItem{
					Name:   obj.GetName(),
					Status: phase,
					Age:    obj.GetCreationTimestamp().UTC().Format(time.RFC3339),
				})
			}
			return nil, ProjectsListOutput{
				KubeconfigID: resolved.KubeconfigID,
				Context:      resolved.Context,
				Source:       "project.openshift.io/v1",
				Count:        len(items),
				Items:        items,
			}, nil
		}
	}

	list, err := resolved.Client.CoreV1().Namespaces().List(runCtx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	items := make([]NamespaceItem, 0, len(list.Items))
	for _, ns := range list.Items {
		items = append(items, NamespaceItem{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    ns.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return nil, ProjectsListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Source:       "v1/Namespace (OpenShift Project API unavailable)",
		Count:        len(items),
		Items:        items,
	}, nil
}

type EventsListInput struct {
	ClusterRef
	Namespace     string `json:"namespace,omitempty" jsonschema:"optional namespace; omit for all namespaces"`
	FieldSelector string `json:"fieldSelector,omitempty" jsonschema:"optional field selector e.g. type=Warning"`
}

type EventItem struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	Object    string `json:"object,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

type EventsListOutput struct {
	KubeconfigID string      `json:"kubeconfig_id"`
	Context      string      `json:"context"`
	Count        int         `json:"count"`
	Items        []EventItem `json:"items"`
}

func (c *Controller) EventsListTool(ctx context.Context, _ *mcp.CallToolRequest, input EventsListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	ns := strings.TrimSpace(input.Namespace)
	list, err := resolved.Client.CoreV1().Events(ns).List(runCtx, metav1.ListOptions{
		FieldSelector: strings.TrimSpace(input.FieldSelector),
		Limit:         500,
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]EventItem, 0, len(list.Items))
	for _, e := range list.Items {
		obj := ""
		if e.InvolvedObject.Kind != "" {
			obj = e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name
		}
		last := ""
		if !e.LastTimestamp.IsZero() {
			last = e.LastTimestamp.UTC().Format(time.RFC3339)
		} else if !e.EventTime.IsZero() {
			last = e.EventTime.UTC().Format(time.RFC3339)
		}
		items = append(items, EventItem{
			Namespace: e.Namespace,
			Name:      e.Name,
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Count:     e.Count,
			Object:    obj,
			LastSeen:  last,
		})
	}
	return nil, EventsListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Count:        len(items),
		Items:        items,
	}, nil
}
