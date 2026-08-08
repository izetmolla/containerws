package render

import (
	"github.com/gofiber/fiber/v3"
)

func (r *Render) Api(c fiber.Ctx, optsParams ...RenderOptionsFunc) error {
	var res map[string]any
	opts := r.NewRenderOptions(optsParams...)

	c.Set("Content-Type", "application/json; charset=utf-8")
	if opts.errorStatus != 0 {
		c.Status(opts.errorStatus)
	}
	if opts.data != nil {
		return c.JSON(opts.data)
	}

	if opts.err != nil || opts.errorData != nil || len(opts.errors) > 0 {
		status := opts.errorStatus
		if status == 0 {
			status = fiber.StatusInternalServerError
		}
		c.Status(status)
		code := opts.errorCode
		if code == "" {
			if opts.err != nil {
				code = "INTERNAL_SERVER_ERROR"
			} else {
				code = "VALIDATION_ERROR"
			}
		}

		message := ""
		if opts.err != nil {
			message = opts.err.Error()
		}

		res = fiber.Map{
			"error":   true,
			"message": message,
			"code":    code,
			"status":  status,
		}

		if len(opts.errors) > 0 {
			res["errors"] = opts.errors
		}

		if opts.errorData != nil {
			res["data"] = opts.errorData
		}
		return c.JSON(res)
	}

	return c.JSON(res)
}

func (r *Render) ApiNotFound(c fiber.Ctx) error {
	return r.Api(c,
		r.WithStatus(fiber.StatusNotFound),
		r.WithData(fiber.Map{
			"error":   true,
			"message": "Not Found",
			"code":    "NOT_FOUND",
			"status":  fiber.StatusNotFound,
			"details": map[string]any{
				"method": c.Method(),
				"url":    c.OriginalURL(),
			},
		}))
}
