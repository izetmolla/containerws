package nvtop

import "strings"

const (
	Key    = "nvtop"
	Binary = "nvtop"
	Name   = "nvtop"
)

const installScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Installing nvtop (GPU process monitor)"
apt-get update -y
apt-get install -y --no-install-recommends nvtop

echo "==> Verifying"
command -v nvtop
nvtop --version 2>/dev/null || nvtop -h 2>&1 | head -n 3 || true
echo "==> nvtop installed"
`

func InstallScript() string {
	return strings.TrimSpace(installScript) + "\n"
}

func UninstallScript() string {
	return strings.TrimSpace(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Fully removing nvtop"
if command -v apt-get >/dev/null 2>&1; then
  apt-get remove -y --purge nvtop || true
  apt-get autoremove -y || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y nvtop || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -Rns --noconfirm nvtop || true
fi
echo "==> nvtop removed"
`) + "\n"
}


func Details() string {
	return "Universal GPU process monitor (htop for graphics). Works across NVIDIA, AMD, Intel, and other DRM GPUs — utilization, VRAM, and per-process use."
}

func Category() (category, subCategory string) {
	return "Monitoring", "GPU"
}

func Tags() []string {
	return []string{"nvtop", "gpu", "vram", "nvidia", "amd", "intel", "monitoring"}
}

func Icon() string  { return "CircuitBoard" }
func Color() string { return "#22C55E" }
func Order() int    { return 60 }
