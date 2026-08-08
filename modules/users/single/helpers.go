package single

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/users/single/vnc"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

func (cc *controller) loadUser(c fiber.Ctx) (*models.User, error) {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	var user models.User
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (cc *controller) respondLoadErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("ERROR"))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}

func (cc *controller) userPayload(c fiber.Ctx, u models.User) fiber.Map {
	payload := fiber.Map{
		"id":                 u.ID,
		"username":           u.Username,
		"email":              u.Email,
		"first_name":         u.FirstName,
		"last_name":          u.LastName,
		"full_name":          strings.TrimSpace(u.FirstName + " " + u.LastName),
		"organization_email": u.OrganizationEmail,
		"image":              u.Image,
		"status":             u.Status,
		"roles":              rolesSlice(u.Roles),
		"is_confirmed":       u.IsConfirmed,
		"created_at":         u.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":         u.UpdatedAt.Format("2006-01-02 15:04:05"),
		"terminal_url":       "/shell?as_user=" + url.QueryEscape(u.Username),
	}

	if u.Username != "" {
		if acc, err := linuxuser.Lookup(u.Username); err == nil && acc != nil {
			payload["linux"] = acc
		} else {
			payload["linux"] = fiber.Map{"username": u.Username, "exists": false}
		}
	} else {
		payload["linux"] = fiber.Map{"exists": false}
	}

	var session models.VncSession
	if err := cc.app.DB().WithContext(c.Context()).Where("user_id = ?", u.ID).First(&session).Error; err == nil {
		payload["vnc"] = vnc.Payload(session, u.Username)
		payload["novnc_url"] = session.ClientURL()
		payload["has_vnc"] = true
	} else {
		payload["has_vnc"] = false
	}
	return payload
}

func toRoles(roles []string) goauth.JSONBArray {
	out := make(goauth.JSONBArray, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	if out == nil {
		return goauth.JSONBArray{}
	}
	return out
}

func rolesSlice(roles goauth.JSONBArray) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		s := strings.TrimSpace(fmt.Sprint(r))
		if s != "" && s != "<nil>" {
			out = append(out, s)
		}
	}
	return out
}
