package filemanager

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/filemanager/list"
	"github.com/izetmolla/containerws/modules/filemanager/ops"
)

// SetupRoutesAPI mounts /filemanager under the authenticated /api group.
// All handlers operate on the host filesystem as the panel user's Linux account
// (no database). Linux DAC permissions gate what each user can see and change.
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/filemanager")
	list.SetupRoutesAPI(api.Group("/list"), appClients)
	ops.SetupRoutesAPI(api.Group("/ops"), appClients)
}
