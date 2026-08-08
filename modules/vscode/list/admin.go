package list

import (
	"strings"

	"github.com/izetmolla/containerws/config"
)

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
