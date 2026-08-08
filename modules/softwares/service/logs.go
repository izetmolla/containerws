package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
)

// HasServiceUnits reports whether this software declares controllable units.
func HasServiceUnits(units []string) bool {
	for _, u := range units {
		if strings.TrimSpace(u) != "" {
			return true
		}
	}
	return false
}

// CanControl is true when the software is explicitly marked controllable and has
// service units and/or explicit start/restart/stop commands.
func CanControl(sw models.Software) bool {
	return sw.IsControllable()
}

// CanControlUnits is a units-only check (legacy / probe helpers). Prefer CanControl(sw).
func CanControlUnits(units []string) bool {
	return HasServiceUnits(units)
}

// ControlSoftware runs start/stop/restart using explicit model commands when set,
// otherwise falls back to ControlUnits (systemctl / docker direct).
func ControlSoftware(action string, sw models.Software) (Status, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart":
	default:
		return Status{}, fmt.Errorf("unsupported action %q", action)
	}

	units := []string(sw.ServiceUnits)
	cmdLine := sw.CommandFor(action)
	if cmdLine == "" {
		return ControlUnits(action, units)
	}

	// Prefer docker-direct / unit path when systemd cannot run systemctl commands.
	if isSystemctlCommand(cmdLine) && !systemdUsable() {
		return ControlUnits(action, units)
	}

	if err := runControlCommand(cmdLine); err != nil {
		allDocker := len(units) > 0
		for _, u := range units {
			if !isDockerUnit(u) {
				allDocker = false
				break
			}
		}
		if allDocker {
			if st, derr := ControlUnits(action, units); derr == nil {
				return st, nil
			}
		}
		st := ProbeUnits(units)
		return st, err
	}
	return ProbeUnits(units), nil
}

func isSystemctlCommand(cmd string) bool {
	c := strings.TrimSpace(cmd)
	return strings.HasPrefix(c, "systemctl ") || strings.HasPrefix(c, "/bin/systemctl ") ||
		strings.HasPrefix(c, "/usr/bin/systemctl ")
}

func runControlCommand(cmdLine string) error {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return errors.New("empty control command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-lc", cmdLine)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s: %s", cmdLine, msg)
	}
	return nil
}

// LogLine is one journal / service log line.
type LogLine struct {
	Unit string `json:"unit,omitempty"`
	Text string `json:"text"`
	At   string `json:"at,omitempty"`
}

// TailLogs returns the last n lines from journalctl for the given units (no follow).
func TailLogs(ctx context.Context, units []string, n int) ([]LogLine, error) {
	clean, err := cleanUnits(units)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		n = 100
	}
	if n > 2000 {
		n = 2000
	}
	if _, err := exec.LookPath("journalctl"); err != nil {
		return nil, errors.New("journalctl not available — live service logs require systemd journal")
	}

	args := journalArgs(clean, n, false)
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("journalctl: %s", msg)
	}
	return parseJournalOutput(string(out), clean), nil
}

// StreamLogs follows journalctl -f and calls emit for each line until ctx cancels.
func StreamLogs(ctx context.Context, units []string, n int, emit func(LogLine) error) error {
	clean, err := cleanUnits(units)
	if err != nil {
		return err
	}
	if n <= 0 {
		n = 100
	}
	if n > 2000 {
		n = 2000
	}
	if _, err := exec.LookPath("journalctl"); err != nil {
		return errors.New("journalctl not available — live service logs require systemd journal")
	}

	args := journalArgs(clean, n, true)
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var last string
		for sc.Scan() {
			last = sc.Text()
		}
		if last != "" {
			errCh <- errors.New(last)
			return
		}
		errCh <- nil
	}()

	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if err := emit(LogLine{
				Text: line,
				Unit: guessUnit(line, clean),
				At:   time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				errCh <- err
				return
			}
		}
		if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ctx.Err()
	case err := <-errCh:
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		if waitErr != nil && ctx.Err() == nil {
			return waitErr
		}
		return nil
	}
}

func cleanUnits(units []string) ([]string, error) {
	clean := make([]string, 0, len(units))
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := validateUnit(u); err != nil {
			return nil, err
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return nil, errors.New("no service units configured")
	}
	return clean, nil
}

func journalArgs(units []string, n int, follow bool) []string {
	args := make([]string, 0, 8+len(units)*2)
	args = append(args, "--no-pager", "-o", "short-iso")
	for _, u := range units {
		args = append(args, "-u", u)
	}
	args = append(args, "-n", strconv.Itoa(n))
	if follow {
		args = append(args, "-f")
	}
	return args
}

func parseJournalOutput(raw string, units []string) []LogLine {
	lines := strings.Split(raw, "\n")
	out := make([]LogLine, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, LogLine{
			Text: line,
			Unit: guessUnit(line, units),
		})
	}
	return out
}

func guessUnit(line string, units []string) string {
	lower := strings.ToLower(line)
	for _, u := range units {
		base := strings.TrimSuffix(strings.ToLower(u), ".service")
		base = strings.TrimSuffix(base, ".socket")
		if strings.Contains(lower, strings.ToLower(u)) || strings.Contains(lower, base+"[") {
			return u
		}
	}
	if len(units) == 1 {
		return units[0]
	}
	return ""
}
