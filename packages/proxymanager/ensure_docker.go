package proxymanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/dockerclient"
)

// ComponentCheck is the result of verifying the active engine's dependencies.
type ComponentCheck struct {
	Engine      string   `json:"engine"`
	Runtime     string   `json:"runtime"`
	Ready       bool     `json:"ready"`
	Missing     []string `json:"missing,omitempty"`
	Details     []string `json:"details,omitempty"`
	DockerReady bool     `json:"docker_ready"`
}

// CheckComponents validates that binaries / Docker needed for settings are present.
func CheckComponents(settings *models.ProxySettings, runtime *RuntimeStatus) ComponentCheck {
	out := ComponentCheck{Ready: true}
	if settings == nil {
		out.Ready = false
		out.Missing = []string{"proxy settings"}
		return out
	}
	out.Engine = settings.ActiveEngine
	switch settings.ActiveEngine {
	case models.ProxyEngineFiber:
		out.Runtime = "in-process"
		out.Details = append(out.Details, "Fiber uses in-process reverse proxy")
		return out

	case models.ProxyEngineNginx:
		out.Runtime = settings.NginxRuntime
		if settings.NginxRuntime == models.ProxyRuntimeHost {
			if runtime == nil || !runtime.NginxInstalled {
				out.Ready = false
				out.Missing = append(out.Missing, "nginx binary")
			} else {
				out.Details = append(out.Details, "nginx binary: "+runtime.NginxBinary)
			}
			return out
		}
		return checkDockerRuntime(out, runtime, "nginx")

	case models.ProxyEngineTraefik:
		out.Runtime = settings.TraefikRuntime
		if settings.TraefikRuntime == models.ProxyRuntimeHost {
			if runtime == nil || !runtime.TraefikInstalled {
				out.Ready = false
				out.Missing = append(out.Missing, "traefik binary")
			} else {
				out.Details = append(out.Details, "traefik binary: "+runtime.TraefikBinary)
			}
			return out
		}
		return checkDockerRuntime(out, runtime, "traefik")

	default:
		out.Ready = false
		out.Missing = append(out.Missing, fmt.Sprintf("unsupported engine %q", settings.ActiveEngine))
		return out
	}
}

func checkDockerRuntime(out ComponentCheck, runtime *RuntimeStatus, label string) ComponentCheck {
	out.Runtime = models.ProxyRuntimeDocker
	if runtime != nil && runtime.DockerAvailable {
		out.DockerReady = true
		out.Details = append(out.Details, label+" docker runtime: docker available")
		return out
	}
	out.Ready = false
	out.DockerReady = false
	msg := "docker engine"
	if runtime != nil && runtime.DockerError != "" {
		msg = "docker engine (" + runtime.DockerError + ")"
	}
	out.Missing = append(out.Missing, msg)
	return out
}

// EnsureDockerRunning pings Docker and, if unavailable, best-effort starts dockerd
// (needed in direct-mode containers without systemd).
func EnsureDockerRunning(ctx context.Context) error {
	if err := pingDocker(ctx); err == nil {
		return nil
	}
	// Prefer the Softwares install helper when present (handles iptables/cgroup).
	if _, err := os.Stat("/usr/local/lib/containerws/ensure-dockerd.sh"); err == nil {
		cmd := exec.CommandContext(ctx, "/usr/local/lib/containerws/ensure-dockerd.sh")
		out, runErr := cmd.CombinedOutput()
		if runErr == nil {
			dockerclient.Reset()
			if err := pingDocker(ctx); err == nil {
				return nil
			}
		} else {
			_ = out
		}
	}
	if _, err := exec.LookPath("dockerd"); err != nil {
		return fmt.Errorf("dockerd not installed: %w", err)
	}
	if liveDockerd() {
		// Stale process without a working API — kill and restart.
		_ = exec.Command("pkill", "-x", "dockerd").Run()
		_ = exec.Command("pkill", "-x", "containerd").Run()
		time.Sleep(time.Second)
	}
	clearStaleDockerPidfiles()
	_ = os.MkdirAll("/var/run", 0o755)
	_ = os.MkdirAll("/var/log", 0o755)
	_ = os.MkdirAll("/sys/fs/cgroup/docker", 0o755)
	_ = preferDockerDaemonNoIptables()
	cmd := exec.Command(
		"dockerd",
		"--host=unix://"+dockerclient.SockPath(),
		"--pidfile=/var/run/docker-fresh.pid",
	)
	logf, err := os.OpenFile("/var/log/dockerd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	if err := cmd.Start(); err != nil {
		if logf != nil {
			_ = logf.Close()
		}
		return fmt.Errorf("start dockerd: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		if logf != nil {
			_ = logf.Close()
		}
	}()
	dockerclient.Reset()
	return waitDockerReady(ctx, 45*time.Second)
}

func preferDockerDaemonNoIptables() error {
	path := "/etc/docker/daemon.json"
	_ = os.MkdirAll("/etc/docker", 0o755)
	cfg := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	cfg["iptables"] = false
	cfg["ip-forward"] = true
	if _, ok := cfg["storage-driver"]; !ok {
		cfg["storage-driver"] = "overlay2"
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func pingDocker(ctx context.Context) error {
	cli, err := dockerclient.Client()
	if err != nil {
		return err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = cli.Ping(pingCtx)
	return err
}

func waitDockerReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		dockerclient.Reset()
		if err := pingDocker(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return fmt.Errorf("docker not ready: %w", last)
}

func liveDockerd() bool {
	out, err := exec.Command("ps", "-eo", "stat,comm").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		stat, comm := fields[0], fields[1]
		if comm == "dockerd" && !strings.Contains(stat, "Z") {
			return true
		}
	}
	return false
}

func clearStaleDockerPidfiles() {
	for _, path := range []string{
		"/var/run/docker.pid",
		"/run/docker.pid",
		"/var/run/docker-direct.pid",
		"/var/run/docker-fresh.pid",
		"/run/docker/containerd/containerd.pid",
		"/var/run/docker/containerd/containerd.pid",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid := strings.TrimSpace(string(b))
		if pidAlive(pid) {
			continue
		}
		_ = os.Remove(path)
	}
	if liveDockerd() {
		return
	}
	for _, sock := range []string{
		"/var/run/docker.sock",
		"/run/docker.sock",
		"/run/docker/containerd/containerd.sock",
		"/run/docker/containerd/containerd.sock.ttrpc",
		"/run/docker/containerd/containerd-debug.sock",
		"/var/run/docker/containerd/containerd.sock",
		"/var/run/docker/containerd/containerd.sock.ttrpc",
		"/var/run/docker/containerd/containerd-debug.sock",
	} {
		_ = os.Remove(sock)
	}
}

func pidAlive(pid string) bool {
	if pid == "" {
		return false
	}
	b, err := os.ReadFile("/proc/" + pid + "/stat")
	if err != nil {
		return false
	}
	// Format: pid (comm) state ... — comm may contain spaces/parens.
	s := string(b)
	i := strings.LastIndex(s, ") ")
	if i < 0 || i+2 >= len(s) {
		return false
	}
	state := s[i+2]
	return state != 'Z'
}
