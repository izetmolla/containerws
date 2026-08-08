#!/usr/bin/env bash
# build.sh — build frontend assets and/or the Go binary locally.
#
# Usage:
#   ./scripts/build.sh              # frontend + backend
#   ./scripts/build.sh frontend     # assets only → frontend/static
#   ./scripts/build.sh backend      # go binary only (expects frontend built)
#   ./scripts/build.sh all
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TARGET="${1:-all}"
BIN="${BIN:-containerws}"
PKG="github.com/izetmolla/containerws"

GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
VERSION="$(git describe --tags --abbrev=0 --match='v*' 2>/dev/null | sed 's/^v//' || echo '0.0.0-dev')"

build_frontend() {
  echo "==> Building frontend (Node $(node -v 2>/dev/null || echo '?'), pnpm)"
  (
    cd frontend
    if [[ -f pnpm-lock.yaml ]]; then
      pnpm install --frozen-lockfile
    else
      pnpm install
    fi
    pnpm run build
  )
  test -f frontend/static/index.html || {
    echo "error: frontend/static/index.html missing after build" >&2
    exit 1
  }
  echo "    frontend/static ready"
}

build_backend() {
  echo "==> Building backend v${VERSION} (${GIT_COMMIT})"
  go build -ldflags="-s -w -X ${PKG}/version.Version=${VERSION} -X ${PKG}/version.CommitSHA=${GIT_COMMIT}" -o "$BIN" .
  echo "    wrote ./${BIN}"
}

case "$TARGET" in
  frontend) build_frontend ;;
  backend) build_backend ;;
  all|"")
    build_frontend
    build_backend
    ;;
  -h|--help)
    sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
    ;;
  *)
    echo "unknown target: $TARGET (use frontend|backend|all)" >&2
    exit 1
    ;;
esac
