package remotepkg

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

// SetupRoutesAPI mounts remote package registry management under /softwares/remotepkg.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/list", cc.GetRegistriesListAPI)
	api.Get("/packages", cc.GetRemotePackagesAPI)
	api.Post("/", cc.CreateRegistryAPI)
	api.Put("/:id", cc.UpdateRegistryAPI)
	api.Delete("/:id", cc.DeleteRegistryAPI)
}
