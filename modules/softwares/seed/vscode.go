package seed

import "strings"

// Install Visual Studio Code (deb/rpm) with a container-safe launcher and
// Desktop shortcuts for every login user.
const vscodeInstallScript = `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export PATH="/usr/local/bin:$PATH"

` + desktopHelpers + `

arch="$(uname -m)"
echo "==> Installing Visual Studio Code (arch=${arch})"

cws_pkg_install ca-certificates curl desktop-file-utils wget xdg-utils || true
cws_pkg_install libgtk-3-0 libnss3 libxss1 libgbm1 libdrm2 libxkbfile1 2>/dev/null || \
  cws_pkg_install gtk3 nss libXScrnSaver mesa-libgbm libdrm libxkbfile 2>/dev/null || true
cws_pkg_install libasound2t64 2>/dev/null || cws_pkg_install libasound2 2>/dev/null || \
  cws_pkg_install alsa-lib 2>/dev/null || true

case "$arch" in
  x86_64|amd64)
    if cws_is_deb_host; then code_os="linux-deb-x64"; else code_os="linux-rpm-x64"; fi
    ;;
  aarch64|arm64)
    if cws_is_deb_host; then code_os="linux-deb-arm64"; else code_os="linux-rpm-arm64"; fi
    ;;
  armv7l|armhf)
    code_os="linux-deb-armhf"
    ;;
  *)
    echo "ERROR: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

tmppkg="$(mktemp /tmp/vscode.XXXXXX)"
echo "==> Downloading latest VS Code (${code_os})"
curl -fsSL -L -o "$tmppkg" \
  "https://code.visualstudio.com/sha/download?build=stable&os=${code_os}"

cws_pkg_install_file "$tmppkg"
rm -f "$tmppkg"

CODE_BIN=""
for c in /usr/share/code/code /usr/bin/code /usr/share/code/bin/code /usr/lib64/code/code /usr/lib/code/code; do
  if [ -x "$c" ]; then
    CODE_BIN="$c"
    break
  fi
done
if [ -z "$CODE_BIN" ] && command -v code >/dev/null 2>&1; then
  CODE_BIN="$(command -v code)"
fi
if [ -z "$CODE_BIN" ] || [ ! -x "$CODE_BIN" ]; then
  echo "ERROR: code binary not found after install" >&2
  exit 1
fi

WRAPPER="/usr/local/bin/code-desktop"
cat > "$WRAPPER" <<EOF
#!/bin/bash
set -e
export HOME="\${HOME:-/root}"
export ELECTRON_OZONE_PLATFORM_HINT="\${ELECTRON_OZONE_PLATFORM_HINT:-auto}"
exec "${CODE_BIN}" \\
  --no-sandbox \\
  --disable-gpu-sandbox \\
  --disable-dev-shm-usage \\
  --user-data-dir="\${HOME}/.config/Code" \\
  "\$@"
EOF
chmod 755 "$WRAPPER"
ln -sfn "$WRAPPER" /usr/local/bin/code-gui 2>/dev/null || true

APP_VERSION="latest"
set +o pipefail
ver_out="$("$CODE_BIN" --version 2>/dev/null || true)"
set -o pipefail
if [ -n "$ver_out" ]; then
  APP_VERSION="$(printf '%s\n' "$ver_out" | awk 'NR==1{print; exit}')"
fi

ICON="vscode"
for ic in \
  /usr/share/pixmaps/com.visualstudio.code.png \
  /usr/share/pixmaps/vscode.png \
  /usr/share/icons/hicolor/256x256/apps/vscode.png \
  /usr/share/code/resources/app/resources/linux/code.png
do
  if [ -f "$ic" ]; then
    ICON="$ic"
    break
  fi
done

tmp_desktop="$(mktemp /tmp/vscode.XXXXXX.desktop)"
cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Visual Studio Code
Comment=Code Editing. Redefined.
GenericName=Text Editor
Exec=${WRAPPER} %F
Icon=${ICON}
Terminal=false
Categories=TextEditor;Development;IDE;
MimeType=text/plain;inode/directory;application/x-code-workspace;
StartupNotify=true
StartupWMClass=Code
Keywords=vscode;
Actions=new-empty-window;

[Desktop Action new-empty-window]
Name=New Empty Window
Exec=${WRAPPER} --new-window %F
Icon=${ICON}
EOF
chmod +x "$tmp_desktop"
publish_desktop_launcher "Visual_Studio_Code.desktop" "$tmp_desktop"
# Prefer our sandbox-safe launcher over the stock package entry.
install -m 755 "$tmp_desktop" /usr/share/applications/code-desktop.desktop 2>/dev/null || true
rm -f "$tmp_desktop"

echo "==> Visual Studio Code installed (${APP_VERSION})"
echo "    binary:  $CODE_BIN"
echo "    wrapper: $WRAPPER"
echo "    tip: open from Desktop as any user, or run: code-desktop"
`

const vscodeUninstallScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing code"
systemctl stop code.service 2>/dev/null || true
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge code || true
  apt-get autoremove -y || true
fi
# AppImage / manual paths
rm -rf /opt/code /opt/cursor /usr/local/bin/code 2>/dev/null || true
rm -f /usr/share/applications/code*.desktop /usr/local/share/applications/code*.desktop 2>/dev/null || true
echo "==> code removed"
`

func vscodeCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "Visual Studio Code",
			Details:     "Install Visual Studio Code (deb/rpm) with a container-safe launcher and Desktop shortcuts for all users.",
			Category:    "Desktop",
			SubCategory: "Editors",
			Tags:        []string{"vscode", "code", "editor", "ide", "desktop"},
			Icon:        "Code",
			Color:       "#007ACC",
			Order:       6,
			IsActive:    true,
		},
		Versions: []VersionMeta{
			{
				Version:       "allusers-1",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(vscodeInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(vscodeUninstallScript) + "\n",
			},
		},
	}
}
