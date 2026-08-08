package authorization

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

func (cc *controller) CheckApi(c fiber.Ctx) error {
	auth := cc.app.Authorization()
	ctx := c.Context()
	r := cc.app.Render()

	result, err := auth.CheckSession(c)
	if err != nil {
		switch {
		case errors.Is(err, goauth.ErrMissingRefreshToken),
			errors.Is(err, goauth.ErrInvalidRefreshToken):
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized))
		case errors.Is(err, goauth.ErrSessionNotFound),
			errors.Is(err, goauth.ErrSessionExpired):
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized))
		default:
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}

	user, err := gorm.G[models.User](cc.app.DB()).Where("id = ?", result.UserID).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(goauth.ErrSessionNotFound), r.WithStatus(fiber.StatusUnauthorized))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	return r.Api(c,
		r.WithContext(ctx),
		r.WithStatus(fiber.StatusOK),
		r.WithData(fiber.Map{
			"tokens":     result.Tokens,
			"session_id": result.SessionID,
			"user":       user,
		}),
	)
}
