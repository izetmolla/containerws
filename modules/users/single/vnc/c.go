package vnc

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

// SetupRoutesAPI mounts per-user VNC profile routes under /users/single.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/:id/vnc", cc.GetVncProfileAPI)
	api.Get("/:id/vnc/addresses", cc.ListVncAddressesAPI)
	api.Get("/:id/vnc/wallpaper", cc.GetWallpaperAPI)
	api.Post("/:id/vnc/wallpaper", cc.UploadWallpaperAPI)
	api.Delete("/:id/vnc/wallpaper", cc.DeleteWallpaperAPI)
	api.Post("/:id/vnc", cc.CreateVncProfileAPI)
	api.Put("/:id/vnc", cc.UpdateVncProfileAPI)
	api.Post("/:id/vnc/password", cc.SetVncPasswordAPI)
	api.Post("/:id/vnc/start", cc.StartVncProfileAPI)
	api.Post("/:id/vnc/stop", cc.StopVncProfileAPI)
}
