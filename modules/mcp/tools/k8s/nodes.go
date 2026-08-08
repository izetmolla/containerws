package k8s

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type NodesLogInput struct {
	ClusterRef
	Name      string `json:"name" jsonschema:"required node name"`
	Query     string `json:"query" jsonschema:"required e.g. kubelet or /var/log/kubelet.log"`
	TailLines *int   `json:"tailLines,omitempty"`
}

type NodesLogOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Node         string `json:"node"`
	Logs         string `json:"logs"`
}

func (c *Controller) NodesLogTool(ctx context.Context, _ *mcp.CallToolRequest, input NodesLogInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	query := strings.TrimSpace(input.Query)
	if name == "" || query == "" {
		return nil, nil, fmt.Errorf("name and query are required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	req := resolved.Client.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(name).
		SubResource("proxy").
		Suffix("logs", query)
	if input.TailLines != nil && *input.TailLines > 0 {
		req = req.Param("tailLines", strconv.Itoa(*input.TailLines))
	}
	raw, err := req.Do(runCtx).Raw()
	if err != nil {
		return nil, nil, err
	}
	return nil, NodesLogOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Node:         name,
		Logs:         string(raw),
	}, nil
}

type NodesStatsSummaryInput struct {
	ClusterRef
	Name string `json:"name" jsonschema:"required node name"`
}

type NodesStatsSummaryOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Node         string `json:"node"`
	SummaryJSON  string `json:"summary_json"`
}

func (c *Controller) NodesStatsSummaryTool(ctx context.Context, _ *mcp.CallToolRequest, input NodesStatsSummaryInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	raw, err := resolved.Client.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(name).
		SubResource("proxy").
		Suffix("stats", "summary").
		Do(runCtx).Raw()
	if err != nil {
		return nil, nil, err
	}
	return nil, NodesStatsSummaryOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Node:         name,
		SummaryJSON:  string(raw),
	}, nil
}

type NodesTopInput struct {
	ClusterRef
	Name          string `json:"name,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
}

type NodesTopOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	MetricsJSON  string `json:"metrics_json"`
}

func (c *Controller) NodesTopTool(ctx context.Context, _ *mcp.CallToolRequest, input NodesTopInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	path := "/apis/metrics.k8s.io/v1beta1/nodes"
	if name := strings.TrimSpace(input.Name); name != "" {
		path = path + "/" + name
	}
	req := resolved.Client.CoreV1().RESTClient().Get().AbsPath(path)
	if sel := strings.TrimSpace(input.LabelSelector); sel != "" {
		req = req.Param("labelSelector", sel)
	}
	raw, err := req.Do(runCtx).Raw()
	if err != nil {
		return nil, nil, fmt.Errorf("metrics.k8s.io unavailable (is Metrics Server installed?): %w", err)
	}
	return nil, NodesTopOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		MetricsJSON:  string(raw),
	}, nil
}
