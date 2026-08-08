package iftop

import "strings"

const (
	Key    = "iftop"
	Binary = "iftop"
	Name   = "iftop"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing iftop (live network bandwidth by host)"
apt-get update -y
apt-get install -y --no-install-recommends iftop

echo "==> Verifying"
command -v iftop
iftop -h 2>&1 | head -n 3 || true
echo "==> iftop installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing iftop"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge iftop || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y iftop || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm iftop || true
fi
echo "==> iftop removed"
`) + "\n"
}


func Details() string {
	return "Live network bandwidth monitor grouped by remote host. Use it from a shell when you need connection-level traffic detail beyond dashboard totals."
}

func Category() (category, subCategory string) {
	return "Monitoring", "Network"
}

func Tags() []string {
	return []string{"iftop", "network", "bandwidth", "traffic", "monitoring"}
}

func Icon() string  { return "Network" }
func Color() string { return "#F59E0B" }
func Order() int    { return 52 }
