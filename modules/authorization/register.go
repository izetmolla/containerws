package authorization

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

type RegisterBody struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func (cc *controller) RegisterAPI(c fiber.Ctx) error {
	auth := cc.app.Authorization()
	ctx := c.Context()
	r := cc.app.Render()
	body := new(RegisterBody)
	if err := c.Bind().JSON(body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	userDb, err := gorm.G[models.User](cc.app.DB()).Where(models.User{Email: body.Email}).First(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	if userDb.ID != "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(r.ErrorField("email", "Email already exists"))), r.WithStatus(fiber.StatusBadRequest))
	}

	hashedPassword, err := goauth.HashPassword(body.Password)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	user := models.User{
		FirstName: strings.TrimSpace(body.FirstName),
		LastName:  strings.TrimSpace(body.LastName),
		Email:     strings.TrimSpace(body.Email),
		Password:  hashedPassword,
		Status:    models.Active,
		Roles:     goauth.JSONBArray([]any{"guest"}),
	}
	if err := gorm.G[models.User](cc.app.DB()).Omit("Username", "LdapUsername").Create(ctx, &user); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	tokens, sessionID, err := auth.Authorize(append([]goauth.AuthorizeOptionsFunc{
		auth.WithUserID(user.ID),
		auth.WithUserRoles(user.Roles),
	}, auth.SessionMetaFromRequest(c, "credentials")...)...)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	return r.Api(c,
		r.WithContext(ctx),
		r.WithStatus(fiber.StatusOK),
		r.WithData(fiber.Map{
			"tokens":     tokens,
			"session_id": sessionID,
			"user":       user,
		}),
	)
}
