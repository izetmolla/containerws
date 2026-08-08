package vnc

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/users/single/rdp"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	rdpsetup "github.com/izetmolla/containerws/modules/vncnovnc/rdp/install/setup"
	"gorm.io/gorm"
)

type createBody struct {
	VncPassword string `json:"vnc_password"`
	Start       bool   `json:"start"`
}

type passwordBody struct {
	VncPassword string `json:"vnc_password"`
}

type updateBody struct {
	Status               *string `json:"status"`
	Address              *string `json:"address"`
	Geometry             *string `json:"geometry"`
	Depth                *int    `json:"depth"`
	Dpi                  *int    `json:"dpi"`
	Framerate            *int    `json:"framerate"`
	LocalhostOnly        *bool   `json:"localhost_only"`
	AlwaysShared         *bool   `json:"always_shared"`
	AcceptSetDesktopSize *bool   `json:"accept_set_desktop_size"`
	SecurityTypes        *string `json:"security_types"`
	CompareFB            *int    `json:"compare_fb"`
	ImprovedHextile      *bool   `json:"improved_hextile"`
	DesktopSession       *string `json:"desktop_session"`
	Quality              *int    `json:"quality"`
	Compression          *int    `json:"compression"`
	Autoconnect          *bool   `json:"autoconnect"`
	Reconnect            *bool   `json:"reconnect"`
	ReconnectDelay       *int    `json:"reconnect_delay"`
	Resize               *string `json:"resize"`
	ViewOnly             *bool   `json:"view_only"`
	ShowDot              *bool   `json:"show_dot"`
	ViewClip             *bool   `json:"view_clip"`
	Shared               *bool   `json:"shared"`
	Bell                 *string `json:"bell"`
	Logging              *string `json:"logging"`
	Restart              bool    `json:"restart"`
}

func (cc *controller) ListVncAddressesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	if _, err := cc.loadUser(c); err != nil {
		return cc.respondLoadErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": adduser.ListBindAddresses(),
	}))
}

func (cc *controller) GetVncProfileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": Payload(*session, user.Username),
	}))
}

func (cc *controller) CreateVncProfileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body createBody
	_ = c.Bind().Body(&body)
	pass := strings.TrimSpace(body.VncPassword)
	if pass == "" {
		return r.Api(c, r.WithError(errors.New("vnc_password is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	if len(pass) > 8 {
		pass = pass[:8]
	}
	session, warning := EnsureSession(cc.app.DB(), user, pass, body.Start)
	if session == nil {
		return r.Api(c, r.WithError(errors.New(warning)), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":      Payload(*session, user.Username),
		"novnc_url": session.ClientURL(),
		"message":   "VNC profile ready",
		"warning":   warning,
	}))
}

func (cc *controller) UpdateVncProfileAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}

	var body updateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	updates := map[string]any{}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status == "" {
			status = models.VncSessionStatusActive
		}
		updates["status"] = status
	}
	if body.Address != nil {
		addr := adduser.NormalizeBindAddress(*body.Address)
		if !adduser.IsAddressAllowed(addr) {
			return r.Api(c, r.WithError(errors.New("address is not an available host interface")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
		}
		updates["address"] = addr
		updates["localhost_only"] = adduser.IsLoopbackBind(addr)
	} else if body.LocalhostOnly != nil {
		updates["localhost_only"] = *body.LocalhostOnly
		if *body.LocalhostOnly {
			updates["address"] = adduser.BindHost
		}
	}
	if body.Geometry != nil {
		g := strings.TrimSpace(*body.Geometry)
		if g == "" {
			g = models.VncDefaultGeometry
		}
		updates["geometry"] = g
	}
	if body.Depth != nil {
		d := *body.Depth
		if d <= 0 {
			d = models.VncDefaultDepth
		}
		updates["depth"] = d
	}
	if body.Dpi != nil {
		d := *body.Dpi
		if d <= 0 {
			d = models.VncDefaultDPI
		}
		updates["dpi"] = d
	}
	if body.Framerate != nil {
		f := *body.Framerate
		if f <= 0 {
			f = models.VncDefaultFramerate
		}
		updates["framerate"] = f
	}
	if body.AlwaysShared != nil {
		updates["always_shared"] = *body.AlwaysShared
	}
	if body.AcceptSetDesktopSize != nil {
		updates["accept_set_desktop_size"] = *body.AcceptSetDesktopSize
	}
	if body.SecurityTypes != nil {
		st := strings.TrimSpace(*body.SecurityTypes)
		if st == "" {
			st = models.VncDefaultSecurityTypes
		}
		updates["security_types"] = st
	}
	if body.CompareFB != nil {
		updates["compare_fb"] = *body.CompareFB
	}
	if body.ImprovedHextile != nil {
		updates["improved_hextile"] = *body.ImprovedHextile
	}
	if body.DesktopSession != nil {
		ds := strings.ToLower(strings.TrimSpace(*body.DesktopSession))
		if ds == "" {
			ds = models.VncDefaultDesktop
		}
		updates["desktop_session"] = ds
	}
	if body.Quality != nil {
		q := min(max(*body.Quality, 0), 9)
		updates["quality"] = q
	}
	if body.Compression != nil {
		comp := min(max(*body.Compression, 0), 9)
		updates["compression"] = comp
	}
	if body.Autoconnect != nil {
		updates["autoconnect"] = *body.Autoconnect
	}
	if body.Reconnect != nil {
		updates["reconnect"] = *body.Reconnect
	}
	if body.ReconnectDelay != nil {
		d := max(*body.ReconnectDelay, 0)
		updates["reconnect_delay"] = d
	}
	if body.Resize != nil {
		resize := strings.ToLower(strings.TrimSpace(*body.Resize))
		switch resize {
		case "off", "scale", "remote":
			updates["resize"] = resize
		default:
			updates["resize"] = models.VncDefaultResize
		}
	}
	if body.ViewOnly != nil {
		updates["view_only"] = *body.ViewOnly
	}
	if body.ShowDot != nil {
		updates["show_dot"] = *body.ShowDot
	}
	if body.ViewClip != nil {
		updates["view_clip"] = *body.ViewClip
	}
	if body.Shared != nil {
		updates["shared"] = *body.Shared
	}
	if body.Bell != nil {
		bell := strings.ToLower(strings.TrimSpace(*body.Bell))
		if bell != "on" && bell != "off" {
			bell = "on"
		}
		updates["bell"] = bell
	}
	if body.Logging != nil {
		logging := strings.ToLower(strings.TrimSpace(*body.Logging))
		if logging == "" {
			logging = "warn"
		}
		updates["logging"] = logging
	}

	if len(updates) == 0 && !body.Restart {
		return r.Api(c, r.WithError(errors.New("no fields to update")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	if len(updates) > 0 {
		if err := db.WithContext(ctx).Model(session).Updates(updates).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	_ = db.WithContext(ctx).Where("id = ?", session.ID).First(session).Error

	// Keep ~/.xsession in sync with desktop_session so RDP uses the same desktop.
	if user.Username != "" && (body.DesktopSession != nil || body.Restart || len(updates) > 0) {
		session.ApplyDefaults()
		_ = adduser.ApplyUserDesktopSession(user.Username, session.DesktopSession, session.WallpaperPath)
	}

	warning := ""
	msg := "VNC settings saved"
	// Always restart the desktop after settings save so geometry/desktop/env apply.
	restart := body.Restart || len(updates) > 0
	if restart {
		if w := startSession(db, user, session); w != "" {
			warning = w
			msg = "VNC settings saved (restart: " + w + ")"
		} else {
			msg = "VNC settings saved and desktop restarted"
			_ = db.WithContext(ctx).Where("id = ?", session.ID).First(session).Error
			if session.RdpEnabled {
				if err := rdp.SyncXrdpXvnc(db); err == nil {
					if err := rdpsetup.RestartXrdp(); err != nil {
						if warning == "" {
							warning = "RDP service restart: " + err.Error()
						}
					} else {
						msg = "VNC settings saved; desktop and RDP restarted"
					}
				}
			}
		}
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":      Payload(*session, user.Username),
		"novnc_url": session.ClientURL(),
		"message":   msg,
		"warning":   warning,
	}))
}

func (cc *controller) SetVncPasswordAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body passwordBody
	_ = c.Bind().Body(&body)
	pass := strings.TrimSpace(body.VncPassword)
	if pass == "" {
		return r.Api(c, r.WithError(errors.New("vnc_password is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	if len(pass) > 8 {
		pass = pass[:8]
	}
	if err := db.WithContext(ctx).Model(session).Update("vnc_password", pass).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	session.VncPassword = pass
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    Payload(*session, user.Username),
		"message": "VNC password updated",
	}))
}

func (cc *controller) StartVncProfileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if user.Username == "" || session.VncPassword == "" {
		return r.Api(c,
			r.WithError(errors.New("set a VNC password before starting the desktop session")),
			r.WithStatus(fiber.StatusBadRequest),
			r.WithErrorCode("VNC_PASSWORD_REQUIRED"),
		)
	}
	if w := startSession(db, user, session); w != "" {
		return r.Api(c, r.WithError(errors.New(w)), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(c.Context()).Where("id = ?", session.ID).First(session).Error
	if session.RdpEnabled {
		if err := rdp.SyncXrdpXvnc(db); err == nil {
			_ = rdpsetup.RestartXrdp()
		}
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":      Payload(*session, user.Username),
		"novnc_url": session.ClientURL(),
		"message":   "VNC started",
	}))
}

func (cc *controller) StopVncProfileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if user.Username != "" {
		_ = adduser.StopUserSession(user.Username, session.VncPort, session.NoVncPort)
	}
	_ = db.WithContext(c.Context()).Where("id = ?", session.ID).First(session).Error
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    Payload(*session, user.Username),
		"message": "VNC stopped",
	}))
}

func startSession(db *gorm.DB, user *models.User, session *models.VncSession) string {
	session.ApplyDefaults()
	bind := adduser.NormalizeBindAddress(session.Address)
	localhostOnly := adduser.IsLoopbackBind(bind)
	started, err := adduser.StartUserSession(adduser.StartOptions{
		Username:             user.Username,
		Password:             session.VncPassword,
		VncPort:              session.VncPort,
		NoVncPort:            session.NoVncPort,
		Geometry:             session.GeometryOrDefault(),
		Depth:                strconv.Itoa(session.Depth),
		DPI:                  strconv.Itoa(session.Dpi),
		Framerate:            strconv.Itoa(session.Framerate),
		BindAddress:          bind,
		ServerFromProfile:    true,
		LocalhostOnly:        localhostOnly,
		AlwaysShared:         session.AlwaysShared,
		AcceptSetDesktopSize: session.AcceptSetDesktopSize,
		SecurityTypes:        session.SecurityTypes,
		CompareFB:            session.CompareFB,
		ImprovedHextile:      session.ImprovedHextile,
		DesktopSession:       session.DesktopSession,
		WallpaperPath:        session.WallpaperPath,
	})
	if err != nil {
		return err.Error()
	}
	if started == nil {
		return "start returned empty result"
	}
	_ = db.Model(session).Updates(map[string]any{
		"address":        started.Address,
		"localhost_only": localhostOnly,
		"vnc_port":       started.VncPort,
		"no_vnc_port":    started.NoVncPort,
		"status":         models.VncSessionStatusActive,
	}).Error
	session.Address = started.Address
	session.LocalhostOnly = localhostOnly
	session.VncPort = started.VncPort
	session.NoVncPort = started.NoVncPort
	session.Status = models.VncSessionStatusActive
	return ""
}

// EnsureSession creates or updates a VNC profile for the user (used by user create).
func EnsureSession(db *gorm.DB, user *models.User, vncPass string, start bool) (*models.VncSession, string) {
	var existing models.VncSession
	err := db.Where("user_id = ?", user.ID).First(&existing).Error
	if err == nil {
		_ = db.Model(&existing).Update("vnc_password", vncPass).Error
		existing.VncPassword = vncPass
		if start && user.Username != "" {
			if w := startSession(db, user, &existing); w != "" {
				return &existing, "vnc start: " + w
			}
			_ = db.First(&existing, "id = ?", existing.ID)
		}
		return &existing, ""
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err.Error()
	}

	vncPort, noVncPort := 0, 0
	if user.Username != "" {
		if asg, aerr := adduser.AllocateOrReusePorts(user.Username, nil); aerr == nil {
			vncPort, noVncPort = asg.VncPort, asg.NoVncPort
		}
	}
	session := models.VncSession{
		UserID:               user.ID,
		Status:               models.VncSessionStatusActive,
		Address:              "127.0.0.1",
		VncPort:              vncPort,
		NoVncPort:            noVncPort,
		VncPassword:          vncPass,
		Geometry:             models.VncDefaultGeometry,
		Depth:                models.VncDefaultDepth,
		Dpi:                  models.VncDefaultDPI,
		Framerate:            models.VncDefaultFramerate,
		LocalhostOnly:        true,
		AlwaysShared:         true,
		AcceptSetDesktopSize: true,
		SecurityTypes:        models.VncDefaultSecurityTypes,
		CompareFB:            0,
		ImprovedHextile:      true,
		DesktopSession:       models.VncDefaultDesktop,
		Quality:              models.VncDefaultQuality,
		Compression:          models.VncDefaultCompression,
		Autoconnect:          true,
		Reconnect:            true,
		ReconnectDelay:       models.VncDefaultReconnectDelay,
		Resize:               models.VncDefaultResize,
		ViewOnly:             false,
		ShowDot:              false,
		ViewClip:             false,
		Shared:               true,
		Bell:                 "on",
		Logging:              "warn",
	}
	if err := db.Create(&session).Error; err != nil {
		return nil, err.Error()
	}
	warning := ""
	if start && user.Username != "" {
		if w := startSession(db, user, &session); w != "" {
			warning = "vnc start: " + w
		} else {
			_ = db.First(&session, "id = ?", session.ID)
		}
	}
	return &session, warning
}
