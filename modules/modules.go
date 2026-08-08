package modules

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/frontend"
	"github.com/izetmolla/containerws/modules/authorization"
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
	"github.com/izetmolla/containerws/modules/users"
	"github.com/izetmolla/containerws/modules/vncnovnc"
	"github.com/izetmolla/containerws/modules/vscode"
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
