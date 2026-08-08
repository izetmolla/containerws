package rdp

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/vncnovnc/rdp/install"
)

// SetupRoutesAPI mounts /api/vnc-novnc/rdp (optional xrdp, separate from VNC packages).
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/rdp")
	install.SetupRoutesAPI(api, appClients)
}
