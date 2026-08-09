#!/usr/bin/env bash
# Container Workspace — native binary installer
# Installs the cws/containerws binary from GitHub Releases (or a local file),
# prepares host directories, links the CLI, and installs an OS daemon that runs:
#   cws --start
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/izetmolla/containerws/main/install/install.sh | bash
#   sudo bash install.sh
#   sudo bash install.sh --version v1.2.3
#   sudo bash install.sh --binary ./containerws
#   sudo bash install.sh --uninstall
#
if [ -z "${BASH_VERSION:-}" ]; then
  echo "error: this installer requires bash." >&2
  exit 1
fi
set -euo pipefail

readonly CWS_REPO_DEFAULT="izetmolla/containerws"
readonly CWS_BIN_NAME="containerws"
readonly CWS_CLI_NAME="cws"
readonly CWS_INSTALL_DIR="/usr/local/lib/containerws"
readonly CWS_BIN_DIR="${CWS_INSTALL_DIR}/bin"
readonly CWS_BIN_PATH="${CWS_BIN_DIR}/${CWS_BIN_NAME}"
readonly CWS_CLI_PATH="/usr/local/bin/${CWS_CLI_NAME}"
readonly CWS_ALIAS_PATH="/usr/local/bin/${CWS_BIN_NAME}"
readonly CWS_CONFIG_ROOT="/config/containerws"
readonly CWS_ETC_DIR="/etc/containerws"
readonly CWS_ENV_FILE="${CWS_ETC_DIR}/environment"
readonly CWS_SERVICE_NAME="containerws"
readonly CWS_UNIT_PATH="/etc/systemd/system/${CWS_SERVICE_NAME}.service"
readonly CWS_LAUNCHD_LABEL="com.izetmolla.containerws"
readonly CWS_LAUNCHD_PATH="/Library/LaunchDaemons/${CWS_LAUNCHD_LABEL}.plist"
readonly CWS_OPENRC_PATH="/etc/init.d/${CWS_SERVICE_NAME}"
readonly CWS_FREEBSD_RC="/usr/local/etc/rc.d/${CWS_SERVICE_NAME}"
readonly CWS_PIDFILE="/var/run/containerws.pid"
readonly CWS_DAEMON_WRAPPER="${CWS_BIN_DIR}/cws-daemon.sh"
readonly CWS_CRON_FILE="/etc/cron.d/containerws"
readonly CWS_LOG_OUT="/var/log/containerws/cws.out.log"
readonly CWS_LOG_ERR="/var/log/containerws/cws.err.log"

CWS_REPO="${CWS_REPO:-$CWS_REPO_DEFAULT}"
CWS_VERSION="${CWS_VERSION:-}"
CWS_BINARY_SRC="${CWS_BINARY_SRC:-}"
CWS_NO_START="${CWS_NO_START:-0}"
CWS_ACTION="install"

# ---------------------------------------------------------------------------
# UI
# ---------------------------------------------------------------------------
_cws_init_colors() {
  if [[ -t 1 ]] && command -v tput >/dev/null 2>&1 && [[ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]]; then
    C_RESET="$(tput sgr0)"
    C_BOLD="$(tput bold)"
    C_DIM="$(tput dim 2>/dev/null || true)"
    C_RED="$(tput setaf 1)"
    C_GREEN="$(tput setaf 2)"
    C_YELLOW="$(tput setaf 3)"
    C_CYAN="$(tput setaf 6)"
  else
    C_RESET="" C_BOLD="" C_DIM=""
    C_RED="" C_GREEN="" C_YELLOW="" C_CYAN=""
  fi
}

die()  { printf '%s\n' "${C_RED}${C_BOLD}error:${C_RESET} $*" >&2; exit 1; }
info() { printf '%s\n' "${C_CYAN}→${C_RESET} $*"; }
ok()   { printf '%s\n' "${C_GREEN}✓${C_RESET} $*"; }
warn() { printf '%s\n' "${C_YELLOW}!${C_RESET} $*"; }

usage() {
  cat <<EOF
Container Workspace installer — native binary + OS daemon

Usage:
  sudo bash install.sh [options]

Options:
  --version VER     Install release tag (e.g. v1.2.3). Default: latest
  --binary PATH     Install from a local binary instead of downloading
  --repo OWNER/NAME GitHub repo (default: ${CWS_REPO_DEFAULT})
  --no-start        Install and enable daemon but do not start it yet
  --uninstall       Stop daemon and remove installed files
  -h, --help        Show this help

Environment:
  CWS_VERSION, CWS_BINARY_SRC, CWS_REPO, CWS_NO_START=1

Install layout:
  ${CWS_BIN_PATH}
  ${CWS_CLI_PATH}  →  ${CWS_BIN_NAME}
  ${CWS_ALIAS_PATH}  →  ${CWS_BIN_NAME}
  ${CWS_CONFIG_ROOT}/{database,ssl,vnc-sessions}
  ${CWS_ENV_FILE}
  systemd / launchd / OpenRC / FreeBSD / direct → ${CWS_CLI_NAME} --start
EOF
}

# ---------------------------------------------------------------------------
# Args / root
# ---------------------------------------------------------------------------
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        [[ $# -ge 2 ]] || die "--version requires a value"
        CWS_VERSION="$2"
        shift 2
        ;;
      --binary)
        [[ $# -ge 2 ]] || die "--binary requires a path"
        CWS_BINARY_SRC="$2"
        shift 2
        ;;
      --repo)
        [[ $# -ge 2 ]] || die "--repo requires owner/name"
        CWS_REPO="$2"
        shift 2
        ;;
      --no-start)
        CWS_NO_START=1
        shift
        ;;
      --uninstall)
        CWS_ACTION="uninstall"
        shift
        ;;
      --cws-inner)
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1 (try --help)"
        ;;
    esac
  done
}

_cws_resolve_self() {
  local src="${BASH_SOURCE[0]:-}"
  if [[ -n "$src" && -r "$src" && -f "$src" && "$src" != /dev/fd/* && "$src" != /proc/self/fd/* ]]; then
    printf '%s' "$src"
    return 0
  fi
  if [[ -n "$src" && -r "$src" ]]; then
    local copy
    copy="$(mktemp /tmp/cws-install.XXXXXX.sh)"
    cat "$src" >"$copy"
    chmod +x "$copy"
    printf '%s' "$copy"
    return 0
  fi
  return 1
}

ensure_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    return 0
  fi
  local self
  if ! self="$(_cws_resolve_self)"; then
    die "Root required. Re-run with sudo, or: curl -sSL … -o /tmp/cws-install.sh && sudo bash /tmp/cws-install.sh"
  fi
  warn "Root required — re-executing with sudo…"
  local -a args=(--cws-inner --repo "$CWS_REPO")
  [[ -n "$CWS_VERSION" ]] && args+=(--version "$CWS_VERSION")
  [[ -n "$CWS_BINARY_SRC" ]] && args+=(--binary "$CWS_BINARY_SRC")
  [[ "$CWS_NO_START" == "1" ]] && args+=(--no-start)
  [[ "$CWS_ACTION" == "uninstall" ]] && args+=(--uninstall)
  exec sudo -E bash "$self" "${args[@]}"
}

# ---------------------------------------------------------------------------
# Platform
# ---------------------------------------------------------------------------
detect_platform() {
  local uname_s uname_m
  uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
  uname_m="$(uname -m)"

  case "$uname_s" in
    linux*) OS="linux" ;;
    darwin*) OS="darwin" ;;
    freebsd*) OS="freebsd" ;;
    openbsd*) OS="openbsd" ;;
    msys*|mingw*|cygwin*) OS="windows" ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac

  case "$uname_m" in
    x86_64|amd64) ARCH="amd64" ;;
    i386|i686|x86) ARCH="386" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armv7) ARCH="armv7" ;;
    armv6l|armv6) ARCH="armv6" ;;
    armv5*|arm) ARCH="armv5" ;;
    riscv64) ARCH="riscv64" ;;
    *) die "unsupported architecture: $uname_m" ;;
  esac

  if [[ "$OS" == "windows" ]]; then
    die "Windows host install is not supported by this script. Use a released .zip manually."
  fi
}

# True when systemd is actually PID 1 / usable (not merely "systemctl" on PATH).
systemd_usable() {
  command -v systemctl >/dev/null 2>&1 || return 1
  [[ -d /run/systemd/system ]] || return 1
  local state
  state="$(systemctl is-system-running 2>/dev/null || true)"
  case "$state" in
    running|degraded|maintenance|initializing|starting) return 0 ;;
  esac
  # Some minimal environments report "offline" / "unknown" even with a real PID 1 systemd.
  if [[ "$(ps -p 1 -o comm= 2>/dev/null || true)" == "systemd" ]]; then
    return 0
  fi
  return 1
}

detect_init() {
  INIT_SYSTEM="direct"
  if [[ "$OS" == "darwin" ]]; then
    INIT_SYSTEM="launchd"
    return 0
  fi
  if [[ "$OS" == "freebsd" ]]; then
    INIT_SYSTEM="freebsd-rc"
    return 0
  fi
  if systemd_usable; then
    INIT_SYSTEM="systemd"
    return 0
  fi
  # OpenRC only when it is actually managing the system (not merely packages present).
  if command -v rc-status >/dev/null 2>&1 && rc-status >/dev/null 2>&1; then
    INIT_SYSTEM="openrc"
    return 0
  fi
  if command -v rc-update >/dev/null 2>&1 && [[ -d /run/openrc || -d /etc/runlevels ]]; then
    INIT_SYSTEM="openrc"
    return 0
  fi
  # Classic SysV only when PID 1 looks like sysvinit (avoid Docker images that ship
  # update-rc.d but cannot supervise services).
  local pid1
  pid1="$(ps -p 1 -o comm= 2>/dev/null || true)"
  if [[ -d /etc/init.d ]] && command -v update-rc.d >/dev/null 2>&1; then
    case "$pid1" in
      init|sysvinit|busybox)
        INIT_SYSTEM="sysv"
        return 0
        ;;
    esac
  fi
  # Containers / WSL / hosts without a usable init — run cws --start ourselves.
  INIT_SYSTEM="direct"
}

# ---------------------------------------------------------------------------
# OS packages needed to fetch/unpack and run
# ---------------------------------------------------------------------------
pkg_install() {
  local -a pkgs=("$@")
  [[ ${#pkgs[@]} -eq 0 ]] && return 0

  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y --no-install-recommends "${pkgs[@]}"
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y "${pkgs[@]}"
  elif command -v yum >/dev/null 2>&1; then
    yum install -y "${pkgs[@]}"
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache "${pkgs[@]}"
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm "${pkgs[@]}"
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install "${pkgs[@]}"
  elif command -v pkg >/dev/null 2>&1 && [[ "$OS" == "freebsd" ]]; then
    pkg install -y "${pkgs[@]}"
  elif command -v brew >/dev/null 2>&1; then
    brew install "${pkgs[@]}"
  else
    warn "No supported package manager found; ensure ${pkgs[*]} are installed."
  fi
}

prepare_os() {
  info "Preparing OS packages…"
  local -a need=()
  local p

  for p in curl ca-certificates tar gzip; do
    case "$p" in
      ca-certificates)
        # Present under different names; skip if curl already works with HTTPS.
        if ! command -v update-ca-certificates >/dev/null 2>&1 \
          && ! [[ -f /etc/ssl/certs/ca-certificates.crt ]] \
          && ! [[ -f /etc/pki/tls/certs/ca-bundle.crt ]] \
          && ! [[ -f /etc/ssl/cert.pem ]]; then
          need+=("$p")
        fi
        ;;
      curl)
        command -v curl >/dev/null 2>&1 || need+=("$p")
        ;;
      tar)
        command -v tar >/dev/null 2>&1 || need+=("$p")
        ;;
      gzip)
        command -v gzip >/dev/null 2>&1 || need+=("$p")
        ;;
    esac
  done

  # Optional but useful for Softwares / service management on Linux hosts.
  if [[ "$OS" == "linux" ]]; then
    command -v systemctl >/dev/null 2>&1 || true
  fi

  if [[ ${#need[@]} -gt 0 ]]; then
    pkg_install "${need[@]}"
  fi

  command -v curl >/dev/null 2>&1 || die "curl is required"
  command -v tar >/dev/null 2>&1 || die "tar is required"
  ok "Host tools ready"
}

# ---------------------------------------------------------------------------
# Directories
# ---------------------------------------------------------------------------
prepare_dirs() {
  info "Creating directories…"
  mkdir -p \
    "$CWS_BIN_DIR" \
    /usr/local/bin \
    "${CWS_CONFIG_ROOT}/database" \
    "${CWS_CONFIG_ROOT}/ssl" \
    "${CWS_CONFIG_ROOT}/vnc-sessions" \
    "$CWS_ETC_DIR" \
    /var/lib/containerws \
    /var/log/containerws

  chmod 755 /config /config/containerws "$CWS_INSTALL_DIR" "$CWS_BIN_DIR" 2>/dev/null || true
  chmod 755 "${CWS_CONFIG_ROOT}/database" "${CWS_CONFIG_ROOT}/ssl" "${CWS_CONFIG_ROOT}/vnc-sessions"
  ok "Directories ready under ${CWS_CONFIG_ROOT} and ${CWS_INSTALL_DIR}"
}

write_env_file() {
  if [[ -f "$CWS_ENV_FILE" ]]; then
    info "Keeping existing ${CWS_ENV_FILE}"
    return 0
  fi
  cat >"$CWS_ENV_FILE" <<EOF
# Container Workspace daemon environment
ENV=production
# DATABASE_URL=${CWS_CONFIG_ROOT}/database/database.sqlite
# Optional: MCP_PORT=9100
# Optional: ENABLE_HTTPS=true
EOF
  chmod 644 "$CWS_ENV_FILE"
  ok "Wrote ${CWS_ENV_FILE}"
}

# ---------------------------------------------------------------------------
# Binary download / install
# ---------------------------------------------------------------------------
github_api() {
  local url="$1"
  local -a curl_opts=(-fsSL)
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl_opts+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  curl_opts+=(-H "Accept: application/vnd.github+json")
  curl "${curl_opts[@]}" "$url"
}

resolve_version() {
  if [[ -n "$CWS_VERSION" ]]; then
    [[ "$CWS_VERSION" == v* ]] || CWS_VERSION="v${CWS_VERSION}"
    return 0
  fi
  info "Resolving latest release from GitHub (${CWS_REPO})…"
  local json tag
  json="$(github_api "https://api.github.com/repos/${CWS_REPO}/releases/latest")" || die "failed to query GitHub releases"
  tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [[ -n "$tag" ]] || die "could not parse latest release tag"
  CWS_VERSION="$tag"
  ok "Latest release: ${CWS_VERSION}"
}

archive_name() {
  # Matches .goreleaser.yml name_template
  printf '%s_%s_%s_%s.tar.gz' "$CWS_BIN_NAME" "${CWS_VERSION#v}" "$OS" "$ARCH"
}

download_binary() {
  resolve_version
  local asset archive_url tmp
  asset="$(archive_name)"
  archive_url="https://github.com/${CWS_REPO}/releases/download/${CWS_VERSION}/${asset}"
  tmp="$(mktemp -d /tmp/cws-install.XXXXXX)"
  trap 'rm -rf "'"$tmp"'"' RETURN

  info "Downloading ${asset}…"
  if ! curl -fsSL -o "${tmp}/${asset}" "$archive_url"; then
    die "download failed: ${archive_url}"
  fi

  info "Extracting…"
  tar -xzf "${tmp}/${asset}" -C "$tmp"
  if [[ -f "${tmp}/${CWS_BIN_NAME}" ]]; then
    CWS_BINARY_SRC="${tmp}/${CWS_BIN_NAME}"
  elif [[ -f "${tmp}/bin/${CWS_BIN_NAME}" ]]; then
    CWS_BINARY_SRC="${tmp}/bin/${CWS_BIN_NAME}"
  else
    CWS_BINARY_SRC="$(find "$tmp" -type f -name "$CWS_BIN_NAME" | head -n1 || true)"
  fi
  [[ -n "$CWS_BINARY_SRC" && -f "$CWS_BINARY_SRC" ]] || die "archive did not contain ${CWS_BIN_NAME}"
  install_binary_from_src
  trap - RETURN
  rm -rf "$tmp"
}

install_binary_from_src() {
  [[ -n "$CWS_BINARY_SRC" && -f "$CWS_BINARY_SRC" ]] || die "binary source missing"
  info "Installing binary to ${CWS_BIN_PATH}"
  install -m 0755 "$CWS_BINARY_SRC" "$CWS_BIN_PATH"
  # Absolute symlinks so PATH shims stay valid regardless of cwd.
  ln -sfn "$CWS_BIN_PATH" "$CWS_CLI_PATH"
  ln -sfn "$CWS_BIN_PATH" "$CWS_ALIAS_PATH"
  ok "CLI linked: ${CWS_CLI_PATH} and ${CWS_ALIAS_PATH}"
  if [[ -x "$CWS_BIN_PATH" ]]; then
    "$CWS_BIN_PATH" version 2>/dev/null || true
  fi
}

# ---------------------------------------------------------------------------
# Daemon units
# ---------------------------------------------------------------------------
write_systemd_unit() {
  cat >"$CWS_UNIT_PATH" <<EOF
[Unit]
Description=Container Workspace (cws)
Documentation=https://github.com/${CWS_REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-${CWS_ENV_FILE}
Environment=ENV=production
WorkingDirectory=/
ExecStart=${CWS_BIN_PATH} --start
Restart=on-failure
RestartSec=3
KillMode=mixed
TimeoutStopSec=30
LimitNOFILE=1048576

# Keep logs in journal by default
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
  chmod 644 "$CWS_UNIT_PATH"
}

write_launchd_plist() {
  cat >"$CWS_LAUNCHD_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${CWS_LAUNCHD_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${CWS_BIN_PATH}</string>
    <string>--start</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>ENV</key>
    <string>production</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>/</string>
  <key>StandardOutPath</key>
  <string>${CWS_LOG_OUT}</string>
  <key>StandardErrorPath</key>
  <string>${CWS_LOG_ERR}</string>
</dict>
</plist>
EOF
  chmod 644 "$CWS_LAUNCHD_PATH"
}

write_openrc_service() {
  cat >"$CWS_OPENRC_PATH" <<EOF
#!/sbin/openrc-run
description="Container Workspace (cws)"
command="${CWS_BIN_PATH}"
command_args="--start"
command_background=yes
pidfile="/run/\${RC_SVCNAME}.pid"
output_log="${CWS_LOG_OUT}"
error_log="${CWS_LOG_ERR}"

depend() {
  need net
  after firewall
}

start_pre() {
  checkpath --directory /var/log/containerws
  if [ -f "${CWS_ENV_FILE}" ]; then
    set -a
    # shellcheck disable=SC1090
    . "${CWS_ENV_FILE}"
    set +a
  fi
  export ENV="\${ENV:-production}"
}
EOF
  chmod 755 "$CWS_OPENRC_PATH"
}

write_freebsd_rc() {
  cat >"$CWS_FREEBSD_RC" <<EOF
#!/bin/sh
# PROVIDE: containerws
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="${CWS_SERVICE_NAME}"
rcvar="\${name}_enable"
command="${CWS_BIN_PATH}"
command_args="--start"
pidfile="${CWS_PIDFILE}"
start_cmd="containerws_start"
stop_cmd="containerws_stop"
status_cmd="containerws_status"

load_rc_config \$name
: \${containerws_enable:="NO"}

containerws_start() {
  export ENV=production
  if [ -f "${CWS_ENV_FILE}" ]; then
    set -a
    . "${CWS_ENV_FILE}"
    set +a
  fi
  /usr/sbin/daemon -p "\${pidfile}" -f \${command} \${command_args}
}

containerws_stop() {
  if [ -f "\${pidfile}" ]; then
    kill "\$(cat "\${pidfile}")" 2>/dev/null || true
    rm -f "\${pidfile}"
  fi
}

containerws_status() {
  if [ -f "\${pidfile}" ] && kill -0 "\$(cat "\${pidfile}")" 2>/dev/null; then
    echo "\${name} is running as pid \$(cat "\${pidfile}")."
  else
    echo "\${name} is not running."
    return 1
  fi
}

run_rc_command "\$1"
EOF
  chmod 755 "$CWS_FREEBSD_RC"
}

# ---------------------------------------------------------------------------
# Direct daemon (no systemd / OpenRC) — used in Docker, WSL, bare containers
# ---------------------------------------------------------------------------
write_direct_daemon_wrapper() {
  mkdir -p "$CWS_BIN_DIR" /var/log/containerws
  cat >"$CWS_DAEMON_WRAPPER" <<EOF
#!/usr/bin/env bash
# Container Workspace direct daemon helper (cws --start)
set -euo pipefail
BIN="${CWS_BIN_PATH}"
PIDFILE="${CWS_PIDFILE}"
ENV_FILE="${CWS_ENV_FILE}"
LOG_OUT="${CWS_LOG_OUT}"
LOG_ERR="${CWS_LOG_ERR}"

load_env() {
  export ENV="\${ENV:-production}"
  if [[ -f "\$ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "\$ENV_FILE"
    set +a
  fi
}

is_running() {
  [[ -f "\$PIDFILE" ]] || return 1
  local pid
  pid="\$(tr -d '[:space:]' <"\$PIDFILE" 2>/dev/null || true)"
  [[ -n "\$pid" ]] || return 1
  kill -0 "\$pid" 2>/dev/null || return 1
  return 0
}

cmd_start() {
  if is_running; then
    echo "containerws already running (pid \$(cat "\$PIDFILE"))"
    return 0
  fi
  load_env
  mkdir -p "\$(dirname "\$PIDFILE")" "\$(dirname "\$LOG_OUT")"
  # Drop a stale pidfile
  rm -f "\$PIDFILE"
  nohup "\$BIN" --start >>"\$LOG_OUT" 2>>"\$LOG_ERR" &
  local pid=\$!
  echo "\$pid" >"\$PIDFILE"
  # Give the server a moment to bind or crash
  sleep 1
  if ! kill -0 "\$pid" 2>/dev/null; then
    echo "error: cws --start exited immediately; see \$LOG_ERR" >&2
    rm -f "\$PIDFILE"
    return 1
  fi
  echo "started containerws (pid \$pid)"
}

cmd_stop() {
  if ! [[ -f "\$PIDFILE" ]]; then
    # Best-effort: kill stray cws --start for our binary path
    pkill -f "\$BIN --start" 2>/dev/null || true
    echo "containerws not running"
    return 0
  fi
  local pid
  pid="\$(tr -d '[:space:]' <"\$PIDFILE")"
  if [[ -n "\$pid" ]] && kill -0 "\$pid" 2>/dev/null; then
    kill "\$pid" 2>/dev/null || true
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "\$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "\$pid" 2>/dev/null || true
  fi
  rm -f "\$PIDFILE"
  echo "stopped containerws"
}

cmd_status() {
  if is_running; then
    echo "containerws is running (pid \$(cat "\$PIDFILE"))"
    return 0
  fi
  echo "containerws is not running"
  return 1
}

cmd_restart() {
  cmd_stop || true
  cmd_start
}

case "\${1:-}" in
  start) cmd_start ;;
  stop) cmd_stop ;;
  restart) cmd_restart ;;
  status) cmd_status ;;
  *)
    echo "usage: \$0 {start|stop|restart|status}" >&2
    exit 1
    ;;
esac
EOF
  chmod 755 "$CWS_DAEMON_WRAPPER"
}

install_direct_cron() {
  # Persist across reboot when possible (skipped if no cron.d).
  if [[ ! -d /etc/cron.d ]]; then
    return 0
  fi
  cat >"$CWS_CRON_FILE" <<EOF
# Restart Container Workspace after reboot (direct daemon mode)
@reboot root ${CWS_DAEMON_WRAPPER} start >/dev/null 2>&1
EOF
  chmod 644 "$CWS_CRON_FILE"
}

daemon_is_running() {
  case "$INIT_SYSTEM" in
    systemd)
      systemctl is-active --quiet "$CWS_SERVICE_NAME" 2>/dev/null
      ;;
    launchd)
      launchctl print "system/${CWS_LAUNCHD_LABEL}" 2>/dev/null | grep -q 'state = running'
      ;;
    openrc)
      rc-service "$CWS_SERVICE_NAME" status >/dev/null 2>&1
      ;;
    freebsd-rc|sysv)
      service "$CWS_SERVICE_NAME" status >/dev/null 2>&1
      ;;
    direct)
      [[ -x "$CWS_DAEMON_WRAPPER" ]] && "$CWS_DAEMON_WRAPPER" status >/dev/null 2>&1
      ;;
    *)
      return 1
      ;;
  esac
}

verify_daemon_started() {
  [[ "$CWS_NO_START" == "1" ]] && return 0
  local i
  for i in 1 2 3 4 5 6 7 8 9 10; do
    if daemon_is_running; then
      ok "Daemon is running (${CWS_CLI_NAME} --start)"
      return 0
    fi
    # Also accept a live listener on :9000 (covers briefly-racy pid checks).
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS -o /dev/null --connect-timeout 1 "http://127.0.0.1:9000/" 2>/dev/null \
        || curl -fsS -o /dev/null --connect-timeout 1 -k "https://127.0.0.1:9000/" 2>/dev/null; then
        ok "Daemon is listening on :9000"
        return 0
      fi
    fi
    sleep 0.5
  done
  warn "Daemon did not stay up after install."
  case "$INIT_SYSTEM" in
    systemd)
      systemctl status "$CWS_SERVICE_NAME" --no-pager -l 2>&1 | tail -n 30 || true
      journalctl -u "$CWS_SERVICE_NAME" -n 40 --no-pager 2>&1 | tail -n 40 || true
      ;;
    direct)
      [[ -f "$CWS_LOG_ERR" ]] && tail -n 40 "$CWS_LOG_ERR" || true
      [[ -f "$CWS_LOG_OUT" ]] && tail -n 20 "$CWS_LOG_OUT" || true
      ;;
  esac
  die "failed to start ${CWS_CLI_NAME} --start (init=${INIT_SYSTEM})"
}

install_daemon() {
  info "Installing ${INIT_SYSTEM} daemon (${CWS_CLI_NAME} --start)…"
  case "$INIT_SYSTEM" in
    systemd)
      write_systemd_unit
      systemctl daemon-reload
      systemctl enable "$CWS_SERVICE_NAME"
      if [[ "$CWS_NO_START" != "1" ]]; then
        systemctl restart "$CWS_SERVICE_NAME"
        ok "systemd service ${CWS_SERVICE_NAME} enabled and started"
      else
        ok "systemd service ${CWS_SERVICE_NAME} enabled (not started)"
      fi
      ;;
    launchd)
      write_launchd_plist
      launchctl bootout system/"$CWS_LAUNCHD_LABEL" 2>/dev/null || true
      launchctl bootstrap system "$CWS_LAUNCHD_PATH"
      if [[ "$CWS_NO_START" != "1" ]]; then
        launchctl kickstart -k system/"$CWS_LAUNCHD_LABEL" || launchctl enable system/"$CWS_LAUNCHD_LABEL"
        ok "launchd job ${CWS_LAUNCHD_LABEL} loaded"
      else
        ok "launchd plist installed at ${CWS_LAUNCHD_PATH}"
      fi
      ;;
    openrc)
      write_openrc_service
      rc-update add "$CWS_SERVICE_NAME" default
      if [[ "$CWS_NO_START" != "1" ]]; then
        rc-service "$CWS_SERVICE_NAME" restart || rc-service "$CWS_SERVICE_NAME" start
        ok "OpenRC service ${CWS_SERVICE_NAME} started"
      else
        ok "OpenRC service ${CWS_SERVICE_NAME} added (not started)"
      fi
      ;;
    freebsd-rc)
      write_freebsd_rc
      if ! grep -q '^containerws_enable=' /etc/rc.conf 2>/dev/null; then
        echo 'containerws_enable="YES"' >>/etc/rc.conf
      fi
      if [[ "$CWS_NO_START" != "1" ]]; then
        service "$CWS_SERVICE_NAME" restart || service "$CWS_SERVICE_NAME" start
        ok "FreeBSD rc service ${CWS_SERVICE_NAME} started"
      else
        ok "FreeBSD rc service installed (not started)"
      fi
      ;;
    sysv)
      write_openrc_service
      update-rc.d "$CWS_SERVICE_NAME" defaults || true
      if [[ "$CWS_NO_START" != "1" ]]; then
        service "$CWS_SERVICE_NAME" restart || service "$CWS_SERVICE_NAME" start || true
      fi
      ok "SysV init script installed at ${CWS_OPENRC_PATH}"
      ;;
    direct)
      write_direct_daemon_wrapper
      install_direct_cron
      # Prefer our wrapper even if a leftover systemd unit exists but systemd is down.
      if [[ "$CWS_NO_START" != "1" ]]; then
        "$CWS_DAEMON_WRAPPER" restart
        ok "Started ${CWS_CLI_NAME} --start in direct daemon mode (pidfile ${CWS_PIDFILE})"
      else
        ok "Direct daemon helper installed at ${CWS_DAEMON_WRAPPER} (not started)"
      fi
      ;;
    *)
      die "internal error: unknown init system ${INIT_SYSTEM}"
      ;;
  esac
  verify_daemon_started
}

# ---------------------------------------------------------------------------
# Uninstall
# ---------------------------------------------------------------------------
do_uninstall() {
  info "Stopping and removing Container Workspace…"
  case "$INIT_SYSTEM" in
    systemd)
      systemctl disable --now "$CWS_SERVICE_NAME" 2>/dev/null || true
      rm -f "$CWS_UNIT_PATH"
      systemctl daemon-reload 2>/dev/null || true
      ;;
    launchd)
      launchctl bootout system/"$CWS_LAUNCHD_LABEL" 2>/dev/null || true
      rm -f "$CWS_LAUNCHD_PATH"
      ;;
    openrc)
      rc-service "$CWS_SERVICE_NAME" stop 2>/dev/null || true
      rc-update del "$CWS_SERVICE_NAME" default 2>/dev/null || true
      rm -f "$CWS_OPENRC_PATH"
      ;;
    freebsd-rc)
      service "$CWS_SERVICE_NAME" stop 2>/dev/null || true
      rm -f "$CWS_FREEBSD_RC"
      ;;
    sysv)
      service "$CWS_SERVICE_NAME" stop 2>/dev/null || true
      update-rc.d -f "$CWS_SERVICE_NAME" remove 2>/dev/null || true
      rm -f "$CWS_OPENRC_PATH"
      ;;
    direct)
      if [[ -x "$CWS_DAEMON_WRAPPER" ]]; then
        "$CWS_DAEMON_WRAPPER" stop 2>/dev/null || true
      fi
      rm -f "$CWS_CRON_FILE" "$CWS_PIDFILE"
      ;;
  esac
  # Always try to stop a leftover direct daemon.
  if [[ -x "$CWS_DAEMON_WRAPPER" ]]; then
    "$CWS_DAEMON_WRAPPER" stop 2>/dev/null || true
  fi
  pkill -f "${CWS_BIN_PATH} --start" 2>/dev/null || true

  rm -f "$CWS_CLI_PATH" "$CWS_ALIAS_PATH" "$CWS_CRON_FILE" "$CWS_PIDFILE"
  rm -rf "$CWS_INSTALL_DIR"
  # Keep /config/containerws data and /etc/containerws by default.
  ok "Binaries and daemon removed. Data kept in ${CWS_CONFIG_ROOT} and ${CWS_ETC_DIR}"
}

print_done() {
  echo
  ok "Container Workspace installed."
  cat <<EOF

  Binary:   ${CWS_BIN_PATH}
  CLI:      ${CWS_CLI_PATH}  (also ${CWS_ALIAS_PATH})
  Config:   ${CWS_CONFIG_ROOT}
  Env file: ${CWS_ENV_FILE}
  Daemon:   ${INIT_SYSTEM} → ${CWS_CLI_NAME} --start

  Web UI:   http://127.0.0.1:9000
  Version:  ${CWS_VERSION:-local}

EOF
  case "$INIT_SYSTEM" in
    systemd)
      cat <<EOF
  Status:   systemctl status ${CWS_SERVICE_NAME}
  Logs:     journalctl -u ${CWS_SERVICE_NAME} -f
EOF
      ;;
    launchd)
      cat <<EOF
  Status:   launchctl print system/${CWS_LAUNCHD_LABEL}
  Logs:     /var/log/containerws/
EOF
      ;;
    openrc)
      cat <<EOF
  Status:   rc-service ${CWS_SERVICE_NAME} status
EOF
      ;;
    freebsd-rc)
      cat <<EOF
  Status:   service ${CWS_SERVICE_NAME} status
EOF
      ;;
    direct|sysv)
      cat <<EOF
  Status:   ${CWS_DAEMON_WRAPPER} status
  Start:    ${CWS_DAEMON_WRAPPER} start
  Stop:     ${CWS_DAEMON_WRAPPER} stop
  Logs:     ${CWS_LOG_OUT}
            ${CWS_LOG_ERR}
EOF
      ;;
  esac
  echo
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  _cws_init_colors
  parse_args "$@"
  ensure_root
  detect_platform
  detect_init

  if [[ "$CWS_ACTION" == "uninstall" ]]; then
    do_uninstall
    return 0
  fi

  cat <<EOF

${C_BOLD}${C_CYAN}Container Workspace — native installer${C_RESET}
  OS/arch: ${OS}/${ARCH}
  Init:    ${INIT_SYSTEM}
  Repo:    ${CWS_REPO}

EOF

  prepare_os
  prepare_dirs
  write_env_file

  if [[ -n "$CWS_BINARY_SRC" ]]; then
    [[ -f "$CWS_BINARY_SRC" ]] || die "binary not found: ${CWS_BINARY_SRC}"
    install_binary_from_src
  else
    download_binary
  fi

  install_daemon
  print_done
}

main "$@"
