package brew

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const installScriptURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

type bootstrapState struct {
	mu        sync.RWMutex
	running   bool
	startedAt time.Time
	finished  bool
	success   bool
	error     string
	log       string
}

var bootstrap = &bootstrapState{}

// BootstrapStatus is exposed via API.
func BootstrapStatus() map[string]any {
	bootstrap.mu.RLock()
	defer bootstrap.mu.RUnlock()
	return map[string]any{
		"running":    bootstrap.running,
		"finished":   bootstrap.finished,
		"success":    bootstrap.success,
		"error":      bootstrap.error,
		"log":        truncateLog(bootstrap.log, 8000),
		"started_at": formatTime(bootstrap.startedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func truncateLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

// MaybeStart starts Homebrew bootstrap in the background if brew is missing
// and no install is already running.
func MaybeStart(_ any) {
	if ResolveBrewPath() != "" {
		EnsureBrewShellPath()
		return
	}
	StartBootstrap()
}

// StartBootstrap launches the official NONINTERACTIVE Homebrew installer.
func StartBootstrap() bool {
	bootstrap.mu.Lock()
	if bootstrap.running {
		bootstrap.mu.Unlock()
		return false
	}
	if ResolveBrewPath() != "" {
		bootstrap.running = false
		bootstrap.finished = true
		bootstrap.success = true
		bootstrap.error = ""
		bootstrap.log = "brew already installed"
		bootstrap.mu.Unlock()
		EnsureBrewShellPath()
		return false
	}
	bootstrap.running = true
	bootstrap.finished = false
	bootstrap.success = false
	bootstrap.error = ""
	bootstrap.log = ""
	bootstrap.startedAt = time.Now()
	bootstrap.mu.Unlock()

	go runBootstrap()
	return true
}

func runBootstrap() {
	defer func() {
		bootstrap.mu.Lock()
		bootstrap.running = false
		bootstrap.finished = true
		if ResolveBrewPath() != "" {
			bootstrap.success = true
			bootstrap.error = ""
		} else if bootstrap.error == "" {
			bootstrap.error = "brew binary not found after install"
			bootstrap.success = false
		}
		bootstrap.mu.Unlock()
		if ResolveBrewPath() != "" {
			EnsureBrewShellPath()
			AppendBootstrapNote("shellenv: brew added to PATH / profile.d")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	script := `/bin/bash -c "$(curl -fsSL ` + installScriptURL + `)"`
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"NONINTERACTIVE=1",
		"HOMEBREW_NO_AUTO_UPDATE=1",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	bootstrap.mu.Lock()
	bootstrap.log = buf.String()
	if err != nil {
		bootstrap.error = err.Error()
		bootstrap.success = false
	}
	bootstrap.mu.Unlock()
}

// AppendBootstrapNote adds a short line to the bootstrap log (best-effort).
func AppendBootstrapNote(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	bootstrap.mu.Lock()
	defer bootstrap.mu.Unlock()
	if bootstrap.log != "" {
		bootstrap.log += "\n"
	}
	bootstrap.log += line
}
