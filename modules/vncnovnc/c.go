package vncnovnc

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vncnovnc/install"
	"github.com/izetmolla/containerws/modules/vncnovnc/rdp"
	"github.com/izetmolla/containerws/modules/vncnovnc/service"
	"github.com/izetmolla/containerws/modules/vncnovnc/users"
)

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/vnc-novnc")
	users.SetupRoutesAPI(api.Group("/users"), appClients)
	install.SetupRoutesAPI(api, appClients)
	rdp.SetupRoutesAPI(api, appClients)
	service.SetupRoutesAPI(api.Group("/service"), appClients)
}
