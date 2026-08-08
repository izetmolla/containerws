package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"github.com/izetmolla/containerws/packages/proxymanager"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.GetLogsAPI)
}

func (cc *controller) GetLogsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	tail := c.Query("tail")
	if tail == "" {
		tail = "200"
	}
	settings, err := proxymanager.EnsureSettings(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out, source, err := fetchLogs(c.Context(), cc.app.DB(), settings, tail)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": fiber.Map{
				"engine":  settings.ActiveEngine,
				"runtime": runtimeOf(settings),
				"source":  source,
				"lines":   []string{},
				"text":    "",
				"error":   err.Error(),
			},
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"engine":  settings.ActiveEngine,
			"runtime": runtimeOf(settings),
			"source":  source,
			"lines":   strings.Split(strings.TrimRight(out, "\n"), "\n"),
			"text":    out,
			"error":   "",
		},
	}))
}

func runtimeOf(s *models.ProxySettings) string {
	switch s.ActiveEngine {
	case models.ProxyEngineNginx:
		return s.NginxRuntime
	case models.ProxyEngineTraefik:
		return s.TraefikRuntime
	default:
		return "in-process"
	}
}

func fetchLogs(ctx context.Context, db *gorm.DB, settings *models.ProxySettings, tail string) (text, source string, err error) {
	switch settings.ActiveEngine {
	case models.ProxyEngineFiber:
		var runs []models.ProxyApplyRun
		_ = db.Where("engine = ?", models.ProxyEngineFiber).Order("started_at desc").Limit(5).Find(&runs)
		var b strings.Builder
		b.WriteString("Fiber engine uses in-process proxying (no container access log).\n")
		b.WriteString("Recent apply runs:\n")
		for _, run := range runs {
			fmt.Fprintf(&b, "--- %s %s ---\n%s\n", run.StartedAt.Format(time.RFC3339), run.Status, run.LogText)
			if run.ErrorText != "" {
				fmt.Fprintf(&b, "ERROR: %s\n", run.ErrorText)
			}
		}
		return b.String(), "apply-runs", nil

	case models.ProxyEngineNginx:
		if settings.NginxRuntime == models.ProxyRuntimeDocker {
			return dockerLogs(ctx, db, settings.DockerEnvironmentID, settings.NginxContainerName, tail)
		}
		return hostFileOrJournal(ctx, []string{
			"/var/log/nginx/error.log",
			"/var/log/nginx/access.log",
		}, "nginx", tail)

	case models.ProxyEngineTraefik:
		if settings.TraefikRuntime == models.ProxyRuntimeDocker {
			return dockerLogs(ctx, db, settings.DockerEnvironmentID, settings.TraefikContainerName, tail)
		}
		return hostFileOrJournal(ctx, nil, settings.TraefikSystemdUnit, tail)

	default:
		return "", "", fmt.Errorf("unsupported engine %q", settings.ActiveEngine)
	}
}

func dockerLogs(ctx context.Context, db *gorm.DB, envID, name, tail string) (string, string, error) {
	if name == "" {
		return "", "docker", fmt.Errorf("container name is empty")
	}
	var env *models.DockerEnvironment
	if db != nil && strings.TrimSpace(envID) != "" {
		var row models.DockerEnvironment
		if err := db.Where("id = ?", envID).First(&row).Error; err == nil {
			env = &row
		}
	} else if db != nil {
		var row models.DockerEnvironment
		if err := db.Where("is_default = ? AND is_disabled = ?", true, false).First(&row).Error; err == nil {
			env = &row
		}
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return "", "docker", err
	}
	reader, err := cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: true,
	})
	if err != nil {
		return "", "docker:" + name, fmt.Errorf("container logs: %w", err)
	}
	defer reader.Close()
	// Docker multiplexed stream — strip 8-byte headers when present.
	raw, err := io.ReadAll(io.LimitReader(reader, 512*1024))
	if err != nil {
		return "", "docker:" + name, err
	}
	return demuxDockerLogs(raw), "docker:" + name, nil
}

func demuxDockerLogs(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// Heuristic: if first byte is 0-2 and length looks multiplexed, demux.
	var b strings.Builder
	i := 0
	for i+8 <= len(raw) {
		stream := raw[i]
		if stream > 2 {
			return string(raw)
		}
		size := int(raw[i+4])<<24 | int(raw[i+5])<<16 | int(raw[i+6])<<8 | int(raw[i+7])
		i += 8
		if size < 0 || i+size > len(raw) {
			return string(raw)
		}
		b.Write(raw[i : i+size])
		i += size
	}
	if b.Len() == 0 {
		return string(raw)
	}
	return b.String()
}

func hostFileOrJournal(ctx context.Context, files []string, unit, tail string) (string, string, error) {
	for _, f := range files {
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			text, err := tailFile(f, 200)
			return text, f, err
		}
	}
	if unit == "" {
		unit = "nginx"
	}
	if _, err := exec.LookPath("journalctl"); err == nil {
		n := tail
		cmd := exec.CommandContext(ctx, "journalctl", "-u", unit, "-n", n, "--no-pager")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), "journalctl:" + unit, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return string(out), "journalctl:" + unit, nil
	}
	dir, _ := os.Getwd()
	hint := filepath.Join(dir, "data", "proxymanager")
	return "", "host", fmt.Errorf("no log files found for %v and journalctl unavailable (checked under %s)", files, hint)
}

func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	lines := make([]string, 0, n)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return strings.Join(lines, "\n"), sc.Err()
}
