package seed

import "strings"

// Install Cursor (deb preferred / AppImage fallback) with a container-safe
// launcher and Desktop shortcuts for every login user.
const cursorInstallScript = `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export PATH="/usr/local/bin:$PATH"

` + desktopHelpers + `

arch="$(uname -m)"
echo "==> Installing Cursor (arch=${arch})"

cws_pkg_install ca-certificates curl desktop-file-utils wget xdg-utils || true
cws_pkg_install libgtk-3-0 libnss3 libxss1 2>/dev/null || \
  cws_pkg_install gtk3 nss libXScrnSaver 2>/dev/null || true
cws_pkg_install libasound2t64 2>/dev/null || cws_pkg_install libasound2 2>/dev/null || \
  cws_pkg_install alsa-lib 2>/dev/null || true

case "$arch" in
  x86_64|amd64)
    cursor_platform="linux-x64"
    cursor_deb_arch="amd64"
    ;;
  aarch64|arm64)
    cursor_platform="linux-arm64"
    cursor_deb_arch="arm64"
    ;;
  *)
    echo "ERROR: unsupported architecture for Cursor: $arch" >&2
    exit 1
    ;;
esac

workdir="$(mktemp -d /tmp/cursor-install.XXXXXX)"
trap 'rm -rf "$workdir"' EXIT
cd "$workdir"

# Prefer .deb on Debian/Ubuntu — AppImages hang without FUSE in containers
# when the runtime tries to mount itself.
echo "==> Resolving latest Cursor download URL"
download_ok=0
for url in \
  "https://downloader.cursor.sh/linux/deb/${cursor_deb_arch}" \
  "https://www.cursor.com/api/download?platform=${cursor_platform}&releaseTrack=stable" \
  "https://api2.cursor.sh/updates/download/golden/${cursor_platform}/cursor/latest"
do
  echo "    trying: $url"
  rm -f cursor.pkg
  if curl -fsSL -L --connect-timeout 20 --max-time 600 --retry 2 \
      -o cursor.pkg "$url"; then
    size="$(wc -c < cursor.pkg | tr -d ' ')"
    if [ "$size" -gt 1000000 ]; then
      download_ok=1
      echo "    downloaded ${size} bytes"
      break
    fi
    echo "    WARN: download too small (${size} bytes), trying next mirror"
  else
    echo "    WARN: download failed, trying next mirror"
  fi
done

if [ "$download_ok" -ne 1 ]; then
  echo "ERROR: failed to download Cursor package" >&2
  exit 1
fi

CURSOR_BIN=""
pkg_kind="unknown"
if command -v dpkg-deb >/dev/null 2>&1 && dpkg-deb -I cursor.pkg >/dev/null 2>&1; then
  pkg_kind="deb"
elif command -v file >/dev/null 2>&1 && file cursor.pkg | grep -qi 'rpm\|RPM'; then
  pkg_kind="rpm"
elif command -v file >/dev/null 2>&1 && file cursor.pkg | grep -qi 'debian\|ar archive'; then
  pkg_kind="deb"
else
  # ELF AppImage or opaque binary from the golden CDN.
  pkg_kind="appimage"
fi
echo "    package kind: ${pkg_kind}"

if [ "$pkg_kind" = "deb" ] && cws_is_deb_host; then
  echo "==> Installing Cursor .deb"
  cws_pkg_install_file ./cursor.pkg
elif [ "$pkg_kind" = "rpm" ] && cws_is_rpm_host; then
  echo "==> Installing Cursor .rpm"
  cws_pkg_install_file ./cursor.pkg
else
  echo "==> Installing Cursor AppImage to /opt/cursor"
  install -d /opt/cursor
  echo "    copying AppImage ($(wc -c < cursor.pkg | tr -d ' ') bytes)..."
  install -m 755 cursor.pkg /opt/cursor/cursor.AppImage

  # NEVER run the AppImage "normally" in containers — without /dev/fuse it
  # blocks forever trying to mount. Extract with a hard timeout instead.
  echo "    extracting AppImage (no FUSE; timeout 180s)..."
  rm -rf /opt/cursor/squashfs-root
  cd /opt/cursor
  extract_ok=0
  set +e
  # APPIMAGE_EXTRACT_AND_RUN skips some mount paths; --appimage-extract unpacks.
  timeout 180 env APPIMAGE_EXTRACT_AND_RUN=1 \
    ./cursor.AppImage --appimage-extract >/tmp/cursor-appimage-extract.log 2>&1
  extract_rc=$?
  set -e
  if [ "$extract_rc" -eq 0 ] && [ -d /opt/cursor/squashfs-root ]; then
    extract_ok=1
    echo "    extract ok"
  else
    echo "    WARN: AppImage --appimage-extract failed (rc=${extract_rc})"
    tail -n 20 /tmp/cursor-appimage-extract.log 2>/dev/null || true
    echo "    trying unsquashfs fallback..."
    cws_pkg_install squashfs-tools 2>/dev/null || true
    if command -v unsquashfs >/dev/null 2>&1; then
      # Locate embedded squashfs magic without executing the AppImage.
      offset="$(grep -aob 'hsqs' /opt/cursor/cursor.AppImage 2>/dev/null | head -n1 | cut -d: -f1 || true)"
      if [ -n "$offset" ]; then
        echo "    squashfs offset=${offset}"
        set +e
        timeout 180 unsquashfs -o "$offset" -d /opt/cursor/squashfs-root /opt/cursor/cursor.AppImage \
          >/tmp/cursor-unsquashfs.log 2>&1
        usq_rc=$?
        set -e
        if [ "$usq_rc" -eq 0 ] && [ -d /opt/cursor/squashfs-root ]; then
          extract_ok=1
          echo "    unsquashfs ok"
        else
          echo "    WARN: unsquashfs failed (rc=${usq_rc})"
          tail -n 20 /tmp/cursor-unsquashfs.log 2>/dev/null || true
        fi
      else
        echo "    WARN: could not find squashfs magic in AppImage"
      fi
    fi
  fi

  if [ "$extract_ok" -eq 1 ]; then
    for c in \
      squashfs-root/usr/share/cursor/cursor \
      squashfs-root/usr/bin/cursor \
      squashfs-root/cursor \
      squashfs-root/AppRun
    do
      if [ -x "/opt/cursor/$c" ]; then
        ln -sfn "/opt/cursor/$c" /opt/cursor/cursor
        echo "    linked /opt/cursor/cursor -> /opt/cursor/$c"
        break
      fi
    done
  else
    echo "ERROR: could not extract Cursor AppImage (FUSE unavailable and extract failed)" >&2
    echo "       Prefer installing via .deb mirror, or enable /dev/fuse in the container." >&2
    exit 1
  fi
  cd "$workdir"
  ln -sfn /opt/cursor/cursor /usr/local/bin/cursor-appimage 2>/dev/null || \
    ln -sfn /opt/cursor/cursor.AppImage /usr/local/bin/cursor-appimage
fi

for c in \
  /usr/bin/cursor \
  /usr/local/bin/cursor \
  /opt/cursor/cursor \
  /usr/share/cursor/cursor \
  /usr/share/cursor/bin/cursor
do
  if [ -x "$c" ]; then
    CURSOR_BIN="$c"
    break
  fi
done
# Do not prefer the raw AppImage as CURSOR_BIN — it hangs without FUSE.
if [ -z "$CURSOR_BIN" ] && command -v cursor >/dev/null 2>&1; then
  CURSOR_BIN="$(command -v cursor)"
fi
if [ -z "$CURSOR_BIN" ]; then
  echo "ERROR: Cursor binary not found after install" >&2
  ls -la /opt/cursor 2>/dev/null || true
  dpkg -L cursor 2>/dev/null | head -n 40 || true
  exit 1
fi

WRAPPER="/usr/local/bin/cursor-desktop"
cat > "$WRAPPER" <<EOF
#!/bin/bash
set -e
export HOME="\${HOME:-/root}"
export ELECTRON_OZONE_PLATFORM_HINT="\${ELECTRON_OZONE_PLATFORM_HINT:-auto}"
exec "${CURSOR_BIN}" \\
  --no-sandbox \\
  --disable-gpu-sandbox \\
  --disable-dev-shm-usage \\
  --user-data-dir="\${HOME}/.config/Cursor" \\
  "\$@"
EOF
chmod 755 "$WRAPPER"
ln -sfn "$WRAPPER" /usr/local/bin/cursor 2>/dev/null || true

APP_VERSION="latest"
set +o pipefail
ver_out="$(timeout 30 "$CURSOR_BIN" --version 2>/dev/null || true)"
set -o pipefail
if [ -n "$ver_out" ]; then
  APP_VERSION="$(printf '%s\n' "$ver_out" | awk 'NR==1{print; exit}')"
fi

ICON="cursor"
for ic in \
  /usr/share/pixmaps/cursor.png \
  /usr/share/icons/hicolor/256x256/apps/cursor.png \
  /usr/share/cursor/resources/app/resources/linux/code.png \
  /opt/cursor/squashfs-root/co.anysphere.cursor.png \
  /opt/cursor/squashfs-root/usr/share/pixmaps/co.anysphere.cursor.png
do
  if [ -f "$ic" ]; then
    ICON="$ic"
    break
  fi
done

tmp_desktop="$(mktemp /tmp/cursor.XXXXXX.desktop)"
if [ -f /usr/share/applications/cursor.desktop ]; then
  # Rewrite Exec to our sandbox-safe wrapper while keeping Name/Icon.
  sed -e "s|^Exec=.*|Exec=${WRAPPER} %F|" \
      -e "s|^TryExec=.*|TryExec=${WRAPPER}|" \
      /usr/share/applications/cursor.desktop > "$tmp_desktop" || true
fi
if [ ! -s "$tmp_desktop" ]; then
  cat > "$tmp_desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Cursor
Comment=AI-powered code editor
Exec=${WRAPPER} %F
Icon=${ICON}
Terminal=false
Categories=Development;IDE;
StartupNotify=true
StartupWMClass=Cursor
EOF
fi
chmod +x "$tmp_desktop"
publish_desktop_launcher "Cursor.desktop" "$tmp_desktop"
rm -f "$tmp_desktop"

echo "==> Cursor installed (${APP_VERSION})"
echo "    binary:  $CURSOR_BIN"
echo "    wrapper: $WRAPPER"
echo "    tip: open from Desktop as any user, or run: cursor-desktop"
`

const cursorUninstallScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing cursor"
systemctl stop cursor.service 2>/dev/null || true
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge cursor || true
  apt-get autoremove -y || true
fi
# AppImage / manual paths
rm -rf /opt/cursor /opt/cursor /usr/local/bin/cursor 2>/dev/null || true
rm -f /usr/share/applications/cursor*.desktop /usr/local/share/applications/cursor*.desktop 2>/dev/null || true
echo "==> cursor removed"
`

func cursoraiCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "Cursor",
			Details:     "Install Cursor AI code editor (deb preferred; AppImage extracted without FUSE) with a container-safe launcher and Desktop shortcuts for all users.",
			Category:    "Desktop",
			SubCategory: "Editors",
			Tags:        []string{"cursor", "ai", "editor", "ide", "desktop"},
			Icon:        "Sparkles",
			Color:       "#000000",
			Order:       7,
			IsActive:    true,
		},
		Versions: []VersionMeta{
			{
				Version:       "allusers-2",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(cursorInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(cursorUninstallScript) + "\n",
			},
		},
	}
}
