package modules

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/frontend"
	"github.com/izetmolla/containerws/modules/authorization"
	"github.com/izetmolla/containerws/modules/brew"
	"github.com/izetmolla/containerws/modules/cloudshell"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/dashboard"
	"github.com/izetmolla/containerws/modules/docker"
	"github.com/izetmolla/containerws/modules/filemanager"
	"github.com/izetmolla/containerws/modules/general"
	"github.com/izetmolla/containerws/modules/kubernetes"
	"github.com/izetmolla/containerws/modules/mcp"
	"github.com/izetmolla/containerws/modules/novnc"
	"github.com/izetmolla/containerws/modules/proxymanager"
	"github.com/izetmolla/containerws/modules/settings"
	"github.com/izetmolla/containerws/modules/softwares"
	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/modules/users"
	"github.com/izetmolla/containerws/modules/vncnovnc"
	"github.com/izetmolla/containerws/modules/vscode"
	"gorm.io/gorm"
)

const remoteDesktopPrefix = "/remotedesktop"

func SetupRoutes(app *fiber.App, appClients *config.AppClients) {
	auth := appClients.Authorization()
	api := app.Group("/api")
	app.Get("/static/*", frontend.UseStatic())

	authorization.SetupRoutesAPI(api, appClients)
	authorization.SetupRoutesView(app, appClients)

	// MCP streamable HTTP (token auth via MCP_TOKEN; mounted before JWT middleware).
	mcp.SetupRoutesAPI(app, appClients)

	// Proxy Manager Fiber engine — Host-based reverse proxy must run before web auth
	// so external virtual hosts are not redirected to sign-in.
	proxymanager.SetupRoutesView(app, appClients)

	api.Use(auth.HandleRefreshToken)
	api.Use(auth.UseAPIAuthorization(
		auth.WithRoles([]string{}),
		auth.WithExcludedPaths([]string{"/api/authorization/"}),
	))
	app.Use(auth.UseWEBAuthorization(
		auth.WithRoles([]string{}),
		auth.WithExcludedPaths([]string{
			"/api",
			"/mcp",
			"/static",
			"/novnc", // dual-auth inside novnc (JWT ?access_token= or session cookie)
			"/codeserver",
			"/sign-in",
			"/register",
			"/forgot-password",
		}),
	))

	softwares.SetupRoutesAPI(api, appClients)
	brew.SetupRoutesAPI(api, appClients)
	// Ensure existing Homebrew installs are on PATH for shells / MCP bash.
	brew.EnsureBrewShellPath()

	// Serialize Brew installs with Softwares / VNC package jobs on one queue.
	swinstall.SetBrewActionRunner(func(action, kind string, names []string) (jobID, message string, err error) {
		job, runErr := brew.RunActionSync(action, names, kind)
		if job != nil {
			jobID = job.ID
			if job.Status == "success" {
				brew.ApplyJobOwnership(appClients.DB(), job)
				message = "brew " + action + " completed"
			} else if job.Error != "" {
				message = job.Error
			}
		}
		return jobID, message, runErr
	})
	brew.SetSoftwaresQueueEnqueue(func(db *gorm.DB, action string, names []string, kind string) (int, any, error) {
		n, snap, err := swinstall.EnqueueBrewActions(db, action, names, kind)
		return n, snap, err
	})
	// Detect brew CLI installs (outside the portal) and mirror them into Softwares installed.
	brew.StartSyncAsync(appClients.DB())

	docker.SetupRoutesAPI(api, appClients)
	filemanager.SetupRoutesAPI(api, appClients)
	kubernetes.SetupRoutesAPI(api, appClients)
	proxymanager.SetupRoutesAPI(api, appClients)
	cloudshell.SetupRoutesAPI(api, appClients)
	dashboard.SetupRoutesAPI(api, appClients)
	general.SetupRoutesAPI(api, appClients)
	settings.SetupRoutesAPI(api, appClients)
	users.SetupRoutesAPI(api, appClients)
	vncnovnc.SetupRoutesAPI(api, appClients)
	vscode.SetupRoutesAPI(api, appClients)

	// noVNC proxy → active VncSession address:no_vnc_port (auth required).
	novnc.SetupRoutesView(app, appClients)
	// VS Code Server proxy → CodeserverSession address:port (auth required).
	codeserver.SetupRoutesView(app, appClients)

	api.Use(appClients.ApiNotFound)
	app.Use(appClients.ViewNotFound)
}
