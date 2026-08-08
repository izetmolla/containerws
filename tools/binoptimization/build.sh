#!/usr/bin/env bash
# Build + push the binoptimization toolkit image for amd64 and arm64.
#
# Same multi-arch pattern as ../../deploy.sh (buildx + docker-container driver).
# On DGX Spark (aarch64) this cross-builds linux/amd64 via QEMU in the builder.
#
# Tags (same image, two names):
#   izetmolla/binoptimization:latest
#   izetmolla/containerws:binoptimization
#
# Usage:
#   ./build.sh
#   ./build.sh --platform linux/arm64
#   ./build.sh --no-push          # host arch only, load locally
#   ./build.sh --dry-run
#
# Env:
#   DOCKER_USER   Registry namespace (default: izetmolla)
#   IMAGE_NAME    Primary tag (default: izetmolla/binoptimization:latest)
#   ALIAS_NAME    Alias tag (default: izetmolla/containerws:binoptimization)
#   PLATFORMS     Same as --platform (default: linux/amd64,linux/arm64)
#   BUILDER       buildx builder name (default: containerws-binopt)
#   UPX_VERSION   Passed as build-arg (default: Dockerfile ARG)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKERFILE="${ROOT}/Dockerfile"
CONTEXT="${ROOT}"

DOCKER_USER="${DOCKER_USER:-izetmolla}"
IMAGE_NAME="${IMAGE_NAME:-${DOCKER_USER}/binoptimization:latest}"
ALIAS_NAME="${ALIAS_NAME:-${DOCKER_USER}/containerws:binoptimization}"

DEFAULT_PLATFORMS="linux/amd64,linux/arm64"
PLATFORMS="${PLATFORMS:-${DEFAULT_PLATFORMS}}"
BUILDER="${BUILDER:-containerws-binopt}"
UPX_VERSION="${UPX_VERSION:-}"

DRY_RUN=false
DO_PUSH=true

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Build the binoptimization toolkit Dockerfile for linux/amd64 + linux/arm64
(one multi-arch tag) and push to Docker Hub — same flow as deploy.sh.

  ${IMAGE_NAME}
  ${ALIAS_NAME}

Options:
  --dry-run              Print commands only
  --no-push              Build host arch only and load locally (no registry push)
  --platform <list>      Override platforms (default: ${DEFAULT_PLATFORMS})
  -h, --help             Show this help

Env:
  DOCKER_USER   Registry namespace (default: izetmolla)
  IMAGE_NAME    Primary image:tag
  ALIAS_NAME    Alias image:tag (omit / set empty to skip)
  PLATFORMS     Same as --platform
  BUILDER       buildx builder name (default: containerws-binopt)
  UPX_VERSION   Optional build-arg override

Examples:
  $(basename "$0")                         # push amd64+arm64
  $(basename "$0") --platform linux/arm64  # arm64 only (e.g. DGX Spark)
  $(basename "$0") --no-push               # local host-arch build
  $(basename "$0") --dry-run
EOF
}

log() { printf '[binopt] %s\n' "$*"; }
die() { printf '[binopt] error: %s\n' "$*" >&2; exit 1; }

host_platform() {
  local arch
  arch="$(uname -m)"
  case "${arch}" in
    x86_64) printf 'linux/amd64\n' ;;
    aarch64|arm64) printf 'linux/arm64\n' ;;
    armv7l) printf 'linux/arm/v7\n' ;;
    *) die "unsupported host arch: ${arch}" ;;
  esac
}

is_multi_platform() {
  [[ "${PLATFORMS}" == *","* ]]
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --no-push)
      DO_PUSH=false
      # Local --load cannot be multi-arch; default to host unless user set PLATFORMS
      if [[ "${PLATFORMS}" == "${DEFAULT_PLATFORMS}" ]]; then
        PLATFORMS="$(host_platform)"
      fi
      shift
      ;;
    --platform)
      [[ $# -ge 2 ]] || die "--platform requires a value"
      PLATFORMS="$2"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    -*)
      die "unknown option: $1"
      ;;
    *)
      die "unexpected argument: $1"
      ;;
  esac
done

# Multi-arch images must be pushed (cannot docker load a manifest list)
if is_multi_platform && [[ "${DO_PUSH}" != true ]]; then
  die "multi-platform builds require push (omit --no-push, or use --platform $(host_platform))"
fi

[[ -f "${DOCKERFILE}" ]] || die "Dockerfile not found: ${DOCKERFILE}"
[[ -f "${ROOT}/optimize" ]] || die "optimize script not found: ${ROOT}/optimize"

run() {
  if [[ "${DRY_RUN}" == true ]]; then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

require_buildx() {
  if ! command -v docker >/dev/null 2>&1; then
    die "docker not found in PATH"
  fi

  local buildx_out=""
  if ! buildx_out="$(docker buildx version 2>&1)"; then
    die "docker buildx is missing or broken (needed for multi-arch).

  Install the Buildx plugin, then re-run:
    # Debian/Ubuntu (Docker CE)
    apt-get update && apt-get install -y docker-buildx-plugin
    # or: https://docs.docker.com/build/install-buildx/

  Verify:
    docker version
    docker buildx version

  Last error:
    ${buildx_out}"
  fi
  log "buildx: ${buildx_out}"
}

ensure_builder() {
  require_buildx

  if docker buildx inspect "${BUILDER}" >/dev/null 2>&1; then
    run docker buildx use "${BUILDER}"
    run docker buildx inspect --bootstrap >/dev/null
    return 0
  fi

  if [[ "${BUILDER}" == "default" ]]; then
    run docker buildx use default
    run docker buildx inspect --bootstrap >/dev/null
    return 0
  fi

  log "creating buildx builder: ${BUILDER}"
  if [[ "${DRY_RUN}" == true ]]; then
    run docker buildx create --name="${BUILDER}" --driver=docker-container --bootstrap --use
    return 0
  fi

  local create_err=""
  if ! create_err="$(
    docker buildx create \
      --name="${BUILDER}" \
      --driver=docker-container \
      --bootstrap \
      --use 2>&1
  )"; then
    if [[ "${create_err}" == *"unknown flag: --name"* ]] || [[ "${create_err}" == *"unknown flag: --driver"* ]]; then
      die "docker buildx create failed — Buildx plugin is not usable on this host.

  Symptom: ${create_err}

  Fix:
    apt-get install -y docker-buildx-plugin
    # or: curl -fsSL https://get.docker.com | sh
  Then: docker buildx version"
    fi
    die "docker buildx create failed: ${create_err}"
  fi
  run docker buildx inspect --bootstrap >/dev/null
}

# On aarch64 hosts (DGX Spark), ensure binfmt/QEMU can run amd64 build steps.
ensure_qemu() {
  if ! is_multi_platform; then
    return 0
  fi
  local host
  host="$(host_platform)"
  if [[ "${host}" == "linux/arm64" && "${PLATFORMS}" == *"linux/amd64"* ]]; then
    log "host is arm64 — registering QEMU binfmt for cross-building amd64"
    # Idempotent; safe if already installed. Needs privileged once.
    if [[ "${DRY_RUN}" == true ]]; then
      run docker run --privileged --rm tonistiigi/binfmt --install all
      return 0
    fi
    if ! docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null 2>&1; then
      log "warning: could not install binfmt (amd64 cross-build may fail)"
      log "  try: docker run --privileged --rm tonistiigi/binfmt --install all"
    fi
  fi
}

main() {
  local out tag_args=() build_args=()

  log "dockerfile:  ${DOCKERFILE}"
  log "context:     ${CONTEXT}"
  log "image:       ${IMAGE_NAME}"
  if [[ -n "${ALIAS_NAME}" ]]; then
    log "alias:       ${ALIAS_NAME}"
  fi
  log "platforms:   ${PLATFORMS}"
  log "host:        $(host_platform) ($(uname -m))"
  log "push:        ${DO_PUSH}"
  log "builder:     ${BUILDER}"

  ensure_builder
  ensure_qemu

  if [[ "${DO_PUSH}" == true ]]; then
    out=--push
  else
    out=--load
  fi

  tag_args+=(-t "${IMAGE_NAME}")
  if [[ -n "${ALIAS_NAME}" ]]; then
    tag_args+=(-t "${ALIAS_NAME}")
  fi

  if [[ -n "${UPX_VERSION}" ]]; then
    build_args+=(--build-arg "UPX_VERSION=${UPX_VERSION}")
  fi

  # optimize must be executable in the build context
  chmod +x "${ROOT}/optimize" 2>/dev/null || true

  log "building…"
  run docker buildx build \
    --platform "${PLATFORMS}" \
    -f "${DOCKERFILE}" \
    "${tag_args[@]}" \
    "${build_args[@]}" \
    "${out}" \
    "${CONTEXT}"

  if [[ "${DO_PUSH}" == true ]]; then
    log "pushed ${IMAGE_NAME} (${PLATFORMS})"
    if [[ -n "${ALIAS_NAME}" ]]; then
      log "pushed ${ALIAS_NAME} (${PLATFORMS})"
    fi
    log "verify: docker buildx imagetools inspect ${IMAGE_NAME}"
  else
    log "loaded ${IMAGE_NAME} locally (${PLATFORMS})"
  fi
  log "done"
}

main
