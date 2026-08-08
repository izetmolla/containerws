package single

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"gorm.io/gorm"
)

type streamEvent struct {
	Type       string    `json:"type"`
	Line       string    `json:"line,omitempty"`
	Stream     string    `json:"stream,omitempty"`
	Message    string    `json:"message,omitempty"`
	Success    *bool     `json:"success,omitempty"`
	ConnectURL string    `json:"connect_url,omitempty"`
	Data       fiber.Map `json:"data,omitempty"`
}

// StreamCreateCodeserverSessionAPI creates a new workspace and streams progress over SSE.
func (cc *controller) StreamCreateCodeserverSessionAPI(c fiber.Ctx) error {
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

	addr := strings.TrimSpace(body.Address)
	if addr == "" {
		addr = adduser.BindHost
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		path = "/workspace"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	name := models.CodeserverWorkspaceName(body.Name, absPath)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		var writeMu sync.Mutex
		writeEvent := func(ev streamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		okTrue := true
		okFalse := false
		fail := func(msg string) {
			_ = writeEvent(streamEvent{Type: "error", Message: msg, Success: &okFalse})
			_ = writeEvent(streamEvent{Type: "done", Message: msg, Success: &okFalse})
		}
		logLine := func(line, stream string) {
			_ = writeEvent(streamEvent{Type: "log", Line: line, Stream: stream})
		}
		sys := func(line string) {
			_ = writeEvent(streamEvent{Type: "system", Line: line, Stream: "system"})
		}

		if !writeEvent(streamEvent{
			Type:    "start",
			Message: fmt.Sprintf("Creating workspace %q for %s", name, user.Username),
		}) {
			return
		}

		sys(fmt.Sprintf("User: %s (%s)", user.Username, user.ID))
		sys(fmt.Sprintf("Name: %s", name))
		sys(fmt.Sprintf("Folder: %s", absPath))
		sys(fmt.Sprintf("Address: %s", addr))

		sys("Ensuring folder exists…")
		if err := codeserver.EnsureFolder(absPath); err != nil {
			fail("create folder: " + err.Error())
			return
		}
		logLine("folder ready: "+absPath, "stdout")

		if repo := strings.TrimSpace(body.GitRepo); repo != "" {
			sys("Cloning repository into folder…")
			if err := codeserver.CloneRepoInto(absPath, codeserver.CloneOptions{
				Repo:   repo,
				Branch: strings.TrimSpace(body.GitBranch),
				Token:  strings.TrimSpace(body.GitToken),
			}, func(line, stream string) {
				if stream == "system" {
					sys(line)
					return
				}
				logLine(line, stream)
			}); err != nil {
				fail("git clone: " + err.Error())
				return
			}
			logLine("repository ready", "stdout")
		}

		sys("Checking VS Code Server CLI…")
		if _, err := codeserver.LookupCodeCLI(); err != nil {
			fail(err.Error())
			return
		}
		logLine("VS Code Server CLI found", "stdout")

		sys("Creating workspace record…")
		row := models.CodeserverSession{
			UserID:  userID,
			Name:    name,
			Status:  models.CodeserverSessionStatusInactive,
			Path:    absPath,
			Address: addr,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			fail("create workspace: " + err.Error())
			return
		}
		logLine("workspace id: "+row.ID, "stdout")

		used := codeserver.UsedPorts(db)
		port := body.Port
		if port <= 0 {
			var perr error
			port, perr = adduser.PickUnusedLocalPort(used)
			if perr != nil {
				fail("allocate port: " + perr.Error())
				return
			}
		}
		sys(fmt.Sprintf("Starting serve-web on %s:%d…", addr, port))

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
			fail(err.Error())
			return
		}
		logLine(fmt.Sprintf("listening pid=%d port=%d", result.Pid, result.Port), "stdout")
		if result.LogPath != "" {
			sys("process log: " + result.LogPath)
		}

		if err := db.WithContext(ctx).Model(&row).Updates(map[string]any{
			"status":  models.CodeserverSessionStatusActive,
			"path":    absPath,
			"name":    name,
			"address": addr,
			"port":    result.Port,
			"pid":     result.Pid,
		}).Error; err != nil {
			_ = codeserver.KillPID(result.Pid)
			fail("update workspace: " + err.Error())
			return
		}

		_ = db.WithContext(ctx).Preload("User").Where("id = ?", row.ID).First(&row)
		connectURL := codeserver.PublicClientURLForFolder(row.ID, row.Path)
		sys("Workspace ready")
		logLine("connect: "+connectURL, "stdout")

		_ = writeEvent(streamEvent{
			Type:       "done",
			Message:    "Workspace created",
			Success:    &okTrue,
			ConnectURL: connectURL,
			Data:       sessionPayload(row),
		})
	})
}

// StreamReactivateCodeserverSessionAPI restarts serve-web for an existing workspace.
func (cc *controller) StreamReactivateCodeserverSessionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	db := cc.app.DB()

	row, err := cc.loadSession(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if err := cc.requireSessionAccess(c, row); err != nil {
		return err
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	sessionID := row.ID
	return c.SendStreamWriter(func(w *bufio.Writer) {
		var writeMu sync.Mutex
		writeEvent := func(ev streamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		okTrue := true
		okFalse := false
		fail := func(msg string) {
			_ = writeEvent(streamEvent{Type: "error", Message: msg, Success: &okFalse})
			_ = writeEvent(streamEvent{Type: "done", Message: msg, Success: &okFalse})
		}
		logLine := func(line, stream string) {
			_ = writeEvent(streamEvent{Type: "log", Line: line, Stream: stream})
		}
		sys := func(line string) {
			_ = writeEvent(streamEvent{Type: "system", Line: line, Stream: "system"})
		}

		var existing models.CodeserverSession
		if err := db.WithContext(ctx).Preload("User").Where("id = ?", sessionID).First(&existing).Error; err != nil {
			fail("workspace not found")
			return
		}

		label := models.CodeserverWorkspaceName(existing.Name, existing.Path)
		wasLive := codeserver.IsLive(existing)
		action := "Starting"
		if wasLive {
			action = "Restarting"
		}

		if !writeEvent(streamEvent{
			Type:    "start",
			Message: fmt.Sprintf("%s workspace %q", action, label),
		}) {
			return
		}

		sys(fmt.Sprintf("Workspace: %s", existing.ID))
		sys(fmt.Sprintf("Name: %s", label))
		sys(fmt.Sprintf("Status: %s · live=%v", existing.Status, wasLive))

		folder := strings.TrimSpace(existing.Path)
		if folder == "" {
			folder = "/workspace"
		}
		absPath, err := filepath.Abs(folder)
		if err != nil {
			fail(err.Error())
			return
		}
		addr := strings.TrimSpace(existing.Address)
		if addr == "" {
			addr = adduser.BindHost
		}

		sys(fmt.Sprintf("Folder: %s", absPath))
		sys(fmt.Sprintf("Address: %s", addr))

		sys("Ensuring folder exists…")
		if err := codeserver.EnsureFolder(absPath); err != nil {
			fail("create folder: " + err.Error())
			return
		}
		logLine("folder ready: "+absPath, "stdout")

		sys("Checking VS Code Server CLI…")
		if _, err := codeserver.LookupCodeCLI(); err != nil {
			fail(err.Error())
			return
		}
		logLine("VS Code Server CLI found", "stdout")

		if wasLive || existing.Pid > 0 || existing.Port > 0 {
			sys("Stopping previous process…")
			if err := codeserver.StopProcess(&existing); err != nil {
				sys("stop warning: " + err.Error())
			} else {
				logLine("previous process stopped", "stdout")
			}
		}

		used := codeserver.UsedPorts(db)
		delete(used, existing.Port)
		port, perr := adduser.PickUnusedLocalPort(used)
		if perr != nil {
			fail("allocate port: " + perr.Error())
			return
		}
		sys(fmt.Sprintf("Starting serve-web on %s:%d…", addr, port))

		linuxName := strings.TrimSpace(existing.User.Username)
		if linuxName == "" {
			linuxName = strings.TrimSpace(existing.User.LdapUsername)
		}

		result, err := codeserver.StartServeWeb(codeserver.StartOptions{
			Folder:    absPath,
			Host:      addr,
			Port:      port,
			LinuxUser: linuxName,
			Token:     "none",
		})
		if err != nil {
			fail(err.Error())
			return
		}
		logLine(fmt.Sprintf("listening pid=%d port=%d", result.Pid, result.Port), "stdout")
		if result.LogPath != "" {
			sys("process log: " + result.LogPath)
		}

		if err := db.WithContext(ctx).Model(&existing).Updates(map[string]any{
			"status":  models.CodeserverSessionStatusActive,
			"path":    absPath,
			"address": addr,
			"port":    result.Port,
			"pid":     result.Pid,
		}).Error; err != nil {
			_ = codeserver.KillPID(result.Pid)
			fail("update workspace: " + err.Error())
			return
		}

		_ = db.WithContext(ctx).Preload("User").Where("id = ?", existing.ID).First(&existing)
		connectURL := codeserver.PublicClientURLForFolder(existing.ID, existing.Path)
		sys("Workspace ready")
		logLine("connect: "+connectURL, "stdout")

		msg := "Workspace started"
		if wasLive {
			msg = "Workspace restarted"
		}
		_ = writeEvent(streamEvent{
			Type:       "done",
			Message:    msg,
			Success:    &okTrue,
			ConnectURL: connectURL,
			Data:       sessionPayload(existing),
		})
	})
}
