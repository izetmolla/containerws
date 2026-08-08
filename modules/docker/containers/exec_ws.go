package containers

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/izetmolla/containerws/modules/docker/environments"
)

type execWSControl struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Data string `json:"data,omitempty"`
}

type execWSOut struct {
	binary []byte
	json   any
}

func (cc *controller) HandleExecWS(conn *websocket.Conn) {
	// Serialize every write: fasthttp/websocket allows only one concurrent writer.
	// A dedicated writer loop also avoids races with RecoverHandler / control frames.
	var (
		writeMu   sync.Mutex
		writeDead bool
	)
	safeClose := func() {
		writeMu.Lock()
		defer writeMu.Unlock()
		if writeDead {
			return
		}
		writeDead = true
		_ = conn.Close()
	}
	// Stash so RecoverHandler can close without writing.
	conn.Locals("exec_ws_close", safeClose)

	writeErr := func(message string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if writeDead {
			return
		}
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": message})
		writeDead = true
		_ = conn.Close()
	}

	id := strings.TrimSpace(conn.Params("id"))
	if id == "" {
		writeErr("container id is required")
		return
	}

	command := strings.TrimSpace(conn.Query("command"))
	if command == "" {
		command = "/bin/sh"
	}
	user := strings.TrimSpace(conn.Query("user"))
	envID := strings.TrimSpace(conn.Query("environment_id"))

	_, cli, err := environments.ClientFromQuery(cc.app.DB(), envID)
	if err != nil {
		writeErr(err.Error())
		return
	}
	defer cli.Close()

	cmd := parseExecCommand(command)
	if len(cmd) == 0 {
		writeErr("command is required")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	createResp, err := cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		User:         user,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	})
	if err != nil {
		writeErr(err.Error())
		return
	}

	hijacked, err := cli.ContainerExecAttach(ctx, createResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		writeErr(err.Error())
		return
	}
	defer hijacked.Close()

	outCh := make(chan execWSOut, 64)
	var writerOnce sync.Once
	closeWriter := func() {
		writerOnce.Do(func() { close(outCh) })
	}
	defer closeWriter()

	// Single writer — the only goroutine that calls conn.Write*.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for msg := range outCh {
			writeMu.Lock()
			if writeDead {
				writeMu.Unlock()
				continue
			}
			var werr error
			if msg.json != nil {
				werr = conn.WriteJSON(msg.json)
			} else if len(msg.binary) > 0 {
				werr = conn.WriteMessage(websocket.BinaryMessage, msg.binary)
			}
			if werr != nil {
				writeDead = true
				writeMu.Unlock()
				cancel()
				_ = hijacked.CloseWrite()
				return
			}
			writeMu.Unlock()
		}
	}()

	sendJSON := func(v any) {
		select {
		case <-ctx.Done():
			return
		case outCh <- execWSOut{json: v}:
		}
	}
	sendBinary := func(b []byte) {
		select {
		case <-ctx.Done():
			return
		case outCh <- execWSOut{binary: b}:
		}
	}

	sendJSON(map[string]any{
		"type":    "ready",
		"exec_id": createResp.ID,
		"command": strings.Join(cmd, " "),
		"user":    user,
	})

	// Docker TTY → websocket (via outCh).
	dockerDone := make(chan struct{})
	go func() {
		defer close(dockerDone)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := hijacked.Reader.Read(buf)
			if n > 0 {
				out := make([]byte, n)
				copy(out, buf[:n])
				sendBinary(out)
			}
			if readErr != nil {
				if readErr != io.EOF && ctx.Err() == nil {
					sendJSON(map[string]any{
						"type":    "error",
						"message": readErr.Error(),
					})
				}
				sendJSON(map[string]any{
					"type":    "exit",
					"message": "Console session closed",
				})
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
		return nil
	})

	type clientMsg struct {
		mt  int
		msg []byte
	}
	inCh := make(chan clientMsg, 16)
	go func() {
		defer close(inCh)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
			// Copy payload — the websocket buffer may be reused.
			payload := append([]byte(nil), msg...)
			select {
			case inCh <- clientMsg{mt: mt, msg: payload}:
			case <-ctx.Done():
				return
			}
		}
	}()

	shutdown := func() {
		cancel()
		_ = hijacked.CloseWrite()
		safeClose()
		<-dockerDone
		closeWriter()
		<-writerDone
	}

	for {
		select {
		case <-dockerDone:
			closeWriter()
			<-writerDone
			safeClose()
			return
		case <-ctx.Done():
			shutdown()
			return
		case m, ok := <-inCh:
			if !ok {
				shutdown()
				return
			}
			switch m.mt {
			case websocket.BinaryMessage:
				if len(m.msg) > 0 {
					if _, err := hijacked.Conn.Write(m.msg); err != nil {
						shutdown()
						return
					}
				}
			case websocket.TextMessage:
				var ctrl execWSControl
				if err := json.Unmarshal(m.msg, &ctrl); err != nil {
					if _, werr := hijacked.Conn.Write(m.msg); werr != nil {
						shutdown()
						return
					}
					continue
				}
				switch ctrl.Type {
				case "resize":
					if ctrl.Cols > 0 && ctrl.Rows > 0 {
						_ = resizeExec(cli, createResp.ID, ctrl.Cols, ctrl.Rows)
					}
				case "input":
					if ctrl.Data != "" {
						if _, err := hijacked.Conn.Write([]byte(ctrl.Data)); err != nil {
							shutdown()
							return
						}
					}
				case "ping":
					sendJSON(map[string]any{"type": "pong"})
				case "kill":
					sendJSON(map[string]any{"type": "exit", "message": "Disconnected"})
					shutdown()
					return
				}
			}
		}
	}
}

func resizeExec(cli *client.Client, execID string, cols, rows uint16) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Width:  uint(cols),
		Height: uint(rows),
	})
}

func parseExecCommand(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	// Common shell paths: keep as a single argv entry.
	switch command {
	case "/bin/bash", "/bin/sh", "/bin/ash", "/bin/zsh", "/bin/dash",
		"bash", "sh", "ash", "zsh", "dash":
		if !strings.HasPrefix(command, "/") {
			return []string{"/bin/" + command}
		}
		return []string{command}
	}
	return splitCommand(command)
}

// splitCommand splits a command string on whitespace while respecting simple
// single/double quotes (Portainer-style custom commands).
func splitCommand(s string) []string {
	var (
		parts   []string
		current strings.Builder
		quote   rune
		escape  bool
	)
	flush := func() {
		if current.Len() == 0 {
			return
		}
		parts = append(parts, current.String())
		current.Reset()
	}
	for _, r := range s {
		switch {
		case escape:
			current.WriteRune(r)
			escape = false
		case r == '\\' && quote != '\'':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return parts
}
