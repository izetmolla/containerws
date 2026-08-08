package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	DesiredRunning = "running"
	DesiredStopped = "stopped"
)

type stateFile struct {
	Desired   string    `json:"desired"`
	UpdatedAt time.Time `json:"updated_at"`
}

var stateMu sync.Mutex

func statePath() string {
	if v := strings.TrimSpace(os.Getenv("CWS_VNC_SERVICE_STATE")); v != "" {
		return v
	}
	return "/config/containerws/vnc-service.json"
}

func serviceLogPath() string {
	if v := strings.TrimSpace(os.Getenv("CWS_VNC_SERVICE_LOG")); v != "" {
		return v
	}
	return "/config/containerws/vnc-service.log"
}

func loadDesired() string {
	stateMu.Lock()
	defer stateMu.Unlock()
	data, err := os.ReadFile(statePath())
	if err != nil {
		return DesiredStopped
	}
	var st stateFile
	if err := json.Unmarshal(data, &st); err != nil {
		return DesiredStopped
	}
	switch strings.ToLower(strings.TrimSpace(st.Desired)) {
	case DesiredRunning:
		return DesiredRunning
	default:
		return DesiredStopped
	}
}

func saveDesired(desired string) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	desired = strings.ToLower(strings.TrimSpace(desired))
	if desired != DesiredRunning {
		desired = DesiredStopped
	}
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(stateFile{
		Desired:   desired,
		UpdatedAt: time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func appendServiceLog(line string) {
	path := serviceLogPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format(time.RFC3339) + " " + line + "\n")
}
