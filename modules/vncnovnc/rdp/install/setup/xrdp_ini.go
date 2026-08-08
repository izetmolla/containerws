package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const xrdpIniPath = "/etc/xrdp/xrdp.ini"

// XvncDesktopTarget describes a panel VNC session RDP should open via libvnc.
type XvncDesktopTarget struct {
	Username string
	VncPort  int
	// Address is the TigerVNC listen address (usually 127.0.0.1).
	Address string
	// RdpAddress / RdpPort are where xrdp itself should listen for this user.
	RdpAddress string
	RdpPort    int
}

// EnsureXrdpXvncConfig configures xrdp to use Xvnc (libvnc), ask for
// username/password on the RDP login dialog, and auto-run that session so the
// client goes straight to the desktop after credentials.
//
// When targets are provided (RDP-enabled users with a VNC port), RDP connects
// directly to those TigerVNC sessions (same desktop as noVNC) using the VNC
// password from the login dialog. Otherwise sesman starts Xvnc with port=-1.
func EnsureXrdpXvncConfig(targets ...XvncDesktopTarget) error {
	if _, err := os.Stat("/etc/xrdp"); err != nil {
		return nil
	}

	globals := readXrdpGlobals(xrdpIniPath)
	globals["autorun"] = "Xvnc"
	globals["hidelogwindow"] = "true"

	live := make([]XvncDesktopTarget, 0, len(targets))
	for _, t := range targets {
		u := strings.TrimSpace(t.Username)
		if u == "" || t.VncPort <= 0 {
			continue
		}
		addr := strings.TrimSpace(t.Address)
		if addr == "" || addr == "0.0.0.0" || addr == "::" {
			addr = "127.0.0.1"
		}
		live = append(live, XvncDesktopTarget{
			Username:   u,
			VncPort:    t.VncPort,
			Address:    addr,
			RdpAddress: strings.TrimSpace(t.RdpAddress),
			RdpPort:    t.RdpPort,
		})
	}

	if len(live) > 0 {
		globals["port"] = formatXrdpListenPort(live[0].RdpAddress, live[0].RdpPort)
	}

	var body strings.Builder
	body.WriteString("; ContainerWS xrdp — Xvnc desktop (credentials from RDP login)\n")
	body.WriteString("[Globals]\n")

	writeSession := func(sec, name, addr string, port int) {
		fmt.Fprintf(&body, `[%s]
name=%s
lib=libvnc.so
username=ask
password=ask
ip=%s
port=%d
xserverbpp=24

`, sec, name, addr, port)
	}

	if len(live) == 1 {
		globals["autorun"] = "Xvnc"
		writeIniMap(&body, globals)
		body.WriteString("\n[Logging]\n")
		body.WriteString("LogFile=/var/log/xrdp.log\n")
		body.WriteString("LogLevel=INFO\n")
		body.WriteString("EnableSyslog=true\n")
		body.WriteString("SyslogLevel=INFO\n")
		body.WriteString("\n")
		t := live[0]
		body.WriteString(fmt.Sprintf("; Connect to %s TigerVNC (password = VNC password)\n", t.Username))
		writeSession("Xvnc", "Xvnc", t.Address, t.VncPort)
	} else if len(live) > 1 {
		globals["autorun"] = ""
		writeIniMap(&body, globals)
		body.WriteString("\n[Logging]\n")
		body.WriteString("LogFile=/var/log/xrdp.log\n")
		body.WriteString("LogLevel=INFO\n")
		body.WriteString("EnableSyslog=true\n")
		body.WriteString("SyslogLevel=INFO\n")
		body.WriteString("\n")
		for i, t := range live {
			sec := "Xvnc"
			if i > 0 {
				sec = fmt.Sprintf("Xvnc%d", i+1)
			}
			writeSession(sec, t.Username, t.Address, t.VncPort)
		}
	} else {
		writeIniMap(&body, globals)
		body.WriteString("\n[Logging]\n")
		body.WriteString("LogFile=/var/log/xrdp.log\n")
		body.WriteString("LogLevel=INFO\n")
		body.WriteString("EnableSyslog=true\n")
		body.WriteString("SyslogLevel=INFO\n")
		body.WriteString("\n")
		body.WriteString(`[Xvnc]
name=Xvnc
lib=libvnc.so
username=ask
password=ask
ip=127.0.0.1
port=-1
xserverbpp=24
`)
	}

	if err := os.WriteFile(xrdpIniPath, []byte(body.String()), 0o644); err != nil {
		return err
	}
	_ = EnsureStartwmPrefersUserSession()
	return nil
}

func formatXrdpListenPort(addr string, port int) string {
	if port <= 0 {
		port = 3389
	}
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == "0.0.0.0" || addr == "::" || addr == "127.0.0.1" || addr == "::1" {
		return fmt.Sprintf("%d", port)
	}
	return fmt.Sprintf("tcp://%s:%d", addr, port)
}

// RestartXrdp reloads xrdp + sesman so ini changes apply.
func RestartXrdp() error {
	return StartXrdp()
}

// StartXrdp starts (or restarts) xrdp + sesman.
func StartXrdp() error {
	prepareXrdpRuntime()
	_ = exec.Command("bash", "-lc",
		"if command -v systemctl >/dev/null 2>&1 && systemctl list-units --type=service >/dev/null 2>&1; then "+
			"systemctl restart xrdp-sesman 2>/dev/null || true; "+
			"systemctl restart xrdp 2>/dev/null || systemctl start xrdp xrdp-sesman 2>/dev/null || true; "+
			"elif [ -x /etc/init.d/xrdp ]; then "+
			"/etc/init.d/xrdp restart 2>/dev/null || /etc/init.d/xrdp start 2>/dev/null || true; "+
			"elif command -v service >/dev/null 2>&1; then "+
			"service xrdp restart 2>/dev/null || service xrdp start 2>/dev/null || true; "+
			"else "+
			"pkill -x xrdp >/dev/null 2>&1 || true; pkill -x xrdp-sesman >/dev/null 2>&1 || true; "+
			"/usr/sbin/xrdp-sesman >/dev/null 2>&1 || true; /usr/sbin/xrdp >/dev/null 2>&1 || true; "+
			"fi",
	).Run()

	for range 10 {
		if isXrdpRunning() {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("xrdp did not stay running — check /var/log/xrdp.log")
}

// StopXrdp stops xrdp + sesman.
func StopXrdp() error {
	_ = exec.Command("bash", "-lc",
		"if command -v systemctl >/dev/null 2>&1 && systemctl list-units --type=service >/dev/null 2>&1; then "+
			"systemctl stop xrdp 2>/dev/null || true; "+
			"systemctl stop xrdp-sesman 2>/dev/null || true; "+
			"elif [ -x /etc/init.d/xrdp ]; then "+
			"/etc/init.d/xrdp stop 2>/dev/null || true; "+
			"elif command -v service >/dev/null 2>&1; then "+
			"service xrdp stop 2>/dev/null || true; "+
			"fi; "+
			"pkill -x xrdp >/dev/null 2>&1 || true; "+
			"pkill -x xrdp-sesman >/dev/null 2>&1 || true",
	).Run()

	for range 10 {
		if !isXrdpRunning() {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("xrdp is still running")
}

func prepareXrdpRuntime() {
	_ = os.MkdirAll("/run/xrdp", 0o755)
	_ = os.MkdirAll("/var/log", 0o755)
	for _, p := range []string{"/var/log/xrdp.log", "/var/log/xrdp-sesman.log"} {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
		if err == nil {
			_ = f.Close()
		}
		_ = os.Chmod(p, 0o666)
	}
}

func readXrdpGlobals(path string) map[string]string {
	defaults := map[string]string{
		"ini_version":                "1",
		"fork":                       "true",
		"port":                       "3389",
		"use_vsock":                  "false",
		"tcp_nodelay":                "true",
		"tcp_keepalive":              "true",
		"security_layer":             "negotiate",
		"crypt_level":                "high",
		"ssl_protocols":              "TLSv1.2, TLSv1.3",
		"allow_channels":             "true",
		"allow_multimon":             "true",
		"bitmap_cache":               "true",
		"bitmap_compression":         "true",
		"bulk_compression":           "true",
		"max_bpp":                    "32",
		"new_cursors":                "true",
		"use_fastpath":               "both",
		"autorun":                    "Xvnc",
		"hidelogwindow":              "true",
		"skip_sfu_border_check":      "true",
		"suppress_output_fullscreen": "true",
	}

	f, err := os.Open(path)
	if err != nil {
		return defaults
	}
	defer f.Close()

	inGlobals := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inGlobals = strings.EqualFold(line, "[Globals]")
			continue
		}
		if !inGlobals {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		if k == "autorun" || k == "hidelogwindow" {
			continue
		}
		defaults[k] = v
	}
	return defaults
}

func writeIniMap(b *strings.Builder, m map[string]string) {
	order := []string{
		"ini_version", "fork", "port", "use_vsock", "tcp_nodelay", "tcp_keepalive",
		"security_layer", "crypt_level", "certificate", "key_file", "ssl_protocols",
		"allow_channels", "allow_multimon", "bitmap_cache", "bitmap_compression",
		"bulk_compression", "max_bpp", "new_cursors", "use_fastpath", "autorun",
		"hidelogwindow", "skip_sfu_border_check", "suppress_output_fullscreen",
	}
	seen := map[string]struct{}{}
	for _, k := range order {
		if v, ok := m[k]; ok && v != "" {
			fmt.Fprintf(b, "%s=%s\n", k, v)
			seen[k] = struct{}{}
		} else if ok && (k == "autorun") {
			fmt.Fprintf(b, "%s=\n", k)
			seen[k] = struct{}{}
		}
	}
	for k, v := range m {
		if _, ok := seen[k]; ok {
			continue
		}
		if v == "" {
			continue
		}
		fmt.Fprintf(b, "%s=%s\n", k, v)
	}
}
