package seed

import "strings"

const nodeInstallScript = `#!/bin/bash
# Install NVM + Node LTS + pnpm.
# NOTE: do not enable "set -u" while sourcing nvm.sh — nvm uses unbound vars.
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
mkdir -p "$HOME"

if [ -z "${1:-}" ]; then
    NVM_VERSION="v0.40.5"
else
    NVM_VERSION=$1
fi

echo "Installing NVM - $NVM_VERSION"

apt-get update
apt-get install --no-install-recommends -y curl ca-certificates bash

# Fresh install into $NVM_DIR when missing or broken.
need_nvm_install=1
if [ -s "$NVM_DIR/nvm.sh" ]; then
  set +u
  # shellcheck disable=SC1091
  . "$NVM_DIR/nvm.sh"
  if command -v nvm >/dev/null 2>&1; then
    need_nvm_install=0
    echo "NVM already present at $NVM_DIR"
  fi
  set -u
fi

if [ "$need_nvm_install" -eq 1 ]; then
  echo "Downloading NVM $NVM_VERSION..."
  rm -rf "$NVM_DIR"
  curl -fsSL "https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION}/install.sh" -o /tmp/nvm-install.sh
  # Installer must not inherit nounset from the parent shell.
  bash /tmp/nvm-install.sh
  rm -f /tmp/nvm-install.sh
fi

# Load nvm without nounset.
set +u
# shellcheck disable=SC1091
. "$NVM_DIR/nvm.sh"
if ! command -v nvm >/dev/null 2>&1; then
  echo "ERROR: nvm failed to load from $NVM_DIR/nvm.sh" >&2
  exit 1
fi

echo "Installing Node.js LTS..."
nvm install --lts
nvm alias default 'lts/*'
nvm use default
set -u

NODE_BIN="$(command -v node)"
NPM_BIN="$(command -v npm)"
echo "Node: $($NODE_BIN -v) ($NODE_BIN)"
echo "npm:  $($NPM_BIN -v) ($NPM_BIN)"

echo "Installing pnpm..."
"$NPM_BIN" install -g pnpm@latest
echo "pnpm: $(command -v pnpm) ($(pnpm -v))"

# Persist nvm for login / interactive shells.
PROFILE_SNIPPET='
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
[ -s "$NVM_DIR/bash_completion" ] && . "$NVM_DIR/bash_completion"
'
for profile in "$HOME/.bashrc" "$HOME/.profile" /etc/profile.d/nvm.sh; do
  dir="$(dirname "$profile")"
  mkdir -p "$dir"
  if [ ! -f "$profile" ] || ! grep -q 'NVM_DIR=.*\.nvm' "$profile" 2>/dev/null; then
    printf '%s\n' "$PROFILE_SNIPPET" >> "$profile"
  fi
done

echo "NVM + Node LTS install complete."
`

const nodeUninstallScript = `#!/bin/bash
set -euo pipefail
export HOME="${HOME:-/root}"
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"

echo "==> Removing NVM, Node, and pnpm"
rm -rf "$NVM_DIR"
rm -f "$HOME/.npmrc" 2>/dev/null || true
rm -rf "$HOME/.local/share/pnpm" "$HOME/.local/state/pnpm" 2>/dev/null || true
command -v npm >/dev/null 2>&1 && npm uninstall -g pnpm 2>/dev/null || true
for f in "$HOME/.bashrc" "$HOME/.profile" /etc/profile; do
  [ -f "$f" ] || continue
  tmp="$(mktemp)"
  grep -vE 'NVM_DIR|nvm\.sh|pnpm' "$f" > "$tmp" || true
  mv "$tmp" "$f"
done
echo "==> Node.js / NVM fully removed"
`

func nodeCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "Node.js",
			Details:     "Install NVM (v0.40.5), the Node.js LTS release, and the latest pnpm package manager.",
			Category:    "Development",
			SubCategory: "Runtimes",
			Tags:        []string{"node", "nvm", "javascript", "pnpm"},
			Icon:        "Hexagon",
			Color:       "#339933",
			Order:       2,
			IsActive:    true,
		},
		Versions: []VersionMeta{
			{
				Version:       "nvm-v0.40.5-lts",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(nodeInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(nodeUninstallScript) + "\n",
			},
		},
	}
}
