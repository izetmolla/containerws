package single

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Post("/", cc.CreateVncSessionAPI)
	api.Get("/:id", cc.GetVncSessionAPI)
	api.Put("/:id", cc.UpdateVncSessionAPI)
	api.Delete("/:id", cc.DeleteVncSessionAPI)
	api.Post("/:id/password", cc.SetVncPasswordAPI)
	api.Post("/:id/disable", cc.DisableVncSessionAPI)
	api.Post("/:id/enable", cc.EnableVncSessionAPI)
	api.Post("/:id/quick", cc.QuickVncSessionAPI)
	api.Post("/:id/restart", cc.RestartVncSessionAPI)
}
