#!/usr/bin/env bash
# release-local.sh — build release binaries with GoReleaser on this machine and
# publish them to GitHub Releases (same artifacts as Actions, usually faster).
#
# Usage:
#   ./scripts/release-local.sh patch              # bump patch, build, publish
#   ./scripts/release-local.sh minor|major
#   ./scripts/release-local.sh v0.2.0             # exact new version
#   ./scripts/release-local.sh --existing-tag v0.1.0  # publish an existing local tag
#   ./scripts/release-local.sh patch --snapshot  # cross-build only (no GitHub Release)
#   ./scripts/release-local.sh patch --yes
#   ./scripts/release-local.sh patch --skip-preflight
#   ./scripts/release-local.sh patch --no-push   # publish release but do not push git tag
#   ./scripts/release-local.sh patch --skip-brew # skip Homebrew tap SSH publish
#
# Requires: go, pnpm, goreleaser ≥2, git, ssh access to GitHub
# Auth (GitHub Release): GITHUB_TOKEN / GH_TOKEN, or `gh auth login` (repo scope)
# Homebrew tap (https://github.com/izetmolla/homebrew-tap):
#   After binaries publish, this script pushes dist/homebrew/Casks/containerws.rb
#   over SSH (git@github.com) using your GitHub SSH key — no HOMEBREW_TAP_TOKEN needed.
#   Optional: HOMEBREW_TAP_SSH_KEY=/path/to/id_ed25519  (else ~/.ssh/id_ed25519|id_rsa)
#
# Tip: prefer this script for full releases. There is no Actions Release workflow
# on v* tags — only CI runs on branch pushes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION_ARG=""
EXISTING_TAG=""
YES=0
SKIP_PREFLIGHT=0
PUSH=1
SNAPSHOT=0
SKIP_BREW=0

usage() {
  sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    major|minor|patch|v[0-9]*.[0-9]*.[0-9]*)
      VERSION_ARG="$1"
      shift
      ;;
    --existing-tag)
      shift
      EXISTING_TAG="${1:-}"
      if [[ -z "$EXISTING_TAG" ]]; then
        echo "error: --existing-tag requires a tag (e.g. v0.1.0)" >&2
        exit 1
      fi
      shift
      ;;
    --yes|-y) YES=1; shift ;;
    --skip-preflight|--skip-build) SKIP_PREFLIGHT=1; shift ;;
    --no-push) PUSH=0; shift ;;
    --snapshot) SNAPSHOT=1; shift ;;
    --skip-brew) SKIP_BREW=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$VERSION_ARG" && -z "$EXISTING_TAG" ]]; then
  echo "error: pass major|minor|patch, vX.Y.Z, or --existing-tag vX.Y.Z" >&2
  exit 1
fi
if [[ -n "$VERSION_ARG" && -n "$EXISTING_TAG" ]]; then
  echo "error: use either a version bump/exact tag or --existing-tag, not both" >&2
  exit 1
fi

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' is required but not installed" >&2
    exit 1
  fi
}

need_cmd go
need_cmd pnpm
need_cmd goreleaser
need_cmd git

resolve_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    echo "$GITHUB_TOKEN"
    return
  fi
  if [[ -n "${GH_TOKEN:-}" ]]; then
    echo "$GH_TOKEN"
    return
  fi
  if command -v gh >/dev/null 2>&1; then
    if token="$(gh auth token 2>/dev/null)" && [[ -n "$token" ]]; then
      echo "$token"
      return
    fi
  fi
  return 1
}

build_frontend() {
  echo "==> Building frontend"
  (
    cd "$ROOT/frontend"
    pnpm install --frozen-lockfile
    pnpm run build
  )
  test -f "$ROOT/frontend/static/index.html"
}

TAG=""

if [[ -n "$EXISTING_TAG" ]]; then
  if [[ ! "$EXISTING_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: tag must match vX.Y.Z (got: $EXISTING_TAG)" >&2
    exit 1
  fi
  if ! git rev-parse "$EXISTING_TAG" >/dev/null 2>&1; then
    echo "error: local tag ${EXISTING_TAG} not found" >&2
    exit 1
  fi
  TAG="$EXISTING_TAG"
  HEAD_SHA="$(git rev-parse HEAD)"
  TAG_SHA="$(git rev-parse "$TAG^{}")"
  if [[ "$HEAD_SHA" != "$TAG_SHA" ]]; then
    echo "error: HEAD (${HEAD_SHA:0:7}) does not match ${TAG} (${TAG_SHA:0:7})" >&2
    echo "       check out that commit/tag before publishing" >&2
    exit 1
  fi
  if [[ "$SKIP_PREFLIGHT" -eq 0 ]]; then
    echo "==> Preflight (frontend + go test)"
    build_frontend
    go test ./...
    goreleaser check
  else
    build_frontend
  fi
else
  RELEASE_ARGS=("$VERSION_ARG" --no-push)
  if [[ "$YES" -eq 1 ]]; then
    RELEASE_ARGS+=(--yes)
  fi
  if [[ "$SKIP_PREFLIGHT" -eq 1 ]]; then
    RELEASE_ARGS+=(--skip-build)
    build_frontend
  fi
  chmod +x "$ROOT/scripts/release.sh" "$ROOT/scripts/bump-version.sh"
  echo "==> Creating release commit/tag via release.sh (no push yet)"
  "$ROOT/scripts/release.sh" "${RELEASE_ARGS[@]}"
  TAG="$(git describe --tags --abbrev=0 --match='v*')"
fi

echo
echo "Release tag: ${TAG}"
echo

if [[ "$SNAPSHOT" -eq 1 ]]; then
  echo "==> GoReleaser snapshot (no GitHub publish)"
  goreleaser release --snapshot --clean --skip=publish
  echo
  echo "Artifacts under dist/"
  exit 0
fi

if ! TOKEN="$(resolve_token)"; then
  echo "error: no GitHub token for publishing" >&2
  echo "  export GITHUB_TOKEN=ghp_...   # repo scope" >&2
  echo "  # or: gh auth login" >&2
  exit 1
fi

echo "==> GoReleaser publish → GitHub Release ${TAG}"
export GITHUB_TOKEN="$TOKEN"
export GH_TOKEN="$TOKEN"
# Local releases publish the Homebrew tap over SSH after GoReleaser (see below).
# Keep GoReleaser's token upload off so a missing/invalid PAT does not fail the release.
export HOMEBREW_TAP_TOKEN=""
goreleaser release --clean

echo
echo "✓ Published https://github.com/izetmolla/containerws/releases/tag/${TAG}"

if [[ "$SKIP_BREW" -eq 1 ]]; then
  echo "==> Skipping Homebrew tap publish (--skip-brew)"
else
  chmod +x "$ROOT/scripts/publish-homebrew-tap.sh"
  echo "==> Homebrew tap publish over SSH → https://github.com/izetmolla/homebrew-tap"
  "$ROOT/scripts/publish-homebrew-tap.sh" --tag "$TAG"
fi

if [[ "$PUSH" -eq 1 ]]; then
  BRANCH="$(git rev-parse --abbrev-ref HEAD)"
  if [[ "$BRANCH" == "HEAD" ]]; then
    echo "Detached HEAD — pushing tag only"
    git push origin "refs/tags/${TAG}"
  else
    echo "==> Pushing ${BRANCH} and tag ${TAG}"
    git push origin "$BRANCH"
    git push origin "refs/tags/${TAG}"
  fi
else
  echo "Skipped git push (--no-push). Tag/release exist; push when ready:"
  echo "  git push origin HEAD && git push origin ${TAG}"
fi
