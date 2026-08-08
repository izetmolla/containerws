package proxymanager

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/proxymanager/apply"
	"github.com/izetmolla/containerws/modules/proxymanager/certificates"
	"github.com/izetmolla/containerws/modules/proxymanager/fiberproxy"
	"github.com/izetmolla/containerws/modules/proxymanager/hosts"
	"github.com/izetmolla/containerws/modules/proxymanager/logs"
	"github.com/izetmolla/containerws/modules/proxymanager/redirects"
	"github.com/izetmolla/containerws/modules/proxymanager/settings"
)

// SetupRoutesAPI mounts /api/proxymanager/...
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/proxymanager")
	settings.SetupRoutesAPI(api.Group("/settings"), appClients)
	hosts.SetupRoutesAPI(api.Group("/hosts"), appClients)
	certificates.SetupRoutesAPI(api.Group("/certificates"), appClients)
	redirects.SetupRoutesAPI(api.Group("/redirects"), appClients)
	apply.SetupRoutesAPI(api.Group("/apply"), appClients)
	logs.SetupRoutesAPI(api.Group("/logs"), appClients)
	// Boot: enablement + component check + auto-apply (async).
	StartAsync(appClients)
}

// SetupRoutesView mounts the Fiber reverse-proxy middleware (active when engine=fiber).
func SetupRoutesView(app fiber.Router, appClients *config.AppClients) {
	fiberproxy.SetupRoutesView(app, appClients)
}
