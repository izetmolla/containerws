package adduser

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type startBody struct {
	Username    string `json:"username"`
	UserID      string `json:"user_id"`
	SessionID   string `json:"session_id"`
	VncPassword string `json:"vnc_password"`
	Password    string `json:"password"` // alias
	VncPort     int    `json:"vnc_port"`
	NoVncPort   int    `json:"no_vnc_port"`
	Geometry    string `json:"geometry"`
	CreateUser  bool   `json:"create_user"`
	LinuxPasswd string `json:"linux_password"`
}

type stopBody struct {
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	VncPort   int    `json:"vnc_port"`
	NoVncPort int    `json:"no_vnc_port"`
}

func (cc *controller) ListPortsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	rows, err := LoadPortMap()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"map_file": mapFilePath(),
			"ports":    rows,
		},
	}))
}

func (cc *controller) StartUserSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body startBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	username, password, session, err := cc.resolveStartTargets(c, &body)
	if err != nil {
		return cc.respondErr(c, err)
	}

	result, err := StartUserSession(StartOptions{
		Username:    username,
		Password:    password,
		VncPort:     body.VncPort,
		NoVncPort:   body.NoVncPort,
		Geometry:    body.Geometry,
		CreateUser:  body.CreateUser,
		LinuxPasswd: body.LinuxPasswd,
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("START_FAILED"))
	}

	// Persist ports onto VncSession when we have one / can create linkage.
	if session != nil {
		updates := map[string]any{
			"address":     result.Address,
			"vnc_port":    result.VncPort,
			"no_vnc_port": result.NoVncPort,
			"status":      models.VncSessionStatusActive,
		}
		if password != "" {
			updates["vnc_password"] = password
		}
		if err := db.WithContext(ctx).Model(session).Updates(updates).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		_ = db.WithContext(ctx).Preload("User").Where("id = ?", session.ID).First(session)
	} else if uid := strings.TrimSpace(body.UserID); uid != "" {
		var row models.VncSession
		err := db.WithContext(ctx).Where("user_id = ?", uid).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = models.VncSession{
				UserID:      uid,
				Status:      models.VncSessionStatusActive,
				Address:     result.Address,
				VncPort:     result.VncPort,
				NoVncPort:   result.NoVncPort,
				VncPassword: password,
			}
			if err := db.WithContext(ctx).Create(&row).Error; err != nil {
				return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
			}
			session = &row
		} else if err == nil {
			_ = db.WithContext(ctx).Model(&row).Updates(map[string]any{
				"address":      result.Address,
				"vnc_port":     result.VncPort,
				"no_vnc_port":  result.NoVncPort,
				"status":       models.VncSessionStatusActive,
				"vnc_password": password,
			}).Error
			session = &row
		}
	}

	payload := fiber.Map{
		"username":     result.Username,
		"home_dir":     result.HomeDir,
		"vnc_port":     result.VncPort,
		"no_vnc_port":  result.NoVncPort,
		"display":      result.Display,
		"address":      result.Address,
		"novnc_url":    result.NovncURL,
		"reused_ports": result.Reused,
		"connect_url":  "/novnc",
	}
	if session != nil {
		payload["session_id"] = session.ID
		payload["user_id"] = session.UserID
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    payload,
		"message": "VNC/noVNC started for user on free ports",
	}))
}

func (cc *controller) StopUserSessionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body stopBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	username := strings.TrimSpace(body.Username)
	vncPort, novncPort := body.VncPort, body.NoVncPort

	if username == "" && strings.TrimSpace(body.SessionID) != "" {
		var row models.VncSession
		if err := cc.app.DB().WithContext(c.Context()).Preload("User").Where("id = ?", body.SessionID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return r.Api(c, r.WithError(errors.New("session not found")), r.WithStatus(fiber.StatusNotFound))
			}
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		username = row.User.Username
		if vncPort <= 0 {
			vncPort = row.VncPort
		}
		if novncPort <= 0 {
			novncPort = row.NoVncPort
		}
	}
	if username == "" && strings.TrimSpace(body.UserID) != "" {
		var u models.User
		if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", body.UserID).First(&u).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound))
		}
		username = u.Username
	}
	if username == "" {
		return r.Api(c, r.WithError(errors.New("username, user_id, or session_id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	if err := StopUserSession(username, vncPort, novncPort); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("STOP_FAILED"))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"username": username,
			"stopped":  true,
		},
		"message": "VNC/noVNC stopped for user",
	}))
}

func (cc *controller) resolveStartTargets(c fiber.Ctx, body *startBody) (username, password string, session *models.VncSession, err error) {
	db := cc.app.DB()
	ctx := c.Context()
	password = strings.TrimSpace(body.VncPassword)
	if password == "" {
		password = strings.TrimSpace(body.Password)
	}
	username = strings.TrimSpace(body.Username)

	if strings.TrimSpace(body.SessionID) != "" {
		var row models.VncSession
		if e := db.WithContext(ctx).Preload("User").Where("id = ?", body.SessionID).First(&row).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return "", "", nil, fiber.NewError(fiber.StatusNotFound, "session not found")
			}
			return "", "", nil, e
		}
		session = &row
		if username == "" {
			username = row.User.Username
		}
		if password == "" {
			password = row.VncPassword
		}
		if body.VncPort <= 0 {
			body.VncPort = row.VncPort
		}
		if body.NoVncPort <= 0 {
			body.NoVncPort = row.NoVncPort
		}
	}

	if username == "" && strings.TrimSpace(body.UserID) != "" {
		var u models.User
		if e := db.WithContext(ctx).Where("id = ?", body.UserID).First(&u).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return "", "", nil, fiber.NewError(fiber.StatusNotFound, "user not found")
			}
			return "", "", nil, e
		}
		username = u.Username
	}

	if username == "" {
		return "", "", nil, fiber.NewError(fiber.StatusBadRequest, "username, user_id, or session_id is required")
	}
	if password == "" {
		return "", "", nil, fiber.NewError(fiber.StatusBadRequest, "vnc_password is required")
	}
	return username, password, session, nil
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("ERROR"))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}
