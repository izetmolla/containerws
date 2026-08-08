package users

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vncnovnc/users/list"
	"github.com/izetmolla/containerws/modules/vncnovnc/users/single"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	list.SetupRoutesAPI(router.Group("/list"), appClients)
	single.SetupRoutesAPI(router.Group("/single"), appClients)
}
