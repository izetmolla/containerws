package authorization

import (
	"os"
	"os/user"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxauth"
)

// LocalSignInAPI issues a session for the panel host Linux user when:
//   - Settings option LOCALHOST_AUTO_LOGIN is enabled
//   - The TCP peer is loopback (127.0.0.1 / ::1) via Fiber IsFromLocal
//
// Uses the process user (user.Current), preferring SUDO_USER when the
// process runs as root under sudo.
func (cc *controller) LocalSignInAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()

	if !models.LocalhostAutoLoginEnabled(db) {
		return r.Api(c,
			r.WithError(fiber.NewError(fiber.StatusForbidden, "localhost auto-login is disabled")),
			r.WithStatus(fiber.StatusForbidden),
			r.WithErrorCode("LOCALHOST_AUTO_LOGIN_DISABLED"),
		)
	}
	if !c.IsFromLocal() {
		return r.Api(c,
			r.WithError(fiber.NewError(fiber.StatusForbidden, "localhost auto-login requires a loopback connection")),
			r.WithStatus(fiber.StatusForbidden),
			r.WithErrorCode("NOT_LOOPBACK"),
		)
	}

	linuxName, err := resolveHostLinuxUser()
	if err != nil || linuxName == "" {
		return r.Api(c,
			r.WithError(fiber.NewError(fiber.StatusInternalServerError, "could not resolve host Linux user")),
			r.WithStatus(fiber.StatusInternalServerError),
		)
	}
	if !linuxauth.UserExists(linuxName) {
		return r.Api(c,
			r.WithError(fiber.NewError(fiber.StatusBadRequest, "Linux user "+linuxName+" not found on this host")),
			r.WithStatus(fiber.StatusBadRequest),
			r.WithErrorCode("LINUX_USER_MISSING"),
		)
	}

	user, err := cc.findOrCreateLinuxUser(c, linuxName)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return cc.completeSignIn(c, user, "localhost")
}

func resolveHostLinuxUser() (string, error) {
	// Prefer the real interactive user when the panel runs under sudo.
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" {
		if linuxauth.UserExists(sudoUser) {
			return sudoUser, nil
		}
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(u.Username)
	// Strip domain prefix if present (WSL / AD-style DOMAIN\user).
	if i := strings.LastIndexAny(name, `/\`); i >= 0 && i+1 < len(name) {
		name = name[i+1:]
	}
	return name, nil
}
