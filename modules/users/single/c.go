package single

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/users/single/keys"
	"github.com/izetmolla/containerws/modules/users/single/rdp"
	"github.com/izetmolla/containerws/modules/users/single/vnc"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Post("/", cc.CreateUserAPI)
	api.Get("/:id", cc.GetUserAPI)
	api.Put("/:id", cc.UpdateUserAPI)
	api.Delete("/:id", cc.DeleteUserAPI)
	api.Post("/:id/password", cc.SetPasswordAPI)
	api.Post("/:id/linux", cc.ProvisionLinuxAPI)
	api.Put("/:id/linux", cc.UpdateLinuxAPI)
	api.Delete("/:id/linux", cc.DeleteLinuxAPI)
	api.Post("/:id/linux/password", cc.SetLinuxPasswordAPI)
	api.Post("/:id/linux/groups", cc.SetLinuxGroupsAPI)
	api.Post("/:id/linux/lock", cc.LockLinuxAPI)
	api.Post("/:id/linux/unlock", cc.UnlockLinuxAPI)
	vnc.SetupRoutesAPI(api, appClients)
	rdp.SetupRoutesAPI(api, appClients)
	keys.SetupRoutesAPI(api, appClients)
}
