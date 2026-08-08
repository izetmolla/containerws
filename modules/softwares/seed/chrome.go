package seed

import "strings"

// Install Google Chrome (amd64) or Chromium (arm64), multi-distro (apt/dnf),
// with Desktop shortcuts for every login user.
const chromeInstallScript = `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export PATH="/usr/local/bin:$PATH"

` + desktopHelpers + `

arch="$(uname -m)"
echo "==> Installing Google Chrome / Chromium (arch=${arch})"

cws_pkg_install ca-certificates curl desktop-file-utils wget xdg-utils || true
# Best-effort GUI deps (names differ across distros).
cws_pkg_install libgtk-3-0 libnss3 libxss1 libgbm1 libdrm2 2>/dev/null || \
  cws_pkg_install gtk3 nss libXScrnSaver mesa-libgbm libdrm 2>/dev/null || true
cws_pkg_install libasound2t64 2>/dev/null || cws_pkg_install libasound2 2>/dev/null || \
  cws_pkg_install alsa-lib 2>/dev/null || true

if cws_is_deb_host; then
  echo "==> Cleaning snap/chromium-browser stub (if present)"
  apt-get remove -y chromium-browser 2>/dev/null || true
  dpkg --remove --force-remove-reinstreq chromium-browser 2>/dev/null || true
  snap remove chromium 2>/dev/null || true
  apt-get -f install -y 2>/dev/null || true
fi

APP_NAME=""
APP_BIN=""
APP_ICON=""
APP_VERSION="latest"
WRAPPER="/usr/local/bin/chrome-desktop"

pick_xtradeb_codename() {
  local want="" try=""
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    want="${VERSION_CODENAME:-}"
  fi
  for try in "$want" resolute questing plucky oracular noble jammy; do
    [ -n "$try" ] || continue
    if curl -fsSIL \
      "https://ppa.launchpadcontent.net/xtradeb/apps/ubuntu/dists/${try}/main/binary-arm64/Packages.gz" \
      >/dev/null 2>&1; then
      echo "$try"
      return 0
    fi
  done
  return 1
}

install_chromium_xtradeb_arm64() {
  local codename="$1"
  local pkg_gz tmpdir base ver fname
  echo "==> Installing native Chromium .deb from xtradeb (${codename}, no snap)"
  pkg_gz="$(mktemp /tmp/xtradeb-packages.XXXXXX.gz)"
  curl -fsSL -o "$pkg_gz" \
    "https://ppa.launchpadcontent.net/xtradeb/apps/ubuntu/dists/${codename}/main/binary-arm64/Packages.gz"
  tmpdir="$(mktemp -d /tmp/chromium-debs.XXXXXX)"
  base="https://ppa.launchpadcontent.net/xtradeb/apps/ubuntu"
  ver="$(
    gzip -dc "$pkg_gz" | awk '
      /^Package: chromium$/ {inpkg=1; next}
      inpkg && /^Package:/ {exit}
      inpkg && /^Version:/ {print $2; exit}
    '
  )"
  if [ -z "$ver" ]; then
    echo "ERROR: chromium package not found in xtradeb ${codename}" >&2
    rm -f "$pkg_gz"; rm -rf "$tmpdir"; return 1
  fi
  echo "    chromium version: ${ver}"
  for pkg in chromium-common chromium-sandbox chromium; do
    fname="$(
      gzip -dc "$pkg_gz" | awk -v pkg="$pkg" -v ver="$ver" '
        $1=="Package:" && $2==pkg {want=1; next}
        want && $1=="Package:" {want=0}
        want && $1=="Version:" && $2==ver {matchver=1}
        want && matchver && $1=="Filename:" {print $2; exit}
      '
    )"
    if [ -z "$fname" ]; then
      [ "$pkg" = "chromium-sandbox" ] && continue
      echo "ERROR: missing ${pkg} (=${ver}) in xtradeb index" >&2
      rm -f "$pkg_gz"; rm -rf "$tmpdir"; return 1
    fi
    curl -fsSL -o "${tmpdir}/$(basename "$fname")" "${base}/${fname}"
  done
  rm -f "$pkg_gz"
  cws_pkg_install gzip 2>/dev/null || true
  if cws_is_deb_host; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get install --no-install-recommends -y "${tmpdir}"/*.deb || {
      apt-get install -f -y
      apt-get install --no-install-recommends -y "${tmpdir}"/*.deb
    }
  else
    cws_pkg_install_file "${tmpdir}"/*.rpm 2>/dev/null || cws_pkg_install_file "${tmpdir}"/*.deb
  fi
  rm -rf "$tmpdir"
}

case "$arch" in
  x86_64|amd64)
    tmppkg="$(mktemp /tmp/chrome.XXXXXX)"
    if cws_is_deb_host; then
      echo "==> Downloading Google Chrome stable (.deb)"
      curl -fsSL -o "${tmppkg}.deb" \
        "https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb"
      cws_pkg_install_file "${tmppkg}.deb"
      rm -f "${tmppkg}.deb"
    else
      echo "==> Downloading Google Chrome stable (.rpm)"
      curl -fsSL -o "${tmppkg}.rpm" \
        "https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm"
      cws_pkg_install_file "${tmppkg}.rpm"
      rm -f "${tmppkg}.rpm"
    fi
    APP_NAME="Google Chrome"
    APP_BIN="$(command -v google-chrome-stable || command -v google-chrome || true)"
    APP_ICON="google-chrome"
    ;;
  aarch64|arm64)
    echo "==> Google Chrome has no official arm64 Linux build; installing Chromium"
    if cws_is_deb_host; then
      xtradeb_codename="$(pick_xtradeb_codename)" || {
        echo "ERROR: no xtradeb Chromium arm64 repo found" >&2
        exit 1
      }
      install_chromium_xtradeb_arm64 "$xtradeb_codename"
    else
      cws_pkg_install chromium || cws_pkg_install chromium-browser
    fi
    APP_NAME="Chromium"
    APP_BIN="$(command -v chromium || command -v chromium-browser || true)"
    APP_ICON="chromium"
    ;;
  *)
    echo "ERROR: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ -z "$APP_BIN" ] || [ ! -x "$APP_BIN" ]; then
  echo "ERROR: browser binary not found after install" >&2
  exit 1
fi

APP_VERSION="$("$APP_BIN" --version 2>/dev/null | awk '{print $NF}' || echo latest)"

cat > "$WRAPPER" <<EOF
#!/bin/bash
set -e
export HOME="\${HOME:-/root}"
exec "${APP_BIN}" \\
  --no-sandbox \\
  --disable-gpu-sandbox \\
  --disable-dev-shm-usage \\
  --user-data-dir="\${HOME}/.config/cws-chrome" \\
  "\$@"
EOF
chmod 755 "$WRAPPER"
ln -sfn "$WRAPPER" /usr/local/bin/chromium-desktop 2>/dev/null || true
ln -sfn "$WRAPPER" /usr/local/bin/google-chrome-desktop 2>/dev/null || true

for ic in \
  /usr/share/icons/hicolor/256x256/apps/chromium.png \
  /usr/share/icons/hicolor/256x256/apps/chromium-browser.png \
  /usr/share/pixmaps/chromium.png \
  /usr/share/icons/hicolor/256x256/apps/google-chrome.png \
  /usr/share/pixmaps/google-chrome.png
do
  if [ -f "$ic" ]; then
    APP_ICON="$ic"
    break
  fi
done

tmp_desktop="$(mktemp /tmp/chrome.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=${APP_NAME}
Comment=Web Browser
Exec=${WRAPPER} %U
Icon=${APP_ICON}
Terminal=false
Categories=Network;WebBrowser;
StartupNotify=true
StartupWMClass=${APP_NAME}
EOF
chmod +x "$tmp_desktop"

launcher_name="Google_Chrome.desktop"
[ "$APP_NAME" = "Chromium" ] && launcher_name="Chromium.desktop"
publish_desktop_launcher "$launcher_name" "$tmp_desktop"
rm -f "$tmp_desktop"

echo "==> ${APP_NAME} installed (${APP_VERSION})"
echo "    binary:  $APP_BIN"
echo "    wrapper: $WRAPPER"
echo "    tip: open from Desktop as any user, or run: chrome-desktop"
`

const chromeUninstallScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing google-chrome-stable"
systemctl stop google-chrome-stable.service 2>/dev/null || true
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge google-chrome-stable || true
  apt-get autoremove -y || true
fi
# AppImage / manual paths
rm -rf /opt/google-chrome-stable /opt/cursor /usr/local/bin/google-chrome-stable 2>/dev/null || true
rm -f /usr/share/applications/google-chrome-stable*.desktop /usr/local/share/applications/google-chrome-stable*.desktop 2>/dev/null || true
echo "==> google-chrome-stable removed"
`

func chromeCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "Google Chrome",
			Details:     "Install Google Chrome (amd64) or Chromium (arm64) for apt/dnf hosts, with Desktop shortcuts for all users.",
			Category:    "Desktop",
			SubCategory: "Browsers",
			Tags:        []string{"chrome", "chromium", "browser", "desktop"},
			Icon:        "Globe",
			Color:       "#4285F4",
			Order:       5,
			IsActive:    true,
		},
		Versions: []VersionMeta{
			{
				Version:       "allusers-1",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(chromeInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(chromeUninstallScript) + "\n",
			},
		},
	}
}
