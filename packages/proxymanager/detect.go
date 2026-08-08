package proxymanager

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"gorm.io/gorm"
)

// DockerNetworkInfo is a brief network listing for the settings UI.
type DockerNetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope,omitempty"`
}

// RuntimeStatus describes detected runtimes for the status UI.
type RuntimeStatus struct {
	DockerAvailable  bool                `json:"docker_available"`
	DockerError      string              `json:"docker_error,omitempty"`
	DockerNetworks   []DockerNetworkInfo `json:"docker_networks,omitempty"`
	HostIPs          []string            `json:"host_ips,omitempty"`
	NginxBinary      string              `json:"nginx_binary,omitempty"`
	NginxInstalled   bool                `json:"nginx_installed"`
	TraefikBinary    string              `json:"traefik_binary,omitempty"`
	TraefikInstalled bool                `json:"traefik_installed"`
	SystemdAvailable bool                `json:"systemd_available"`
	ActiveEngine     string              `json:"active_engine"`
	NginxRuntime     string              `json:"nginx_runtime"`
	TraefikRuntime   string              `json:"traefik_runtime"`
	ConfigDir        string              `json:"config_dir"`
	Dirty            bool                `json:"dirty"`
	LastAppliedAt    string              `json:"last_applied_at,omitempty"`
	LastApplyError   string              `json:"last_apply_error,omitempty"`
	LastApplyEngine  string              `json:"last_apply_engine,omitempty"`
	NginxContainer   string              `json:"nginx_container,omitempty"`
	TraefikContainer string              `json:"traefik_container,omitempty"`
}

// DetectRuntime probes host binaries and Docker availability.
func DetectRuntime(ctx context.Context, db *gorm.DB) (*RuntimeStatus, error) {
	settings, err := EnsureSettings(db)
	if err != nil {
		return nil, err
	}
	st := &RuntimeStatus{
		ActiveEngine:     settings.ActiveEngine,
		NginxRuntime:     settings.NginxRuntime,
		TraefikRuntime:   settings.TraefikRuntime,
		ConfigDir:        settings.ConfigDir,
		Dirty:            settings.Dirty,
		LastApplyError:   settings.LastApplyError,
		LastApplyEngine:  settings.LastApplyEngine,
		NginxContainer:   settings.NginxContainerName,
		TraefikContainer: settings.TraefikContainerName,
		SystemdAvailable: systemdUsable(),
	}
	if settings.LastAppliedAt != nil {
		st.LastAppliedAt = settings.LastAppliedAt.UTC().Format(time.RFC3339)
	}

	nginxPath := strings.TrimSpace(settings.NginxBinaryPath)
	if nginxPath == "" {
		if p, lookErr := exec.LookPath("nginx"); lookErr == nil {
			nginxPath = p
		}
	}
	if nginxPath != "" {
		if _, statErr := os.Stat(nginxPath); statErr == nil {
			st.NginxBinary = nginxPath
			st.NginxInstalled = true
		}
	}

	traefikPath := strings.TrimSpace(settings.TraefikBinaryPath)
	if traefikPath == "" {
		if p, lookErr := exec.LookPath("traefik"); lookErr == nil {
			traefikPath = p
		}
	}
	if traefikPath != "" {
		if _, statErr := os.Stat(traefikPath); statErr == nil {
			st.TraefikBinary = traefikPath
			st.TraefikInstalled = true
		}
	}

	cli, dockerErr := dockerclient.Client()
	if dockerErr != nil {
		st.DockerAvailable = false
		st.DockerError = dockerErr.Error()
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, err := cli.Ping(pingCtx); err != nil {
			st.DockerAvailable = false
			st.DockerError = err.Error()
		} else {
			st.DockerAvailable = true
			netCtx, netCancel := context.WithTimeout(ctx, 5*time.Second)
			defer netCancel()
			if nets, nerr := cli.NetworkList(netCtx, network.ListOptions{}); nerr == nil {
				out := make([]DockerNetworkInfo, 0, len(nets))
				for _, n := range nets {
					out = append(out, DockerNetworkInfo{
						ID:     n.ID,
						Name:   n.Name,
						Driver: n.Driver,
						Scope:  n.Scope,
					})
				}
				sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
				st.DockerNetworks = out
			}
		}
	}
	st.HostIPs = listHostIPs()
	return st, nil
}

func listHostIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func systemdUsable() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	if st, err := os.Stat("/run/systemd/system"); err == nil && st.IsDir() {
		return true
	}
	out, err := exec.Command("systemctl", "is-system-running").CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err == nil {
		switch state {
		case "running", "degraded", "maintenance", "initializing", "starting":
			return true
		}
	}
	return false
}

// WriteFileAtomic writes data to path creating parent dirs.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AppendLog is a small helper for apply logs.
func AppendLog(buf *strings.Builder, format string, args ...any) {
	if buf == nil {
		return
	}
	fmt.Fprintf(buf, format+"\n", args...)
}
