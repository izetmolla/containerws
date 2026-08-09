package cloudshell

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
)

type wsControlMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Data string `json:"data,omitempty"`
}

type wsConnSink struct {
	conn *websocket.Conn
	mu   *sync.Mutex
}

func (s *wsConnSink) WriteBinary(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, b)
}

func (s *wsConnSink) WriteJSON(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

func (cc *controller) HandleTerminalWS(conn *websocket.Conn) {
	raw := conn.Locals("cloudshell_user")
	cliUser, ok := raw.(*cliUserContext)
	if !ok || cliUser == nil {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "missing user context"})
		_ = conn.Close()
		return
	}

	resumeID := ""
	if conn.Query("session_id") != "" {
		resumeID = conn.Query("session_id")
	}
	title := conn.Query("title")

	var (
		ms   *managedSession
		err  error
	)

	if resumeID != "" {
		existing := sessions.get(resumeID)
		if existing == nil || existing.isClosed() {
			_ = conn.WriteJSON(map[string]any{
				"type":       "error",
				"code":       "SESSION_NOT_FOUND",
				"message":    "session not found or expired",
				"session_id": resumeID,
			})
			_ = conn.Close()
			return
		}
		if existing.UserID != cliUser.UserID {
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": "session belongs to another user"})
			_ = conn.Close()
			return
		}
		ms = existing
	}

	createdNew := false
	if ms == nil {
		ms, err = sessions.create(cliUser, title)
		if err != nil {
			_ = conn.WriteJSON(map[string]any{"type": "error", "message": err.Error()})
			_ = conn.Close()
			return
		}
		createdNew = true
	}

	var writeMu sync.Mutex
	sink := &wsConnSink{conn: conn, mu: &writeMu}

	snap, attachErr := ms.Attach(sink)
	if attachErr != nil {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": "failed to attach session"})
		_ = conn.Close()
		return
	}
	defer ms.Detach(sink)

	resumed := !createdNew
	_ = sink.WriteJSON(map[string]any{
		"type":       "ready",
		"session_id": ms.ID,
		"resumed":    resumed,
		"user":       cliUser.ShellUser,
		"home":       cliUser.HomeDir,
		"shell":      cliUser.Shell,
		"title":      ms.Title,
		"message": fmt.Sprintf(
			"%s as %s (%s)",
			map[bool]string{true: "Resumed", false: "Connected"}[resumed],
			cliUser.DisplayName,
			cliUser.ShellUser,
		),
	})

	// Replay recent output so the client sees prior scrollback after resume.
	if len(snap) > 0 {
		_ = sink.WriteBinary(snap)
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
		return nil
	})

	for {
		if ms.isClosed() {
			break
		}
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
		ms.touch()

		switch mt {
		case websocket.BinaryMessage:
			if len(msg) > 0 {
				_, _ = ms.Write(msg)
			}
		case websocket.TextMessage:
			var ctrl wsControlMessage
			if err := json.Unmarshal(msg, &ctrl); err != nil {
				_, _ = ms.Write(msg)
				continue
			}
			switch ctrl.Type {
			case "resize":
				if ctrl.Cols > 0 && ctrl.Rows > 0 {
					_ = ms.Resize(ctrl.Cols, ctrl.Rows)
				}
			case "input":
				if ctrl.Data != "" {
					_, _ = ms.Write([]byte(ctrl.Data))
				}
			case "ping":
				_ = sink.WriteJSON(map[string]any{"type": "pong"})
			case "kill":
				ms.Destroy()
				_ = sink.WriteJSON(map[string]any{"type": "exit", "message": "session killed"})
				return
			}
		}
	}
}

func (cc *controller) ListSessionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ctx, err := cc.resolveUser(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized))
	}

	dbRows, err := listShellSessionsFromDB(ctx.UserID)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	data := make([]map[string]any, 0, len(dbRows))
	seen := make(map[string]struct{}, len(dbRows))

	for _, row := range dbRows {
		live := sessions.get(row.ID)
		alive := live != nil && !live.isClosed()
		if !alive {
			// PTY gone (process restart / idle kill) — close durable row.
			_ = markShellSessionClosed(row.ID, ctx.UserID)
			continue
		}
		attached := false
		live.mu.Lock()
		attached = live.attach != nil
		lastActive := live.lastActive
		live.mu.Unlock()
		if lastActive.IsZero() && row.LastActiveAt != nil {
			lastActive = *row.LastActiveAt
		}
		data = append(data, map[string]any{
			"id":          row.ID,
			"title":       row.Title,
			"created_at":  row.CreatedAt,
			"last_active": lastActive,
			"attached":    attached,
			"alive":       true,
			"status":      row.Status,
			"shell_user":  row.ShellUser,
			"cwd":         live.LiveCwd(),
			"cols":        row.Cols,
			"rows":        row.Rows,
		})
		seen[row.ID] = struct{}{}
	}

	// Include in-memory sessions not yet flushed to DB (rare race).
	for _, s := range sessions.listForUser(ctx.UserID) {
		if _, ok := seen[s.ID]; ok {
			continue
		}
		data = append(data, s.Info())
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": data}))
}

func (cc *controller) KillSessionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ctx, err := cc.resolveUser(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized))
	}
	id := c.Params("id")
	ms := sessions.get(id)
	if ms != nil && !ms.isClosed() {
		if ms.UserID != ctx.UserID {
			return r.Api(c, r.WithError(fmt.Errorf("forbidden")), r.WithStatus(fiber.StatusForbidden))
		}
		ms.Destroy()
	} else {
		_ = markShellSessionClosed(id, ctx.UserID)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"id": id, "killed": true},
	}))
}

type ptySession struct {
	cmd  *exec.Cmd
	ptmx *os.File
	user *cliUserContext
}

func startPTYSession(cliUser *cliUserContext) (*ptySession, error) {
	home := ensureAbsHome(cliUser.HomeDir)
	shell := cliUser.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-l")
	cmd.Dir = home
	cmd.Env = buildShellEnv(cliUser, home)
	applyLinuxCredentials(cmd, cliUser.ShellUser)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: defaultSessionRows, Cols: defaultSessionCols})

	return &ptySession{cmd: cmd, ptmx: ptmx, user: cliUser}, nil
}

func applyLinuxCredentials(cmd *exec.Cmd, username string) {
	applyUnixCredentials(cmd, username)
}


func (s *ptySession) Resize(cols, rows uint16) error {
	if s == nil || s.ptmx == nil {
		return nil
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// CurrentCwd returns the live working directory of the shell process via /proc.
func (s *ptySession) CurrentCwd() string {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		if s != nil && s.user != nil && s.user.HomeDir != "" {
			return s.user.HomeDir
		}
		return ""
	}
	pid := s.cmd.Process.Pid
	if pid <= 0 {
		return ""
	}
	target, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		if s.user != nil && s.user.HomeDir != "" {
			return s.user.HomeDir
		}
		return ""
	}
	return target
}

func (s *ptySession) Close() {
	if s == nil {
		return
	}
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = signalHangup(s.cmd.Process)
		done := make(chan struct{})
		go func() {
			_, _ = s.cmd.Process.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = s.cmd.Process.Kill()
		}
	}
}

func buildShellEnv(cliUser *cliUserContext, home string) []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	// Prefer Linuxbrew when present (panel bootstrap may install it off PATH).
	for _, extra := range []string{
		"/home/linuxbrew/.linuxbrew/bin",
		"/home/linuxbrew/.linuxbrew/sbin",
	} {
		if st, err := os.Stat(extra); err == nil && st.IsDir() && !strings.Contains(path, extra) {
			path = extra + string(os.PathListSeparator) + path
		}
	}
	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"HOME=" + home,
		"USER=" + cliUser.ShellUser,
		"LOGNAME=" + cliUser.ShellUser,
		"SHELL=" + cliUser.Shell,
		"PATH=" + path,
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		fmt.Sprintf("PS1=\\[\\e[32m\\]%s@containerws\\[\\e[0m\\]:\\[\\e[34m\\]\\w\\[\\e[0m\\]\\$ ", cliUser.ShellUser),
	}
	for _, key := range []string{"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "NVM_DIR", "HOMEBREW_PREFIX", "HOMEBREW_CELLAR", "HOMEBREW_REPOSITORY"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}
