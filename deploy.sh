#!/usr/bin/env bash
# Discover Dockerfiles under docker/<os>/<version>/ and build + push to Docker Hub.
#
# Multi-arch by default: one tag for both amd64 and arm64 via buildx.
#   izetmolla/containerws:ubuntu-24.10  →  linux/amd64 + linux/arm64
#
# Image naming:
#   izetmolla/containerws:<os>-<version>         ← Dockerfile
#   izetmolla/containerws:<os>-<version>-<extra> ← Dockerfile.<extra>
#   izetmolla/containerws:latest                 ← also applied to ubuntu-26.04
#
# Toolkit images (tools/binoptimization) are NOT built here — publish them
# manually. OS Dockerfiles import BINOPT_IMAGE (default: izetmolla/binoptimization:latest).
#
# If docker/<os>/<version>/publish.sh exists, that script is used instead.
#
# Usage:
#   ./deploy.sh
#   ./deploy.sh ubuntu 24.10
#   ./deploy.sh --platform linux/arm64
#   ./deploy.sh --no-push          # host arch only, load locally
#   ./deploy.sh --multithread      # parallel builds (one worker per CPU)
#   ./deploy.sh --dry-run
#   ./deploy.sh --list

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="${ROOT}/docker"
DOCKER_USER="${DOCKER_USER:-izetmolla}"
IMAGE_REPO="${IMAGE_REPO:-${DOCKER_USER}/containerws}"
BINOPT_IMAGE="${BINOPT_IMAGE:-${DOCKER_USER}/binoptimization:latest}"

# Canonical :latest points at this OS/version (base Dockerfile only).
LATEST_OS="${LATEST_OS:-ubuntu}"
LATEST_VERSION="${LATEST_VERSION:-26.04}"

DEFAULT_PLATFORMS="linux/amd64,linux/arm64"
PLATFORMS="${PLATFORMS:-${DEFAULT_PLATFORMS}}"
BUILDER="${BUILDER:-containerws-deploy}"

DRY_RUN=false
DO_PUSH=true
LIST_ONLY=false
MULTITHREAD=false
# JOBS from env enables parallel mode when set to a positive integer.
JOBS="${JOBS:-0}"
FILTER_OS=""
FILTER_VERSION=""
EXTRA_ARGS=()

usage() {
  cat <<EOF
Usage: $(basename "$0") [options] [os] [version]

Scan ${DOCKER_DIR}/<os>/<version>/ for Dockerfiles, build a multi-arch image
(linux/amd64 + linux/arm64 by default) under one tag, and push to Docker Hub.

  ${IMAGE_REPO}:ubuntu-24.10  ←  docker/ubuntu/24.10/Dockerfile

Does not build tools/* (e.g. binoptimization) — publish those manually.
OS builds import: ${BINOPT_IMAGE}

Options:
  --dry-run              Print commands only
  --no-push              Build host arch only and load locally (no registry push)
  --list                 List discovered images and exit
  --platform <list>      Override platforms (default: ${DEFAULT_PLATFORMS})
  --multithread          Build version dirs in parallel (uses all CPUs)
  --multithreat          Alias for --multithread
  --jobs <N>             Max parallel builds (implies --multithread; default: nproc)
  -h, --help             Show this help

Arguments:
  os                     Limit to one OS folder (e.g. ubuntu)
  version                Limit to one version folder (e.g. 24.10)

Env:
  DOCKER_USER   Registry namespace (default: izetmolla)
  IMAGE_REPO    OS image repo (default: izetmolla/containerws)
  BINOPT_IMAGE  Toolkit tag imported by OS Dockerfiles
                (default: izetmolla/binoptimization:latest)
  PLATFORMS     Same as --platform
  BUILDER       buildx builder name (default: containerws-deploy)
  JOBS          Same as --jobs
  LATEST_OS     OS folder that also gets :latest (default: ubuntu)
  LATEST_VERSION  Version that also gets :latest (default: 26.04)

Examples:
  $(basename "$0")                              # all OS → push amd64+arm64
  $(basename "$0") ubuntu 24.10                 # one OS tag, both arches
  $(basename "$0") --platform linux/arm64       # arm64 only
  $(basename "$0") --no-push                    # local host-arch build
  $(basename "$0") --multithread                # parallel builds across all CPUs
  $(basename "$0") --jobs 4                     # at most 4 concurrent builds
  $(basename "$0") --dry-run
EOF
}

log() { printf '[deploy] %s\n' "$*"; }
die() { printf '[deploy] error: %s\n' "$*" >&2; exit 1; }

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

cpu_count() {
  local n
  n="$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null || echo 1)"
  [[ "${n}" =~ ^[1-9][0-9]*$ ]] || n=1
  printf '%s\n' "${n}"
}

is_multi_platform() {
  [[ "${PLATFORMS}" == *","* ]]
}

if [[ "${JOBS}" =~ ^[1-9][0-9]*$ ]]; then
  MULTITHREAD=true
elif [[ "${JOBS}" == "0" ]]; then
  :
else
  die "JOBS must be a positive integer (got: ${JOBS})"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; EXTRA_ARGS+=(--dry-run); shift ;;
    --no-push)
      DO_PUSH=false
      EXTRA_ARGS+=(--no-push)
      # Local --load cannot be multi-arch; default to host unless user set PLATFORMS
      if [[ "${PLATFORMS}" == "${DEFAULT_PLATFORMS}" ]]; then
        PLATFORMS="$(host_platform)"
      fi
      shift
      ;;
    --list) LIST_ONLY=true; shift ;;
    --platform)
      [[ $# -ge 2 ]] || die "--platform requires a value"
      PLATFORMS="$2"
      EXTRA_ARGS+=(--platform "$2")
      shift 2
      ;;
    --multithread|--multithreat)
      MULTITHREAD=true
      shift
      ;;
    --jobs)
      [[ $# -ge 2 ]] || die "--jobs requires a value"
      [[ "$2" =~ ^[1-9][0-9]*$ ]] || die "--jobs must be a positive integer"
      JOBS="$2"
      MULTITHREAD=true
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    -*)
      die "unknown option: $1"
      ;;
    *)
      if [[ -z "${FILTER_OS}" ]]; then
        FILTER_OS="$1"
      elif [[ -z "${FILTER_VERSION}" ]]; then
        FILTER_VERSION="$1"
      else
        die "unexpected argument: $1"
      fi
      shift
      ;;
  esac
done

if [[ "${MULTITHREAD}" == true && "${JOBS}" -le 0 ]]; then
  JOBS="$(cpu_count)"
fi

# Multi-arch images must be pushed (cannot docker load a manifest list)
if is_multi_platform && [[ "${DO_PUSH}" != true ]]; then
  die "multi-platform builds require push (omit --no-push, or use --platform $(host_platform))"
fi

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

# docker/<os>/<version>/Dockerfile → izetmolla/containerws:<os>-<version>
image_tag_for() {
  local dockerfile="$1"
  local dir os version base extra

  dir="$(dirname "${dockerfile}")"
  version="$(basename "${dir}")"
  os="$(basename "$(dirname "${dir}")")"
  base="$(basename "${dockerfile}")"

  # Ignore files are not Dockerfiles (e.g. Dockerfile.dockerignore).
  [[ "${base}" == *.dockerignore ]] && return 1

  if [[ "${base}" == "Dockerfile" ]]; then
    printf '%s:%s-%s\n' "${IMAGE_REPO}" "${os}" "${version}"
    return 0
  fi
  if [[ "${base}" == Dockerfile.* ]]; then
    extra="${base#Dockerfile.}"
    [[ "${extra}" == "app" ]] && return 1
    printf '%s:%s-%s-%s\n' "${IMAGE_REPO}" "${os}" "${version}" "${extra}"
    return 0
  fi
  return 1
}

# True when this tag is the canonical workspace image that should also be :latest.
is_latest_source_tag() {
  local tag="$1"
  [[ "${tag}" == "${IMAGE_REPO}:${LATEST_OS}-${LATEST_VERSION}" ]]
}

latest_tag() {
  printf '%s:latest\n' "${IMAGE_REPO}"
}

build_version() {
  git -C "${ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev
}

build_commit() {
  git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown
}

deploy_dockerfile() {
  local dockerfile="$1"
  local tag out version commit
  local -a tags=() tag_args=()

  tag="$(image_tag_for "${dockerfile}")" || return 0
  tags=("${tag}")
  if is_latest_source_tag "${tag}"; then
    tags+=("$(latest_tag)")
  fi

  if [[ "${LIST_ONLY}" == true ]]; then
    local t
    for t in "${tags[@]}"; do
      printf '%s\t%s\t(%s)\n' "${t}" "${dockerfile#"${ROOT}"/}" "${PLATFORMS}"
    done
    return 0
  fi

  version="$(build_version)"
  commit="$(build_commit)"

  log "building ${tag}"
  log "  dockerfile: ${dockerfile#"${ROOT}"/}"
  log "  platforms:  ${PLATFORMS}"
  log "  version:    ${version} (${commit})"
  log "  binopt:     ${BINOPT_IMAGE} (must already exist on Hub / local)"
  if is_latest_source_tag "${tag}"; then
    log "  also tag:   $(latest_tag)"
  fi

  if [[ "${DO_PUSH}" == true ]]; then
    out=--push
  else
    out=--load
  fi

  local t
  for t in "${tags[@]}"; do
    tag_args+=(-t "${t}")
  done

  run docker buildx build \
    --platform "${PLATFORMS}" \
    -f "${dockerfile}" \
    "${tag_args[@]}" \
    --build-arg "VERSION=${version}" \
    --build-arg "COMMIT_SHA=${commit}" \
    --build-arg "BINOPT_IMAGE=${BINOPT_IMAGE}" \
    "${out}" \
    "${ROOT}"

  if [[ "${DO_PUSH}" == true ]]; then
    log "pushed ${tag} (${PLATFORMS})"
    if is_latest_source_tag "${tag}"; then
      log "pushed $(latest_tag) ← ${LATEST_OS}-${LATEST_VERSION}"
    fi
    log "verify: docker buildx imagetools inspect ${tag}"
  else
    log "loaded ${tag} locally (${PLATFORMS})"
    if is_latest_source_tag "${tag}"; then
      log "loaded $(latest_tag) locally ← ${LATEST_OS}-${LATEST_VERSION}"
    fi
  fi
}

discover_version_dirs() {
  local dir os version f has_df

  [[ -d "${DOCKER_DIR}" ]] || die "docker directory not found: ${DOCKER_DIR}"

  while IFS= read -r -d '' dir; do
    version="$(basename "${dir}")"
    os="$(basename "$(dirname "${dir}")")"

    if [[ -n "${FILTER_OS}" && "${os}" != "${FILTER_OS}" ]]; then
      continue
    fi
    if [[ -n "${FILTER_VERSION}" && "${version}" != "${FILTER_VERSION}" ]]; then
      continue
    fi

    # Require at least one real Dockerfile (ignore Dockerfile.dockerignore).
    has_df=0
    for f in "${dir}/Dockerfile" "${dir}"/Dockerfile.*; do
      [[ -f "${f}" ]] || continue
      [[ "$(basename "${f}")" == *.dockerignore ]] && continue
      has_df=1
      break
    done
    [[ "${has_df}" -eq 1 ]] || continue

    printf '%s\0' "${dir}"
  done < <(find "${DOCKER_DIR}" -mindepth 2 -maxdepth 2 -type d -print0 | sort -z)
}

list_dockerfiles() {
  local dir="$1"
  local f
  # Emit real Dockerfiles only — skip Dockerfile.dockerignore and friends.
  while IFS= read -r -d '' f; do
    [[ "$(basename "${f}")" == *.dockerignore ]] && continue
    printf '%s\0' "${f}"
  done < <(find "${dir}" -maxdepth 1 -type f \( -name 'Dockerfile' -o -name 'Dockerfile.*' \) -print0 | sort -z)
}

deploy_version_dir() {
  local dir="$1"
  local script="${dir}/publish.sh"
  local dockerfile

  if [[ -f "${script}" ]]; then
    log "delegating → ${script#"${ROOT}"/}"
    chmod +x "${script}" 2>/dev/null || true
    DOCKER_USER="${DOCKER_USER}" IMAGE_REPO="${IMAGE_REPO}" PLATFORMS="${PLATFORMS}" \
      BUILDER="${BUILDER}" BINOPT_IMAGE="${BINOPT_IMAGE}" "${script}" "${EXTRA_ARGS[@]}"
    return 0
  fi

  while IFS= read -r -d '' dockerfile; do
    [[ "$(basename "${dockerfile}")" == "Dockerfile.app" ]] && continue
    [[ "$(basename "${dockerfile}")" == *.dockerignore ]] && continue
    deploy_dockerfile "${dockerfile}"
  done < <(list_dockerfiles "${dir}")
}

# Run up to $1 background workers over the remaining args (version dirs).
# Worker pool sized like Go goroutines + GOMAXPROCS — one slot per CPU by default.
deploy_parallel() {
  local max_jobs="$1"
  shift
  local dir fails=0 active=0

  [[ "${max_jobs}" -ge 1 ]] || max_jobs=1
  log "multithread: ${max_jobs} worker(s) (cpus=$(cpu_count))"

  for dir in "$@"; do
    while (( active >= max_jobs )); do
      if wait -n; then
        :
      else
        fails=$((fails + 1))
        log "failed: a parallel build exited non-zero"
      fi
      active=$((active - 1))
    done
    deploy_version_dir "${dir}" &
    active=$((active + 1))
  done

  while (( active > 0 )); do
    if wait -n; then
      :
    else
      fails=$((fails + 1))
      log "failed: a parallel build exited non-zero"
    fi
    active=$((active - 1))
  done

  if [[ "${fails}" -gt 0 ]]; then
    die "${fails} parallel build(s) failed"
  fi
}

main() {
  local dir count=0
  local -a dirs=()

  log "docker root: ${DOCKER_DIR}"
  log "image repo:  ${IMAGE_REPO}"
  log "binopt:      ${BINOPT_IMAGE} (manual — not built by deploy)"
  log "platforms:   ${PLATFORMS}"
  log "push:        ${DO_PUSH}"
  if [[ "${MULTITHREAD}" == true && "${LIST_ONLY}" != true ]]; then
    log "parallel:    ${JOBS} job(s)"
  fi
  if [[ -n "${FILTER_OS}" ]]; then
    log "filter:      ${FILTER_OS}${FILTER_VERSION:+/${FILTER_VERSION}}"
  fi

  if [[ "${LIST_ONLY}" != true ]]; then
    ensure_builder
  fi

  while IFS= read -r -d '' dir; do
    dirs+=("${dir}")
  done < <(discover_version_dirs)

  count="${#dirs[@]}"
  if [[ "${count}" -eq 0 ]]; then
    die "no version dirs with Dockerfile under ${DOCKER_DIR}${FILTER_OS:+ (filter: ${FILTER_OS}${FILTER_VERSION:+/${FILTER_VERSION}})}"
  fi

  if [[ "${LIST_ONLY}" == true ]]; then
    for dir in "${dirs[@]}"; do
      deploy_version_dir "${dir}"
    done
    log "listed ${count} version dir(s)"
  elif [[ "${MULTITHREAD}" == true ]]; then
    deploy_parallel "${JOBS}" "${dirs[@]}"
    log "done (${count} version dir(s), ${JOBS} parallel worker(s))"
  else
    for dir in "${dirs[@]}"; do
      deploy_version_dir "${dir}"
    done
    log "done (${count} version dir(s))"
  fi
}

main
