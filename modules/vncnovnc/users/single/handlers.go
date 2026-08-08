package single

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/novnc"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"gorm.io/gorm"
)

type createVncSessionBody struct {
	UserID      string `json:"user_id"`
	Address     string `json:"address"`
	NoVncPort   int    `json:"no_vnc_port"`
	VncPort     int    `json:"vnc_port"`
	VncPassword string `json:"vnc_password"`
	Status      string `json:"status"`
}

type updateVncSessionBody struct {
	Address   *string `json:"address"`
	NoVncPort *int    `json:"no_vnc_port"`
	VncPort   *int    `json:"vnc_port"`
	Status    *string `json:"status"`
}

type setPasswordBody struct {
	VncPassword string `json:"vnc_password"`
}

func (cc *controller) CreateVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body createVncSessionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	userID := strings.TrimSpace(body.UserID)
	if userID == "" {
		return r.Api(c, r.WithError(errors.New("user_id is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}

	var user models.User
	if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("user not found")), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("USER_NOT_FOUND"))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	var existing models.VncSession
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&existing).Error
	if err == nil {
		return r.Api(c, r.WithError(errors.New("user already has a VNC session")), r.WithStatus(fiber.StatusConflict), r.WithErrorCode("SESSION_EXISTS"))
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = models.VncSessionStatusActive
	}
	addr := strings.TrimSpace(body.Address)
	if addr == "" {
		addr = "127.0.0.1"
	}

	// Prefer caller ports; otherwise allocate random free ports (multi-user).
	noVncPort := body.NoVncPort
	vncPort := body.VncPort
	if noVncPort <= 0 || vncPort <= 0 {
		if strings.TrimSpace(user.Username) != "" {
			asg, allocErr := adduser.AllocateOrReusePorts(user.Username, nil)
			if allocErr != nil {
				return r.Api(c, r.WithError(allocErr), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("PORT_ALLOC"))
			}
			if vncPort <= 0 {
				vncPort = asg.VncPort
			}
			if noVncPort <= 0 {
				noVncPort = asg.NoVncPort
			}
		} else {
			if noVncPort <= 0 {
				noVncPort = 6080
			}
			if vncPort <= 0 {
				vncPort = 5901
			}
		}
	}

	row := models.VncSession{
		UserID:      userID,
		Status:      status,
		Address:     addr,
		NoVncPort:   noVncPort,
		VncPort:     vncPort,
		VncPassword: body.VncPassword,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(&row)

	startMsg := "VNC session created"
	if status == models.VncSessionStatusActive &&
		strings.TrimSpace(user.Username) != "" &&
		strings.TrimSpace(body.VncPassword) != "" {
		if started, startErr := adduser.StartUserSession(adduser.StartOptions{
			Username:  user.Username,
			Password:  body.VncPassword,
			VncPort:   vncPort,
			NoVncPort: noVncPort,
		}); startErr != nil {
			startMsg = "VNC session created (desktop start failed: " + startErr.Error() + ")"
		} else if started != nil {
			row.VncPort = started.VncPort
			row.NoVncPort = started.NoVncPort
			row.Address = started.Address
			_ = db.WithContext(ctx).Model(&row).Updates(map[string]any{
				"vnc_port":    started.VncPort,
				"no_vnc_port": started.NoVncPort,
				"address":     started.Address,
			}).Error
			startMsg = "VNC session created and desktop started"
		}
	}

	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    sessionPayload(row),
		"message": startMsg,
	}))
}

func (cc *controller) GetVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": sessionPayload(*row),
	}))
}

func (cc *controller) UpdateVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}

	var body updateVncSessionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	updates := map[string]any{}
	if body.Address != nil {
		addr := strings.TrimSpace(*body.Address)
		if addr == "" {
			addr = "127.0.0.1"
		}
		updates["address"] = addr
	}
	if body.NoVncPort != nil {
		port := *body.NoVncPort
		if port <= 0 {
			port = 6080
		}
		updates["no_vnc_port"] = port
	}
	if body.VncPort != nil {
		port := *body.VncPort
		if port <= 0 {
			port = 5901
		}
		updates["vnc_port"] = port
	}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status == "" {
			status = models.VncSessionStatusActive
		}
		updates["status"] = status
	}
	if len(updates) == 0 {
		return r.Api(c, r.WithError(errors.New("no fields to update")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}

	if err := db.WithContext(ctx).Model(row).Updates(updates).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sessionPayload(*row),
		"message": "VNC session updated",
	}))
}

func (cc *controller) DeleteVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if username := strings.TrimSpace(row.User.Username); username != "" {
		_ = adduser.StopUserSession(username, row.VncPort, row.NoVncPort)
		_ = adduser.RemovePortAssignment(username)
	}
	if err := db.WithContext(ctx).Delete(row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "VNC session deleted",
		"data":    fiber.Map{"id": row.ID},
	}))
}

func (cc *controller) SetVncPasswordAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body setPasswordBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	password := strings.TrimSpace(body.VncPassword)
	if password == "" {
		return r.Api(c, r.WithError(errors.New("vnc_password is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}

	username := strings.TrimSpace(row.User.Username)
	msg := "VNC password updated"
	if username != "" {
		// Apply to this session's Linux user + ports (rewrite passwd and restart VNC/noVNC).
		if started, applyErr := adduser.ApplyUserVncPassword(username, password, row.VncPort, row.NoVncPort); applyErr != nil {
			// Still persist DB so UI/admin state matches the intended password.
			_ = db.WithContext(ctx).Model(row).Update("vnc_password", password).Error
			return r.Api(c, r.WithError(applyErr), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("PASSWORD_APPLY_FAILED"), r.WithData(fiber.Map{
				"data":    sessionPayload(*row),
				"message": "password saved to DB but live VNC apply failed",
			}))
		} else if started != nil {
			_ = db.WithContext(ctx).Model(row).Updates(map[string]any{
				"vnc_password": password,
				"address":      started.Address,
				"vnc_port":     started.VncPort,
				"no_vnc_port":  started.NoVncPort,
				"status":       models.VncSessionStatusActive,
			}).Error
			msg = "VNC password updated and applied to user desktop"
		}
	} else if err := db.WithContext(ctx).Model(row).Update("vnc_password", password).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sessionPayload(*row),
		"message": msg,
	}))
}

func (cc *controller) DisableVncSessionAPI(c fiber.Ctx) error {
	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if username := strings.TrimSpace(row.User.Username); username != "" {
		_ = adduser.StopUserSession(username, row.VncPort, row.NoVncPort)
	}
	return cc.setStatus(c, models.VncSessionStatusInactive, "VNC session disabled")
}

func (cc *controller) EnableVncSessionAPI(c fiber.Ctx) error {
	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	username := strings.TrimSpace(row.User.Username)
	password := strings.TrimSpace(row.VncPassword)
	if username != "" && password != "" {
		if started, startErr := adduser.StartUserSession(adduser.StartOptions{
			Username:  username,
			Password:  password,
			VncPort:   row.VncPort,
			NoVncPort: row.NoVncPort,
		}); startErr == nil && started != nil {
			_ = cc.app.DB().WithContext(c.Context()).Model(row).Updates(map[string]any{
				"address":     started.Address,
				"vnc_port":    started.VncPort,
				"no_vnc_port": started.NoVncPort,
			}).Error
		}
	}
	return cc.setStatus(c, models.VncSessionStatusActive, "VNC session enabled")
}

// QuickVncSessionAPI marks the session active and returns the in-app noVNC URL.
func (cc *controller) QuickVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := db.WithContext(ctx).Model(row).Update("status", models.VncSessionStatusActive).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":        sessionPayload(*row),
		"connect_url": novnc.ClientURLForSession(row.ID),
		"message":     "Session ready — opening noVNC",
	}))
}

// RestartVncSessionAPI restarts the per-user TigerVNC + noVNC processes on the session ports.
func (cc *controller) RestartVncSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	username := strings.TrimSpace(row.User.Username)
	if username == "" {
		return r.Api(c, r.WithError(errors.New("session user has no linux username")), r.WithStatus(fiber.StatusBadRequest))
	}
	password := strings.TrimSpace(row.VncPassword)
	if password == "" {
		return r.Api(c, r.WithError(errors.New("session has no vnc_password")), r.WithStatus(fiber.StatusBadRequest))
	}

	started, startErr := adduser.StartUserSession(adduser.StartOptions{
		Username:  username,
		Password:  password,
		VncPort:   row.VncPort,
		NoVncPort: row.NoVncPort,
	})
	if startErr != nil {
		return r.Api(c, r.WithError(startErr), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("RESTART_FAILED"))
	}

	_ = db.WithContext(ctx).Model(row).Updates(map[string]any{
		"status":      models.VncSessionStatusActive,
		"address":     started.Address,
		"vnc_port":    started.VncPort,
		"no_vnc_port": started.NoVncPort,
	}).Error
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sessionPayload(*row),
		"message": "VNC/noVNC restarted for user",
	}))
}

func (cc *controller) setStatus(c fiber.Ctx, status, message string) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := db.WithContext(ctx).Model(row).Update("status", status).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sessionPayload(*row),
		"message": message,
	}))
}

func (cc *controller) loadSession(c fiber.Ctx) (*models.VncSession, error) {
	ctx := c.Context()
	db := cc.app.DB()

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid session id")
	}

	var row models.VncSession
	if err := db.WithContext(ctx).Preload("User").Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "session not found")
		}
		return nil, err
	}
	return &row, nil
}

func (cc *controller) respondLoadErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("ERROR"))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}

func sessionPayload(row models.VncSession) fiber.Map {
	full := strings.TrimSpace(row.User.FirstName + " " + row.User.LastName)
	return fiber.Map{
		"id":           row.ID,
		"user_id":      row.UserID,
		"username":     row.User.Username,
		"email":        row.User.Email,
		"full_name":    full,
		"status":       row.Status,
		"address":      row.Address,
		"no_vnc_port":  row.NoVncPort,
		"vnc_port":     row.VncPort,
		"has_password": row.VncPassword != "",
		"created_at":   row.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":   row.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
