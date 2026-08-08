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
	api.Post("/", cc.CreateCodeserverSessionAPI)
	api.Post("/stream", cc.StreamCreateCodeserverSessionAPI)
	api.Post("/open-editor", cc.OpenEditorAPI)
	api.Get("/:id", cc.GetCodeserverSessionAPI)
	api.Put("/:id", cc.UpdateCodeserverSessionAPI)
	api.Delete("/:id", cc.DeleteCodeserverSessionAPI)
	api.Post("/:id/disable", cc.DisableCodeserverSessionAPI)
	api.Post("/:id/enable", cc.EnableCodeserverSessionAPI)
	api.Post("/:id/open", cc.OpenCodeserverSessionAPI)
	api.Post("/:id/reactivate/stream", cc.StreamReactivateCodeserverSessionAPI)
}
