package install

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/packages/softwaresync"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	softwaresync.SetEnqueueMissing(EnqueueMissingInstalls)
	api.Post("/queue", cc.EnqueueAPI)
	api.Get("/queue", cc.GetQueueAPI)
	api.Post("/queue/retry", cc.RetryQueueAPI)
	api.Post("/queue/dismiss", cc.DismissQueueAPI)
	api.Post("/jobs/:jobId/cancel", cc.CancelInstallAPI)
	api.Get("/jobs/:jobId/stream", cc.StreamJobAPI)
	api.Get("/:id/job", cc.GetLatestJobAPI)
	api.Post("/:id/stream", cc.StreamInstallAPI)
	// Legacy non-streaming install kept for compatibility.
	api.Post("/:id", cc.InstallSoftwareAPI)
}
