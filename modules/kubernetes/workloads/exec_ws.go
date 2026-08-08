package workloads

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
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

type termSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func (q *termSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	return &size
}

func (q *termSizeQueue) push(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		return
	}
	select {
	case q.ch <- remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
		// Drop oldest if full, then push.
		select {
		case <-q.ch:
		default:
		}
		select {
		case q.ch <- remotecommand.TerminalSize{Width: cols, Height: rows}:
		default:
		}
	}
}

// HandlePodExecWS bridges a browser WebSocket to kubectl-style pod exec (TTY).
// Protocol matches Docker exec: binary = PTY bytes; JSON control = resize/ping/kill/input.
func (cc *controller) HandlePodExecWS(conn *websocket.Conn) {
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

	ns := strings.TrimSpace(conn.Params("namespace"))
	name := strings.TrimSpace(conn.Params("name"))
	if ns == "" || name == "" {
		writeErr("namespace and pod name are required")
		return
	}

	command := strings.TrimSpace(conn.Query("command"))
	if command == "" {
		command = "/bin/sh"
	}
	containerName := strings.TrimSpace(conn.Query("container"))
	cmd := parseExecCommand(command)
	if len(cmd) == 0 {
		writeErr("command is required")
		return
	}

	restCfg, _, err := kubecli.RestConfig(cc.app)
	if err != nil {
		writeErr(err.Error())
		return
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		writeErr(err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if containerName == "" {
		pod, err := cli.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			writeErr(err.Error())
			return
		}
		if len(pod.Spec.Containers) == 0 {
			writeErr("pod has no containers")
			return
		}
		containerName = pod.Spec.Containers[0].Name
	}

	req := cli.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(restCfg, "POST", req.URL())
	if err != nil {
		writeErr(err.Error())
		return
	}

	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()
	defer stdinW.Close()

	sizeQ := &termSizeQueue{ch: make(chan remotecommand.TerminalSize, 8)}
	defer close(sizeQ.ch)

	outCh := make(chan execWSOut, 64)
	var writerOnce sync.Once
	closeWriter := func() {
		writerOnce.Do(func() { close(outCh) })
	}
	defer closeWriter()

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
				_ = stdinW.Close()
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

	stdout := &wsWriter{send: sendBinary}
	stderr := &wsWriter{send: sendBinary}

	sendJSON(map[string]any{
		"type":      "ready",
		"namespace": ns,
		"name":      name,
		"container": containerName,
		"command":   strings.Join(cmd, " "),
	})

	streamDone := make(chan error, 1)
	go func() {
		streamDone <- executor.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             stdinR,
			Stdout:            stdout,
			Stderr:            stderr,
			Tty:               true,
			TerminalSizeQueue: sizeQ,
		})
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
			payload := append([]byte(nil), msg...)
			select {
			case inCh <- clientMsg{mt: mt, msg: payload}:
			case <-ctx.Done():
				return
			}
		}
	}()

	shutdown := func(message string) {
		cancel()
		_ = stdinW.Close()
		if message != "" {
			sendJSON(map[string]any{"type": "exit", "message": message})
		}
		safeClose()
		<-streamDone
		closeWriter()
		<-writerDone
	}

	for {
		select {
		case err := <-streamDone:
			msg := "Console session closed"
			if err != nil && ctx.Err() == nil {
				sendJSON(map[string]any{"type": "error", "message": err.Error()})
				msg = err.Error()
			}
			sendJSON(map[string]any{"type": "exit", "message": msg})
			closeWriter()
			<-writerDone
			safeClose()
			return
		case <-ctx.Done():
			shutdown("")
			return
		case m, ok := <-inCh:
			if !ok {
				shutdown("Disconnected")
				return
			}
			switch m.mt {
			case websocket.BinaryMessage:
				if len(m.msg) > 0 {
					if _, err := stdinW.Write(m.msg); err != nil {
						shutdown("Console session closed")
						return
					}
				}
			case websocket.TextMessage:
				var ctrl execWSControl
				if err := json.Unmarshal(m.msg, &ctrl); err != nil {
					if _, werr := stdinW.Write(m.msg); werr != nil {
						shutdown("Console session closed")
						return
					}
					continue
				}
				switch ctrl.Type {
				case "resize":
					sizeQ.push(ctrl.Cols, ctrl.Rows)
				case "input":
					if ctrl.Data != "" {
						if _, err := stdinW.Write([]byte(ctrl.Data)); err != nil {
							shutdown("Console session closed")
							return
						}
					}
				case "ping":
					sendJSON(map[string]any{"type": "pong"})
				case "kill":
					sendJSON(map[string]any{"type": "exit", "message": "Disconnected"})
					shutdown("")
					return
				}
			}
		}
	}
}

type wsWriter struct {
	send func([]byte)
}

func (w *wsWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	out := make([]byte, len(p))
	copy(out, p)
	w.send(out)
	return len(p), nil
}

func parseExecCommand(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
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
