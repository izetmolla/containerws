package service

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
	api.Get("/status", cc.StatusAPI)
	api.Post("/start", cc.StartAPI)
	api.Post("/stop", cc.StopAPI)
	api.Get("/logs/stream", cc.StreamLogsAPI)
	api.Get("/logs", cc.LogsAPI)
}
