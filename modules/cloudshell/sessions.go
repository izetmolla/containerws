package cloudshell

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/containerws/models"
)

const (
	sessionIdleTTL     = 60 * time.Minute
	sessionRingBytes   = 512 * 1024
	defaultSessionRows = 24
	defaultSessionCols = 80
)

// managedSession keeps a PTY alive across WebSocket disconnects so clients can resume.
type managedSession struct {
	ID        string
	UserID    string
	Title     string
	CreatedAt time.Time

	pty *ptySession

	mu         sync.Mutex
	lastActive time.Time
	closed     bool
	idleTimer  *time.Timer

	// Single attached writer (one browser tab at a time).
	attach writeSink

	ring *byteRing
}

type writeSink interface {
	WriteBinary([]byte) error
	WriteJSON(any) error
}

type sessionRegistry struct {
	mu   sync.Mutex
	byID map[string]*managedSession
}

var sessions = &sessionRegistry{byID: make(map[string]*managedSession)}

func (r *sessionRegistry) create(user *cliUserContext, title string) (*managedSession, error) {
	ptySess, err := startPTYSession(user)
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	if title == "" {
		title = "session-" + id[:8]
	}
	ms := &managedSession{
		ID:         id,
		UserID:     user.UserID,
		Title:      title,
		CreatedAt:  time.Now().UTC(),
		lastActive: time.Now().UTC(),
		pty:        ptySess,
		ring:       newByteRing(sessionRingBytes),
	}
	r.mu.Lock()
	r.byID[id] = ms
	r.mu.Unlock()

	persistShellSessionCreate(ms, user)
	go ms.readLoop()
	return ms, nil
}

func (r *sessionRegistry) get(id string) *managedSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byID[id]
}

func (r *sessionRegistry) remove(id string) {
	r.mu.Lock()
	delete(r.byID, id)
	r.mu.Unlock()
}

func (r *sessionRegistry) listForUser(userID string) []*managedSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*managedSession, 0)
	for _, s := range r.byID {
		if s.UserID == userID && !s.isClosed() {
			out = append(out, s)
		}
	}
	return out
}

// LiveCwdForSession returns the live PTY cwd for a shell session owned by userID.
func LiveCwdForSession(sessionID, userID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	ms := sessions.get(sessionID)
	if ms == nil || ms.isClosed() {
		return ""
	}
	if userID != "" && ms.UserID != userID {
		return ""
	}
	return ms.LiveCwd()
}

func (ms *managedSession) isClosed() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.closed
}

func (ms *managedSession) touch() {
	ms.mu.Lock()
	ms.lastActive = time.Now().UTC()
	ms.mu.Unlock()
}

func (ms *managedSession) cancelIdleLocked() {
	if ms.idleTimer != nil {
		ms.idleTimer.Stop()
		ms.idleTimer = nil
	}
}

func (ms *managedSession) scheduleIdle() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.cancelIdleLocked()
	id := ms.ID
	ms.idleTimer = time.AfterFunc(sessionIdleTTL, func() {
		s := sessions.get(id)
		if s == nil {
			return
		}
		s.mu.Lock()
		attached := s.attach != nil
		s.mu.Unlock()
		if attached {
			return
		}
		s.Destroy()
	})
}

func (ms *managedSession) Attach(sink writeSink) (snapshot []byte, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.closed {
		return nil, io.ErrClosedPipe
	}
	ms.cancelIdleLocked()
	ms.attach = sink
	ms.lastActive = time.Now().UTC()
	persistShellSessionStatus(ms.ID, models.ShellSessionActive)
	return ms.ring.Snapshot(), nil
}

func (ms *managedSession) Detach(sink writeSink) {
	ms.mu.Lock()
	if ms.attach == sink {
		ms.attach = nil
		ms.lastActive = time.Now().UTC()
		ms.mu.Unlock()
		persistShellSessionStatus(ms.ID, models.ShellSessionDetached)
		ms.scheduleIdle()
		return
	}
	ms.mu.Unlock()
}

func (ms *managedSession) Resize(cols, rows uint16) error {
	ms.touch()
	persistShellSessionResize(ms.ID, cols, rows)
	return ms.pty.Resize(cols, rows)
}

func (ms *managedSession) Write(p []byte) (int, error) {
	ms.touch()
	return ms.pty.ptmx.Write(p)
}

func (ms *managedSession) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := ms.pty.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			ms.mu.Lock()
			ms.ring.Write(chunk)
			sink := ms.attach
			ms.mu.Unlock()
			if sink != nil {
				_ = sink.WriteBinary(chunk)
			}
		}
		if err != nil {
			ms.mu.Lock()
			sink := ms.attach
			ms.mu.Unlock()
			if sink != nil {
				if err != io.EOF {
					_ = sink.WriteJSON(map[string]any{"type": "error", "message": err.Error()})
				}
				_ = sink.WriteJSON(map[string]any{"type": "exit", "message": "shell closed"})
			}
			ms.Destroy()
			return
		}
	}
}

func (ms *managedSession) Destroy() {
	ms.mu.Lock()
	if ms.closed {
		ms.mu.Unlock()
		return
	}
	ms.closed = true
	ms.cancelIdleLocked()
	ms.attach = nil
	ms.mu.Unlock()

	sessions.remove(ms.ID)
	persistShellSessionStatus(ms.ID, models.ShellSessionClosed)
	ms.pty.Close()
}

func (ms *managedSession) Info() map[string]any {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	cwd := ""
	if ms.pty != nil {
		cwd = ms.pty.CurrentCwd()
	}
	return map[string]any{
		"id":          ms.ID,
		"title":       ms.Title,
		"created_at":  ms.CreatedAt,
		"last_active": ms.lastActive,
		"attached":    ms.attach != nil,
		"alive":       !ms.closed,
		"cwd":         cwd,
	}
}

// LiveCwd returns the shell process working directory for this session.
func (ms *managedSession) LiveCwd() string {
	if ms == nil || ms.isClosed() || ms.pty == nil {
		return ""
	}
	return ms.pty.CurrentCwd()
}

// byteRing is a fixed-size circular buffer of recent PTY output for resume replay.
type byteRing struct {
	buf  []byte
	size int
	pos  int
	full bool
}

func newByteRing(n int) *byteRing {
	if n < 1024 {
		n = 1024
	}
	return &byteRing{buf: make([]byte, n), size: n}
}

func (r *byteRing) Write(p []byte) {
	for _, b := range p {
		r.buf[r.pos] = b
		r.pos = (r.pos + 1) % r.size
		if r.pos == 0 {
			r.full = true
		}
	}
}

func (r *byteRing) Snapshot() []byte {
	if !r.full && r.pos == 0 {
		return nil
	}
	if !r.full {
		out := make([]byte, r.pos)
		copy(out, r.buf[:r.pos])
		return out
	}
	out := make([]byte, r.size)
	copy(out, r.buf[r.pos:])
	copy(out[r.size-r.pos:], r.buf[:r.pos])
	return out
}
