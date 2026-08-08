package identity

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"gorm.io/gorm"
)

// Context is the Linux identity used for filesystem operations.
type Context struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	ShellUser   string `json:"shell_user"`
	HomeDir     string `json:"home_dir"`
	UID         uint32 `json:"uid"`
	GID         uint32 `json:"gid"`
	IsAdmin     bool   `json:"is_admin"`
	IsRootLinux bool   `json:"is_root_linux"`
}

// Resolve maps the authenticated panel user to a Linux account.
// Optional query as_user=alice lets admins act as another Linux user.
func Resolve(c fiber.Ctx, app *config.AppClients) (*Context, error) {
	if app == nil {
		return nil, fiber.ErrUnauthorized
	}
	auth := app.Authorization()
	if auth == nil {
		return nil, fiber.ErrUnauthorized
	}

	authUser, err := auth.User(c, c.Context(), true)
	if err != nil || authUser == nil || authUser.UserID == "" {
		return nil, fiber.ErrUnauthorized
	}

	roles := app.FreshUserRoles(c.Context(), authUser.UserID, authUser.Roles)
	isAdmin := hasAdminRole(app, roles)

	dbUser, err := gorm.G[models.User](app.DB()).Where("id = ?", authUser.UserID).First(c.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.ErrUnauthorized
		}
		return nil, err
	}

	shellUser := strings.TrimSpace(dbUser.Username)
	if shellUser == "" {
		shellUser = strings.TrimSpace(dbUser.LdapUsername)
	}
	if shellUser == "" {
		shellUser = "root"
	}

	if as := strings.TrimSpace(c.Query("as_user")); as != "" {
		if !isAdmin {
			return nil, fiber.NewError(fiber.StatusForbidden, "as_user requires admin")
		}
		if u, err := user.Lookup(as); err == nil {
			shellUser = u.Username
		} else {
			return nil, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("linux user %q not found", as))
		}
	}

	homeDir := "/root"
	uid := uint32(0)
	gid := uint32(0)
	isRootLinux := shellUser == "root"

	if acc, err := linuxuser.Lookup(shellUser); err == nil && acc != nil && acc.Exists {
		if acc.HomeDir != "" {
			homeDir = acc.HomeDir
		}
		if v, err := strconv.ParseUint(acc.UID, 10, 32); err == nil {
			uid = uint32(v)
		}
		if v, err := strconv.ParseUint(acc.GID, 10, 32); err == nil {
			gid = uint32(v)
		}
		isRootLinux = uid == 0
	} else if u, err := user.Lookup(shellUser); err == nil {
		if u.HomeDir != "" {
			homeDir = u.HomeDir
		}
		if v, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
			uid = uint32(v)
		}
		if v, err := strconv.ParseUint(u.Gid, 10, 32); err == nil {
			gid = uint32(v)
		}
		isRootLinux = uid == 0
	} else if cu, err := user.Current(); err == nil {
		shellUser = cu.Username
		if cu.HomeDir != "" {
			homeDir = cu.HomeDir
		}
		if v, err := strconv.ParseUint(cu.Uid, 10, 32); err == nil {
			uid = uint32(v)
		}
		if v, err := strconv.ParseUint(cu.Gid, 10, 32); err == nil {
			gid = uint32(v)
		}
		isRootLinux = uid == 0
	}

	homeDir = filepath.Clean(homeDir)
	_ = os.MkdirAll(homeDir, 0o755)

	return &Context{
		UserID:      dbUser.ID,
		Username:    firstNonEmpty(dbUser.Username, dbUser.LdapUsername, shellUser),
		ShellUser:   shellUser,
		HomeDir:     homeDir,
		UID:         uid,
		GID:         gid,
		IsAdmin:     isAdmin,
		IsRootLinux: isRootLinux,
	}, nil
}

// Run executes fn with filesystem credentials of the Linux user when the
// process is root. Otherwise fn runs as the current process user.
func (ctx *Context) Run(fn func() error) error {
	if ctx == nil {
		return fn()
	}
	return withCredentials(ctx.UID, ctx.GID, fn)
}

func hasAdminRole(app *config.AppClients, userRoles []string) bool {
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
