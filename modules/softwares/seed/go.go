package seed

import "strings"

const goInstallScript = `#!/bin/bash
# Installing GO Lang
set -euo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
mkdir -p "$HOME"

if [ -z "${1:-}" ]; then
    GO_VERSION="1.26.5"
else
    GO_VERSION=$1
fi

echo "Installing GO Lang - $GO_VERSION"

apt update && apt install wget curl tar -y
cd /tmp
wget -q --show-progress https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz
rm -f go$GO_VERSION.linux-amd64.tar.gz

export GOROOT=/usr/local/go
export GOPATH="${GOPATH:-$HOME/go}"
export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
mkdir -p "$GOPATH" "$GOCACHE" "$GOMODCACHE" "$GOPATH/bin"
export PATH="$GOPATH/bin:$GOROOT/bin:$PATH"

# Persist env for login shells
grep -q 'GOROOT=/usr/local/go' /etc/profile || {
  echo 'export GOROOT=/usr/local/go' >> /etc/profile
  echo 'export GOPATH=$HOME/go' >> /etc/profile
  echo 'export GOCACHE=$HOME/.cache/go-build' >> /etc/profile
  echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> /etc/profile
}
grep -q 'GOROOT=/usr/local/go' "$HOME/.bashrc" || {
  echo 'export GOROOT=/usr/local/go' >> "$HOME/.bashrc"
  echo 'export GOPATH=$HOME/go' >> "$HOME/.bashrc"
  echo 'export GOCACHE=$HOME/.cache/go-build' >> "$HOME/.bashrc"
  echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> "$HOME/.bashrc"
}

echo "Go toolchain: $(go version)"
echo "Installing Air..."
go install github.com/air-verse/air@latest
echo "Air installed: $(command -v air)"
`

const goUninstallScript = `#!/bin/bash
# Full remove of Go toolchain + Air helpers installed by the catalog script.
set -euo pipefail

export HOME="${HOME:-/root}"
export GOPATH="${GOPATH:-$HOME/go}"

echo "==> Stopping nothing (Go has no service)"
echo "==> Removing /usr/local/go"
rm -rf /usr/local/go

echo "==> Removing Air binary if present"
rm -f "$GOPATH/bin/air" /usr/local/bin/air 2>/dev/null || true

echo "==> Cleaning profile exports (best effort)"
for f in /etc/profile "$HOME/.bashrc"; do
  [ -f "$f" ] || continue
  tmp="$(mktemp)"
  grep -vE 'GOROOT=/usr/local/go|GOPATH=\$HOME/go|GOCACHE=\$HOME/\.cache/go-build|GOPATH/bin:\$GOROOT/bin' "$f" > "$tmp" || true
  mv "$tmp" "$f"
done

echo "==> Go fully removed"
`

func goCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:        "Go",
			Details:     "Install the Go toolchain (default 1.26.5), configure GOROOT/GOPATH, and install Air for live reloads.",
			Category:    "Development",
			SubCategory: "Runtimes",
			Tags:        []string{"go", "golang", "air", "runtime"},
			Icon:        "Box",
			Color:       "#00ADD8",
			Order:       1,
			IsActive:    true,
		},
		Versions: []VersionMeta{
			{
				Version:         "1.26.5",
				IsLatest:        true,
				InstallScript:   strings.TrimSpace(goInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(goUninstallScript) + "\n",
			},
		},
	}
}
