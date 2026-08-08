package install

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vncnovnc/rdp/install/setup"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/install")
	setup.SetupRoutesAPI(api.Group("/setup"), appClients)
}
