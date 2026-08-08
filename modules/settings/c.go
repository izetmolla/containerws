package settings

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/settings/environments"
	"github.com/izetmolla/containerws/modules/settings/general"
	"github.com/izetmolla/containerws/modules/settings/mcp"
	"github.com/izetmolla/containerws/modules/settings/options"
	"github.com/izetmolla/containerws/modules/settings/update"
)

// SetupRoutesAPI mounts /api/settings/*.
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/settings")
	general.SetupRoutesAPI(api.Group("/general"), appClients)
	environments.SetupRoutesAPI(api.Group("/environments"), appClients)
	options.SetupRoutesAPI(api.Group("/options"), appClients)
	mcp.SetupRoutesAPI(api.Group("/mcp"), appClients)
	update.SetupRoutesAPI(api.Group("/update"), appClients)
}
