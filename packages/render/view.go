package render

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/gofiber/fiber/v3"
)

func (r *Render) View(c fiber.Ctx, options ...RenderOptionsFunc) error {
	opts := r.NewRenderOptions(options...)

	// Set response headers
	c.Set("Content-Type", "text/html; charset=utf-8")
	if opts.errorStatus != 0 {
		c.Status(opts.errorStatus)
	} else {
		c.Status(fiber.StatusOK)
	}

	if opts.template != "" {
		return r.renderTemplate(c, opts.template, opts.data)
	}

	// Parse the template
	tmpl, err := template.New("index.html").Parse(r.prepareThemeString())
	if err != nil {
		return c.SendString(staticErrorText(err))
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r.prepareViewData(c, opts)); err != nil {
		return c.SendString(staticErrorText(err))
	}

	return c.SendString(buf.String())
}

func (r *Render) ViewNotFound(c fiber.Ctx) error {
	return r.View(c,
		r.WithStatus(fiber.StatusNotFound),
		r.WithData(fiber.Map{
			"error":   true,
			"message": "Not Found",
		}))
}

func (r *Render) renderTemplate(c fiber.Ctx, template string, data any) error {
	return c.SendString(fmt.Sprintf("TemplateID: %s, Data: %v", template, data))
}
