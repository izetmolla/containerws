package setup

import (
	"os"
	"strings"
)

// pathFileExists is shared with platform status checks.
func pathFileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// BuildSetupScript returns a bash install script for the detected host.
// It installs packages and shared helpers but does NOT enable or start systemd services.
func BuildSetupScript(plan HostPlan) (string, error) {
	if !plan.Supported {
		return "", errUnsupported(plan)
	}
	switch plan.Family {
	case FamilyDebian:
		return debianSetupScript(plan), nil
	case FamilyRHEL:
		return rhelSetupScript(plan), nil
	case FamilyArch:
		return archSetupScript(plan), nil
	default:
		return "", errUnsupported(plan)
	}
}

func errUnsupported(plan HostPlan) error {
	return &UnsupportedError{
		Distro:  plan.Distro,
		ID:      plan.DistroID,
		Version: plan.DistroVersion,
		OS:      plan.OS,
	}
}

// UnsupportedError is returned when the host OS/family cannot be installed.
type UnsupportedError struct {
	Distro  string
	ID      string
	Version string
	OS      string
}

func (e *UnsupportedError) Error() string {
	return "unsupported OS for VNC/noVNC setup: " + e.OS + " / " + e.Distro + " (" + e.ID + " " + e.Version + ")"
}

func debianSetupScript(plan HostPlan) string {
	pkgs := strings.Join(plan.Packages, " ")
	optional := strings.Join(plan.OptionalPackages, " ")
	header := setupHeader(plan)
	return header + `
export DEBIAN_FRONTEND=noninteractive

echo "==> apt-get update"
apt-get update

echo "==> Installing required packages"
apt-get install --no-install-recommends -y ` + pkgs + `

echo "==> Installing optional packages (best-effort)"
apt-get install --no-install-recommends -y ` + optional + ` 2>/dev/null || true

` + sharedPostInstall()
}

func rhelSetupScript(plan HostPlan) string {
	mgr := plan.PackageManager
	if mgr == "" {
		mgr = "dnf"
	}
	pkgs := strings.Join(plan.Packages, " ")
	optional := strings.Join(plan.OptionalPackages, " ")
	header := setupHeader(plan)
	return header + `
echo "==> ${PKG_MGR} install"
PKG_MGR="` + mgr + `"
$PKG_MGR -y install ` + pkgs + ` || $PKG_MGR -y install ` + pkgs + `

echo "==> Optional packages (best-effort)"
$PKG_MGR -y install ` + optional + ` 2>/dev/null || true

# Ensure core XFCE binaries exist even if package names drifted.
if ! command -v xfwm4 >/dev/null 2>&1 || ! command -v xfdesktop >/dev/null 2>&1 || ! command -v Thunar >/dev/null 2>&1; then
  echo "==> Completing XFCE desktop components"
  $PKG_MGR -y install xfwm4 xfdesktop Thunar xfce4-panel xfce4-settings xfce4-appfinder 2>/dev/null || true
fi

` + sharedPostInstall()
}

func archSetupScript(plan HostPlan) string {
	pkgs := strings.Join(plan.Packages, " ")
	header := setupHeader(plan)
	return header + `
echo "==> pacman -Sy --noconfirm"
pacman -Sy --noconfirm --needed ` + pkgs + `

` + sharedPostInstall()
}

func setupHeader(plan HostPlan) string {
	return `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export PATH="/usr/local/bin:$PATH"

STATE_DIR=/config/containerws
NOVNC_WEB_ROOT=/usr/local/share/containerws-novnc
WALLPAPER_DIR=/usr/share/backgrounds/containerws
WALLPAPER="${WALLPAPER_DIR}/desktop.jpg"
WALLPAPER_URL="${WALLPAPER_URL:-https://images.unsplash.com/photo-1506905925346-21bda4d32df4?auto=format&fit=crop&w=2560&q=85}"
WALLPAPER_URL_FALLBACK="${WALLPAPER_URL_FALLBACK:-https://images.unsplash.com/photo-1469474968028-56623f02e42e?auto=format&fit=crop&w=2560&q=85}"

mkdir -p "$HOME" "$STATE_DIR" /etc/containerws /var/log/containerws "$STATE_DIR/vnc-sessions"

echo "========================================"
echo " ContainerWS VNC/noVNC setup (packages only)"
echo " Distro:  ` + shellEscape(plan.Distro) + `"
echo " ID:      ` + shellEscape(plan.DistroID) + `"
echo " Version: ` + shellEscape(plan.DistroVersion) + `"
echo " Arch:    ` + shellEscape(plan.Arch) + `"
echo " Device:  ` + shellEscape(plan.DeviceType) + `"
echo " Family:  ` + shellEscape(string(plan.Family)) + `"
echo " Manager: ` + shellEscape(plan.PackageManager) + `"
echo " NOTE:    systemd units are NOT enabled or started"
echo "========================================"
`
}

func sharedPostInstall() string {
	return `
NOVNC_SRC=""
if [ -f /usr/share/novnc/vnc.html ] || [ -f /usr/share/novnc/vnc_lite.html ]; then
  NOVNC_SRC=/usr/share/novnc
fi
if [ -z "$NOVNC_SRC" ] && [ -d /usr/share/novnc ]; then
  NOVNC_SRC=/usr/share/novnc
fi
if [ -z "$NOVNC_SRC" ]; then
  echo "ERROR: noVNC web files not found under /usr/share/novnc" >&2
  ls -la /usr/share/novnc 2>&1 || true
  exit 1
fi
echo "    noVNC package root: $NOVNC_SRC"

mkdir -p "$NOVNC_WEB_ROOT"
find "$NOVNC_WEB_ROOT" -mindepth 1 -maxdepth 1 ! -name index.html -exec rm -rf {} + 2>/dev/null || true
for item in "$NOVNC_SRC"/*; do
  [ -e "$item" ] || continue
  base="$(basename "$item")"
  [ "$base" = "index.html" ] && continue
  ln -sfn "$item" "$NOVNC_WEB_ROOT/$base"
done
cat > "$NOVNC_WEB_ROOT/index.html" <<'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="0;url=vnc.html?autoconnect=true&reconnect=true&reconnect_delay=2000&resize=remote&quality=9&compression=0&show_dot=false">
  <title>ContainerWS Desktop</title>
  <script>
    location.replace(
      "vnc.html?autoconnect=true&reconnect=true&reconnect_delay=2000&resize=remote&quality=9&compression=0&show_dot=false"
    );
  </script>
</head>
<body>
  <p>Opening high-quality desktop…
    <a href="vnc.html?autoconnect=true&reconnect=true&resize=remote&quality=9&compression=0">Continue</a>
  </p>
</body>
</html>
EOF
echo "    noVNC web root: $NOVNC_WEB_ROOT"

echo "==> Installing desktop wallpaper (best-effort)"
mkdir -p "$WALLPAPER_DIR" "${STATE_DIR}/wallpapers"
download_wallpaper() {
  local url="$1"
  local dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 20 --max-time 120 -o "$dest" "$url"
  else
    wget -q --timeout=120 -O "$dest" "$url"
  fi
}
if [ ! -s "$WALLPAPER" ] || [ "${WALLPAPER_FORCE:-0}" = "1" ]; then
  tmpwp="$(mktemp /tmp/cws-wallpaper.XXXXXX.jpg)"
  if download_wallpaper "$WALLPAPER_URL" "$tmpwp" \
    || download_wallpaper "$WALLPAPER_URL_FALLBACK" "$tmpwp"; then
    if [ "$(wc -c < "$tmpwp")" -gt 50000 ]; then
      mv -f "$tmpwp" "$WALLPAPER"
      cp -f "$WALLPAPER" "${STATE_DIR}/wallpapers/desktop.jpg"
      echo "    wallpaper: $WALLPAPER"
    else
      echo "    WARN: downloaded wallpaper looks invalid; keeping defaults" >&2
      rm -f "$tmpwp"
    fi
  else
    echo "    WARN: could not download wallpaper; XFCE default will be used" >&2
    rm -f "$tmpwp"
  fi
else
  echo "    wallpaper already present: $WALLPAPER"
fi

cat > /usr/local/bin/containerws-set-wallpaper <<'EOF'
#!/bin/bash
set -euo pipefail
WALL="${1:-/usr/share/backgrounds/containerws/desktop.jpg}"
[ -s "$WALL" ] || exit 0
apply_prop() {
  local path="$1" type="$2" value="$3"
  if xfconf-query -c xfce4-desktop -p "$path" >/dev/null 2>&1; then
    xfconf-query -c xfce4-desktop -p "$path" -s "$value" 2>/dev/null || true
  else
    xfconf-query -c xfce4-desktop -p "$path" -n -t "$type" -s "$value" 2>/dev/null || true
  fi
}
for mon in \
  monitor0 monitor1 \
  monitorVNC-0 monitorVNC-1 \
  monitorNone-0 monitorNone-1 \
  monitorVirtual-0 \
  monitorHDMI-1 monitorDP-1 monitoreDP-1
do
  apply_prop "/backdrop/screen0/${mon}/workspace0/last-image" string "$WALL"
  apply_prop "/backdrop/screen0/${mon}/workspace0/image-style" int 5
  apply_prop "/backdrop/screen0/${mon}/workspace0/image-path" string "$WALL"
  apply_prop "/backdrop/screen0/${mon}/last-image" string "$WALL"
  apply_prop "/backdrop/screen0/${mon}/image-style" int 5
done
if command -v xfconf-query >/dev/null 2>&1; then
  while IFS= read -r path; do
    case "$path" in
      *last-image|*image-path) apply_prop "$path" string "$WALL" ;;
      *image-style) apply_prop "$path" int 5 ;;
    esac
  done < <(xfconf-query -c xfce4-desktop -l 2>/dev/null | grep -E 'last-image$|image-path$|image-style$' || true)
fi
xfdesktop --reload 2>/dev/null || true
EOF
chmod +x /usr/local/bin/containerws-set-wallpaper

# Multi-user start/stop helpers (invoked by adduser API — not systemd).
cat > /usr/local/bin/containerws-vnc-user-start <<'EOF'
#!/bin/bash
set -euo pipefail
# Usage: containerws-vnc-user-start <username> <display> <rfbport> [geometry] [depth] [dpi] [framerate]
# Binds RFB to 127.0.0.1 only (-localhost yes -rfbport).
UNAME="${1:?username required}"
DISPLAY_NUM="${2:?display required}"
RFB_PORT="${3:?rfbport required}"
GEOMETRY="${4:-1920x1080}"
DEPTH="${5:-24}"
DPI="${6:-96}"
FRAMERATE="${7:-60}"

HOME_DIR="$(getent passwd "$UNAME" | cut -d: -f6)"
[ -n "$HOME_DIR" ] || { echo "user $UNAME has no home" >&2; exit 1; }

VNC_BIN="vncserver"
command -v tigervncserver >/dev/null 2>&1 && VNC_BIN="tigervncserver"

mkdir -p "$HOME_DIR/.config/tigervnc"
if [ -d "$HOME_DIR/.vnc" ] && [ ! -L "$HOME_DIR/.vnc" ]; then
  cp -a "$HOME_DIR/.vnc/." "$HOME_DIR/.config/tigervnc/" 2>/dev/null || true
  rm -rf "$HOME_DIR/.vnc"
fi
ln -sfn "$HOME_DIR/.config/tigervnc" "$HOME_DIR/.vnc"
chown -R "$UNAME":"$UNAME" "$HOME_DIR/.config/tigervnc" "$HOME_DIR/.vnc"

runuser -u "$UNAME" -- env HOME="$HOME_DIR" USER="$UNAME" "$VNC_BIN" -kill ":${DISPLAY_NUM}" >/dev/null 2>&1 || true
rm -f "/tmp/.X${DISPLAY_NUM}-lock" "/tmp/.X11-unix/X${DISPLAY_NUM}" \
  "$HOME_DIR/.config/tigervnc/"*":${DISPLAY_NUM}.pid" \
  "$HOME_DIR/.config/tigervnc/"*":${DISPLAY_NUM}.log" 2>/dev/null || true

runuser -u "$UNAME" -- env HOME="$HOME_DIR" USER="$UNAME" "$VNC_BIN" ":${DISPLAY_NUM}" \
  -rfbport "${RFB_PORT}" \
  -geometry "${GEOMETRY}" \
  -depth "${DEPTH}" \
  -dpi "${DPI}" \
  -localhost yes \
  -AlwaysShared \
  -AcceptSetDesktopSize \
  -FrameRate "${FRAMERATE}" \
  -CompareFB 0 \
  -SecurityTypes VncAuth \
  -xstartup "$HOME_DIR/.config/tigervnc/xstartup"
EOF
chmod +x /usr/local/bin/containerws-vnc-user-start

cat > /usr/local/bin/containerws-vnc-user-stop <<'EOF'
#!/bin/bash
UNAME="${1:?username required}"
DISPLAY_NUM="${2:?display required}"
HOME_DIR="$(getent passwd "$UNAME" | cut -d: -f6)"
VNC_BIN="vncserver"
command -v tigervncserver >/dev/null 2>&1 && VNC_BIN="tigervncserver"
if [ -n "$HOME_DIR" ]; then
  runuser -u "$UNAME" -- env HOME="$HOME_DIR" USER="$UNAME" "$VNC_BIN" -kill ":${DISPLAY_NUM}" >/dev/null 2>&1 || true
fi
rm -f "/tmp/.X${DISPLAY_NUM}-lock" "/tmp/.X11-unix/X${DISPLAY_NUM}" 2>/dev/null || true
exit 0
EOF
chmod +x /usr/local/bin/containerws-vnc-user-stop

cat > /usr/local/bin/containerws-novnc-user <<'EOF'
#!/bin/bash
set -euo pipefail
# Usage: containerws-novnc-user <novnc_port> <vnc_port> [web_root]
# Binds websockify to 127.0.0.1 only (not public).
NOVNC_PORT="${1:?novnc port required}"
VNC_PORT="${2:?vnc port required}"
WEB_ROOT="${3:-/usr/local/share/containerws-novnc}"
if [ ! -f "$WEB_ROOT/vnc.html" ] && [ ! -f "$WEB_ROOT/vnc_lite.html" ]; then
  WEB_ROOT=/usr/share/novnc
fi
exec websockify --web="${WEB_ROOT}" "127.0.0.1:${NOVNC_PORT}" "127.0.0.1:${VNC_PORT}"
EOF
chmod +x /usr/local/bin/containerws-novnc-user

# Mark setup complete — services are intentionally NOT started.
printf '%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${STATE_DIR}/vnc-setup-complete"
cat > "${STATE_DIR}/vnc-setup.json" <<EOF
{"completed_at":"$(date -u +%Y-%m-%dT%H:%M:%SZ)","novnc_web_root":"${NOVNC_WEB_ROOT}","services_started":false}
EOF

echo ""
echo "========================================"
echo " XFCE + TigerVNC + noVNC packages ready"
echo "========================================"
echo " Web root: $NOVNC_WEB_ROOT"
echo " Helpers:  containerws-vnc-user-start / containerws-vnc-user-stop / containerws-novnc-user"
echo " Services: NOT enabled — use /api/vnc-novnc/install/adduser to start per-user sessions"
echo "========================================"
`
}

func shellEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "$", "")
	return s
}
