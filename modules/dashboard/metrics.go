package dashboard

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/modules/softwares/buildin"
	"github.com/izetmolla/containerws/packages/machine"
)

func (cc *controller) GetMetricsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	metrics := machine.CollectMetrics()
	tools := buildin.CollectToolStatuses(cc.app.DB())
	return r.Api(c, r.WithData(fiber.Map{
		"data":  metrics,
		"tools": tools,
	}))
}

func (cc *controller) GetToolsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	return r.Api(c, r.WithData(fiber.Map{
		"data": buildin.CollectToolStatuses(cc.app.DB()),
	}))
}
