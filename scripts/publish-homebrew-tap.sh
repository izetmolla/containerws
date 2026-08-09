#!/usr/bin/env bash
# publish-homebrew-tap.sh — push the GoReleaser-generated cask to izetmolla/homebrew-tap
# over SSH (git@github.com), using your GitHub account SSH key / agent.
#
# Usage:
#   ./scripts/publish-homebrew-tap.sh v0.1.3
#   ./scripts/publish-homebrew-tap.sh --tag v0.1.3
#   ./scripts/publish-homebrew-tap.sh --tag v0.1.3 --cask dist/homebrew/Casks/containerws.rb
#
# Env:
#   HOMEBREW_TAP_SSH_URL   default git@github.com:izetmolla/homebrew-tap.git
#   HOMEBREW_TAP_SSH_KEY   optional path to private key (else ssh-agent / default keys)
#   HOMEBREW_TAP_BRANCH    default main
#   GIT_AUTHOR_NAME / GIT_AUTHOR_EMAIL  optional commit identity override
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TAG=""
CASK_SRC=""
DRY_RUN=0

TAP_SSH_URL="${HOMEBREW_TAP_SSH_URL:-git@github.com:izetmolla/homebrew-tap.git}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-main}"
CASK_REL="Casks/containerws.rb"

usage() {
  sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      shift
      TAG="${1:-}"
      [[ -n "$TAG" ]] || { echo "error: --tag requires a value" >&2; exit 1; }
      shift
      ;;
    --cask)
      shift
      CASK_SRC="${1:-}"
      [[ -n "$CASK_SRC" ]] || { echo "error: --cask requires a path" >&2; exit 1; }
      shift
      ;;
    --dry-run|-n) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    v[0-9]*)
      TAG="$1"
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TAG" ]]; then
  TAG="$(git describe --tags --abbrev=0 --match='v*' 2>/dev/null || true)"
fi
if [[ -z "$TAG" ]]; then
  echo "error: pass a release tag (e.g. v0.1.3)" >&2
  exit 1
fi
[[ "$TAG" == v* ]] || TAG="v${TAG}"

if [[ -z "$CASK_SRC" ]]; then
  CASK_SRC="${ROOT}/dist/homebrew/Casks/containerws.rb"
fi
if [[ ! -f "$CASK_SRC" ]]; then
  echo "error: cask not found: ${CASK_SRC}" >&2
  echo "  Run goreleaser first (it writes dist/homebrew/Casks/containerws.rb)." >&2
  exit 1
fi

resolve_ssh_key() {
  if [[ -n "${HOMEBREW_TAP_SSH_KEY:-}" ]]; then
    printf '%s' "$HOMEBREW_TAP_SSH_KEY"
    return 0
  fi
  local k
  for k in \
    "${HOME}/.ssh/id_ed25519" \
    "${HOME}/.ssh/id_ecdsa" \
    "${HOME}/.ssh/id_rsa" \
    /root/.ssh/id_ed25519 \
    /root/.ssh/id_ecdsa \
    /root/.ssh/id_rsa
  do
    if [[ -f "$k" ]]; then
      printf '%s' "$k"
      return 0
    fi
  done
  return 1
}

setup_ssh() {
  local key=""
  if key="$(resolve_ssh_key)"; then
    export HOMEBREW_TAP_SSH_KEY="$key"
    export GIT_SSH_COMMAND="ssh -i ${key} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -F /dev/null"
    echo "→ SSH key: ${key}"
  else
    # Fall back to ssh-agent / default ssh config (still public-key auth).
    export GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=accept-new"
    echo "→ SSH: using agent / default keys (set HOMEBREW_TAP_SSH_KEY to pin a key)"
  fi
}

echo "==> Publishing Homebrew cask to ${TAP_SSH_URL} (${TAG})"
echo "    source: ${CASK_SRC}"
setup_ssh

# Quick probe (non-fatal if github.com is briefly unreachable — clone will fail clearly).
if ! git ls-remote --exit-code "$TAP_SSH_URL" "refs/heads/${TAP_BRANCH}" >/dev/null 2>&1; then
  echo "error: cannot reach ${TAP_SSH_URL} over SSH" >&2
  echo "  Ensure your SSH public key is added to GitHub and can push izetmolla/homebrew-tap." >&2
  echo "  Test: ssh -T git@github.com" >&2
  exit 1
fi

TMP="$(mktemp -d /tmp/cws-homebrew-tap.XXXXXX)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

git clone --depth 1 --branch "$TAP_BRANCH" "$TAP_SSH_URL" "$TMP/tap"
mkdir -p "$TMP/tap/Casks"
cp -f "$CASK_SRC" "$TMP/tap/${CASK_REL}"

# Drop legacy Formula if still present from old GoReleaser `brews` publishes.
if [[ -f "$TMP/tap/Formula/containerws.rb" ]]; then
  rm -f "$TMP/tap/Formula/containerws.rb"
  rmdir "$TMP/tap/Formula" 2>/dev/null || true
fi

cd "$TMP/tap"
git add -A
if git diff --cached --quiet; then
  echo "✓ Homebrew tap already up to date for ${TAG}"
  exit 0
fi

AUTHOR_NAME="${GIT_AUTHOR_NAME:-${GIT_COMMITTER_NAME:-containerws-release}}"
AUTHOR_EMAIL="${GIT_AUTHOR_EMAIL:-${GIT_COMMITTER_EMAIL:-noreply@github.com}}"
export GIT_AUTHOR_NAME="$AUTHOR_NAME"
export GIT_AUTHOR_EMAIL="$AUTHOR_EMAIL"
export GIT_COMMITTER_NAME="$AUTHOR_NAME"
export GIT_COMMITTER_EMAIL="$AUTHOR_EMAIL"

MSG="Brew cask update for containerws ${TAG}"
git commit -m "$MSG"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "(dry-run) would push:"
  git log -1 --oneline
  git show --stat HEAD
  exit 0
fi

git push origin "HEAD:${TAP_BRANCH}"
echo "✓ Updated https://github.com/izetmolla/homebrew-tap (${CASK_REL}) for ${TAG}"
echo "  Install: brew install --cask izetmolla/tap/containerws"
echo "  Then:    sudo containerws setup"
