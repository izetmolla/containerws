package intelgputools

import "strings"

const (
	Key    = "intel-gpu-tools"
	Binary = "intel_gpu_top"
	Name   = "intel-gpu-tools"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing intel-gpu-tools (intel_gpu_top)"
apt-get update -y
apt-get install -y --no-install-recommends intel-gpu-tools

echo "==> Verifying"
command -v intel_gpu_top
intel_gpu_top -h 2>&1 | head -n 5 || true
echo "==> intel-gpu-tools installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing intel-gpu-tools"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge intel-gpu-tools || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y intel-gpu-tools || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm intel-gpu-tools || true
fi
echo "==> intel-gpu-tools removed"
`) + "\n"
}


func Details() string {
	return "Intel GPU utilities including intel_gpu_top for live engine utilization, frequency, and power on i915/Xe graphics. Best paired with an Intel GPU."
}

func Category() (category, subCategory string) {
	return "Monitoring", "GPU"
}

func Tags() []string {
	return []string{"intel-gpu-tools", "intel_gpu_top", "gpu", "intel", "i915", "monitoring"}
}

func Icon() string  { return "Cpu" }
func Color() string { return "#3B82F6" }
func Order() int    { return 61 }
