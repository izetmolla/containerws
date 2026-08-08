package traefik

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/izetmolla/containerws/models"
)

// ApplyHost validates the binary and writes a runnable hint. Prefer Docker runtime for
// automatic start; host mode succeeds when systemd unit reloads, otherwise returns a
// clear actionable error without pretending the process is running.
func ApplyHost(ctx context.Context, settings *models.ProxySettings, configDir string) error {
	bin := strings.TrimSpace(settings.TraefikBinaryPath)
	if bin == "" {
		if p, err := exec.LookPath("traefik"); err == nil {
			bin = p
		}
	}
	if bin == "" {
		return fmt.Errorf("traefik binary not found on host; install traefik or switch runtime to Docker")
	}
	conf := filepath.Join(configDir, "traefik.yml")
	unit := settings.TraefikSystemdUnit
	if unit == "" {
		unit = "traefik"
	}

	hint := filepath.Join(configDir, "RUN.txt")
	_ = os.WriteFile(hint, []byte(fmt.Sprintf("Start with:\n  %s --configFile=%s\n", bin, conf)), 0o644)

	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "restart", unit)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl restart %s failed: %w (%s). Config written to %s — start manually: %s --configFile=%s",
				unit, err, strings.TrimSpace(string(out)), conf, bin, conf)
		}
		return nil
	}

	return fmt.Errorf("systemd unavailable; config written to %s. Start manually: %s --configFile=%s (or switch Traefik runtime to Docker)",
		conf, bin, conf)
}
