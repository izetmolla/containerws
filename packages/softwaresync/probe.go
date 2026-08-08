package softwaresync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProbeResult is the outcome of checking whether a software binary/path exists.
type ProbeResult struct {
	Present bool
	Detail  string
}

// ProbeInstalled checks whether the named catalog software is present on this host.
func ProbeInstalled(name string, serviceUnits []string) ProbeResult {
	name = strings.TrimSpace(name)
	if check, ok := probes[strings.ToLower(name)]; ok {
		return check()
	}
	// Fallback: any configured systemd unit file, or skip with unknown.
	for _, unit := range serviceUnits {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		path := filepath.Join("/etc/systemd/system", unit)
		if fileExists(path) {
			return ProbeResult{Present: true, Detail: path}
		}
		path = filepath.Join("/lib/systemd/system", unit)
		if fileExists(path) {
			return ProbeResult{Present: true, Detail: path}
		}
		path = filepath.Join("/usr/lib/systemd/system", unit)
		if fileExists(path) {
			return ProbeResult{Present: true, Detail: path}
		}
	}
	return ProbeResult{Present: false, Detail: "no probe registered for " + name}
}

var probes = map[string]func() ProbeResult{
	"go": func() ProbeResult {
		return anyPath(
			"/usr/local/go/bin/go",
			lookPath("go"),
		)
	},
	"node.js": func() ProbeResult {
		home := envHome()
		nvm := filepath.Join(home, ".nvm", "nvm.sh")
		if fileExists(nvm) {
			out, err := bashOK(fmt.Sprintf(
				`[ -s %q ] && . %q && command -v node`,
				nvm, nvm,
			))
			if err == nil && strings.TrimSpace(out) != "" {
				return ProbeResult{Present: true, Detail: strings.TrimSpace(out)}
			}
		}
		if p := lookPath("node"); p != "" {
			return ProbeResult{Present: true, Detail: p}
		}
		return ProbeResult{Present: false, Detail: "node not found"}
	},
	"docker engine": func() ProbeResult {
		return anyPath(
			"/usr/local/bin/docker",
			"/usr/bin/docker",
			lookPath("docker"),
		)
	},
	"xfce + novnc": func() ProbeResult {
		if dirExists("/usr/share/novnc") {
			return ProbeResult{Present: true, Detail: "/usr/share/novnc"}
		}
		return anyPath(
			lookPath("tigervncserver"),
			"/usr/bin/tigervncserver",
		)
	},
	"google chrome": func() ProbeResult {
		return anyPath(
			lookPath("google-chrome-stable"),
			lookPath("google-chrome"),
			lookPath("chromium"),
			lookPath("chromium-browser"),
			"/usr/local/bin/chrome-desktop",
		)
	},
	"visual studio code": func() ProbeResult {
		return anyPath(
			"/usr/share/code/code",
			"/usr/bin/code",
			"/usr/local/bin/code-desktop",
			lookPath("code"),
		)
	},
	"vs code server": func() ProbeResult {
		return anyPath(
			"/usr/local/bin/code-cli",
			"/usr/local/lib/vscode-cli/code",
			lookPath("code-cli"),
		)
	},
	"cursor": func() ProbeResult {
		return anyPath(
			lookPath("cursor"),
			"/usr/bin/cursor",
			"/usr/local/bin/cursor",
		)
	},
	"htop": func() ProbeResult {
		return anyPath(lookPath("htop"), "/usr/bin/htop")
	},
	"sysstat": func() ProbeResult {
		return anyPath(
			lookPath("mpstat"),
			lookPath("iostat"),
			"/usr/bin/mpstat",
			"/usr/bin/iostat",
		)
	},
	"iftop": func() ProbeResult {
		return anyPath(lookPath("iftop"), "/usr/sbin/iftop", "/usr/bin/iftop")
	},
	"nvtop": func() ProbeResult {
		return anyPath(lookPath("nvtop"), "/usr/bin/nvtop")
	},
	"intel-gpu-tools": func() ProbeResult {
		return anyPath(
			lookPath("intel_gpu_top"),
			"/usr/bin/intel_gpu_top",
			lookPath("gputop"),
			"/usr/bin/gputop",
		)
	},
	"radeontop": func() ProbeResult {
		return anyPath(lookPath("radeontop"), "/usr/bin/radeontop")
	},
	"gpustat": func() ProbeResult {
		return anyPath(lookPath("gpustat"), "/usr/bin/gpustat")
	},
}

func anyPath(paths ...string) ProbeResult {
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if fileExists(p) || dirExists(p) {
			return ProbeResult{Present: true, Detail: p}
		}
	}
	return ProbeResult{Present: false, Detail: "not found"}
}

func lookPath(bin string) string {
	p, err := exec.LookPath(bin)
	if err != nil {
		return ""
	}
	return p
}

func bashOK(script string) (string, error) {
	cmd := exec.Command("bash", "-lc", script)
	cmd.Env = installEnv()
	out, err := cmd.Output()
	return string(out), err
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func envHome() string {
	if h := strings.TrimSpace(os.Getenv("HOME")); h != "" {
		return h
	}
	return "/root"
}
