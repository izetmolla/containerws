package novnc

import (
	"errors"
	"fmt"
	"os/user"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/setup"
	"gorm.io/gorm"
)

var (
	errUnauthorized     = errors.New("unauthorized")
	errNoSession        = errors.New("vnc session missing")
	errPackagesNotReady = errors.New("vnc packages not installed")
	errNoProfile        = errors.New("vnc profile missing")
	errDesktopStopped   = errors.New("vnc desktop not running")
)

// sessionGateError carries a user-facing explanation for HTML error pages.
type sessionGateError struct {
	kind        error
	title       string
	message     string
	actionURL   string
	actionLabel string
}

func (e *sessionGateError) Error() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	if e.kind != nil {
		return e.kind.Error()
	}
	return "vnc unavailable"
}

func (e *sessionGateError) Unwrap() error { return e.kind }

func gatePackages() error {
	status := setup.CheckStatus()
	if status.Ready {
		return nil
	}
	missing := strings.Join(status.Missing, ", ")
	if missing == "" {
		missing = "required VNC/noVNC components"
	}
	return &sessionGateError{
		kind:        errPackagesNotReady,
		title:       "VNC packages not installed",
		message:     "Install TigerVNC / noVNC on this host before opening a desktop. Missing: " + missing + ".",
		actionURL:   "/vnc-novnc",
		actionLabel: "Open VNC setup",
	}
}

func gateProfile(session *models.VncSession) (username string, err error) {
	if session == nil {
		return "", &sessionGateError{
			kind:        errNoProfile,
			title:       "VNC profile missing",
			message:     "No VNC session profile exists for this user. Create one from the user VNC tab.",
			actionURL:   "/users",
			actionLabel: "Open users",
		}
	}
	if !strings.EqualFold(strings.TrimSpace(session.Status), models.VncSessionStatusActive) {
		return "", &sessionGateError{
			kind:        errNoProfile,
			title:       "VNC profile inactive",
			message:     "This VNC profile is disabled. Enable the session from the user profile, then open noVNC again.",
			actionURL:   "/users",
			actionLabel: "Open users",
		}
	}
	username = strings.TrimSpace(session.User.Username)
	if username == "" {
		return "", &sessionGateError{
			kind:        errNoProfile,
			title:       "Linux user missing",
			message:     "This VNC profile is not linked to a Linux username. Fix the panel user username, then recreate or restart the session.",
			actionURL:   "/users",
			actionLabel: "Open users",
		}
	}
	if _, lookupErr := user.Lookup(username); lookupErr != nil {
		return "", &sessionGateError{
			kind:        errNoProfile,
			title:       "Linux account missing",
			message:     fmt.Sprintf("Linux user %q does not exist on this host. Provision the account before opening the desktop.", username),
			actionURL:   "/users",
			actionLabel: "Open users",
		}
	}
	if strings.TrimSpace(session.VncPassword) == "" {
		return "", &sessionGateError{
			kind:        errNoProfile,
			title:       "VNC password not set",
			message:     "This profile has no VNC password stored. Set a password on the user VNC tab, then open noVNC again.",
			actionURL:   "/users",
			actionLabel: "Open users",
		}
	}
	return username, nil
}

// ensureDesktopRunning verifies the per-user TigerVNC/noVNC listeners are up.
// If the profile is complete but stopped, it starts the desktop once.
func ensureDesktopRunning(db *gorm.DB, session *models.VncSession, username string) error {
	if adduser.IsSessionLive(username, session.VncPort, session.NoVncPort) {
		return nil
	}

	started, startErr := adduser.StartUserSession(adduser.StartOptions{
		Username:  username,
		Password:  strings.TrimSpace(session.VncPassword),
		VncPort:   session.VncPort,
		NoVncPort: session.NoVncPort,
	})
	if startErr != nil {
		return &sessionGateError{
			kind:        errDesktopStopped,
			title:       "VNC desktop is not running",
			message:     "The profile exists, but the desktop failed to start: " + startErr.Error(),
			actionURL:   "/vnc-novnc",
			actionLabel: "Check VNC service",
		}
	}
	if started != nil && db != nil {
		_ = db.Model(session).Updates(map[string]any{
			"address":     started.Address,
			"vnc_port":    started.VncPort,
			"no_vnc_port": started.NoVncPort,
			"status":      models.VncSessionStatusActive,
		}).Error
		session.Address = started.Address
		session.VncPort = started.VncPort
		session.NoVncPort = started.NoVncPort
	}

	if !adduser.IsSessionLive(username, session.VncPort, session.NoVncPort) {
		return &sessionGateError{
			kind:        errDesktopStopped,
			title:       "VNC desktop is not running",
			message:     "Start was requested, but the VNC/noVNC ports are still not listening. Check service logs, then try again.",
			actionURL:   "/vnc-novnc/logs",
			actionLabel: "View VNC logs",
		}
	}
	return nil
}

func (cc *Controller) requireActiveSession(ctx fiber.Ctx) (*models.VncSession, error) {
	if err := gatePackages(); err != nil {
		return nil, err
	}

	user, err := cc.resolvePanelUser(ctx)
	if err != nil || user == nil || user.UserID == "" {
		return nil, errUnauthorized
	}

	db := cc.app.DB()
	if db == nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "database unavailable")
	}

	sessionID := resolveSessionID(ctx)
	var session models.VncSession

	q := db.WithContext(ctx.Context()).Preload("User")
	if sessionID != "" {
		err = q.Where("id = ?", sessionID).First(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &sessionGateError{
				kind:        errNoSession,
				title:       "VNC profile missing",
				message:     "No VNC session profile matches this link. Create one from the user VNC tab first.",
				actionURL:   "/users",
				actionLabel: "Open users",
			}
		}
		if err != nil {
			return nil, fmt.Errorf("load vnc session: %w", err)
		}
		// Owner always; other authenticated panel users may open by session_id (admin tooling).
	} else {
		err = q.Where("user_id = ?", user.UserID).First(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &sessionGateError{
				kind:        errNoSession,
				title:       "VNC profile missing",
				message:     "You do not have a VNC session profile yet. Create one from your user VNC tab to open the remote desktop.",
				actionURL:   "/users",
				actionLabel: "Open users",
			}
		}
		if err != nil {
			return nil, fmt.Errorf("load vnc session: %w", err)
		}
	}

	username, err := gateProfile(&session)
	if err != nil {
		return nil, err
	}
	if err := ensureDesktopRunning(db.WithContext(ctx.Context()), &session, username); err != nil {
		return nil, err
	}
	return &session, nil
}
