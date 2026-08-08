package list

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
	api.Get("/", cc.GetCodeserverSessionsListAPI)
	api.Get("/columns", cc.GetCodeserverSessionsColumnsAPI)
	api.Get("/available-users", cc.GetAvailableUsersAPI)
	api.Get("/status", cc.GetCodeserverStatusAPI)
	api.Get("/paths", cc.GetCodeserverPathsAPI)
}
