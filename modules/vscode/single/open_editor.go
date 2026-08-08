package single

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/cloudshell"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"gorm.io/gorm"
)

type openEditorBody struct {
	Path           string `json:"path"`
	ShellSessionID string `json:"shell_session_id"`
}

// OpenEditorAPI finds a workspace for (caller, folder) or creates one, starts
// serve-web when needed, and returns a connect URL. Does not overwrite other
// workspaces for the same user.
func (cc *controller) OpenEditorAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	authUserID, _, err := cc.resolveCaller(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized), r.WithErrorCode("UNAUTHORIZED"))
	}

	var body openEditorBody
	_ = c.Bind().Body(&body)

	var user models.User
	if err := db.WithContext(ctx).Where("id = ?", authUserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("user not found")), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("USER_NOT_FOUND"))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	path := strings.TrimSpace(body.Path)
	if sid := strings.TrimSpace(body.ShellSessionID); sid != "" {
		if live := cloudshell.LiveCwdForSession(sid, authUserID); live != "" {
			path = live
		}
	}
	if path == "" {
		path = defaultEditorPath()
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_PATH"))
	}
	if err := codeserver.EnsureFolder(absPath); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("FOLDER_ERROR"))
	}

	if _, err := codeserver.LookupCodeCLI(); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VSCODE_NOT_INSTALLED"))
	}

	cleanPath := filepath.Clean(absPath)
	var existing models.CodeserverSession
	var candidates []models.CodeserverSession
	if err := db.WithContext(ctx).Where("user_id = ?", user.ID).Order("updated_at DESC").Find(&candidates).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	found := false
	for _, cand := range candidates {
		if filepath.Clean(cand.Path) == cleanPath {
			existing = cand
			found = true
			break
		}
	}
	if !found {
		existing = models.CodeserverSession{
			UserID:  user.ID,
			Name:    models.CodeserverWorkspaceName("", absPath),
			Status:  models.CodeserverSessionStatusInactive,
			Path:    absPath,
			Address: adduser.BindHost,
		}
		if err := db.WithContext(ctx).Create(&existing).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}

	live := codeserver.IsLive(existing)
	if live {
		_ = db.WithContext(ctx).Model(&existing).Update("status", models.CodeserverSessionStatusActive).Error
		_ = db.WithContext(ctx).Preload("User").Where("id = ?", existing.ID).First(&existing)
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":        sessionPayload(existing),
			"connect_url": codeserver.PublicClientURLForFolder(existing.ID, existing.Path),
			"message":     "Opening VS Code",
			"reused":      found,
		}))
	}

	if existing.Pid > 0 || existing.Port > 0 {
		_ = codeserver.StopProcess(&existing)
	}

	addr := strings.TrimSpace(existing.Address)
	if addr == "" {
		addr = adduser.BindHost
	}
	used := codeserver.UsedPorts(db)
	port, perr := adduser.PickUnusedLocalPort(used)
	if perr != nil {
		return r.Api(c, r.WithError(perr), r.WithStatus(fiber.StatusInternalServerError))
	}

	linuxName := strings.TrimSpace(user.Username)
	if linuxName == "" {
		linuxName = strings.TrimSpace(user.LdapUsername)
	}

	result, err := codeserver.StartServeWeb(codeserver.StartOptions{
		Folder:    absPath,
		Host:      addr,
		Port:      port,
		LinuxUser: linuxName,
		Token:     "none",
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("START_FAILED"))
	}

	name := models.CodeserverWorkspaceName(existing.Name, absPath)
	if err := db.WithContext(ctx).Model(&existing).Updates(map[string]any{
		"status":  models.CodeserverSessionStatusActive,
		"path":    absPath,
		"name":    name,
		"address": addr,
		"port":    result.Port,
		"pid":     result.Pid,
	}).Error; err != nil {
		_ = codeserver.KillPID(result.Pid)
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	_ = db.WithContext(ctx).Preload("User").Where("id = ?", existing.ID).First(&existing)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":        sessionPayload(existing),
		"connect_url": codeserver.PublicClientURLForFolder(existing.ID, existing.Path),
		"message":     "VS Code ready",
		"reused":      found,
	}))
}

func defaultEditorPath() string {
	for _, candidate := range []string{"/workspace", "/root", os.Getenv("HOME")} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	return "/workspace"
}
