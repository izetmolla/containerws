package setup

import (
	"fmt"
	"os"
	"strings"
)

func pathFileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func BuildSetupScript(plan HostPlan) (string, error) {
	return BuildSetupScriptOpts(plan, false)
}

func BuildSetupScriptOpts(plan HostPlan, reinstall bool) (string, error) {
	if !plan.Supported {
		return "", &UnsupportedError{
			Distro:  plan.Distro,
			ID:      plan.DistroID,
			Version: plan.DistroVersion,
			OS:      plan.OS,
		}
	}
	switch plan.Family {
	case FamilyDebian:
		return debianScript(plan, reinstall), nil
	case FamilyRHEL:
		return rhelScript(plan, reinstall), nil
	case FamilyArch:
		return archScript(plan, reinstall), nil
	default:
		return "", &UnsupportedError{
			Distro:  plan.Distro,
			ID:      plan.DistroID,
			Version: plan.DistroVersion,
			OS:      plan.OS,
		}
	}
}

type UnsupportedError struct {
	Distro  string
	ID      string
	Version string
	OS      string
}

func (e *UnsupportedError) Error() string {
	return "unsupported OS for RDP setup: " + e.OS + " / " + e.Distro + " (" + e.ID + " " + e.Version + ")"
}

func pkgList(pkgs []string) string {
	return strings.Join(pkgs, " ")
}

func debianScript(plan HostPlan, reinstall bool) string {
	pkgs := pkgList(plan.Packages)
	optional := pkgList(plan.OptionalPackages)
	title := "Debian-family"
	installFlag := ""
	if reinstall {
		title = "Debian-family (force reinstall)"
		installFlag = " --reinstall"
	}
	return `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
echo "==> ContainerWS RDP (xrdp) setup — ` + title + `"
echo "==> apt-get update"
apt-get update
echo "==> apt-get install` + installFlag + `"
apt-get install` + installFlag + ` --no-install-recommends -y ` + pkgs + `
` + optionalInstall("apt-get install --no-install-recommends -y "+optional) + `
` + commonPostInstall()
}

func rhelScript(plan HostPlan, reinstall bool) string {
	pkgs := pkgList(plan.Packages)
	mgr := plan.PackageManager
	if mgr == "" {
		mgr = "dnf"
	}
	reinstallHint := ""
	if reinstall {
		reinstallHint = `
echo "==> Force reinstall (best-effort)"
$PKG_MGR -y reinstall ` + pkgs + ` 2>/dev/null || $PKG_MGR -y install ` + pkgs + ` || true
`
	}
	return `#!/bin/bash
set -euo pipefail
PKG_MGR=` + mgr + `
echo "==> ContainerWS RDP (xrdp) setup — RHEL-family ($PKG_MGR)"
if command -v dnf >/dev/null 2>&1; then
  dnf -y install epel-release 2>/dev/null || true
fi
echo "==> ${PKG_MGR} install"
$PKG_MGR -y install ` + pkgs + ` || $PKG_MGR -y install ` + pkgs + `
` + reinstallHint + commonPostInstall()
}

func archScript(plan HostPlan, reinstall bool) string {
	pkgs := pkgList(plan.Packages)
	reinstallHint := ""
	if reinstall {
		reinstallHint = "pacman -S --noconfirm --needed " + pkgs + "\n"
	}
	return `#!/bin/bash
set -euo pipefail
echo "==> ContainerWS RDP (xrdp) setup — Arch"
pacman -Sy --noconfirm ` + pkgs + `
` + reinstallHint + commonPostInstall()
}

func optionalInstall(line string) string {
	if strings.TrimSpace(strings.TrimPrefix(line, "apt-get install --no-install-recommends -y")) == "" {
		return ""
	}
	return fmt.Sprintf("%s 2>/dev/null || true\n", line)
}

func commonPostInstall() string {
	return `
echo "==> Configuring xrdp for Xvnc (credentials from RDP login → desktop)"
STARTWM=/etc/xrdp/startwm.sh
cat > /etc/xrdp/startwm.sh <<'EOF'
#!/bin/sh
if [ -r /etc/default/locale ]; then
  . /etc/default/locale
  export LANG LANGUAGE
fi
unset DBUS_SESSION_BUS_ADDRESS
unset SESSION_MANAGER

# Same desktop as the user's VNC profile when no RDP session type is chosen
if [ -x "$HOME/.xsession" ]; then
  exec "$HOME/.xsession"
fi
if [ -r "$HOME/.xsession" ]; then
  . "$HOME/.xsession"
  exit
fi

if command -v startxfce4 >/dev/null 2>&1; then
  exec startxfce4
fi
if command -v xfce4-session >/dev/null 2>&1; then
  exec xfce4-session
fi
test -x /etc/X11/Xsession && exec /etc/X11/Xsession
exec /bin/sh /etc/X11/xinit/xinitrc
EOF
chmod +x /etc/xrdp/startwm.sh

# Default Xvnc session: ask username/password, auto-run (no session picker)
if [ -d /etc/xrdp ]; then
  cat > /etc/xrdp/xrdp.ini <<'EOF'
; ContainerWS xrdp — Xvnc desktop (credentials from RDP login)
[Globals]
ini_version=1
fork=true
port=3389
tcp_nodelay=true
tcp_keepalive=true
security_layer=negotiate
crypt_level=high
allow_channels=true
bitmap_cache=true
bitmap_compression=true
bulk_compression=true
max_bpp=32
use_fastpath=both
autorun=Xvnc
hidelogwindow=true

[Logging]
LogFile=xrdp.log
LogLevel=INFO
EnableSyslog=true
SyslogLevel=INFO

[Xvnc]
name=Xvnc
lib=libvnc.so
username=ask
password=ask
ip=127.0.0.1
port=-1
xserverbpp=24
EOF
fi

echo "==> Enabling xrdp service (best-effort)"
if command -v systemctl >/dev/null 2>&1; then
  systemctl enable xrdp 2>/dev/null || true
  systemctl restart xrdp 2>/dev/null || systemctl start xrdp 2>/dev/null || true
  systemctl enable xrdp-sesman 2>/dev/null || true
  systemctl restart xrdp-sesman 2>/dev/null || systemctl start xrdp-sesman 2>/dev/null || true
else
  service xrdp restart 2>/dev/null || service xrdp start 2>/dev/null || true
fi

echo ""
echo "============================================"
echo " xrdp packages ready (Xvnc / libvnc)"
echo " Default port: 3389"
echo " Enable per-user from the user VNC page"
echo " Login: username + VNC password → desktop"
echo "============================================"
`
}

func setupEnv() []string {
	env := os.Environ()
	env = append(env, "DEBIAN_FRONTEND=noninteractive")
	return env
}
