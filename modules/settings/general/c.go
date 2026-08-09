package general

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/brew"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.GetGeneralSettingsAPI)
	api.Put("/", cc.UpdateGeneralSettingsAPI)
	api.Put("/modules", cc.UpdateModuleSettingsAPI)
}

type generalSettingsBody struct {
	WorkspaceName        string `json:"workspace_name"`
	WorkspaceDescription string `json:"workspace_description"`
}

type moduleSettingsBody struct {
	DockerEnabled         *bool `json:"docker_enabled"`
	KubernetesEnabled     *bool `json:"kubernetes_enabled"`
	ProxymanagerEnabled   *bool `json:"proxymanager_enabled"`
	BrewEnabled           *bool `json:"brew_enabled"`
	LocalhostAutoLogin    *bool `json:"localhost_auto_login"`
}

func (cc *controller) GetGeneralSettingsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	name, _, _ := models.GetOption(db, models.OptionWorkspaceName)
	desc, _, _ := models.GetOption(db, models.OptionWorkspaceDescription)
	if strings.TrimSpace(name) == "" {
		name = "Container Workspace"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cc.settingsPayload(db, name, desc),
	}))
}

func (cc *controller) UpdateGeneralSettingsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var body generalSettingsBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := strings.TrimSpace(body.WorkspaceName)
	if name == "" {
		name = "Container Workspace"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	desc := strings.TrimSpace(body.WorkspaceDescription)
	if len(desc) > 500 {
		desc = desc[:500]
	}
	if err := models.SetOption(db, models.OptionWorkspaceName, name); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := models.SetOption(db, models.OptionWorkspaceDescription, desc); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.settingsPayload(db, name, desc),
		"message": "Settings saved",
	}))
}

func (cc *controller) UpdateModuleSettingsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var body moduleSettingsBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	if body.DockerEnabled != nil {
		if err := models.SetOptionBool(db, models.OptionDockerModuleEnabled, *body.DockerEnabled); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	if body.KubernetesEnabled != nil {
		if err := models.SetOptionBool(db, models.OptionKubernetesModuleEnabled, *body.KubernetesEnabled); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	if body.ProxymanagerEnabled != nil {
		if err := models.SetOptionBool(db, models.OptionProxymanagerModuleEnabled, *body.ProxymanagerEnabled); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	if body.BrewEnabled != nil {
		if err := models.SetOptionBool(db, models.OptionBrewModuleEnabled, *body.BrewEnabled); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		// Auto-bootstrap Homebrew when enabling the module and brew is missing.
		if *body.BrewEnabled {
			brew.MaybeStart(nil)
		}
	}
	if body.LocalhostAutoLogin != nil {
		if err := models.SetOptionBool(db, models.OptionLocalhostAutoLogin, *body.LocalhostAutoLogin); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	name, _, _ := models.GetOption(db, models.OptionWorkspaceName)
	desc, _, _ := models.GetOption(db, models.OptionWorkspaceDescription)
	if strings.TrimSpace(name) == "" {
		name = "Container Workspace"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.settingsPayload(db, name, desc),
		"message": "Module settings saved",
	}))
}

func (cc *controller) settingsPayload(db *gorm.DB, name, desc string) fiber.Map {
	return fiber.Map{
		"workspace_name":         name,
		"workspace_description":  desc,
		"docker_enabled":         models.ModuleEnabled(db, models.OptionDockerModuleEnabled),
		"kubernetes_enabled":     models.ModuleEnabled(db, models.OptionKubernetesModuleEnabled),
		"proxymanager_enabled":   models.ModuleEnabled(db, models.OptionProxymanagerModuleEnabled),
		"brew_enabled":           models.BrewModuleEnabled(db),
		"localhost_auto_login":   models.LocalhostAutoLoginEnabled(db),
		"docker":                 cc.dockerModuleStatus(db),
		"kubernetes":             cc.kubernetesModuleStatus(db),
		"proxymanager":           cc.proxymanagerModuleStatus(db),
		"brew":                   cc.brewModuleStatus(),
	}
}

func (cc *controller) brewModuleStatus() fiber.Map {
	path := brew.ResolveBrewPath()
	boot := brew.BootstrapStatus()
	installing, _ := boot["running"].(bool)
	return fiber.Map{
		"binary_present": path != "",
		"brew_path":      path,
		"prefix":         brew.BrewPrefix(path),
		"installing":     installing,
		"bootstrap":      boot,
	}
}

func (cc *controller) dockerModuleStatus(db *gorm.DB) fiber.Map {
	out := fiber.Map{
		"binary_present": false,
		"running":        false,
		"installed":      false,
		"software_id":    "",
		"software_name":  "Docker Engine",
	}

	if path, err := exec.LookPath("docker"); err == nil && path != "" {
		out["binary_present"] = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if cli, err := dockerclient.Client(); err == nil {
		if _, err := cli.Ping(ctx); err == nil {
			out["running"] = true
			out["binary_present"] = true
		}
	}

	if db == nil {
		return out
	}
	var sw models.Software
	err := db.Where("name = ? AND is_active = ?", "Docker Engine", true).First(&sw).Error
	if err != nil {
		return out
	}
	out["software_id"] = sw.ID
	out["software_name"] = sw.Name
	row, err := models.GetSoftwareInstalled(db, sw.ID)
	if err == nil && row != nil && !row.Uninstalled {
		out["installed"] = true
	}
	return out
}

func (cc *controller) kubernetesModuleStatus(db *gorm.DB) fiber.Map {
	out := fiber.Map{
		"configured": false,
		"active_id":  "",
		"files":      0,
	}
	if db == nil {
		return out
	}
	active, ok, _ := models.GetOption(db, models.OptionKubeconfigActiveID)
	if ok && strings.TrimSpace(active) != "" {
		out["configured"] = true
		out["active_id"] = strings.TrimSpace(active)
	}
	raw, ok, _ := models.GetOption(db, models.OptionKubeconfigFiles)
	if ok && strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "[]" && strings.TrimSpace(raw) != "null" {
		// Cheap count: non-empty registry JSON implies at least one file when configured above,
		// but still expose a rough presence signal for the UI badge.
		out["files"] = 1
		out["configured"] = true
	}
	path, ok, _ := models.GetOption(db, models.OptionKubeconfigPath)
	if ok && strings.TrimSpace(path) != "" {
		out["configured"] = true
	}
	return out
}

func (cc *controller) proxymanagerModuleStatus(db *gorm.DB) fiber.Map {
	out := fiber.Map{
		"active_engine": "fiber",
		"dirty":         false,
		"configured":    false,
	}
	if db == nil {
		return out
	}
	var s models.ProxySettings
	err := db.Where("id = ?", models.ProxySettingsSingletonID).First(&s).Error
	if err != nil {
		return out
	}
	out["active_engine"] = s.ActiveEngine
	out["dirty"] = s.Dirty
	out["configured"] = true
	if s.LastAppliedAt != nil {
		out["last_applied_at"] = s.LastAppliedAt.UTC().Format(time.RFC3339)
	}
	return out
}
