package softwares

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/softwares/buildin"
	"github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/modules/softwares/list"
	softwarespackage "github.com/izetmolla/containerws/modules/softwares/package"
	"github.com/izetmolla/containerws/modules/softwares/remotepkg"
	"github.com/izetmolla/containerws/modules/softwares/seed"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/modules/softwares/single"
	"github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/izetmolla/containerws/packages/softwaresync"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	seed.SeedIfEmpty(appClients)
	seed.RefreshShortDemoScripts(appClients)
	if db := appClients.DB(); db != nil {
		if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
			// Non-fatal: list/import still work once a registry is created manually.
			_ = err
		}
	}
	api := router.Group("/softwares")
	list.SetupRoutesAPI(api.Group("/list"), appClients)
	single.SetupRoutesAPI(api.Group("/single"), appClients)
	softwarespackage.SetupRoutesAPI(api.Group("/package"), appClients)
	remotepkg.SetupRoutesAPI(api.Group("/remotepkg"), appClients)
	// Register install queue before softwaresync so missing apps can be enqueued.
	install.SetupRoutesAPI(api.Group("/install"), appClients)
	// After catalog seed: probe DB-installed softwares; missing → os_missing + install queue
	// (skips rows marked Uninstalled so user uninstalls stay uninstalled).
	softwaresync.StartAsync(appClients.DB())
	service.SetupRoutesAPI(api.Group("/service"), appClients)
	buildin.SetupRoutesAPI(api, appClients)
}
