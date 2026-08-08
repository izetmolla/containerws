package gpustat

import "strings"

const (
	Key    = "gpustat"
	Binary = "gpustat"
	Name   = "gpustat"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing gpustat (NVIDIA GPU status)"
apt-get update -y
apt-get install -y --no-install-recommends gpustat

echo "==> Verifying"
command -v gpustat
gpustat --version 2>/dev/null || gpustat -h 2>&1 | head -n 5 || true
echo "==> gpustat installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing gpustat"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge gpustat || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y gpustat || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm gpustat || true
fi
echo "==> gpustat removed"
`) + "\n"
}


func Details() string {
	return "Pretty NVIDIA GPU status for the shell: utilization, memory, temperature, and power via nvidia-smi. Best on hosts with NVIDIA drivers."
}

func Category() (category, subCategory string) {
	return "Monitoring", "GPU"
}

func Tags() []string {
	return []string{"gpustat", "gpu", "nvidia", "cuda", "monitoring"}
}

func Icon() string  { return "Zap" }
func Color() string { return "#84CC16" }
func Order() int    { return 63 }
