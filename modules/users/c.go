package users

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/users/list"
	"github.com/izetmolla/containerws/modules/users/single"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/users")
	list.SetupRoutesAPI(api.Group("/list"), appClients)
	single.SetupRoutesAPI(api.Group("/single"), appClients)
}
