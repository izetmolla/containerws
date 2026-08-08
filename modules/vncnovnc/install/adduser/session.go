package adduser

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultGeometry  = "1920x1080"
	DefaultDepth     = "24"
	DefaultDPI       = "96"
	DefaultFramerate = "60"
	DefaultNovncWeb  = "/usr/local/share/containerws-novnc"
)

// StartOptions configures a per-user TigerVNC + websockify session.
type StartOptions struct {
	Username    string
	Password    string
	VncPort     int // 0 = allocate
	NoVncPort   int // 0 = allocate
	Geometry    string
	Depth       string
	DPI         string
	Framerate   string
	CreateUser  bool   // create Linux account if missing
	LinuxPasswd string // only when CreateUser

	// BindAddress is the RFB listen address (127.0.0.1 or a host interface IP).
	// VNC port remains randomly allocated; only the listen interface is chosen.
	BindAddress string

	// When true, use the TigerVNC knobs below as-is. When false, historical defaults apply.
	ServerFromProfile bool

	// TigerVNC server knobs (from VncSession when ServerFromProfile)
	LocalhostOnly        bool
	AlwaysShared         bool
	AcceptSetDesktopSize bool
	SecurityTypes        string
	CompareFB            int
	ImprovedHextile      bool
	DesktopSession       string
	WallpaperPath        string
}

// StartResult is returned after VNC + noVNC are running for the user.
type StartResult struct {
	Username  string `json:"username"`
	HomeDir   string `json:"home_dir"`
	VncPort   int    `json:"vnc_port"`
	NoVncPort int    `json:"no_vnc_port"`
	Display   int    `json:"display"`
	Address   string `json:"address"`
	NovncURL  string `json:"novnc_url"`
	Reused    bool   `json:"reused_ports"`
}

// StartUserSession prepares the Linux user desktop and starts VNC + noVNC on free ports.
func StartUserSession(opts StartOptions) (*StartResult, error) {
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return nil, fmt.Errorf("vnc password is required")
	}

	u, err := ensureLinuxUser(username, opts.CreateUser, opts.LinuxPasswd)
	if err != nil {
		return nil, err
	}

	extraUsed := map[int]struct{}{}
	var assignment PortAssignment
	reused := false

	if opts.VncPort > 0 && opts.NoVncPort > 0 {
		assignment = PortAssignment{
			Username:  username,
			VncPort:   opts.VncPort,
			NoVncPort: opts.NoVncPort,
		}
		// Persist override into map
		_ = upsertAssignment(assignment)
	} else {
		before, _ := LoadPortMap()
		for _, row := range before {
			if row.Username == username {
				reused = true
				break
			}
		}
		assignment, err = AllocateOrReusePorts(username, extraUsed)
		if err != nil {
			return nil, err
		}
	}

	geometry := orDefault(opts.Geometry, DefaultGeometry)
	depth := orDefault(opts.Depth, DefaultDepth)
	dpi := orDefault(opts.DPI, DefaultDPI)
	framerate := orDefault(opts.Framerate, DefaultFramerate)
	server := serverOptsFromStart(opts)

	display := assignment.Display
	if display <= 0 {
		display = loadDisplayFile(username)
	}
	if display <= 0 {
		// Classic 59xx mapping only when port is in that band; otherwise pick free :N.
		if assignment.VncPort >= 5901 && assignment.VncPort <= 5999 {
			display = DisplayFromVNCPort(assignment.VncPort)
		} else {
			var derr error
			display, derr = PickFreeDisplay()
			if derr != nil {
				return nil, derr
			}
		}
	}

	if err := prepareUserVNC(u, password, geometry, depth, dpi, framerate, server); err != nil {
		return nil, err
	}

	// Stop any previous instance for this user, then start on 127.0.0.1 only.
	_ = StopUserSession(username, assignment.VncPort, assignment.NoVncPort)

	if err := startVNC(username, display, assignment.VncPort, geometry, depth, dpi, framerate, server); err != nil {
		return nil, fmt.Errorf("start vnc: %w", err)
	}
	if err := startNoVNC(username, assignment.NoVncPort, assignment.VncPort, server.BindAddress); err != nil {
		_ = stopVNC(username, display)
		return nil, fmt.Errorf("start novnc: %w", err)
	}

	assignment.Display = display
	_ = upsertAssignment(assignment)
	_ = saveDisplayFile(username, display)

	bindAddr := server.BindAddress
	if bindAddr == "" {
		bindAddr = BindHost
	}
	return &StartResult{
		Username:  username,
		HomeDir:   u.HomeDir,
		VncPort:   assignment.VncPort,
		NoVncPort: assignment.NoVncPort,
		Display:   display,
		Address:   bindAddr,
		NovncURL:  fmt.Sprintf("http://%s:%d/vnc.html", BindHost, assignment.NoVncPort),
		Reused:    reused,
	}, nil
}

// StopUserSession stops VNC display and websockify for the user.
func StopUserSession(username string, vncPort, novncPort int) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if vncPort <= 0 || novncPort <= 0 {
		rows, _ := LoadPortMap()
		for _, row := range rows {
			if row.Username == username {
				vncPort = row.VncPort
				novncPort = row.NoVncPort
				break
			}
		}
	}
	var errs []string
	display := 0
	rows, _ := LoadPortMap()
	for _, row := range rows {
		if row.Username == username && row.Display > 0 {
			display = row.Display
			break
		}
	}
	if display <= 0 {
		display = loadDisplayFile(username)
	}
	if display <= 0 && vncPort >= 5901 && vncPort <= 5999 {
		display = DisplayFromVNCPort(vncPort)
	}
	if display > 0 {
		if err := stopVNC(username, display); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err := stopNoVNC(username, novncPort); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func upsertAssignment(row PortAssignment) error {
	portMu.Lock()
	defer portMu.Unlock()
	rows, err := LoadPortMap()
	if err != nil {
		return err
	}
	found := false
	for i := range rows {
		if rows[i].Username == row.Username {
			rows[i] = row
			found = true
			break
		}
	}
	if !found {
		rows = append(rows, row)
	}
	return savePortMap(rows)
}

func ensureLinuxUser(username string, create bool, linuxPass string) (*user.User, error) {
	u, err := user.Lookup(username)
	if err == nil {
		return u, nil
	}
	if !create {
		return nil, fmt.Errorf("linux user %q not found (create_user=false)", username)
	}
	cmd := exec.Command("useradd", "-m", "-s", "/bin/bash", username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("useradd %s: %w (%s)", username, err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(linuxPass) != "" {
		ch := exec.Command("chpasswd")
		ch.Stdin = strings.NewReader(username + ":" + linuxPass + "\n")
		if out, err := ch.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("chpasswd: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	return user.Lookup(username)
}

type vncServerOpts struct {
	BindAddress          string
	LocalhostOnly        bool
	AlwaysShared         bool
	AcceptSetDesktopSize bool
	SecurityTypes        string
	CompareFB            int
	ImprovedHextile      bool
	DesktopSession       string
	WallpaperPath        string
}

func serverOptsFromStart(opts StartOptions) vncServerOpts {
	bind := NormalizeBindAddress(opts.BindAddress)
	if !opts.ServerFromProfile {
		return vncServerOpts{
			BindAddress:          BindHost,
			LocalhostOnly:        true,
			AlwaysShared:         true,
			AcceptSetDesktopSize: true,
			SecurityTypes:        "VncAuth",
			CompareFB:            0,
			ImprovedHextile:      true,
			DesktopSession:       "xfce",
			WallpaperPath:        "",
		}
	}
	sec := strings.TrimSpace(opts.SecurityTypes)
	if sec == "" {
		sec = "VncAuth"
	}
	desk := strings.ToLower(strings.TrimSpace(opts.DesktopSession))
	if desk == "" {
		desk = "xfce"
	}
	localhostOnly := IsLoopbackBind(bind)
	if opts.BindAddress == "" {
		localhostOnly = opts.LocalhostOnly
		if localhostOnly {
			bind = BindHost
		}
	}
	return vncServerOpts{
		BindAddress:          bind,
		LocalhostOnly:        localhostOnly,
		AlwaysShared:         opts.AlwaysShared,
		AcceptSetDesktopSize: opts.AcceptSetDesktopSize,
		SecurityTypes:        sec,
		CompareFB:            opts.CompareFB,
		ImprovedHextile:      opts.ImprovedHextile,
		DesktopSession:       desk,
		WallpaperPath:        strings.TrimSpace(opts.WallpaperPath),
	}
}

func boolYN(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func prepareUserVNC(u *user.User, password, geometry, depth, dpi, framerate string, server vncServerOpts) error {
	cfgDir := filepath.Join(u.HomeDir, ".config", "tigervnc")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	legacy := filepath.Join(u.HomeDir, ".vnc")
	if st, err := os.Lstat(legacy); err == nil && st.Mode()&os.ModeSymlink == 0 && st.IsDir() {
		_ = copyDirBestEffort(legacy, cfgDir)
		_ = os.RemoveAll(legacy)
	}
	_ = os.Remove(legacy)
	if err := os.Symlink(cfgDir, legacy); err != nil && !os.IsExist(err) {
		// If symlink exists pointing elsewhere, replace.
		_ = os.RemoveAll(legacy)
		if err := os.Symlink(cfgDir, legacy); err != nil {
			return err
		}
	}

	if err := writeVncPasswdFile(u, password); err != nil {
		return err
	}

	desktop := server.DesktopSession
	if desktop == "" {
		desktop = "xfce"
	}
	wall := strings.TrimSpace(server.WallpaperPath)
	if err := writeDesktopSessionFiles(u, desktop, wall); err != nil {
		return err
	}

	acceptResize := "0"
	if server.AcceptSetDesktopSize {
		acceptResize = "1"
	}
	configBody := fmt.Sprintf(
		"geometry=%s\ndepth=%s\ndpi=%s\nlocalhost=%s\nAlwaysShared=%s\nAcceptSetDesktopSize=%s\nFrameRate=%s\nCompareFB=%d\nImprovedHextile=%s\nSecurityTypes=%s\n",
		geometry, depth, dpi,
		boolYN(server.LocalhostOnly),
		bool01(server.AlwaysShared),
		acceptResize,
		framerate,
		server.CompareFB,
		bool01(server.ImprovedHextile),
		server.SecurityTypes,
	)
	if err := os.WriteFile(filepath.Join(cfgDir, "config"), []byte(configBody), 0o644); err != nil {
		return err
	}

	xresources := fmt.Sprintf("Xft.dpi: %s\nXft.antialias: 1\nXft.hinting: 1\nXft.hintstyle: hintslight\nXft.rgba: rgb\n", dpi)
	_ = os.WriteFile(filepath.Join(u.HomeDir, ".Xresources"), []byte(xresources), 0o644)

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	_ = chownRecursive(filepath.Join(u.HomeDir, ".config"), uid, gid)
	_ = os.Chown(legacy, uid, gid)
	_ = os.Chown(filepath.Join(u.HomeDir, ".Xresources"), uid, gid)
	return nil
}

// ApplyUserDesktopSession writes TigerVNC xstartup and ~/.xsession so VNC and
// RDP (xrdp startwm) launch the same desktop when the client does not pick one.
func ApplyUserDesktopSession(username, desktop, wallpaperPath string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("linux user %q not found: %w", username, err)
	}
	desktop = strings.ToLower(strings.TrimSpace(desktop))
	if desktop == "" {
		desktop = "xfce"
	}
	return writeDesktopSessionFiles(u, desktop, wallpaperPath)
}

func writeDesktopSessionFiles(u *user.User, desktop, wallpaperPath string) error {
	cfgDir := filepath.Join(u.HomeDir, ".config", "tigervnc")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	body := desktopSessionScript(desktop, wallpaperPath)
	xstartup := filepath.Join(cfgDir, "xstartup")
	if err := os.WriteFile(xstartup, []byte(body), 0o755); err != nil {
		return err
	}
	// xrdp startwm prefers ~/.xsession when present (same desktop as VNC).
	xsession := filepath.Join(u.HomeDir, ".xsession")
	if err := os.WriteFile(xsession, []byte(body), 0o755); err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	_ = os.Chown(xstartup, uid, gid)
	_ = os.Chown(xsession, uid, gid)
	_ = os.Chown(cfgDir, uid, gid)
	return nil
}

func desktopStartCommand(desktop string) string {
	switch desktop {
	case "gnome":
		return "gnome-session"
	case "mate":
		return "mate-session"
	case "lxde":
		return "startlxde"
	case "xfce":
		return "startxfce4"
	default:
		return "start" + desktop
	}
}

func desktopSessionScript(desktop, wallpaperPath string) string {
	desktop = strings.ToLower(strings.TrimSpace(desktop))
	if desktop == "" {
		desktop = "xfce"
	}
	wall := strings.TrimSpace(wallpaperPath)
	if wall == "" || !pathExists(wall) {
		wall = "/usr/share/backgrounds/containerws/desktop.jpg"
	}
	wallQ := shellQuote(wall)
	startCmd := desktopStartCommand(desktop)

	if desktop == "xfce" {
		return fmt.Sprintf(`#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
export XDG_SESSION_TYPE=x11
export XDG_CURRENT_DESKTOP=XFCE
export DESKTOP_SESSION=xfce
export FREETYPE_PROPERTIES="truetype:interpreter-version=40"
[ -r "$HOME/.Xresources" ] && xrdb "$HOME/.Xresources"
(
  sleep 2
  xfconf-query -c xsettings -p /Xft/Antialias -n -t int -s 1 2>/dev/null || true
  xfconf-query -c xfwm4 -p /general/use_compositing -n -t bool -s false 2>/dev/null || true
  xfconf-query -c xfce4-power-manager -p /xfce4-power-manager/dpms-enabled -s false 2>/dev/null || true
  xfconf-query -c xfce4-screensaver -p /saver/enabled -s false 2>/dev/null || true
  /usr/local/bin/containerws-set-wallpaper %s 2>/dev/null || true
) >/dev/null 2>&1 &
if [ "$#" -gt 0 ]; then
  exec "$@"
fi
if command -v dbus-launch >/dev/null 2>&1; then
  exec dbus-launch --exit-with-session startxfce4
fi
exec startxfce4
`, wallQ)
	}

	return fmt.Sprintf(`#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
export XDG_SESSION_TYPE=x11
export XDG_CURRENT_DESKTOP=%s
export DESKTOP_SESSION=%s
[ -r "$HOME/.Xresources" ] && xrdb "$HOME/.Xresources"
if [ "$#" -gt 0 ]; then
  exec "$@"
fi
if command -v dbus-launch >/dev/null 2>&1; then
  exec dbus-launch --exit-with-session %s
fi
exec %s
`, strings.ToUpper(desktop), desktop, startCmd, startCmd)
}

func startVNC(username string, display, rfbPort int, geometry, depth, dpi, framerate string, server vncServerOpts) error {
	helper := "/usr/local/bin/containerws-vnc-user-start"
	var cmd *exec.Cmd
	if _, err := os.Stat(helper); err == nil {
		// Helper historically binds localhost only; pass bind via env for newer helpers.
		cmd = exec.Command(helper, username, strconv.Itoa(display), strconv.Itoa(rfbPort), geometry, depth, dpi, framerate)
		cmd.Env = append(os.Environ(),
			"CWS_VNC_BIND="+NormalizeBindAddress(server.BindAddress),
			"CWS_VNC_LOCALHOST="+boolYN(server.LocalhostOnly),
		)
	} else {
		bind := NormalizeBindAddress(server.BindAddress)
		localhostFlag := "yes"
		interfaceFlag := ""
		if !server.LocalhostOnly && !IsLoopbackBind(bind) {
			localhostFlag = "no"
			interfaceFlag = "-interface " + shellQuote(bind)
		}
		alwaysShared := ""
		if server.AlwaysShared {
			alwaysShared = "-AlwaysShared"
		}
		acceptSize := ""
		if server.AcceptSetDesktopSize {
			acceptSize = "-AcceptSetDesktopSize"
		}
		// Inline fallback: bind RFB via -rfbport / -localhost / -interface.
		script := fmt.Sprintf(`
set -euo pipefail
HOME_DIR="$(getent passwd %s | cut -d: -f6)"
VNC_BIN=vncserver
command -v tigervncserver >/dev/null 2>&1 && VNC_BIN=tigervncserver
runuser -u %s -- env HOME="$HOME_DIR" USER=%s "$VNC_BIN" -kill :%d >/dev/null 2>&1 || true
rm -f /tmp/.X%d-lock /tmp/.X11-unix/X%d
runuser -u %s -- env HOME="$HOME_DIR" USER=%s "$VNC_BIN" :%d \
  -rfbport %d \
  -geometry %s -depth %s -dpi %s -localhost %s %s %s %s \
  -FrameRate %s -CompareFB %d -SecurityTypes %s \
  -xstartup "$HOME_DIR/.config/tigervnc/xstartup"
`, shellQuote(username), shellQuote(username), shellQuote(username), display,
			display, display,
			shellQuote(username), shellQuote(username), display, rfbPort,
			shellQuote(geometry), shellQuote(depth), shellQuote(dpi), shellQuote(localhostFlag),
			interfaceFlag, alwaysShared, acceptSize,
			shellQuote(framerate), server.CompareFB, shellQuote(server.SecurityTypes))
		cmd = exec.Command("bash", "-lc", script)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func stopVNC(username string, display int) error {
	helper := "/usr/local/bin/containerws-vnc-user-stop"
	var cmd *exec.Cmd
	if _, err := os.Stat(helper); err == nil {
		cmd = exec.Command(helper, username, strconv.Itoa(display))
	} else {
		script := fmt.Sprintf(`
HOME_DIR="$(getent passwd %s | cut -d: -f6)"
VNC_BIN=vncserver
command -v tigervncserver >/dev/null 2>&1 && VNC_BIN=tigervncserver
[ -n "$HOME_DIR" ] && runuser -u %s -- env HOME="$HOME_DIR" USER=%s "$VNC_BIN" -kill :%d >/dev/null 2>&1 || true
rm -f /tmp/.X%d-lock /tmp/.X11-unix/X%d
`, shellQuote(username), shellQuote(username), shellQuote(username), display, display, display)
		cmd = exec.Command("bash", "-lc", script)
	}
	_, _ = cmd.CombinedOutput()
	return nil
}

func startNoVNC(username string, novncPort, vncPort int, vncBind string) error {
	dir := filepath.Join(sessionDir(), username)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "novnc.log")
	pidPath := filepath.Join(dir, "novnc.pid")

	web := DefaultNovncWeb
	if _, err := os.Stat(filepath.Join(web, "vnc.html")); err != nil {
		web = "/usr/share/novnc"
	}

	vncHost := NormalizeBindAddress(vncBind)
	// Panel proxy always talks to noVNC on loopback; RFB target follows the VNC bind.
	helper := "/usr/local/bin/containerws-novnc-user"
	var cmd *exec.Cmd
	if _, err := os.Stat(helper); err == nil && IsLoopbackBind(vncHost) {
		cmd = exec.Command(helper, strconv.Itoa(novncPort), strconv.Itoa(vncPort), web)
	} else {
		cmd = exec.Command("websockify", "--web="+web,
			net.JoinHostPort(BindHost, strconv.Itoa(novncPort)),
			net.JoinHostPort(vncHost, strconv.Itoa(vncPort)),
		)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return err
	}
	// Detach: allow process to outlive this request.
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	// Brief wait for listen on 127.0.0.1
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !portIsFreeLocal(novncPort) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("novnc did not listen on %s:%d (see %s)", BindHost, novncPort, logPath)
}

func stopNoVNC(username string, novncPort int) error {
	pidPath := filepath.Join(sessionDir(), username, "novnc.pid")
	if data, err := os.ReadFile(pidPath); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
		_ = os.Remove(pidPath)
	}
	// Also kill by port if still listening
	if novncPort > 0 && !portIsFreeLocal(novncPort) {
		_ = exec.Command("bash", "-lc", fmt.Sprintf(
			`fuser -k %d/tcp >/dev/null 2>&1 || true`, novncPort,
		)).Run()
	}
	return nil
}

// WriteUserVncPassword writes TigerVNC passwd for the Linux user (no restart).
func WriteUserVncPassword(username, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("linux user %q not found: %w", username, err)
	}
	cfgDir := filepath.Join(u.HomeDir, ".config", "tigervnc")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	legacy := filepath.Join(u.HomeDir, ".vnc")
	if _, err := os.Lstat(legacy); err != nil {
		_ = os.Symlink(cfgDir, legacy)
	}
	return writeVncPasswdFile(u, password)
}

// ApplyUserVncPassword updates the user's TigerVNC passwd and restarts VNC/noVNC
// on the given ports so the new password is live for that session.
func ApplyUserVncPassword(username, password string, vncPort, novncPort int) (*StartResult, error) {
	if err := WriteUserVncPassword(username, password); err != nil {
		return nil, err
	}
	return StartUserSession(StartOptions{
		Username:  username,
		Password:  password,
		VncPort:   vncPort,
		NoVncPort: novncPort,
	})
}

func writeVncPasswdFile(u *user.User, password string) error {
	cfgDir := filepath.Join(u.HomeDir, ".config", "tigervnc")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	passFile := filepath.Join(cfgDir, "passwd")
	cmd := exec.Command("bash", "-lc", fmt.Sprintf("printf '%%s\\n' %s | vncpasswd -f > %s", shellQuote(password), shellQuote(passFile)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncpasswd: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(passFile, 0o600)
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	_ = os.Chown(passFile, uid, gid)
	_ = os.Chown(cfgDir, uid, gid)
	return nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func pathExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// UserWallpaperPath is the on-disk wallpaper file for a Linux username.
func UserWallpaperPath(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	return filepath.Join(sessionDir(), username, "wallpaper.jpg")
}

// ApplyWallpaper runs containerws-set-wallpaper as the user (best-effort if desktop live).
func ApplyWallpaper(username, wallPath string) error {
	username = strings.TrimSpace(username)
	wallPath = strings.TrimSpace(wallPath)
	if username == "" || wallPath == "" {
		return fmt.Errorf("username and wallpaper path required")
	}
	if !pathExists(wallPath) {
		return fmt.Errorf("wallpaper file not found")
	}
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	display := loadDisplayFile(username)
	displayEnv := ""
	if display > 0 {
		displayEnv = fmt.Sprintf("DISPLAY=:%d ", display)
	}
	script := fmt.Sprintf(
		`runuser -u %s -- env HOME=%s USER=%s %s/usr/local/bin/containerws-set-wallpaper %s`,
		shellQuote(username), shellQuote(u.HomeDir), shellQuote(username), displayEnv, shellQuote(wallPath),
	)
	cmd := exec.Command("bash", "-lc", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func copyDirBestEffort(src, dst string) error {
	return exec.Command("bash", "-lc", fmt.Sprintf("cp -a %s/. %s/ 2>/dev/null || true", shellQuote(src), shellQuote(dst))).Run()
}

func chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chown(p, uid, gid)
		return nil
	})
}
