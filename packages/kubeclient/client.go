// Package kubeclient builds a Kubernetes clientset from a host kubeconfig file.
// Cluster resources are never stored in the app DB — only the kubeconfig path
// and optional context name are persisted as options.
package kubeclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const defaultKubeconfig = "/root/.kube/config"

var (
	mu        sync.Mutex
	shared    kubernetes.Interface
	sharedKey string
)

// DefaultPath returns the default kubeconfig location on this host.
func DefaultPath() string {
	if v := strings.TrimSpace(os.Getenv("KUBECONFIG")); v != "" {
		// Use the first entry when KUBECONFIG is a list.
		if i := strings.IndexByte(v, ':'); i > 0 {
			return v[:i]
		}
		return v
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		p := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return defaultKubeconfig
}

// ResolvePath returns path if non-empty, otherwise DefaultPath().
func ResolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultPath()
	}
	return path
}

// Config holds the kubeconfig location and optional context override.
type Config struct {
	Path    string
	Context string
}

func (c Config) cacheKey() string {
	return ResolvePath(c.Path) + "\x00" + strings.TrimSpace(c.Context)
}

// Client returns a shared clientset for the given config.
func Client(cfg Config) (kubernetes.Interface, error) {
	key := cfg.cacheKey()
	mu.Lock()
	defer mu.Unlock()
	if shared != nil && sharedKey == key {
		return shared, nil
	}
	cli, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	shared = cli
	sharedKey = key
	return shared, nil
}

// NewClient builds a fresh clientset (no process-wide cache). Prefer for
// concurrent multi-context MCP tools so they do not thrash the shared client.
func NewClient(cfg Config) (kubernetes.Interface, error) {
	restCfg, err := buildREST(cfg)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}

// RestConfig builds a REST config without caching a clientset.
func RestConfig(cfg Config) (*rest.Config, error) {
	return buildREST(cfg)
}

// Reset drops the cached clientset (call after path/context change).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	shared = nil
	sharedKey = ""
}

func buildREST(cfg Config) (*rest.Config, error) {
	path := ResolvePath(cfg.Path)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("kubeconfig not found at %s: %w", path, err)
	}
	loading := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	overrides := &clientcmd.ConfigOverrides{}
	if ctx := strings.TrimSpace(cfg.Context); ctx != "" {
		overrides.CurrentContext = ctx
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
}

// ContextInfo describes a kubeconfig context entry.
type ContextInfo struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	AuthInfo  string `json:"user"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool   `json:"current"`
}

// LoadRaw reads the kubeconfig file from disk.
func LoadRaw(path string) (*clientcmdapi.Config, error) {
	path = ResolvePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return clientcmd.Load(data)
}

// ListContexts returns contexts from the kubeconfig file.
func ListContexts(path, selectedContext string) ([]ContextInfo, string, error) {
	raw, err := LoadRaw(path)
	if err != nil {
		return nil, "", err
	}
	current := strings.TrimSpace(selectedContext)
	if current == "" {
		current = raw.CurrentContext
	}
	out := make([]ContextInfo, 0, len(raw.Contexts))
	for name, ctx := range raw.Contexts {
		if ctx == nil {
			continue
		}
		out = append(out, ContextInfo{
			Name:      name,
			Cluster:   ctx.Cluster,
			AuthInfo:  ctx.AuthInfo,
			Namespace: ctx.Namespace,
			Current:   name == current,
		})
	}
	return out, current, nil
}

// FileExists reports whether the resolved kubeconfig path is readable.
func FileExists(path string) bool {
	_, err := os.Stat(ResolvePath(path))
	return err == nil
}
