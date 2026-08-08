package authorization

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

type ForgotPasswordBody struct {
	Email string `json:"email"`
}

func (cc *controller) ForgotPasswordAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	body := new(ForgotPasswordBody)
	if err := c.Bind().JSON(body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	return r.Api(c, r.WithError(errors.New("not implemented")), r.WithStatus(fiber.StatusNotImplemented))
}
