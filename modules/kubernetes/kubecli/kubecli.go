package kubecli

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/kubeclient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Settings is the persisted kubeconfig location (DB) used to talk to the cluster.
type Settings struct {
	Path    string `json:"path"`
	Context string `json:"context,omitempty"`
	Exists  bool   `json:"exists"`
	Default string `json:"default_path"`
}

// LoadSettings reads kubeconfig path/context from options (DB).
// Active k8s_keys row wins when present.
func LoadSettings(app *config.AppClients) (Settings, error) {
	out := Settings{Default: kubeclient.DefaultPath()}
	if app == nil || app.DB() == nil {
		out.Path = out.Default
		out.Exists = kubeclient.FileExists(out.Path)
		return out, nil
	}
	path, _, err := models.GetOption(app.DB(), models.OptionKubeconfigPath)
	if err != nil {
		return out, err
	}
	ctxName, _, err := models.GetOption(app.DB(), models.OptionKubeconfigContext)
	if err != nil {
		return out, err
	}
	activeID, _, err := models.GetOption(app.DB(), models.OptionKubeconfigActiveID)
	if err != nil {
		return out, err
	}
	activeID = strings.TrimSpace(activeID)
	if activeID != "" {
		var key models.K8sKey
		if err := app.DB().Where("id = ?", activeID).First(&key).Error; err == nil {
			if strings.TrimSpace(key.Path) != "" {
				path = key.Path
			}
			// Materialize managed secrets so client-go can read them.
			if strings.TrimSpace(key.Secret) != "" && !kubeclient.FileExists(key.Path) {
				if kubeclient.IsManagedPath(key.Path) {
					_ = kubeclient.EnsureManagedStore()
				}
				_ = kubeclient.WriteFile(key.Path, []byte(key.Secret))
			}
		}
	}
	out.Path = kubeclient.ResolvePath(path)
	out.Context = strings.TrimSpace(ctxName)
	out.Exists = kubeclient.FileExists(out.Path)
	return out, nil
}

// Client builds a Kubernetes clientset from persisted settings.
func Client(app *config.AppClients) (kubernetes.Interface, Settings, error) {
	s, err := LoadSettings(app)
	if err != nil {
		return nil, s, err
	}
	cli, err := kubeclient.Client(kubeclient.Config{Path: s.Path, Context: s.Context})
	return cli, s, err
}

// RestConfig builds a REST config from persisted kubeconfig settings (for exec/attach).
func RestConfig(app *config.AppClients) (*rest.Config, Settings, error) {
	s, err := LoadSettings(app)
	if err != nil {
		return nil, s, err
	}
	cfg, err := kubeclient.RestConfig(kubeclient.Config{Path: s.Path, Context: s.Context})
	return cfg, s, err
}

// ClientFromCtx is Client with Fiber context for consistency.
func ClientFromCtx(app *config.AppClients, _ fiber.Ctx) (kubernetes.Interface, Settings, error) {
	return Client(app)
}

// NamespaceQuery returns ?namespace= or empty (all namespaces).
func NamespaceQuery(c fiber.Ctx) string {
	return strings.TrimSpace(c.Query("namespace"))
}
