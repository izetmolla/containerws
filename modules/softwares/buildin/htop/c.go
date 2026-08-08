package htop

import "strings"

const (
	Key    = "htop"
	Binary = "htop"
	Name   = "htop"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing htop (interactive process viewer)"
apt-get update -y
apt-get install -y --no-install-recommends htop

echo "==> Verifying"
command -v htop
htop --version || true
echo "==> htop installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing htop"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge htop || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y htop || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm htop || true
fi
echo "==> htop removed"
`) + "\n"
}


func Details() string {
	return "Interactive process viewer for live CPU and memory per process. Useful from the shell when investigating load on this VM."
}

func Category() (category, subCategory string) {
	return "Monitoring", "Processes"
}

func Tags() []string {
	return []string{"htop", "cpu", "memory", "processes", "monitoring"}
}

func Icon() string  { return "Activity" }
func Color() string { return "#22C55E" }
func Order() int    { return 50 }
