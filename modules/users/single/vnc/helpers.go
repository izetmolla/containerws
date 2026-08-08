package vnc

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
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

func (cc *controller) loadSession(c fiber.Ctx, userID string) (*models.VncSession, error) {
	var session models.VncSession
	if err := cc.app.DB().WithContext(c.Context()).Where("user_id = ?", userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "no vnc profile")
		}
		return nil, err
	}
	session.ApplyDefaults()
	return &session, nil
}

func (cc *controller) respondLoadErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("ERROR"))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}

// Payload is the JSON shape for a VNC profile including display/client options.
func Payload(s models.VncSession, username string) fiber.Map {
	s.ApplyDefaults()
	_ = strings.TrimSpace(username)
	live := adduser.IsSessionLiveOn(s.Address, s.VncPort, s.NoVncPort)
	addr := strings.TrimSpace(s.RDPAddress)
	if addr == "" {
		addr = "127.0.0.1"
	}
	return fiber.Map{
		"id":                      s.ID,
		"user_id":                 s.UserID,
		"status":                  s.Status,
		"address":                 s.Address,
		"vnc_port":                s.VncPort,
		"no_vnc_port":             s.NoVncPort,
		"has_password":            s.VncPassword != "",
		"live":                    live,
		"novnc_url":               s.ClientURL(),
		"geometry":                s.Geometry,
		"depth":                   s.Depth,
		"dpi":                     s.Dpi,
		"framerate":               s.Framerate,
		"localhost_only":          s.LocalhostOnly,
		"always_shared":           s.AlwaysShared,
		"accept_set_desktop_size": s.AcceptSetDesktopSize,
		"security_types":          s.SecurityTypes,
		"compare_fb":              s.CompareFB,
		"improved_hextile":        s.ImprovedHextile,
		"desktop_session":         s.DesktopSession,
		"quality":                 s.Quality,
		"compression":             s.Compression,
		"autoconnect":             s.Autoconnect,
		"reconnect":               s.Reconnect,
		"reconnect_delay":         s.ReconnectDelay,
		"resize":                  s.Resize,
		"view_only":               s.ViewOnly,
		"show_dot":                s.ShowDot,
		"view_clip":               s.ViewClip,
		"shared":                  s.Shared,
		"bell":                    s.Bell,
		"logging":                 s.Logging,
		"wallpaper_path":          s.WallpaperPath,
		"has_wallpaper":           strings.TrimSpace(s.WallpaperPath) != "",
		"wallpaper_url":           wallpaperURL(s.UserID),
		"rdp_enabled":             s.RdpEnabled,
		"rdp_address":             addr,
		"rdp_port":                s.RDPPort,
		"created_at":              s.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":              s.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
