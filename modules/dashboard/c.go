package dashboard

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

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api := router.Group("/dashboard")
	api.Get("/metrics", cc.GetMetricsAPI)
	api.Get("/tools", cc.GetToolsAPI)
	api.Get("/processes", cc.GetProcessesAPI)
	api.Post("/processes/:pid/kill", cc.KillProcessAPI)
}
