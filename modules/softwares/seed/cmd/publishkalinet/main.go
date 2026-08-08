package main

// One-shot publisher: Kali-only network monitoring desktop packages → containerwspkg.
// Usage: go run ./modules/softwares/seed/cmd/publishkalinet <registry-root>

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

// Inlined from modules/softwares/seed/desktop_icons.go — keep Desktop launchers working in Kali containers.
const desktopHelpers = `
# --- containerws desktop helpers ---
cws_trust_desktop_file() {
  local user="$1"
  local file="$2"
  [ -f "$file" ] || return 0
  chmod +x "$file" 2>/dev/null || true
  if [ "$user" = "root" ] || { [ "$(id -u)" -eq 0 ] && [ "$user" = "$(id -un)" ]; }; then
    gio set "$file" metadata::trusted true 2>/dev/null || true
    dbus-launch gio set "$file" metadata::trusted true 2>/dev/null || true
    return 0
  fi
  if command -v runuser >/dev/null 2>&1; then
    runuser -u "$user" -- gio set "$file" metadata::trusted true 2>/dev/null || true
  elif command -v su >/dev/null 2>&1; then
    su -s /bin/sh "$user" -c "gio set \"$file\" metadata::trusted true" 2>/dev/null || true
  fi
}

cws_each_user_home() {
  echo "root:/root"
  getent passwd 2>/dev/null | awk -F: '$6 ~ /^\/home\// && $3 >= 1000 { print $1 ":" $6 }'
  for d in /home/*; do
    [ -d "$d" ] || continue
    u="$(basename "$d")"
    getent passwd "$u" >/dev/null 2>&1 || continue
    echo "${u}:${d}"
  done
}

publish_desktop_launcher() {
  local name="$1"
  local src="$2"
  local apps="/usr/share/applications"
  local skel="/etc/skel/Desktop"

  if [ -z "$name" ] || [ ! -f "$src" ]; then
    echo "ERROR: publish_desktop_launcher requires name and source file" >&2
    return 1
  fi

  install -d -m 755 "$apps" "$skel"
  install -m 755 "$src" "${apps}/${name}"
  install -m 755 "$src" "${skel}/${name}"

  local line uname home desk
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    uname="${line%%:*}"
    home="${line#*:}"
    [ -n "$uname" ] && [ -n "$home" ] || continue
    [ -d "$home" ] || continue
    desk="${home}/Desktop"
    install -d -m 755 "$desk"
    install -m 755 "$src" "${desk}/${name}"
    chown -R "${uname}:${uname}" "$desk" 2>/dev/null || chown "${uname}:" "${desk}/${name}" 2>/dev/null || true
    cws_trust_desktop_file "$uname" "${desk}/${name}"
    install -d -m 755 "${home}/.local/share/applications"
    install -m 755 "$src" "${home}/.local/share/applications/${name}"
    chown -R "${uname}:${uname}" "${home}/.local" 2>/dev/null || true
  done <<EOF
$(cws_each_user_home | awk -F: '!seen[$1]++')
EOF

  update-desktop-database "$apps" 2>/dev/null || true
  echo "    desktop launcher published for all users: ${name}"
}
`

const aptPreamble = `#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
# Prefer distro archives only — ignore broken third-party repos (exit 100).
apt_update_safe() {
  local opts=(-o APT::Get::List-Cleanup=0)
  if [[ -f /etc/apt/sources.list.d/ubuntu.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/ubuntu.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list.d/debian.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/debian.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list.d/kali.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/kali.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  apt-get update \
    -o Acquire::AllowInsecureRepositories=true \
    -o Acquire::AllowDowngradeToInsecureRepositories=true
}
apt_update_safe
`

type pkgDef struct {
	Name        string
	Details     string
	Category    string
	SubCategory string
	Tags        []string
	Icon        string
	Color       string
	Image       string
	Order       int
	Install     string
	Uninstall   string
	Upgrade     string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: publishkalinet <registry-root>")
		os.Exit(2)
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		panic(err)
	}

	pkgs := []pkgDef{
		wiresharkPkg(),
		zenmapPkg(),
		etherapePkg(),
		ettercapPkg(),
	}

	active := true
	latest := true
	indexPath := filepath.Join(root, "softwares", "index.json")
	var idx softwarepkg.CatalogIndex
	if raw, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(raw, &idx)
	}

	for _, p := range pkgs {
		slug := strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
		meta := softwarepkg.PackageMeta{
			Name:        p.Name,
			Details:     p.Details,
			Category:    p.Category,
			SubCategory: p.SubCategory,
			Tags:        p.Tags,
			Icon:        p.Icon,
			Image:       p.Image,
			Color:       p.Color,
			Order:       p.Order,
			IsActive:    &active,
		}
		metaPath := filepath.Join(root, "softwares", slug, "package.json")
		mustWriteJSON(metaPath, meta)

		specBase := softwarepkg.InstallSpec{
			Version:         "1.0.1",
			IsLatest:        &latest,
			OS:              "linux",
			Arch:            "any",
			PackageFamily:   "apt",
			InstallScript:   p.Install,
			UninstallScript: p.Uninstall,
			UpgradeScript:   p.Upgrade,
		}

		type target struct {
			distro, version, arch string
		}
		targets := []target{
			{"kali", "rolling", "any"},
			{"kali", "any", "any"},
			{"ubuntu", "any", "any"},
			{"ubuntu", "26.04", "any"},
			{"ubuntu", "26.10", "any"},
			{"ubuntu", "25.10", "any"},
			{"ubuntu", "25.04", "any"},
			{"ubuntu", "24.10", "any"},
			{"debian", "any", "any"},
			{"debian", "12", "any"},
			{"debian", "13", "any"},
		}
		for _, t := range targets {
			spec := specBase
			spec.DistroID = t.distro
			spec.DistroVersion = t.version
			if t.version == "any" {
				spec.DistroVersion = ""
			}
			rel := filepath.Join("softwares", slug, t.distro, t.version, t.arch, "install.json")
			mustWriteJSON(filepath.Join(root, rel), spec)
			fmt.Println("wrote", rel)
		}

		// Merge into index by name.
		found := false
		for i := range idx.Softwares {
			if strings.EqualFold(idx.Softwares[i].Name, p.Name) {
				idx.Softwares[i] = meta
				found = true
				break
			}
		}
		if !found {
			idx.Softwares = append(idx.Softwares, meta)
		}
	}

	mustWriteJSON(indexPath, idx)
	fmt.Println("updated softwares/index.json — packages:", len(pkgs))
}

func mustWriteJSON(path string, v any) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		panic(err)
	}
}

func uninstallScript(aptPkgs ...string) string {
	list := strings.Join(aptPkgs, " ")
	return aptPreamble + fmt.Sprintf(`
apt-get remove -y --purge %s || true
apt-get autoremove -y || true
echo "==> removed: %s"
`, list, list)
}

func upgradeScript(aptPkgs ...string) string {
	list := strings.Join(aptPkgs, " ")
	return aptPreamble + fmt.Sprintf(`
apt-get install -y --only-upgrade %s || apt-get install -y %s
`, list, list)
}

func wiresharkPkg() pkgDef {
	install := aptPreamble + desktopHelpers + `
echo "==> Installing Wireshark (Kali desktop)"
# Preseed so wireshark-common does not prompt about non-root capture.
echo "wireshark-common wireshark-common/install-setuid boolean true" | debconf-set-selections || true
apt-get install -y --no-install-recommends wireshark wireshark-qt || \
  apt-get install -y --no-install-recommends wireshark

APP_BIN="$(command -v wireshark || true)"
if [ -z "$APP_BIN" ]; then
  echo "ERROR: wireshark binary not found" >&2
  exit 1
fi

# Allow non-root capture when wireshark group exists (common on Kali/Debian).
if getent group wireshark >/dev/null 2>&1; then
  while IFS= read -r line; do
    u="${line%%:*}"
    [ -n "$u" ] || continue
    usermod -aG wireshark "$u" 2>/dev/null || true
  done <<EOF
$(cws_each_user_home | awk -F: '!seen[$1]++')
EOF
fi
# dumpcap capabilities (best-effort in containers)
if [ -x /usr/bin/dumpcap ]; then
  setcap cap_net_raw,cap_net_admin+eip /usr/bin/dumpcap 2>/dev/null || true
fi

APP_ICON="wireshark"
for ic in \
  /usr/share/icons/hicolor/256x256/apps/wireshark.png \
  /usr/share/icons/hicolor/48x48/apps/wireshark.png \
  /usr/share/pixmaps/wireshark.png \
  /usr/share/icons/hicolor/scalable/apps/wireshark.svg
do
  if [ -f "$ic" ]; then APP_ICON="$ic"; break; fi
done

tmp_desktop="$(mktemp /tmp/wireshark.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Wireshark
GenericName=Network Protocol Analyzer
Comment=Capture and analyze network traffic
Exec=${APP_BIN} %f
Icon=${APP_ICON}
Terminal=false
Categories=Network;Monitor;System;Security;
Keywords=network;capture;packet;protocol;sniffer;
StartupNotify=true
StartupWMClass=wireshark
EOF
chmod +x "$tmp_desktop"
publish_desktop_launcher "Wireshark.desktop" "$tmp_desktop"
rm -f "$tmp_desktop"

command -v wireshark >/dev/null
echo "==> Wireshark installed — open from Desktop or: wireshark"
`
	return pkgDef{
		Name:        "Wireshark",
		Details:     "GUI packet analyzer for live capture and deep protocol inspection. Desktop launcher for Kali/Ubuntu/Debian.",
		Category:    "Network",
		SubCategory: "Monitoring",
		Tags:        []string{"kali", "ubuntu", "debian", "wireshark", "pcap", "sniffer", "desktop", "network"},
		Icon:        "Radar",
		Color:       "#1679A7",
		Image:       "https://cdn.simpleicons.org/wireshark",
		Order:       210,
		Install:     strings.TrimSpace(install) + "\n",
		Uninstall:   uninstallScript("wireshark", "wireshark-qt"),
		Upgrade:     upgradeScript("wireshark", "wireshark-qt"),
	}
}

func zenmapPkg() pkgDef {
	install := aptPreamble + desktopHelpers + `
echo "==> Installing Zenmap (Nmap GUI) for Kali"
# Kali ships zenmap-kbx; Debian/Ubuntu may use zenmap.
PKG=""
if apt-cache show zenmap-kbx >/dev/null 2>&1; then
  PKG="zenmap-kbx"
elif apt-cache show zenmap >/dev/null 2>&1; then
  PKG="zenmap"
else
  echo "ERROR: neither zenmap-kbx nor zenmap is available" >&2
  exit 1
fi
apt-get install -y --no-install-recommends nmap "$PKG"

APP_BIN="$(command -v zenmap || command -v zenmap-kbx || true)"
if [ -z "$APP_BIN" ]; then
  # Some Kali builds install a desktop-only wrapper
  for c in /usr/bin/zenmap /usr/bin/zenmap-kbx /usr/share/zenmap/zenmap; do
    if [ -x "$c" ]; then APP_BIN="$c"; break; fi
  done
fi
if [ -z "$APP_BIN" ]; then
  echo "ERROR: zenmap binary not found after install" >&2
  exit 1
fi

APP_ICON="zenmap"
for ic in \
  /usr/share/icons/hicolor/128x128/apps/zenmap.png \
  /usr/share/icons/hicolor/48x48/apps/zenmap.png \
  /usr/share/pixmaps/zenmap.png \
  /usr/share/zenmap/pixmaps/zenmap.png \
  /usr/share/icons/hicolor/scalable/apps/nmap.svg
do
  if [ -f "$ic" ]; then APP_ICON="$ic"; break; fi
done

tmp_desktop="$(mktemp /tmp/zenmap.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Zenmap
GenericName=Network Scanner
Comment=GUI frontend for Nmap network discovery and security auditing
Exec=${APP_BIN} %F
Icon=${APP_ICON}
Terminal=false
Categories=Network;Security;System;Monitor;
Keywords=nmap;scan;ports;network;security;
StartupNotify=true
EOF
chmod +x "$tmp_desktop"
publish_desktop_launcher "Zenmap.desktop" "$tmp_desktop"
rm -f "$tmp_desktop"

echo "==> Zenmap installed ($PKG) — open from Desktop or: $APP_BIN"
`
	return pkgDef{
		Name:        "Zenmap",
		Details:     "Official Nmap GUI for network discovery, port scans, and topology maps. Desktop launcher (zenmap-kbx on Kali, zenmap on Ubuntu/Debian).",
		Category:    "Network",
		SubCategory: "Scanning",
		Tags:        []string{"kali", "ubuntu", "debian", "zenmap", "nmap", "scanner", "desktop", "network"},
		Icon:        "ScanSearch",
		Color:       "#3D5AFE",
		Image:       "https://cdn.simpleicons.org/nmap",
		Order:       211,
		Install:     strings.TrimSpace(install) + "\n",
		Uninstall:   uninstallScript("zenmap-kbx", "zenmap"),
		Upgrade:     upgradeScript("zenmap-kbx", "zenmap", "nmap"),
	}
}

func etherapePkg() pkgDef {
	install := aptPreamble + desktopHelpers + `
echo "==> Installing EtherApe (graphical network monitor)"
apt-get install -y --no-install-recommends etherape

APP_BIN="$(command -v etherape || true)"
if [ -z "$APP_BIN" ]; then
  echo "ERROR: etherape binary not found" >&2
  exit 1
fi

# EtherApe typically needs root/capabilities for live capture.
WRAPPER="/usr/local/bin/etherape-desktop"
cat > "$WRAPPER" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "$(id -u)" -eq 0 ]; then
  exec /usr/bin/etherape "$@"
fi
if command -v pkexec >/dev/null 2>&1; then
  exec pkexec /usr/bin/etherape "$@"
fi
exec /usr/bin/etherape "$@"
EOF
chmod 755 "$WRAPPER"

APP_ICON="etherape"
for ic in \
  /usr/share/icons/hicolor/48x48/apps/etherape.png \
  /usr/share/pixmaps/etherape.png \
  /usr/share/icons/hicolor/scalable/apps/etherape.svg
do
  if [ -f "$ic" ]; then APP_ICON="$ic"; break; fi
done

tmp_desktop="$(mktemp /tmp/etherape.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=EtherApe
GenericName=Network Monitor
Comment=Graphical real-time network traffic visualization
Exec=${WRAPPER}
Icon=${APP_ICON}
Terminal=false
Categories=Network;Monitor;System;
Keywords=network;traffic;monitor;graph;
StartupNotify=true
EOF
chmod +x "$tmp_desktop"
publish_desktop_launcher "EtherApe.desktop" "$tmp_desktop"
rm -f "$tmp_desktop"

command -v etherape >/dev/null
echo "==> EtherApe installed — open from Desktop or: etherape-desktop"
`
	return pkgDef{
		Name:        "EtherApe",
		Details:     "Real-time graphical network traffic visualizer (node/link view). Desktop launcher for Kali/Ubuntu/Debian.",
		Category:    "Network",
		SubCategory: "Monitoring",
		Tags:        []string{"kali", "ubuntu", "debian", "etherape", "monitor", "traffic", "desktop", "network"},
		Icon:        "Activity",
		Color:       "#00ACC1",
		Image:       "https://www.google.com/s2/favicons?domain=etherape.sourceforge.io&sz=128",
		Order:       212,
		Install:     strings.TrimSpace(install) + "\n",
		Uninstall:   uninstallScript("etherape"),
		Upgrade:     upgradeScript("etherape"),
	}
}

func ettercapPkg() pkgDef {
	install := aptPreamble + desktopHelpers + `
echo "==> Installing Ettercap (graphical MITM / LAN analysis)"
apt-get install -y --no-install-recommends ettercap-graphical || \
  apt-get install -y --no-install-recommends ettercap-common ettercap-graphical

APP_BIN="$(command -v ettercap || true)"
# GUI binary is often ettercap -G
if [ -z "$APP_BIN" ]; then
  echo "ERROR: ettercap binary not found" >&2
  exit 1
fi

WRAPPER="/usr/local/bin/ettercap-desktop"
cat > "$WRAPPER" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
BIN="$(command -v ettercap)"
# -G starts the GTK interface
if [ "$(id -u)" -eq 0 ]; then
  exec "$BIN" -G "$@"
fi
if command -v pkexec >/dev/null 2>&1; then
  exec pkexec "$BIN" -G "$@"
fi
exec "$BIN" -G "$@"
EOF
chmod 755 "$WRAPPER"

APP_ICON="ettercap"
for ic in \
  /usr/share/pixmaps/ettercap.png \
  /usr/share/icons/hicolor/48x48/apps/ettercap.png \
  /usr/share/icons/hicolor/scalable/apps/ettercap.svg
do
  if [ -f "$ic" ]; then APP_ICON="$ic"; break; fi
done

tmp_desktop="$(mktemp /tmp/ettercap.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Ettercap
GenericName=Network Security Tool
Comment=Graphical multipurpose sniffer / interceptor / logger for switched LAN
Exec=${WRAPPER}
Icon=${APP_ICON}
Terminal=false
Categories=Network;Security;System;Monitor;
Keywords=mitm;sniffer;lan;security;ettercap;
StartupNotify=true
EOF
chmod +x "$tmp_desktop"
publish_desktop_launcher "Ettercap.desktop" "$tmp_desktop"
rm -f "$tmp_desktop"

command -v ettercap >/dev/null
echo "==> Ettercap installed — open from Desktop or: ettercap-desktop"
`
	return pkgDef{
		Name:        "Ettercap",
		Details:     "Graphical sniffer/interceptor for switched LANs (MITM analysis). Desktop launcher via ettercap -G on Kali/Ubuntu/Debian.",
		Category:    "Network",
		SubCategory: "Security",
		Tags:        []string{"kali", "ubuntu", "debian", "ettercap", "mitm", "sniffer", "desktop", "network"},
		Icon:        "Shield",
		Color:       "#C62828",
		Image:       "https://www.google.com/s2/favicons?domain=www.ettercap-project.org&sz=128",
		Order:       213,
		Install:     strings.TrimSpace(install) + "\n",
		Uninstall:   uninstallScript("ettercap-graphical", "ettercap-common"),
		Upgrade:     upgradeScript("ettercap-graphical"),
	}
}
