package sysstat

import "strings"

const (
	Key    = "sysstat"
	Binary = "mpstat"
	Name   = "sysstat"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing sysstat (mpstat, iostat, sar)"
apt-get update -y
apt-get install -y --no-install-recommends sysstat

if [ -f /etc/default/sysstat ]; then
  sed -i 's/^ENABLED=.*/ENABLED="true"/' /etc/default/sysstat || true
fi
systemctl enable --now sysstat 2>/dev/null || true

echo "==> Verifying"
command -v mpstat
command -v iostat
mpstat 1 1 || true
echo "==> sysstat installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing sysstat"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge sysstat || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y sysstat || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm sysstat || true
fi
echo "==> sysstat removed"
`) + "\n"
}


func Details() string {
	return "System activity tools: mpstat (CPU), iostat (disk), and sar (history). Complements the live dashboard with CLI deep-dives."
}

func Category() (category, subCategory string) {
	return "Monitoring", "System"
}

func Tags() []string {
	return []string{"sysstat", "mpstat", "iostat", "sar", "cpu", "disk", "monitoring"}
}

func Icon() string  { return "BarChart3" }
func Color() string { return "#0EA5E9" }
func Order() int    { return 51 }
