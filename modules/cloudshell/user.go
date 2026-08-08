package cloudshell

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type cliUserContext struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	ShellUser   string `json:"shell_user"`
	HomeDir     string `json:"home_dir"`
	Shell       string `json:"shell"`
	Cwd         string `json:"cwd"`
}

func (cc *controller) resolveUser(c fiber.Ctx) (*cliUserContext, error) {
	auth := cc.app.Authorization()
	data, err := auth.GetAuthDataAPI(c)
	if err != nil || data.UserID == "" {
		// Fallback to claims if AuthData is unavailable.
		claims, cerr := auth.GetClaims(c)
		if cerr != nil {
			return nil, fiber.ErrUnauthorized
		}
		uid, _ := claims["user_id"].(string)
		if uid == "" {
			return nil, fiber.ErrUnauthorized
		}
		data.UserID = uid
	}

	dbUser, err := gorm.G[models.User](cc.app.DB()).Where("id = ?", data.UserID).First(c.Context())
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
		// Prefer Linux root in workspace containers when app user has no username.
		shellUser = "root"
	}

	// Admin tooling: /shell?as_user=alice or /api/cloudshell/session?as_user=alice
	if as := strings.TrimSpace(c.Query("as_user")); as != "" {
		if u, err := user.Lookup(as); err == nil {
			shellUser = u.Username
		}
	}

	homeDir := "/root"
	shell := "/bin/bash"
	if u, err := user.Lookup(shellUser); err == nil {
		if u.HomeDir != "" {
			homeDir = u.HomeDir
		}
		if sh := lookupShell(shellUser); sh != "" {
			shell = sh
		}
	} else {
		// Unknown Linux account — stay as current process user but label as app user.
		if cu, err := user.Current(); err == nil {
			shellUser = cu.Username
			if cu.HomeDir != "" {
				homeDir = cu.HomeDir
			}
		}
	}

	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	_ = os.MkdirAll(homeDir, 0o755)

	display := strings.TrimSpace(strings.TrimSpace(dbUser.FirstName + " " + dbUser.LastName))
	if display == "" {
		display = shellUser
	}

	return &cliUserContext{
		UserID:      dbUser.ID,
		Username:    firstNonEmpty(dbUser.Username, dbUser.LdapUsername, shellUser),
		DisplayName: display,
		Email:       dbUser.Email,
		ShellUser:   shellUser,
		HomeDir:     homeDir,
		Shell:       shell,
		Cwd:         homeDir,
	}, nil
}

func (cc *controller) bindWSUser(c fiber.Ctx) error {
	ctx, err := cc.resolveUser(c)
	if err != nil {
		return err
	}
	c.Locals("cloudshell_user", ctx)
	return nil
}

func (cc *controller) GetSessionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ctx, err := cc.resolveUser(c)
	if err != nil {
		if errors.Is(err, fiber.ErrUnauthorized) {
			return r.Api(c, r.WithError(errors.New("unauthorized")), r.WithStatus(fiber.StatusUnauthorized))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	hostname, _ := os.Hostname()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"user":     ctx,
			"hostname": hostname,
			"ws_path":  "/api/cloudshell/ws",
		},
	}))
}

func lookupShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	prefix := username + ":"
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			return parts[6]
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func ensureAbsHome(home string) string {
	if home == "" {
		return "/root"
	}
	if filepath.IsAbs(home) {
		return home
	}
	return filepath.Join("/", home)
}
