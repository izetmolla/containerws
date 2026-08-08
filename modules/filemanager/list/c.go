package list

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
)

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.ListAPI)
	api.Get("/roots", cc.RootsAPI)
	api.Get("/stat", cc.StatAPI)
}
