package setup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/detect", cc.DetectAPI)
	api.Get("/status", cc.StatusAPI)
	api.Post("/stream", cc.StreamSetupAPI)
	api.Post("/jobs/:jobId/cancel", cc.CancelSetupAPI)
	api.Post("/", cc.SetupAPI)

	// Softwares queue yields while VNC package scripts hold the package manager.
	swinstall.SetPackageManagerBusyCheck(PackageManagerHeld)

	// Boot reconcile: Option rows + auto-reinstall when installed-but-missing.
	StartAsync(appClients.DB())
}
