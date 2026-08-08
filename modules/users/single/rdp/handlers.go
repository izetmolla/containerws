package rdp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	rdpsetup "github.com/izetmolla/containerws/modules/vncnovnc/rdp/install/setup"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts per-user RDP routes under /users/single.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/:id/rdp", cc.GetRdpAPI)
	api.Put("/:id/rdp", cc.UpdateRdpAPI)
	api.Post("/:id/rdp/enable", cc.EnableRdpAPI)
	api.Post("/:id/rdp/disable", cc.DisableRdpAPI)
	api.Post("/:id/rdp/start", cc.StartRdpAPI)
	api.Post("/:id/rdp/stop", cc.StopRdpAPI)
}

type updateRdpBody struct {
	Address *string `json:"rdp_address"`
}

type enableRdpBody struct {
	Address string `json:"rdp_address"`
}

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
			return nil, fiber.NewError(fiber.StatusNotFound, "no vnc profile — create a desktop profile before enabling RDP")
		}
		return nil, err
	}
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

func (cc *controller) payload(user *models.User, session *models.VncSession, host rdpsetup.StatusReport) fiber.Map {
	enabled := false
	addr := adduser.BindHost
	port := 0
	hasProfile := session != nil
	if session != nil {
		enabled = session.RdpEnabled
		if a := strings.TrimSpace(session.RDPAddress); a != "" {
			addr = adduser.NormalizeBindAddress(a)
		}
		port = session.RDPPort
	}
	return fiber.Map{
		"enabled":         enabled,
		"packages_ready":  host.Ready,
		"service_running": host.Running,
		"rdp_address":     addr,
		"rdp_port":        port,
		"port":            port, // backwards compatible
		"addresses":       adduser.ListBindAddresses(),
		"missing":         host.Missing,
		"plan":            host.Plan,
		"username":        user.Username,
		"has_profile":     hasProfile,
		"connect_hint":    connectHint(host.Ready, addr, port, user.Username),
	}
}

func (cc *controller) GetRdpAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	host := rdpsetup.CheckStatus()
	var session *models.VncSession
	var row models.VncSession
	if err := cc.app.DB().WithContext(c.Context()).Where("user_id = ?", user.ID).First(&row).Error; err == nil {
		session = &row
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cc.payload(user, session, host),
	}))
}

func (cc *controller) UpdateRdpAPI(c fiber.Ctx) error {
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
	var body updateRdpBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	if body.Address == nil {
		return r.Api(c, r.WithError(errors.New("rdp_address is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	addr := adduser.NormalizeBindAddress(*body.Address)
	if !adduser.IsAddressAllowed(addr) {
		return r.Api(c, r.WithError(errors.New("address is not an available host interface")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	_ = db.WithContext(ctx).Model(session).Update("rdp_address", addr).Error
	session.RDPAddress = addr
	host := rdpsetup.CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.payload(user, session, host),
		"message": "RDP listen address updated",
	}))
}

func (cc *controller) EnableRdpAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	host := rdpsetup.CheckStatus()
	if !host.Ready {
		return r.Api(c, r.WithError(errors.New("RDP packages are not installed")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("RDP_NOT_INSTALLED"))
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if strings.TrimSpace(user.Username) == "" {
		return r.Api(c, r.WithError(errors.New("user needs a linux username for RDP")), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(session.VncPassword) == "" {
		return r.Api(c, r.WithError(errors.New("set a VNC password before enabling RDP (used on the RDP login dialog)")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body enableRdpBody
	_ = c.Bind().Body(&body)

	addr := strings.TrimSpace(body.Address)
	if addr == "" {
		addr = session.RDPAddress
	}
	if addr == "" {
		addr = adduser.BindHost
	}
	addr = adduser.NormalizeBindAddress(addr)
	if !adduser.IsAddressAllowed(addr) {
		return r.Api(c, r.WithError(errors.New("address is not an available host interface")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}

	port, err := allocateRdpPort(db, session, user.Username)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	session.ApplyDefaults()
	if err := adduser.ApplyUserDesktopSession(user.Username, session.DesktopSession, session.WallpaperPath); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("prepare desktop session for RDP: %w", err)), r.WithStatus(fiber.StatusInternalServerError))
	}

	// Ensure TigerVNC is running so Xvnc/libvnc can attach to the same desktop.
	if w := ensureVncRunning(db, user, session); w != "" {
		return r.Api(c, r.WithError(fmt.Errorf("start desktop for RDP: %s", w)), r.WithStatus(fiber.StatusInternalServerError))
	}

	_ = db.WithContext(ctx).Model(session).Updates(map[string]any{
		"rdp_enabled": true,
		"rdp_address": addr,
		"rdp_port":    port,
	}).Error
	session.RdpEnabled = true
	session.RDPAddress = addr
	session.RDPPort = port

	if err := syncXrdpXvnc(db); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("configure xrdp Xvnc: %w", err)), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := rdpsetup.RestartXrdp(); err != nil {
		_ = db.WithContext(ctx).Model(session).Update("rdp_enabled", false).Error
		session.RdpEnabled = false
		return r.Api(c, r.WithError(fmt.Errorf("start xrdp service: %w", err)), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("XRDP_START_FAILED"))
	}

	host = rdpsetup.CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.payload(user, session, host),
		"message": "RDP enabled — Xvnc opens your desktop with the VNC password from the login dialog",
	}))
}

func (cc *controller) DisableRdpAPI(c fiber.Ctx) error {
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
	_ = db.WithContext(ctx).Model(session).Update("rdp_enabled", false).Error
	session.RdpEnabled = false
	_ = syncXrdpXvnc(db)
	_ = rdpsetup.RestartXrdp()
	host := rdpsetup.CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.payload(user, session, host),
		"message": "RDP disabled for this user",
	}))
}

func (cc *controller) StartRdpAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	host := rdpsetup.CheckStatus()
	if !host.Ready {
		return r.Api(c, r.WithError(errors.New("RDP packages are not installed")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("RDP_NOT_INSTALLED"))
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if !session.RdpEnabled {
		return r.Api(c, r.WithError(errors.New("enable RDP for this user before starting the service")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("RDP_NOT_ENABLED"))
	}
	if w := ensureVncRunning(db, user, session); w != "" {
		return r.Api(c, r.WithError(fmt.Errorf("start desktop for RDP: %s", w)), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := syncXrdpXvnc(db); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("configure xrdp Xvnc: %w", err)), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := rdpsetup.StartXrdp(); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("XRDP_START_FAILED"))
	}
	host = rdpsetup.CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.payload(user, session, host),
		"message": "RDP service started",
	}))
}

func (cc *controller) StopRdpAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if !session.RdpEnabled {
		return r.Api(c, r.WithError(errors.New("RDP is not enabled for this user")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("RDP_NOT_ENABLED"))
	}
	if err := rdpsetup.StopXrdp(); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("XRDP_STOP_FAILED"))
	}
	host := rdpsetup.CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.payload(user, session, host),
		"message": "RDP service stopped",
	}))
}

// SyncXrdpXvnc points xrdp at RDP-enabled users' TigerVNC ports (libvnc / Xvnc).
func SyncXrdpXvnc(db *gorm.DB) error {
	return syncXrdpXvnc(db)
}

func syncXrdpXvnc(db *gorm.DB) error {
	type row struct {
		Username   string
		VncPort    int
		Address    string
		RDPAddress string
		RDPPort    int
	}
	var rows []row
	err := db.Table("vnc_sessions AS s").
		Select("u.username AS username, s.vnc_port AS vnc_port, s.address AS address, s.rdp_address AS rdp_address, s.rdp_port AS rdp_port").
		Joins("JOIN users u ON u.id = s.user_id").
		Where("s.rdp_enabled = ? AND s.vnc_port > 0 AND u.username <> ''", true).
		Find(&rows).Error
	if err != nil {
		return err
	}
	targets := make([]rdpsetup.XvncDesktopTarget, 0, len(rows))
	for _, r := range rows {
		addr := adduser.NormalizeBindAddress(r.Address)
		if adduser.IsLoopbackBind(addr) || addr == "" {
			addr = adduser.BindHost
		}
		listen := adduser.NormalizeBindAddress(r.RDPAddress)
		if listen == "" {
			listen = adduser.BindHost
		}
		port := r.RDPPort
		if port <= 0 {
			if isRootUsername(r.Username) {
				port = preferredRdpPort
			} else {
				continue
			}
		}
		targets = append(targets, rdpsetup.XvncDesktopTarget{
			Username:   r.Username,
			VncPort:    r.VncPort,
			Address:    addr,
			RdpAddress: listen,
			RdpPort:    port,
		})
	}
	return rdpsetup.EnsureXrdpXvncConfig(targets...)
}

func ensureVncRunning(db *gorm.DB, user *models.User, session *models.VncSession) string {
	session.ApplyDefaults()
	bind := adduser.NormalizeBindAddress(session.Address)
	localhostOnly := adduser.IsLoopbackBind(bind)

	// Reuse a live desktop — force-restarting breaks Enable when TigerVNC is
	// already up (common when the user is already in noVNC).
	if session.VncPort > 0 &&
		adduser.IsSessionLiveOn(bind, session.VncPort, session.NoVncPort) {
		_ = db.Model(session).Update("status", models.VncSessionStatusActive).Error
		session.Status = models.VncSessionStatusActive
		return ""
	}

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
		AlwaysShared:         true, // RDP + noVNC may connect together
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

// preferredRdpPort is reserved for the root Linux user.
const preferredRdpPort = 3389

func isRootUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), "root")
}

// allocateRdpPort reuses this session's port when still unique, otherwise
// prefers 3389 for root (reclaiming it from other users if needed). Non-root
// users never receive 3389.
func allocateRdpPort(db *gorm.DB, session *models.VncSession, username string) (int, error) {
	used := map[int]struct{}{}
	var rows []models.VncSession
	_ = db.Select("id", "rdp_port", "vnc_port", "no_vnc_port").Find(&rows).Error
	for _, row := range rows {
		if row.ID == session.ID {
			continue
		}
		if row.RDPPort > 0 {
			used[row.RDPPort] = struct{}{}
		}
		if row.VncPort > 0 {
			used[row.VncPort] = struct{}{}
		}
		if row.NoVncPort > 0 {
			used[row.NoVncPort] = struct{}{}
		}
	}

	root := isRootUsername(username)

	if root {
		if err := reclaimPreferredRdpPort(db, session, used); err != nil {
			return 0, err
		}
		delete(used, preferredRdpPort)
		return preferredRdpPort, nil
	}

	// Non-root: never take the root-reserved port.
	used[preferredRdpPort] = struct{}{}

	if session.RDPPort > 0 && session.RDPPort != preferredRdpPort {
		if _, taken := used[session.RDPPort]; !taken {
			return session.RDPPort, nil
		}
	}

	port, err := adduser.PickUnusedLocalPort(used)
	if err != nil {
		return 0, fmt.Errorf("allocate rdp port: %w", err)
	}
	return port, nil
}

// reclaimPreferredRdpPort moves any other session off 3389 onto a free port.
func reclaimPreferredRdpPort(db *gorm.DB, self *models.VncSession, used map[int]struct{}) error {
	var holders []models.VncSession
	err := db.Where("rdp_port = ? AND id <> ?", preferredRdpPort, self.ID).Find(&holders).Error
	if err != nil {
		return fmt.Errorf("find sessions on %d: %w", preferredRdpPort, err)
	}
	for i := range holders {
		holder := &holders[i]
		// Keep 3389 marked used so the alternate pick cannot collide.
		used[preferredRdpPort] = struct{}{}
		alt, err := adduser.PickUnusedLocalPort(used)
		if err != nil {
			return fmt.Errorf("reassign rdp port from %d: %w", preferredRdpPort, err)
		}
		if err := db.Model(holder).Update("rdp_port", alt).Error; err != nil {
			return fmt.Errorf("move session off %d: %w", preferredRdpPort, err)
		}
		holder.RDPPort = alt
		used[alt] = struct{}{}
		delete(used, preferredRdpPort)
	}
	return nil
}

func connectHint(ready bool, addr string, port int, username string) string {
	if !ready {
		return "Install RDP packages first"
	}
	user := strings.TrimSpace(username)
	if user == "" {
		user = "<username>"
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = "127.0.0.1"
	}
	if port <= 0 {
		return "Enable RDP to allocate a listen port for this user"
	}
	return addr + ":" + strconv.Itoa(port) + " · " + user + " · VNC password"
}
