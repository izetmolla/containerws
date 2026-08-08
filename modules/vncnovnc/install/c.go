package install

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/setup"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/install")
	setup.SetupRoutesAPI(api.Group("/setup"), appClients)
	adduser.SetupRoutesAPI(api.Group("/adduser"), appClients)
}
