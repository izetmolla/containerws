#!/usr/bin/env bash
# setup-homebrew-tap.sh — create izetmolla/homebrew-tap and print secret setup steps.
#
# Usage:
#   ./scripts/setup-homebrew-tap.sh
#   ./scripts/setup-homebrew-tap.sh --set-secret   # also set HOMEBREW_TAP_TOKEN from gh auth token
#
# Requires: gh (authenticated as a user that can create izetmolla/homebrew-tap)
set -euo pipefail

OWNER="${HOMEBREW_TAP_OWNER:-izetmolla}"
TAP_REPO="${HOMEBREW_TAP_REPO:-homebrew-tap}"
FULL="${OWNER}/${TAP_REPO}"
APP_REPO="${APP_REPO:-izetmolla/containerws}"
SET_SECRET=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --set-secret) SET_SECRET=1; shift ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' is required but not installed" >&2
    exit 1
  fi
}

need_cmd gh
need_cmd git

if ! gh auth status >/dev/null 2>&1; then
  echo "error: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

if gh repo view "$FULL" >/dev/null 2>&1; then
  echo "✓ Tap already exists: https://github.com/${FULL}"
else
  echo "==> Creating https://github.com/${FULL}"
  gh repo create "$FULL" \
    --public \
    --description "Homebrew tap for Container Workspace (containerws)" \
    --add-readme
  echo "✓ Created ${FULL}"
fi

# Ensure default branch exists with a README (empty repos break some brew taps).
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

gh repo clone "$FULL" "$TMP/tap" -- --depth 1
cd "$TMP/tap"
mkdir -p Casks
if [[ ! -f README.md ]]; then
  cat > README.md <<'EOF'
# homebrew-tap

Homebrew tap for [Container Workspace](https://github.com/izetmolla/containerws).

```bash
brew install izetmolla/tap/containerws
```

Casks are updated automatically by GoReleaser on each release.
EOF
fi
if [[ ! -f Casks/.gitkeep ]]; then
  touch Casks/.gitkeep
fi
if [[ -n "$(git status --porcelain)" ]]; then
  git add README.md Casks/.gitkeep
  git -c user.email="goreleaser@containerws.local" -c user.name="containerws" \
    commit -m "Initialize Homebrew tap for containerws" || true
  git push origin HEAD
fi

echo
echo "==> Next: add HOMEBREW_TAP_TOKEN on ${APP_REPO}"
echo
echo "1. Create a PAT with contents:write on ${FULL}"
echo "   https://github.com/settings/tokens (classic: repo) or fine-grained on ${FULL}"
echo
echo "2. Set the Actions secret:"
echo "   gh secret set HOMEBREW_TAP_TOKEN --repo ${APP_REPO}"
echo

if [[ "$SET_SECRET" -eq 1 ]]; then
  TOKEN="$(gh auth token)"
  if [[ -z "$TOKEN" ]]; then
    echo "error: could not read gh auth token" >&2
    exit 1
  fi
  echo "==> Setting HOMEBREW_TAP_TOKEN on ${APP_REPO} from current gh session"
  printf '%s' "$TOKEN" | gh secret set HOMEBREW_TAP_TOKEN --repo "$APP_REPO"
  echo "✓ Secret set. Publish a release to update Casks/containerws.rb"
fi

echo
echo "Install after the next release:"
echo "  brew install ${OWNER}/tap/containerws"
