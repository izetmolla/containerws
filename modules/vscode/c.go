package vscode

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vscode/list"
	"github.com/izetmolla/containerws/modules/vscode/single"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/vscode")
	list.SetupRoutesAPI(api.Group("/list"), appClients)
	single.SetupRoutesAPI(api.Group("/single"), appClients)
}
