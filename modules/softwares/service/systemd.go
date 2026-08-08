package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unitNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.@-]+$`)

// UnitStatus is the runtime state of one systemd unit.
type UnitStatus struct {
	Unit        string `json:"unit"`
	Active      string `json:"active"` // active, inactive, failed, activating, …
	Sub         string `json:"sub"`
	Description string `json:"description,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Status aggregates units for a software entry.
type Status struct {
	Managed bool         `json:"managed"`
	Overall string       `json:"overall"` // running, stopped, partial, failed, unmanaged, unavailable
	Units   []UnitStatus `json:"units"`
}

func validateUnit(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty unit name")
	}
	if !unitNameRe.MatchString(name) {
		return fmt.Errorf("invalid unit name %q", name)
	}
	return nil
}

func systemctlBinaryPresent() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// systemdUsable reports whether systemd is actually managing this machine.
// Container workspaces often run in "direct" mode (cws as PID 1): systemctl
// exists but is-system-running is "offline" and unit control fails.
func systemdUsable() bool {
	if !systemctlBinaryPresent() {
		return false
	}
	// Primary signal used by Docker Engine install scripts in this repo.
	if st, err := os.Stat("/run/systemd/system"); err == nil && st.IsDir() {
		return true
	}
	out, err := exec.Command("systemctl", "is-system-running").CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err == nil {
		switch state {
		case "running", "degraded", "maintenance", "initializing", "starting":
			return true
		case "offline", "unknown":
			return false
		}
	}
	// Offline / host-is-down style failures.
	if strings.Contains(strings.ToLower(state+" "+errString(err)), "offline") ||
		strings.Contains(strings.ToLower(state+" "+errString(err)), "not been booted with systemd") {
		return false
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isDockerUnit(unit string) bool {
	u := strings.ToLower(strings.TrimSpace(unit))
	return u == "docker.service" || u == "docker.socket" || u == "docker"
}

// ProbeUnits returns status for the given systemd units.
func ProbeUnits(units []string) Status {
	clean := make([]string, 0, len(units))
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return Status{Managed: false, Overall: "unmanaged", Units: []UnitStatus{}}
	}

	out := Status{Managed: true, Units: make([]UnitStatus, 0, len(clean))}
	usable := systemdUsable()
	if !usable {
		// Prefer process/socket probes for Docker when nested DinD runs without systemd.
		allDocker := true
		for _, u := range clean {
			if !isDockerUnit(u) {
				allDocker = false
				break
			}
		}
		if allDocker {
			for _, u := range clean {
				out.Units = append(out.Units, probeDockerDirect(u))
			}
			out.Overall = summarize(out.Units)
			return out
		}
		out.Overall = "unavailable"
		for _, u := range clean {
			msg := "systemd not running (direct mode)"
			if !systemctlBinaryPresent() {
				msg = "systemctl not available"
			}
			out.Units = append(out.Units, UnitStatus{
				Unit:   u,
				Active: "unknown",
				Error:  msg,
			})
		}
		return out
	}

	for _, u := range clean {
		st := probeOne(u)
		// If systemd is up but docker unit looks dead while dockerd is live (race / Type=simple),
		// prefer the live dockerd signal for docker.service.
		if isDockerUnit(u) && (st.Active == "inactive" || st.Active == "failed" || st.Active == "unknown") {
			if d := probeDockerDirect(u); d.Active == "active" {
				st = d
			}
		}
		out.Units = append(out.Units, st)
	}
	out.Overall = summarize(out.Units)
	return out
}

func probeOne(unit string) UnitStatus {
	st := UnitStatus{Unit: unit, Active: "unknown"}
	if err := validateUnit(unit); err != nil {
		st.Error = err.Error()
		return st
	}

	cmd := exec.Command("systemctl", "show", unit,
		"--property=ActiveState,SubState,Description",
		"--no-pager",
	)
	raw, err := cmd.Output()
	if err != nil {
		// Fall back to is-active for a coarse signal.
		activeOut, aerr := exec.Command("systemctl", "is-active", unit).CombinedOutput()
		active := strings.TrimSpace(string(activeOut))
		if active == "" {
			active = "unknown"
		}
		st.Active = active
		if aerr != nil && active == "unknown" {
			st.Error = strings.TrimSpace(err.Error())
		}
		return st
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			st.Active = val
		case "SubState":
			st.Sub = val
		case "Description":
			st.Description = val
		}
	}
	if st.Active == "" {
		st.Active = "unknown"
	}
	return st
}

func probeDockerDirect(unit string) UnitStatus {
	st := UnitStatus{
		Unit:        unit,
		Description: "Docker Engine (direct mode)",
		Active:      "inactive",
		Sub:         "dead",
	}
	if dockerEngineReachable() {
		st.Active = "active"
		st.Sub = "running"
		return st
	}
	if dockerdProcessRunning() {
		st.Active = "activating"
		st.Sub = "start"
		st.Error = "dockerd running but API socket not ready"
		return st
	}
	return st
}

func dockerEngineReachable() bool {
	sock := dockerSockPath()
	if _, err := os.Stat(sock); err != nil {
		return false
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
		Timeout: 800 * time.Millisecond,
	}
	resp, err := client.Get("http://docker/_ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func dockerdProcessRunning() bool {
	if _, err := exec.LookPath("pgrep"); err == nil {
		// Only match non-zombie dockerd (state != Z).
		out, err := exec.Command("ps", "-eo", "stat,comm").Output()
		if err == nil {
			for line := range strings.SplitSeq(string(out), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				stat, comm := fields[0], fields[1]
				if comm == "dockerd" && !strings.Contains(stat, "Z") {
					return true
				}
			}
		}
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(raw)) != "dockerd" {
			continue
		}
		statRaw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// /proc/pid/stat: pid (comm) state ...
		s := string(statRaw)
		idx := strings.LastIndex(s, ") ")
		if idx < 0 || idx+2 >= len(s) {
			continue
		}
		state := s[idx+2]
		if state == 'Z' {
			continue
		}
		return true
	}
	return false
}

func dockerSockPath() string {
	if v := strings.TrimSpace(os.Getenv("DOCKER_HOST")); strings.HasPrefix(v, "unix://") {
		return strings.TrimPrefix(v, "unix://")
	}
	return "/var/run/docker.sock"
}

func dockerdBin() string {
	for _, p := range []string{"/usr/local/bin/dockerd", "/usr/bin/dockerd"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("dockerd"); err == nil {
		return p
	}
	return "dockerd"
}

func summarize(units []UnitStatus) string {
	if len(units) == 0 {
		return "unmanaged"
	}
	active := 0
	inactive := 0
	failed := 0
	unknown := 0
	activating := 0
	for _, u := range units {
		switch u.Active {
		case "active", "reloading":
			active++
		case "activating":
			activating++
		case "inactive", "dead", "deactivating":
			inactive++
		case "failed":
			failed++
		default:
			unknown++
		}
	}
	if failed > 0 {
		return "failed"
	}
	if unknown == len(units) {
		return "unavailable"
	}
	if active == len(units) {
		return "running"
	}
	if activating > 0 && inactive == 0 && failed == 0 && unknown == 0 {
		if active+activating == len(units) {
			return "running"
		}
	}
	if inactive == len(units) {
		return "stopped"
	}
	return "partial"
}

// ControlUnits runs systemctl <action> on the listed units.
// action must be start, stop, or restart.
// When systemd is not usable, docker.service is controlled via dockerd directly.
func ControlUnits(action string, units []string) (Status, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart":
	default:
		return Status{}, fmt.Errorf("unsupported action %q", action)
	}

	clean := make([]string, 0, len(units))
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := validateUnit(u); err != nil {
			return Status{}, err
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return Status{}, errors.New("no service units configured")
	}

	if !systemdUsable() {
		allDocker := true
		for _, u := range clean {
			if !isDockerUnit(u) {
				allDocker = false
				break
			}
		}
		if !allDocker {
			msg := "systemd not running (direct mode) — cannot control this service"
			if !systemctlBinaryPresent() {
				msg = "systemctl not available"
			}
			return ProbeUnits(clean), errors.New(msg)
		}
		if err := controlDockerDirect(action); err != nil {
			return ProbeUnits(clean), err
		}
		return ProbeUnits(clean), nil
	}

	args := append([]string{action}, clean...)
	cmd := exec.Command("systemctl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		// Fall back to direct dockerd control if systemctl fails for docker units.
		allDocker := true
		for _, u := range clean {
			if !isDockerUnit(u) {
				allDocker = false
				break
			}
		}
		if allDocker {
			if derr := controlDockerDirect(action); derr == nil {
				return ProbeUnits(clean), nil
			}
		}
		return ProbeUnits(clean), fmt.Errorf("systemctl %s: %s", action, msg)
	}
	return ProbeUnits(clean), nil
}

func controlDockerDirect(action string) error {
	switch action {
	case "start":
		if dockerEngineReachable() {
			return nil
		}
		bin := dockerdBin()
		cmd := exec.Command(bin, "--host=unix://"+dockerSockPath())
		logFile, err := os.OpenFile("/var/log/dockerd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			cmd.Stdout = nil
			cmd.Stderr = nil
		} else {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
		if err := cmd.Start(); err != nil {
			if logFile != nil {
				_ = logFile.Close()
			}
			return fmt.Errorf("start dockerd: %w", err)
		}
		// Detach — do not wait on dockerd.
		go func() {
			_ = cmd.Wait()
			if logFile != nil {
				_ = logFile.Close()
			}
		}()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if dockerEngineReachable() {
				return nil
			}
			time.Sleep(400 * time.Millisecond)
		}
		if dockerEngineReachable() {
			return nil
		}
		return errors.New("dockerd started but API did not become ready (see /var/log/dockerd.log)")
	case "stop":
		_ = exec.Command("pkill", "-x", "dockerd").Run()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if !dockerdProcessRunning() && !dockerEngineReachable() {
				_ = os.Remove(dockerSockPath())
				return nil
			}
			time.Sleep(300 * time.Millisecond)
		}
		_ = exec.Command("pkill", "-9", "-x", "dockerd").Run()
		_ = os.Remove(dockerSockPath())
		return nil
	case "restart":
		_ = controlDockerDirect("stop")
		return controlDockerDirect("start")
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}
