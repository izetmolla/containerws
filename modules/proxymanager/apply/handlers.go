package apply

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
	fiberproxy "github.com/izetmolla/containerws/packages/proxymanager/fiber"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/status", cc.StatusAPI)
	api.Get("/runs", cc.RunsAPI)
	api.Get("/preview", cc.PreviewAPI)
	api.Post("/", cc.ApplyAPI)
	api.Get("/fiber-table", cc.FiberTableAPI)
}

func appBaseURL(c fiber.Ctx, app *config.AppClients) string {
	cfg := app.ServerConfig()
	scheme := "http"
	if cfg != nil && cfg.ENABLE_HTTPS {
		scheme = "https"
	}
	host := string(c.Host())
	if host == "" && cfg != nil {
		port := cfg.PORT
		if port == "" {
			port = "3000"
		}
		host = "127.0.0.1:" + port
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func (cc *controller) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	settings, err := proxymanager.EnsureSettings(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	runtime, err := proxymanager.DetectRuntime(c.Context(), cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var last models.ProxyApplyRun
	_ = cc.app.DB().Order("started_at desc").First(&last).Error
	table := fiberproxy.Get()
	check := proxymanager.CheckComponents(settings, runtime)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"settings":       settings,
			"runtime":        runtime,
			"last_run":       last,
			"fiber_table":    table,
			"components":     check,
			"module_enabled": models.ModuleEnabled(cc.app.DB(), models.OptionProxymanagerModuleEnabled),
		},
	}))
}

func (cc *controller) RunsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var rows []models.ProxyApplyRun
	if err := cc.app.DB().WithContext(c.Context()).Order("started_at desc").Limit(50).Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) ApplyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	res, err := proxymanager.Apply(c.Context(), cc.app.DB(), proxymanager.ApplyOptions{
		AppBaseURL: appBaseURL(c, cc.app),
	})
	if err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "validation:") {
			status = fiber.StatusBadRequest
		}
		payload := fiber.Map{
			"error":   err.Error(),
			"message": err.Error(),
		}
		if res != nil {
			payload["run"] = res.Run
			payload["files"] = res.Files
			payload["log"] = res.Log
			payload["data"] = res
		}
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithData(payload))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": res, "message": "Applied"}))
}

func (cc *controller) PreviewAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	res, contents, err := proxymanager.PreviewConfigs(c.Context(), cc.app.DB(), appBaseURL(c, cc.app))
	if err != nil && res == nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"result":   res,
			"contents": contents,
			"error":    errString(err),
		},
	}))
}

func (cc *controller) FiberTableAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	table := fiberproxy.Get()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": table}))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
