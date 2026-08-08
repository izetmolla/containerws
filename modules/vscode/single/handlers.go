package single

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"gorm.io/gorm"
)

type createCodeserverSessionBody struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	GitRepo   string `json:"git_repo"`
	GitBranch string `json:"git_branch"`
	GitToken  string `json:"git_token"`
}

type updateCodeserverSessionBody struct {
	Name    *string `json:"name"`
	Path    *string `json:"path"`
	Address *string `json:"address"`
	Port    *int    `json:"port"`
	Status  *string `json:"status"`
}

func (cc *controller) CreateCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body createCodeserverSessionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	authUserID, isAdmin, err := cc.resolveCaller(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized), r.WithErrorCode("UNAUTHORIZED"))
	}

	userID := strings.TrimSpace(body.UserID)
	if userID == "" {
		userID = authUserID
	}
	if !isAdmin && userID != authUserID {
		return r.Api(c, r.WithError(errors.New("you can only create workspaces for yourself")), r.WithStatus(fiber.StatusForbidden), r.WithErrorCode("FORBIDDEN"))
	}

	var user models.User
	if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("user not found")), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("USER_NOT_FOUND"))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = models.CodeserverSessionStatusInactive
	}
	addr := strings.TrimSpace(body.Address)
	if addr == "" {
		addr = adduser.BindHost
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		path = "/workspace"
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if err := codeserver.EnsureFolder(path); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("FOLDER_ERROR"))
	}

	name := models.CodeserverWorkspaceName(body.Name, path)
	row := models.CodeserverSession{
		UserID:  userID,
		Name:    name,
		Status:  status,
		Path:    path,
		Address: addr,
		Port:    body.Port,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(&row)

	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    sessionPayload(row),
		"message": "Workspace created",
	}))
}

func (cc *controller) GetCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
	}
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": sessionPayload(*row),
	}))
}

func (cc *controller) UpdateCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
	}

	var body updateCodeserverSessionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	updates := map[string]any{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		pathForName := row.Path
		if body.Path != nil {
			pathForName = strings.TrimSpace(*body.Path)
		}
		updates["name"] = models.CodeserverWorkspaceName(name, pathForName)
	}
	if body.Path != nil {
		path := strings.TrimSpace(*body.Path)
		if path == "" {
			path = "/workspace"
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if err := codeserver.EnsureFolder(path); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("FOLDER_ERROR"))
		}
		updates["path"] = path
		if body.Name == nil && strings.TrimSpace(row.Name) == "" {
			updates["name"] = models.CodeserverWorkspaceName("", path)
		}
	}
	if body.Address != nil {
		addr := strings.TrimSpace(*body.Address)
		if addr == "" {
			addr = adduser.BindHost
		}
		updates["address"] = addr
	}
	if body.Port != nil {
		port := max(*body.Port, 0)
		updates["port"] = port
	}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status == "" {
			status = models.CodeserverSessionStatusInactive
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
		"message": "Workspace updated",
	}))
}

func (cc *controller) DeleteCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
	}
	if codeserver.IsLive(*row) || row.Pid > 0 || row.Port > 0 {
		_ = codeserver.StopProcess(row)
	}
	if err := db.WithContext(ctx).Delete(row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "Workspace deleted",
		"data":    fiber.Map{"id": row.ID},
	}))
}

func (cc *controller) DisableCodeserverSessionAPI(c fiber.Ctx) error {
	return cc.setStatus(c, models.CodeserverSessionStatusInactive, "Workspace disabled")
}

func (cc *controller) EnableCodeserverSessionAPI(c fiber.Ctx) error {
	return cc.setStatus(c, models.CodeserverSessionStatusActive, "Workspace enabled")
}

// OpenCodeserverSessionAPI marks the session active and returns the in-app proxy URL.
func (cc *controller) OpenCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
	}
	if err := db.WithContext(ctx).Model(row).Update("status", models.CodeserverSessionStatusActive).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(row)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":        sessionPayload(*row),
		"connect_url": codeserver.PublicClientURLForFolder(row.ID, row.Path),
		"message":     "Opening VS Code",
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
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
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

func (cc *controller) loadSession(c fiber.Ctx) (*models.CodeserverSession, error) {
	ctx := c.Context()
	db := cc.app.DB()

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid session id")
	}

	var row models.CodeserverSession
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

func sessionPayload(row models.CodeserverSession) fiber.Map {
	full := strings.TrimSpace(row.User.FirstName + " " + row.User.LastName)
	live := row.Port > 0 && adduser.IsLocalPortListening(row.Port)
	name := models.CodeserverWorkspaceName(row.Name, row.Path)
	return fiber.Map{
		"id":          row.ID,
		"user_id":     row.UserID,
		"username":    row.User.Username,
		"email":       row.User.Email,
		"full_name":   full,
		"name":        name,
		"status":      row.Status,
		"path":        row.Path,
		"address":     row.Address,
		"port":        row.Port,
		"pid":         row.Pid,
		"live":        live,
		"connect_url": codeserver.PublicClientURLForFolder(row.ID, row.Path),
		"created_at":  row.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":  row.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
