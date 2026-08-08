package seed

import "strings"

// Install Microsoft's standalone VS Code CLI (VS Code Server) and a `codeserver`
// helper that runs `code serve-web` for browser access.
//
// Docs: https://code.visualstudio.com/docs/remote/vscode-server
//
// Notes:
//   - Uses the official CLI (`serve-web` / `tunnel`), not the third-party Coder
//     "code-server" package.
//   - License forbids hosting as a multi-user service; this seed installs a
//     single-user CLI wrapper only (no systemd unit).
const vscodeServerInstallScript = `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export DEBIAN_FRONTEND=noninteractive
export PATH="/usr/local/bin:$PATH"

arch="$(uname -m)"
echo "==> Installing Microsoft VS Code Server CLI (arch=${arch})"
echo "    https://code.visualstudio.com/docs/remote/vscode-server"

apt-get update
apt-get install --no-install-recommends -y \
  ca-certificates \
  curl \
  tar \
  gzip \
  wget

case "$arch" in
  x86_64|amd64) cli_os="cli-alpine-x64" ;;
  aarch64|arm64) cli_os="cli-alpine-arm64" ;;
  *)
    echo "ERROR: unsupported architecture for VS Code CLI: $arch" >&2
    exit 1
    ;;
esac

INSTALL_DIR="/usr/local/lib/vscode-cli"
BIN_DIR="/usr/local/bin"
mkdir -p "$INSTALL_DIR" "$BIN_DIR" "$HOME/.vscode-server-web"

tmpdir="$(mktemp -d /tmp/vscode-cli.XXXXXX)"
trap 'rm -rf "$tmpdir"' EXIT

echo "==> Downloading VS Code CLI (${cli_os})"
curl -fsSL -L -A "Mozilla/5.0 containerws" \
  -o "$tmpdir/vscode_cli.tar.gz" \
  "https://code.visualstudio.com/sha/download?build=stable&os=${cli_os}"

tar -xzf "$tmpdir/vscode_cli.tar.gz" -C "$tmpdir"
CODE_SRC=""
for c in "$tmpdir/code" "$tmpdir"/code-*; do
  if [ -f "$c" ] && [ -x "$c" ]; then
    CODE_SRC="$c"
    break
  fi
done
# Some archives extract a bare 'code' file into CWD of tar.
if [ -z "$CODE_SRC" ]; then
  CODE_SRC="$(find "$tmpdir" -type f -name 'code' | head -n1 || true)"
fi
if [ -z "$CODE_SRC" ] || [ ! -f "$CODE_SRC" ]; then
  echo "ERROR: code CLI binary missing from archive" >&2
  find "$tmpdir" -maxdepth 2 -type f | head -n 40 >&2 || true
  exit 1
fi

install -m 0755 "$CODE_SRC" "$INSTALL_DIR/code"
# Keep desktop 'code' (from VS Code .deb) intact; expose CLI under code-cli too.
ln -sfn "$INSTALL_DIR/code" "$BIN_DIR/code-cli"

APP_VERSION="latest"
set +o pipefail
ver_out="$("$INSTALL_DIR/code" --version 2>/dev/null || true)"
set -o pipefail
if [ -n "$ver_out" ]; then
  APP_VERSION="$(printf '%s\n' "$ver_out" | awk 'NR==1{print; exit}')"
fi

# ---------------------------------------------------------------------------
# codeserver — convenience wrapper around: code serve-web
#
# Usage:
#   codeserver                     # serve cwd on :8443
#   codeserver /path/to/project
#   codeserver . --port 8080
#   codeserver /workspace --host 0.0.0.0 --port 8443
#   codeserver . --token auto      # require connection token (safer)
#   codeserver . --tunnel          # use 'code tunnel' (vscode.dev) instead
#
# Env:
#   CODESERVER_PORT   default 8443
#   CODESERVER_HOST   default 0.0.0.0
#   CODESERVER_TOKEN  default none | auto | <secret>
# ---------------------------------------------------------------------------
cat > "$BIN_DIR/codeserver" <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail

CODE_BIN="${CODE_BIN:-/usr/local/lib/vscode-cli/code}"
if [ ! -x "$CODE_BIN" ]; then
  echo "ERROR: VS Code CLI not found at $CODE_BIN (reinstall VS Code Server software)" >&2
  exit 1
fi

HOST="${CODESERVER_HOST:-0.0.0.0}"
PORT="${CODESERVER_PORT:-8443}"
TOKEN_MODE="${CODESERVER_TOKEN:-none}"
MODE="web" # web | tunnel
FOLDER=""
FOREGROUND=1
EXTRA=()

usage() {
  cat <<EOF
codeserver — open a folder with Microsoft VS Code Server

Usage:
  codeserver [folder] [options]

Options:
  --port <n>          Listen port (default: ${PORT} / \$CODESERVER_PORT)
  --host <addr>       Bind address (default: ${HOST} / \$CODESERVER_HOST)
  --token none|auto|<secret>
                      Connection token mode (default: none)
  --tunnel            Use 'code tunnel' for vscode.dev instead of serve-web
  --background, -d    Run in background (pid/log under ~/.vscode-server-web)
  -h, --help          Show this help

Examples:
  codeserver .
  codeserver /workspace --port 8443
  codeserver . --token auto
  codeserver . --tunnel

Docs: https://code.visualstudio.com/docs/remote/vscode-server
EOF
}

# First non-flag arg is the folder (default: cwd).
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --port)
      PORT="${2:-}"; shift 2 || true
      ;;
    --port=*)
      PORT="${1#*=}"; shift
      ;;
    --host)
      HOST="${2:-}"; shift 2 || true
      ;;
    --host=*)
      HOST="${1#*=}"; shift
      ;;
    --token)
      TOKEN_MODE="${2:-}"; shift 2 || true
      ;;
    --token=*)
      TOKEN_MODE="${1#*=}"; shift
      ;;
    --tunnel)
      MODE="tunnel"; shift
      ;;
    --background|-d)
      FOREGROUND=0; shift
      ;;
    --)
      shift
      EXTRA+=("$@")
      break
      ;;
    -*)
      EXTRA+=("$1"); shift
      ;;
    *)
      if [ -z "$FOLDER" ]; then
        FOLDER="$1"
      else
        EXTRA+=("$1")
      fi
      shift
      ;;
  esac
done

FOLDER="$(realpath -m "${FOLDER:-.}")"
if [ ! -d "$FOLDER" ]; then
  echo "ERROR: folder not found: $FOLDER" >&2
  exit 1
fi

DATA_DIR="${HOME:-/root}/.vscode-server-web"
mkdir -p "$DATA_DIR" "$DATA_DIR/logs"
cd "$FOLDER"

if [ "$MODE" = "tunnel" ]; then
  echo "==> Starting VS Code tunnel for: $FOLDER"
  echo "    Authenticate when prompted, then open the printed vscode.dev URL."
  echo "    License: single-user use only (not a multi-user hosted service)."
  exec "$CODE_BIN" tunnel \
    --accept-server-license-terms \
    --cli-data-dir "$DATA_DIR" \
    "${EXTRA[@]}"
fi

ARGS=(
  serve-web
  --accept-server-license-terms
  --host "$HOST"
  --port "$PORT"
  --default-folder "$FOLDER"
  --server-data-dir "$DATA_DIR"
  --cli-data-dir "$DATA_DIR"
)

case "$TOKEN_MODE" in
  none|"")
    ARGS+=(--without-connection-token)
    ;;
  auto)
    # Let the CLI generate a token and print it in the URL.
    ;;
  *)
    ARGS+=(--connection-token "$TOKEN_MODE")
    ;;
esac

ARGS+=("${EXTRA[@]}")

echo "==> VS Code Server (serve-web)"
echo "    folder: $FOLDER"
echo "    listen: http://${HOST}:${PORT}"
echo "    data:   $DATA_DIR"
echo "    tip:    open http://<host>:${PORT}/ in a browser"
echo "    note:   single-user CLI (license forbids multi-user hosting as a service)"
echo

if [ "$FOREGROUND" -eq 1 ]; then
  exec "$CODE_BIN" "${ARGS[@]}"
fi

LOG="$DATA_DIR/logs/serve-web-${PORT}.log"
PIDFILE="$DATA_DIR/serve-web-${PORT}.pid"
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "Already running (pid $(cat "$PIDFILE")). Log: $LOG"
  exit 0
fi
nohup "$CODE_BIN" "${ARGS[@]}" >"$LOG" 2>&1 &
echo $! >"$PIDFILE"
echo "Started in background (pid $(cat "$PIDFILE"))."
echo "Log: $LOG"
WRAPPER
chmod 755 "$BIN_DIR/codeserver"

# Soft alias some people expect.
ln -sfn "$BIN_DIR/codeserver" "$BIN_DIR/code-server-web" 2>/dev/null || true

echo "==> VS Code Server CLI installed (${APP_VERSION})"
echo "    binary:    $INSTALL_DIR/code"
echo "    cli alias: $BIN_DIR/code-cli"
echo "    wrapper:   $BIN_DIR/codeserver"
echo
echo "Try:"
echo "  codeserver .                 # serve current dir on :8443"
echo "  codeserver /path --port 8080"
echo "  codeserver . --tunnel        # vscode.dev tunnel"
echo "  code-cli serve-web -h"
`

const vscodeServerUninstallScript = `#!/bin/bash
set -euo pipefail

echo "==> Removing VS Code Server CLI and codeserver helper"
rm -f /usr/local/bin/code /usr/local/bin/codeserver 2>/dev/null || true
rm -rf "$HOME/.vscode-server" "$HOME/.vscode" 2>/dev/null || true
echo "==> VS Code Server removed"
`

func vscodeServerCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "VS Code Server",
			Details:     "Install Microsoft's VS Code CLI and a codeserver helper (serve-web / tunnel) to open folders in the browser. See https://code.visualstudio.com/docs/remote/vscode-server",
			Category:    "Desktop",
			SubCategory: "Editors",
			Tags:        []string{"vscode", "code-server", "serve-web", "tunnel", "remote", "browser"},
			Icon:        "Cloud",
			Color:       "#0078D4",
			Order:       7,
			IsActive:    true,
			// No ServiceUnits: license forbids hosting as a multi-user service.
		},
		Versions: []VersionMeta{
			{
				Version:       "cli-1",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(vscodeServerInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(vscodeServerUninstallScript) + "\n",
			},
		},
	}
}
