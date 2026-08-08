package authorization

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxauth"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

// SignInBody keeps the frontend `email` field; the value may be a username or email.
type SignInBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (cc *controller) SignInAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	body := new(SignInBody)
	if err := c.Bind().JSON(body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	login := strings.TrimSpace(body.Email)
	password := body.Password
	if login == "" || password == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("email", "Email or username is required"),
			r.ErrorField("password", "Password is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	linuxName := linuxauth.LinuxLoginName(login)
	linuxOK := false
	if linuxName != "" && linuxauth.UserExists(linuxName) {
		ok, err := linuxauth.Authenticate(linuxName, password)
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		linuxOK = ok
	}

	if linuxOK {
		user, err := cc.findOrCreateLinuxUser(c, linuxName)
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		return cc.completeSignIn(c, user, "linux")
	}

	// Fall back to DB password authentication.
	user, err := gorm.G[models.User](cc.app.DB()).
		Where(models.User{Email: login}).
		Or(models.User{OrganizationEmail: login}).
		Or(models.User{Username: login}).
		First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorData(r.ErrorFields(r.ErrorField("email", "Email not found"))), r.WithStatus(fiber.StatusBadRequest))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	if !goauth.CheckPassword(user.Password, password) {
		return r.Api(c, r.WithErrorData(r.ErrorFields(r.ErrorField("password", "Invalid password"))), r.WithStatus(fiber.StatusBadRequest))
	}

	return cc.completeSignIn(c, user, "credentials")
}

func (cc *controller) completeSignIn(c fiber.Ctx, user models.User, method string) error {
	auth := cc.app.Authorization()
	ctx := c.Context()
	r := cc.app.Render()

	tokens, sessionID, err := auth.Authorize(append([]goauth.AuthorizeOptionsFunc{
		auth.WithUserID(user.ID),
		auth.WithUserRoles(user.Roles),
	}, auth.SessionMetaFromRequest(c, method)...)...)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	auth.SetSessionIDCookie(c, sessionID)

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

// findOrCreateLinuxUser loads a DB user for a verified Linux account, creating one if needed.
func (cc *controller) findOrCreateLinuxUser(c fiber.Ctx, linuxName string) (models.User, error) {
	ctx := c.Context()
	db := cc.app.DB()
	email := linuxName + "@localhost"

	user, err := gorm.G[models.User](db).
		Where(models.User{Username: linuxName}).
		Or(models.User{Email: email}).
		Or(models.User{Email: linuxName}).
		First(ctx)
	if err == nil {
		if linuxName == "root" && (user.FirstName == "" || user.LastName == "") {
			user.FirstName = "Root"
			user.LastName = "Administrator"
			if _, uerr := gorm.G[models.User](db).Where("id = ?", user.ID).Updates(ctx, models.User{
				FirstName: user.FirstName,
				LastName:  user.LastName,
			}); uerr != nil {
				return models.User{}, uerr
			}
		}
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, err
	}

	roles := goauth.JSONBArray([]any{"user"})
	firstName := ""
	lastName := ""
	if linuxName == "root" {
		roles = goauth.JSONBArray([]any{"admin"})
		firstName = "Root"
		lastName = "Administrator"
	}

	user = models.User{
		Username:    linuxName,
		FirstName:   firstName,
		LastName:    lastName,
		Email:       email,
		Password:    "!", // not a valid goauth hash — DB password login cannot impersonate
		Status:      models.Active,
		IsConfirmed: true,
		Roles:       roles,
	}
	if err := gorm.G[models.User](db).Omit("LdapUsername").Create(ctx, &user); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (cc *controller) SignInView(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	return r.View(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"title": "Sign In",
	}))
}
