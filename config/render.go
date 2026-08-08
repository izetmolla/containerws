package config

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/packages/render"
)

func (a *AppClients) Render() *render.Render {
	if a == nil {
		return nil
	}
	return a.render
}

func (a *AppClients) Api(c fiber.Ctx, options ...render.RenderOptionsFunc) error {
	if a == nil || a.render == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "render service unavailable",
			"code":    "SERVER_ERROR",
			"status":  fiber.StatusInternalServerError,
		})
	}
	return a.render.Api(c, options...)
}

func (app *AppClients) ApiNotFound(c fiber.Ctx) error {
	if app == nil || app.render == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Not Found",
			"code":    "NOT_FOUND",
			"status":  fiber.StatusNotFound,
		})
	}
	return app.render.ApiNotFound(c)
}

func (app *AppClients) View(c fiber.Ctx, options ...render.RenderOptionsFunc) error {
	if app == nil || app.render == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("render service unavailable")
	}
	return app.render.View(c, options...)
}

func (app *AppClients) ViewNotFound(c fiber.Ctx) error {
	if app == nil || app.render == nil {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}
	return app.render.ViewNotFound(c)
}
