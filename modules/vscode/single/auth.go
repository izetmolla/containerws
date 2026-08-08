package single

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
)

func (cc *controller) resolveCaller(c fiber.Ctx) (userID string, isAdmin bool, err error) {
	ctx := c.Context()
	auth := cc.app.Authorization()
	if auth == nil {
		return "", false, errors.New("unauthorized")
	}
	authUser, err := auth.User(c, ctx, true)
	if err != nil || authUser == nil || authUser.UserID == "" {
		return "", false, errors.New("unauthorized")
	}
	roles := cc.app.FreshUserRoles(ctx, authUser.UserID, authUser.Roles)
	return authUser.UserID, userHasAdminRole(cc.app, roles), nil
}

func (cc *controller) requireSessionAccess(c fiber.Ctx, row *models.CodeserverSession) error {
	r := cc.app.Render()
	authUserID, isAdmin, err := cc.resolveCaller(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized), r.WithErrorCode("UNAUTHORIZED"))
	}
	if isAdmin || row.UserID == authUserID {
		return nil
	}
	return r.Api(c, r.WithError(errors.New("forbidden")), r.WithStatus(fiber.StatusForbidden), r.WithErrorCode("FORBIDDEN"))
}

func userHasAdminRole(app *config.AppClients, userRoles []string) bool {
	if app == nil || len(userRoles) == 0 {
		return false
	}
	auth := app.Authorization()
	if auth == nil {
		return false
	}
	normalized := make([]string, 0, len(userRoles))
	for _, r := range userRoles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		normalized = append(normalized, r)
		if i := strings.IndexByte(r, ':'); i > 0 {
			normalized = append(normalized, r[:i])
		}
	}
	hasRole, canRead, _ := auth.GetRole([]string{"admin"}, normalized)
	if hasRole && canRead {
		return true
	}
	for _, r := range normalized {
		if strings.EqualFold(r, "admin") {
			return true
		}
	}
	return false
}
