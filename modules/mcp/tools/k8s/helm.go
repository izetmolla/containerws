package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (c *Controller) helmEnv(resolved *resolvedCluster) []string {
	env := os.Environ()
	env = append(env, "KUBECONFIG="+resolved.Path)
	if resolved.Context != "" {
		env = append(env, "HELM_KUBECONTEXT="+resolved.Context)
	}
	return env
}

func (c *Controller) runHelm(ctx context.Context, resolved *resolvedCluster, args ...string) (string, string, error) {
	if _, err := exec.LookPath("helm"); err != nil {
		return "", "", fmt.Errorf("helm CLI not found on PATH — install Helm to use helm_* tools")
	}
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "helm", args...)
	cmd.Env = c.helmEnv(resolved)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type HelmInstallInput struct {
	ClusterRef
	Chart     string         `json:"chart" jsonschema:"required chart reference e.g. bitnami/nginx or oci://…"`
	Name      string         `json:"name,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
	Values    map[string]any `json:"values,omitempty"`
}

type HelmInstallOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr,omitempty"`
}

func (c *Controller) HelmInstallTool(ctx context.Context, _ *mcp.CallToolRequest, input HelmInstallInput) (*mcp.CallToolResult, any, error) {
	chart := strings.TrimSpace(input.Chart)
	if chart == "" {
		return nil, nil, fmt.Errorf("chart is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	args := []string{"install"}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		args = append(args, "--generate-name", chart)
	} else {
		args = append(args, name, chart)
	}
	ns := strings.TrimSpace(input.Namespace)
	if ns != "" {
		args = append(args, "--namespace", ns, "--create-namespace")
	}
	if len(input.Values) > 0 {
		b, err := json.Marshal(input.Values)
		if err != nil {
			return nil, nil, err
		}
		f, err := os.CreateTemp("", "helm-values-*.json")
		if err != nil {
			return nil, nil, err
		}
		path := f.Name()
		_, _ = f.Write(b)
		_ = f.Close()
		defer os.Remove(path)
		args = append(args, "-f", path)
	}
	stdout, stderr, err := c.runHelm(ctx, resolved, args...)
	out := HelmInstallOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Stdout:       stdout,
		Stderr:       stderr,
	}
	if err != nil {
		return nil, out, fmt.Errorf("helm install failed: %w: %s", err, stderr)
	}
	return nil, out, nil
}

type HelmListInput struct {
	ClusterRef
	Namespace     string `json:"namespace,omitempty"`
	AllNamespaces bool   `json:"all_namespaces,omitempty"`
}

type HelmListOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr,omitempty"`
}

func (c *Controller) HelmListTool(ctx context.Context, _ *mcp.CallToolRequest, input HelmListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	args := []string{"list", "-o", "json"}
	if input.AllNamespaces {
		args = append(args, "-A")
	} else if ns := strings.TrimSpace(input.Namespace); ns != "" {
		args = append(args, "--namespace", ns)
	}
	stdout, stderr, err := c.runHelm(ctx, resolved, args...)
	out := HelmListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Stdout:       stdout,
		Stderr:       stderr,
	}
	if err != nil {
		return nil, out, fmt.Errorf("helm list failed: %w: %s", err, stderr)
	}
	return nil, out, nil
}

type HelmUninstallInput struct {
	ClusterRef
	Name      string `json:"name" jsonschema:"required release name"`
	Namespace string `json:"namespace,omitempty"`
}

type HelmUninstallOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr,omitempty"`
}

func (c *Controller) HelmUninstallTool(ctx context.Context, _ *mcp.CallToolRequest, input HelmUninstallInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	args := []string{"uninstall", name}
	if ns := strings.TrimSpace(input.Namespace); ns != "" {
		args = append(args, "--namespace", ns)
	}
	stdout, stderr, err := c.runHelm(ctx, resolved, args...)
	out := HelmUninstallOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Stdout:       stdout,
		Stderr:       stderr,
	}
	if err != nil {
		return nil, out, fmt.Errorf("helm uninstall failed: %w: %s", err, stderr)
	}
	return nil, out, nil
}
