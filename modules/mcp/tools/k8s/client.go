package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	"github.com/izetmolla/containerws/packages/kubeclient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const defaultToolTimeout = 45 * time.Second

// ClusterRef selects which kubeconfig secret + context (cluster) a tool uses.
// Empty values fall back to the workspace active kubeconfig.
type ClusterRef struct {
	KubeconfigID string `json:"kubeconfig_id,omitempty" jsonschema:"optional kubeconfig secret id from configuration_contexts_list / targets_list. If omitted, uses the active kubeconfig."`
	Context      string `json:"context,omitempty" jsonschema:"optional Kubernetes context (cluster) name inside the kubeconfig. If omitted, uses the active or file current-context."`
}

type kubeFile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Managed bool   `json:"managed"`
	Active  bool   `json:"active"`
	Exists  bool   `json:"exists"`
}

type resolvedCluster struct {
	KubeconfigID string
	Name         string
	Path         string
	Context      string
	Cluster      string
	User         string
	Namespace    string
	Client       kubernetes.Interface
	REST         *rest.Config
}

func (c *Controller) listKubeFiles() ([]kubeFile, string, error) {
	if c.app == nil || c.app.DB() == nil {
		return nil, "", fmt.Errorf("database unavailable")
	}
	db := c.app.DB()
	raw, ok, err := models.GetOption(db, models.OptionKubeconfigFiles)
	if err != nil {
		return nil, "", err
	}
	activeID, _, err := models.GetOption(db, models.OptionKubeconfigActiveID)
	if err != nil {
		return nil, "", err
	}
	activeID = strings.TrimSpace(activeID)

	var list []kubeFile
	if ok && strings.TrimSpace(raw) != "" {
		type entry struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Path    string `json:"path"`
			Managed bool   `json:"managed"`
		}
		var entries []entry
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return nil, "", fmt.Errorf("corrupt kubeconfig registry: %w", err)
		}
		for _, e := range entries {
			id := strings.TrimSpace(e.ID)
			path := strings.TrimSpace(e.Path)
			if id == "" || path == "" {
				continue
			}
			name := strings.TrimSpace(e.Name)
			if name == "" {
				name = id
			}
			list = append(list, kubeFile{
				ID:      id,
				Name:    name,
				Path:    path,
				Managed: e.Managed || kubeclient.IsManagedPath(path),
				Active:  id == activeID,
				Exists:  kubeclient.FileExists(path),
			})
		}
	}

	// Seed from active path when registry empty (same idea as configapi).
	if len(list) == 0 {
		s, err := kubecli.LoadSettings(c.app)
		if err != nil {
			return nil, activeID, err
		}
		if s.Exists {
			list = append(list, kubeFile{
				ID:      "default",
				Name:    "Default",
				Path:    s.Path,
				Managed: kubeclient.IsManagedPath(s.Path),
				Active:  true,
				Exists:  true,
			})
			if activeID == "" {
				activeID = "default"
			}
		}
	}
	return list, activeID, nil
}

func (c *Controller) resolve(ref ClusterRef) (*resolvedCluster, error) {
	files, activeID, err := c.listKubeFiles()
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(ref.KubeconfigID)
	ctxName := strings.TrimSpace(ref.Context)

	var file kubeFile
	found := false
	if id != "" {
		for _, f := range files {
			if f.ID == id || strings.EqualFold(f.Name, id) {
				file = f
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("kubeconfig secret %q not found — call configuration_contexts_list or targets_list to see available secrets and clusters", id)
		}
	} else {
		for _, f := range files {
			if f.Active || f.ID == activeID {
				file = f
				found = true
				break
			}
		}
		if !found && len(files) > 0 {
			file = files[0]
			found = true
		}
		if !found {
			s, err := kubecli.LoadSettings(c.app)
			if err != nil {
				return nil, err
			}
			if !s.Exists {
				return nil, fmt.Errorf("no kubeconfig secret configured — add one in Kubernetes → Settings, or paste YAML via the UI")
			}
			file = kubeFile{ID: activeID, Name: "active", Path: s.Path, Active: true, Exists: true}
			if ctxName == "" {
				ctxName = s.Context
			}
			found = true
		} else if ctxName == "" {
			s, _ := kubecli.LoadSettings(c.app)
			if file.Active || file.ID == activeID {
				ctxName = s.Context
			}
		}
	}

	if !kubeclient.FileExists(file.Path) {
		return nil, fmt.Errorf("kubeconfig secret %q file missing at %s", file.Name, file.Path)
	}

	contexts, current, err := kubeclient.ListContexts(file.Path, ctxName)
	if err != nil {
		return nil, err
	}
	if ctxName == "" {
		ctxName = current
	}

	clusterName, user, ns := "", "", ""
	for _, cx := range contexts {
		if cx.Name == ctxName {
			clusterName = cx.Cluster
			user = cx.AuthInfo
			ns = cx.Namespace
			break
		}
	}
	if clusterName == "" && ctxName != "" {
		return nil, fmt.Errorf("context %q not found in kubeconfig secret %q — available: %s", ctxName, file.Name, contextNames(contexts))
	}

	cfg := kubeclient.Config{Path: file.Path, Context: ctxName}
	cli, err := kubeclient.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	restCfg, err := kubeclient.RestConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &resolvedCluster{
		KubeconfigID: file.ID,
		Name:         file.Name,
		Path:         file.Path,
		Context:      ctxName,
		Cluster:      clusterName,
		User:         user,
		Namespace:    ns,
		Client:       cli,
		REST:         restCfg,
	}, nil
}

func contextNames(contexts []kubeclient.ContextInfo) string {
	names := make([]string, 0, len(contexts))
	for _, c := range contexts {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}

func toolCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, defaultToolTimeout)
}

func defaultNS(resolved *resolvedCluster, namespace string) string {
	ns := strings.TrimSpace(namespace)
	if ns != "" {
		return ns
	}
	if resolved != nil && strings.TrimSpace(resolved.Namespace) != "" {
		return resolved.Namespace
	}
	return "default"
}
