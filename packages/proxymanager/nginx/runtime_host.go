package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
)

// ApplyHost writes are already done by Generate; this tests and reloads host nginx.
func ApplyHost(ctx context.Context, settings *models.ProxySettings, configDir string) error {
	bin := strings.TrimSpace(settings.NginxBinaryPath)
	if bin == "" {
		if p, err := exec.LookPath("nginx"); err == nil {
			bin = p
		}
	}
	if bin == "" {
		return fmt.Errorf("nginx binary not found on host; install nginx or switch runtime to docker")
	}
	conf := filepath.Join(configDir, "nginx.conf")
	if custom := strings.TrimSpace(settings.NginxConfigPath); custom != "" {
		// Copy generated main into configured path's directory if needed — use generated conf with -c
		_ = custom
	}
	test := exec.CommandContext(ctx, bin, "-t", "-c", conf)
	if out, err := test.CombinedOutput(); err != nil {
		return fmt.Errorf("nginx -t failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Prefer reload signal
	reload := exec.CommandContext(ctx, bin, "-s", "reload", "-c", conf)
	if out, err := reload.CombinedOutput(); err != nil {
		// Fallback systemctl
		unit := settings.NginxSystemdUnit
		if unit == "" {
			unit = "nginx"
		}
		if _, lookErr := exec.LookPath("systemctl"); lookErr == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "reload", unit)
			if out2, err2 := cmd.CombinedOutput(); err2 != nil {
				return fmt.Errorf("nginx reload failed: %v (%s); systemctl: %v (%s)", err, strings.TrimSpace(string(out)), err2, strings.TrimSpace(string(out2)))
			}
			return nil
		}
		return fmt.Errorf("nginx reload failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chtimes(conf, time.Now(), time.Now())
	return nil
}
