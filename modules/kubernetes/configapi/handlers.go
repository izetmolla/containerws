package configapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	"github.com/izetmolla/containerws/packages/kubeclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.GetAPI)
	api.Put("/", cc.UpdateAPI)
	api.Post("/test", cc.TestAPI)
	api.Get("/contexts", cc.ContextsAPI)

	files := api.Group("/files")
	files.Get("/list", cc.ListFilesAPI)
	single := files.Group("/single")
	single.Post("/", cc.CreateFileAPI)
	single.Get("/:id", cc.GetFileAPI)
	single.Put("/:id", cc.UpdateFileAPI)
	single.Delete("/:id", cc.DeleteFileAPI)
	single.Post("/:id/activate", cc.ActivateFileAPI)
}

type updateBody struct {
	Path     string `json:"path"`
	Context  string `json:"context"`
	ActiveID string `json:"active_id"`
}

// GetAPI returns persisted kubeconfig settings, registry, and contexts.
func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	list, activeID, err := ensureSeededRegistry(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	s, err := kubecli.LoadSettings(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	contexts, current, _ := kubeclient.ListContexts(s.Path, s.Context)

	type secretMapRow struct {
		ID       string                   `json:"id"`
		Name     string                   `json:"name"`
		Path     string                   `json:"path"`
		Managed  bool                     `json:"managed"`
		Exists   bool                     `json:"exists"`
		Active   bool                     `json:"active"`
		Contexts []kubeclient.ContextInfo `json:"contexts"`
	}
	secretMap := make([]secretMapRow, 0, len(list))
	for _, f := range list {
		row := secretMapRow{
			ID:      f.ID,
			Name:    f.Name,
			Path:    f.Path,
			Managed: f.Managed || kubeclient.IsManagedPath(f.Path),
			Exists:  kubeclient.FileExists(f.Path) || strings.TrimSpace(f.Secret) != "",
			Active:  f.ID == activeID,
		}
		if row.Exists {
			_ = materializeFile(f)
			ctxs, _, _ := kubeclient.ListContexts(f.Path, "")
			if len(ctxs) == 0 && strings.TrimSpace(f.Secret) != "" {
				ctxs, _, _ = listContextsForEntry(f, f.Secret)
			}
			row.Contexts = ctxs
		}
		secretMap = append(secretMap, row)
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"path":          s.Path,
			"context":       current,
			"exists":        s.Exists,
			"default_path":  s.Default,
			"contexts":      contexts,
			"context_count": len(contexts),
			"active_id":     activeID,
			"files":         toRows(list, activeID),
			"secret_map":    secretMap,
		},
	}))
}

// UpdateAPI sets the active kubeconfig (by id or path) and optional context.
func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body updateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("invalid body")), r.WithStatus(fiber.StatusBadRequest))
	}

	list, _, err := ensureSeededRegistry(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	ctxName := strings.TrimSpace(body.Context)
	activeID := strings.TrimSpace(body.ActiveID)

	if activeID != "" {
		entry, ok := findFile(list, activeID)
		if !ok {
			return r.Api(c, r.WithError(fmt.Errorf("kubeconfig %q not found", activeID)), r.WithStatus(fiber.StatusBadRequest))
		}
		if !kubeclient.FileExists(entry.Path) {
			return r.Api(c, r.WithError(fmt.Errorf("kubeconfig not found at %s", entry.Path)), r.WithStatus(fiber.StatusBadRequest))
		}
		if ctxName != "" {
			contexts, _, err := kubeclient.ListContexts(entry.Path, "")
			if err != nil {
				return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
			}
			found := false
			for _, cx := range contexts {
				if cx.Name == ctxName {
					found = true
					break
				}
			}
			if !found {
				return r.Api(c, r.WithError(fmt.Errorf("context %q not found in kubeconfig", ctxName)), r.WithStatus(fiber.StatusBadRequest))
			}
		}
		if err := activateFile(cc.app.DB(), entry, ctxName); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	} else {
		path := strings.TrimSpace(body.Path)
		if path == "" {
			path = kubeclient.DefaultPath()
		}
		if !filepath.IsAbs(path) {
			return r.Api(c, r.WithError(fmt.Errorf("kubeconfig path must be absolute")), r.WithStatus(fiber.StatusBadRequest))
		}
		if _, err := os.Stat(path); err != nil {
			return r.Api(c, r.WithError(fmt.Errorf("kubeconfig not found at %s", path)), r.WithStatus(fiber.StatusBadRequest))
		}
		if ctxName != "" {
			contexts, _, err := kubeclient.ListContexts(path, "")
			if err != nil {
				return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
			}
			found := false
			for _, cx := range contexts {
				if cx.Name == ctxName {
					found = true
					break
				}
			}
			if !found {
				return r.Api(c, r.WithError(fmt.Errorf("context %q not found in kubeconfig", ctxName)), r.WithStatus(fiber.StatusBadRequest))
			}
		}
		matched := false
		for _, f := range list {
			if f.Path == path {
				if err := activateFile(cc.app.DB(), f, ctxName); err != nil {
					return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
				}
				matched = true
				break
			}
		}
		if !matched {
			if err := models.SetOption(cc.app.DB(), models.OptionKubeconfigPath, path); err != nil {
				return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
			}
			if err := models.SetOption(cc.app.DB(), models.OptionKubeconfigContext, ctxName); err != nil {
				return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
			}
			_ = models.SetOption(cc.app.DB(), models.OptionKubeconfigActiveID, "")
			kubeclient.Reset()
		}
	}

	list, activeIDOut, _ := ensureSeededRegistry(cc.app)
	s, _ := kubecli.LoadSettings(cc.app)
	contexts, current, _ := kubeclient.ListContexts(s.Path, s.Context)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"path":      s.Path,
			"context":   current,
			"exists":    s.Exists,
			"contexts":  contexts,
			"active_id": activeIDOut,
			"files":     toRows(list, activeIDOut),
		},
		"message": "Kubeconfig settings saved",
	}))
}

// TestAPI pings the cluster using the current (or body-overridden) settings.
func (cc *controller) TestAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body updateBody
	_ = c.Bind().Body(&body)

	s, err := kubecli.LoadSettings(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	path := s.Path
	ctxName := s.Context

	if id := strings.TrimSpace(body.ActiveID); id != "" {
		list, _, _ := ensureSeededRegistry(cc.app)
		if entry, ok := findFile(list, id); ok {
			path = entry.Path
		}
	}
	if strings.TrimSpace(body.Path) != "" {
		path = strings.TrimSpace(body.Path)
	}
	if body.Context != "" || strings.TrimSpace(body.Path) != "" || strings.TrimSpace(body.ActiveID) != "" {
		ctxName = strings.TrimSpace(body.Context)
	}

	cli, err := kubeclient.Client(kubeclient.Config{Path: path, Context: ctxName})
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": fiber.Map{"ok": false, "error": err.Error(), "path": path, "context": ctxName},
		}))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 8*time.Second)
	defer cancel()
	ver, err := cli.Discovery().ServerVersion()
	if err != nil {
		kubeclient.Reset()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": fiber.Map{"ok": false, "error": err.Error(), "path": path, "context": ctxName},
		}))
	}
	nsCount := 0
	if list, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 500}); err == nil {
		nsCount = len(list.Items)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"ok":              true,
			"path":            kubeclient.ResolvePath(path),
			"context":         ctxName,
			"version":         ver.GitVersion,
			"platform":        ver.Platform,
			"namespace_count": nsCount,
		},
	}))
}

// ContextsAPI lists contexts from the host kubeconfig file.
func (cc *controller) ContextsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	s, err := kubecli.LoadSettings(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	path := s.Path
	if q := strings.TrimSpace(c.Query("path")); q != "" {
		path = q
	}
	if id := strings.TrimSpace(c.Query("id")); id != "" {
		list, _, _ := ensureSeededRegistry(cc.app)
		if entry, ok := findFile(list, id); ok {
			path = entry.Path
		}
	}
	contexts, current, err := kubeclient.ListContexts(path, s.Context)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"path":     kubeclient.ResolvePath(path),
			"current":  current,
			"contexts": contexts,
		},
	}))
}
