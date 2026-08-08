package seed

// Shared bash helpers inlined into desktop app install scripts.
// publish_desktop_launcher installs a .desktop file system-wide, in /etc/skel
// (for new users), and on every existing login user's Desktop.
const desktopHelpers = `
# --- containerws desktop helpers ---
cws_pkg_install() {
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install --no-install-recommends -y "$@"
  elif command -v dnf >/dev/null 2>&1; then
    dnf -y install "$@"
  elif command -v yum >/dev/null 2>&1; then
    yum -y install "$@"
  else
    echo "ERROR: no supported package manager (apt-get/dnf/yum)" >&2
    return 1
  fi
}

cws_pkg_install_file() {
  local pkg="$1"
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get install --no-install-recommends -y "$pkg" || {
      apt-get install -f -y
      apt-get install --no-install-recommends -y "$pkg"
    }
  elif command -v dnf >/dev/null 2>&1; then
    dnf -y install "$pkg"
  elif command -v yum >/dev/null 2>&1; then
    yum -y localinstall "$pkg"
  else
    echo "ERROR: cannot install local package $pkg" >&2
    return 1
  fi
}

cws_is_rpm_host() {
  command -v dnf >/dev/null 2>&1 || command -v rpm >/dev/null 2>&1
}

cws_is_deb_host() {
  command -v apt-get >/dev/null 2>&1 || command -v dpkg >/dev/null 2>&1
}

# Mark XFCE/GNOME desktop launchers as trusted for a user.
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

# Iterate login users: root + homes under /home (uid >= 1000).
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

# publish_desktop_launcher <Name.desktop> <source.desktop>
# Installs to:
#   /usr/share/applications/
#   /etc/skel/Desktop/          (future users)
#   ~/Desktop for every user    (existing users)
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
