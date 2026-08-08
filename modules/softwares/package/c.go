package softwarespackage

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

// SetupRoutesAPI mounts package editor + GitHub registry endpoints under /softwares/package.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)

	// Static paths before /:id so they are not captured as software ids.
	api.Get("/registry", cc.ListRegistriesAPI)
	api.Post("/registry", cc.CreateRegistryAPI)
	api.Put("/registry/:registryId", cc.UpdateRegistryAPI)
	api.Delete("/registry/:registryId", cc.DeleteRegistryAPI)
	api.Post("/import", cc.ImportSoftwareAPI)

	api.Get("/:id", cc.GetPackageAPI)
	api.Put("/:id", cc.UpdateSoftwareAPI)
	api.Post("/:id/versions", cc.CreateVersionAPI)
	api.Put("/:id/versions/:versionId", cc.UpdateVersionAPI)
	api.Delete("/:id/versions/:versionId", cc.DeleteVersionAPI)
}
