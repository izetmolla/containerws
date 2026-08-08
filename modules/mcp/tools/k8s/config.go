package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/kubeclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ConfigurationContextsListInput struct{}

type ContextTarget struct {
	KubeconfigID  string `json:"kubeconfig_id"`
	SecretName    string `json:"secret_name"`
	Path          string `json:"path"`
	Context       string `json:"context"`
	Cluster       string `json:"cluster"`
	User          string `json:"user"`
	Namespace     string `json:"namespace,omitempty"`
	Server        string `json:"server,omitempty"`
	Active        bool   `json:"active"`
	ActiveContext bool   `json:"active_context"`
	FileExists    bool   `json:"file_exists"`
	AskUserHint   string `json:"ask_user_hint,omitempty"`
}

type ConfigurationContextsListOutput struct {
	Count              int             `json:"count"`
	ActiveKubeconfigID string          `json:"active_kubeconfig_id,omitempty"`
	ActiveContext      string          `json:"active_context,omitempty"`
	MultipleSecrets    bool            `json:"multiple_secrets"`
	MultipleClusters   bool            `json:"multiple_clusters"`
	Hint               string          `json:"hint,omitempty"`
	Items              []ContextTarget `json:"items"`
}

func (c *Controller) ConfigurationContextsListTool(ctx context.Context, _ *mcp.CallToolRequest, _ ConfigurationContextsListInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	return nil, c.buildTargets(), nil
}

func (c *Controller) TargetsListTool(ctx context.Context, _ *mcp.CallToolRequest, _ ConfigurationContextsListInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	return nil, c.buildTargets(), nil
}

func (c *Controller) buildTargets() ConfigurationContextsListOutput {
	files, activeID, err := c.listKubeFiles()
	out := ConfigurationContextsListOutput{Items: []ContextTarget{}}
	if err != nil {
		out.Hint = err.Error()
		return out
	}

	activeCtx := ""
	if c.app != nil && c.app.DB() != nil {
		activeCtx, _, _ = models.GetOption(c.app.DB(), models.OptionKubeconfigContext)
		activeCtx = strings.TrimSpace(activeCtx)
	}

	secretNames := map[string]struct{}{}
	for _, f := range files {
		secretNames[f.ID] = struct{}{}
		if !f.Exists {
			out.Items = append(out.Items, ContextTarget{
				KubeconfigID: f.ID,
				SecretName:   f.Name,
				Path:         f.Path,
				Active:       f.Active || f.ID == activeID,
				FileExists:   false,
				AskUserHint:  "kubeconfig file missing on disk",
			})
			continue
		}
		contexts, current, err := kubeclient.ListContexts(f.Path, "")
		if err != nil {
			out.Items = append(out.Items, ContextTarget{
				KubeconfigID: f.ID,
				SecretName:   f.Name,
				Path:         f.Path,
				Active:       f.Active || f.ID == activeID,
				FileExists:   true,
				AskUserHint:  err.Error(),
			})
			continue
		}
		raw, _ := kubeclient.LoadRaw(f.Path)
		for _, cx := range contexts {
			server := ""
			if raw != nil {
				if cl, ok := raw.Clusters[cx.Cluster]; ok && cl != nil {
					server = cl.Server
				}
			}
			isActiveFile := f.Active || f.ID == activeID
			isActiveCtx := false
			if isActiveFile {
				want := activeCtx
				if want == "" {
					want = current
				}
				isActiveCtx = cx.Name == want
				if isActiveCtx {
					out.ActiveKubeconfigID = f.ID
					out.ActiveContext = cx.Name
				}
			}
			out.Items = append(out.Items, ContextTarget{
				KubeconfigID:  f.ID,
				SecretName:    f.Name,
				Path:          f.Path,
				Context:       cx.Name,
				Cluster:       cx.Cluster,
				User:          cx.AuthInfo,
				Namespace:     cx.Namespace,
				Server:        server,
				Active:        isActiveFile,
				ActiveContext: isActiveCtx,
				FileExists:    true,
			})
		}
	}

	out.Count = len(out.Items)
	out.MultipleSecrets = len(secretNames) > 1
	out.MultipleClusters = out.Count > 1
	switch {
	case out.Count == 0:
		out.Hint = "No kubeconfig secrets configured. Add one under Kubernetes → Settings (paste kubeconfig YAML)."
	case out.MultipleSecrets || out.MultipleClusters:
		out.Hint = "Multiple kubeconfig secrets and/or clusters are available. Ask the user which secret (kubeconfig_id) and which cluster (context) to use before mutating resources."
	}
	return out
}

type ConfigurationViewInput struct {
	ClusterRef
	// Minified defaults to true when omitted. Set false to return the full kubeconfig.
	Minified *bool `json:"minified,omitempty" jsonschema:"if true (default), keep only the selected context and related cluster/user entries"`
}

type ConfigurationViewOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	SecretName   string `json:"secret_name"`
	Context      string `json:"context"`
	Cluster      string `json:"cluster,omitempty"`
	Minified     bool   `json:"minified"`
	YAML         string `json:"yaml"`
}

func (c *Controller) ConfigurationViewTool(ctx context.Context, _ *mcp.CallToolRequest, input ConfigurationViewInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(resolved.Path)
	if err != nil {
		return nil, nil, err
	}
	minified := true
	if input.Minified != nil {
		minified = *input.Minified
	}
	if minified {
		cfg, err := clientcmd.Load(data)
		if err != nil {
			return nil, nil, err
		}
		ctxName := resolved.Context
		if ctxName == "" {
			ctxName = cfg.CurrentContext
		}
		cx, ok := cfg.Contexts[ctxName]
		if !ok || cx == nil {
			return nil, nil, fmt.Errorf("context %q not found", ctxName)
		}
		outCfg := clientcmdapi.NewConfig()
		outCfg.CurrentContext = ctxName
		outCfg.Contexts[ctxName] = cx
		if cl, ok := cfg.Clusters[cx.Cluster]; ok {
			outCfg.Clusters[cx.Cluster] = cl
		}
		if u, ok := cfg.AuthInfos[cx.AuthInfo]; ok {
			outCfg.AuthInfos[cx.AuthInfo] = u
		}
		data, err = clientcmd.Write(*outCfg)
		if err != nil {
			return nil, nil, err
		}
	}
	return nil, ConfigurationViewOutput{
		KubeconfigID: resolved.KubeconfigID,
		SecretName:   resolved.Name,
		Context:      resolved.Context,
		Cluster:      resolved.Cluster,
		Minified:     minified,
		YAML:         string(data),
	}, nil
}

type ConfigurationSetActiveInput struct {
	KubeconfigID string `json:"kubeconfig_id" jsonschema:"required kubeconfig secret id from configuration_contexts_list"`
	Context      string `json:"context,omitempty" jsonschema:"optional context (cluster) name to activate inside that secret"`
}

type ConfigurationSetActiveOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	SecretName   string `json:"secret_name"`
	Context      string `json:"context"`
	Cluster      string `json:"cluster,omitempty"`
	Message      string `json:"message"`
}

func (c *Controller) ConfigurationSetActiveTool(ctx context.Context, _ *mcp.CallToolRequest, input ConfigurationSetActiveInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	id := strings.TrimSpace(input.KubeconfigID)
	if id == "" {
		return nil, nil, fmt.Errorf("kubeconfig_id is required — ask the user which secret to use, then pick from configuration_contexts_list")
	}
	files, _, err := c.listKubeFiles()
	if err != nil {
		return nil, nil, err
	}
	var file kubeFile
	found := false
	for _, f := range files {
		if f.ID == id || strings.EqualFold(f.Name, id) {
			file = f
			found = true
			break
		}
	}
	if !found {
		return nil, nil, fmt.Errorf("kubeconfig secret %q not found", id)
	}
	if !kubeclient.FileExists(file.Path) {
		return nil, nil, fmt.Errorf("kubeconfig file missing at %s", file.Path)
	}
	ctxName := strings.TrimSpace(input.Context)
	contexts, current, err := kubeclient.ListContexts(file.Path, ctxName)
	if err != nil {
		return nil, nil, err
	}
	if ctxName == "" {
		ctxName = current
	} else {
		ok := false
		for _, cx := range contexts {
			if cx.Name == ctxName {
				ok = true
				break
			}
		}
		if !ok {
			return nil, nil, fmt.Errorf("context %q not in secret %q; available: %s", ctxName, file.Name, contextNames(contexts))
		}
	}
	db := c.app.DB()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	if err := models.SetOption(db, models.OptionKubeconfigActiveID, file.ID); err != nil {
		return nil, nil, err
	}
	if err := models.SetOption(db, models.OptionKubeconfigPath, file.Path); err != nil {
		return nil, nil, err
	}
	if err := models.SetOption(db, models.OptionKubeconfigContext, ctxName); err != nil {
		return nil, nil, err
	}
	kubeclient.Reset()

	cluster := ""
	for _, cx := range contexts {
		if cx.Name == ctxName {
			cluster = cx.Cluster
			break
		}
	}
	return nil, ConfigurationSetActiveOutput{
		KubeconfigID: file.ID,
		SecretName:   file.Name,
		Context:      ctxName,
		Cluster:      cluster,
		Message:      fmt.Sprintf("Active kubeconfig secret is now %q (context/cluster %q)", file.Name, ctxName),
	}, nil
}
