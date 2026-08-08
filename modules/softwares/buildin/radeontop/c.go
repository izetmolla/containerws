package radeontop

import "strings"

const (
	Key    = "radeontop"
	Binary = "radeontop"
	Name   = "radeontop"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing radeontop (AMD Radeon GPU utilization)"
apt-get update -y
apt-get install -y --no-install-recommends radeontop

echo "==> Verifying"
command -v radeontop
radeontop -h 2>&1 | head -n 5 || true
echo "==> radeontop installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing radeontop"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge radeontop || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y radeontop || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm radeontop || true
fi
echo "==> radeontop removed"
`) + "\n"
}


func Details() string {
	return "Live AMD Radeon GPU utilization monitor. Shows graphics, memory, and shader activity for amdgpu/radeon devices from a shell."
}

func Category() (category, subCategory string) {
	return "Monitoring", "GPU"
}

func Tags() []string {
	return []string{"radeontop", "gpu", "amd", "radeon", "amdgpu", "monitoring"}
}

func Icon() string  { return "Activity" }
func Color() string { return "#EF4444" }
func Order() int    { return 62 }
