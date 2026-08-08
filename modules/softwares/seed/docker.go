package seed

import "strings"

// Docker Engine install adapted from tmp/Docker.test (static binaries + compose + buildx).
// Tuned for privileged systemd containers (WSL / nested cgroup): iptables backend
// probe, Type=simple unit, and storage/network fallbacks when dockerd fails to start.
const dockerInstallScript = `#!/bin/bash
set -eo pipefail

export HOME="${HOME:-/root}"
export USER="${USER:-root}"
export DEBIAN_FRONTEND=noninteractive
export PATH="/usr/local/bin:$PATH"

DOCKER_CHANNEL="${DOCKER_CHANNEL:-stable}"
DOCKER_VERSION="${DOCKER_VERSION:-29.6.2}"
DOCKER_COMPOSE_VERSION="${DOCKER_COMPOSE_VERSION:-v5.3.1}"
BUILDX_VERSION="${BUILDX_VERSION:-v0.35.0}"

if [ -n "${1:-}" ]; then
  DOCKER_VERSION="$1"
fi

echo "==> Installing Docker Engine ${DOCKER_VERSION} (channel=${DOCKER_CHANNEL})"
echo "==> Compose ${DOCKER_COMPOSE_VERSION}, buildx ${BUILDX_VERSION}"

echo "==> Installing prerequisites"
apt-get update
apt-get install --no-install-recommends -y \
  ca-certificates \
  curl \
  iproute2 \
  iptables \
  nftables \
  python3 \
  wget \
  xz-utils
# Optional helpers when available (ignore failures on minimal images).
apt-get install --no-install-recommends -y fuse-overlayfs kmod 2>/dev/null || true

arch="$(uname -m)"
case "$arch" in
  x86_64)  dockerArch='x86_64'  ; buildx_arch='linux-amd64' ;;
  armhf)   dockerArch='armel'   ; buildx_arch='linux-arm-v6' ;;
  armv7l)  dockerArch='armhf'   ; buildx_arch='linux-arm-v7' ;;
  aarch64) dockerArch='aarch64' ; buildx_arch='linux-arm64' ;;
  *)
    echo "ERROR: unsupported architecture ($arch)" >&2
    exit 1
    ;;
esac

workdir="$(mktemp -d /tmp/docker-install.XXXXXX)"
trap 'rm -rf "$workdir"' EXIT
cd "$workdir"

echo "==> Downloading docker-${DOCKER_VERSION} (${dockerArch})"
wget -q --show-progress -O docker.tgz \
  "https://download.docker.com/linux/static/${DOCKER_CHANNEL}/${dockerArch}/docker-${DOCKER_VERSION}.tgz"

echo "==> Installing docker binaries to /usr/local/bin"
tar --extract --file docker.tgz --strip-components 1 --directory /usr/local/bin/
rm -f docker.tgz
chmod +x /usr/local/bin/docker /usr/local/bin/dockerd /usr/local/bin/containerd \
  /usr/local/bin/containerd-shim-runc-v2 /usr/local/bin/ctr /usr/local/bin/runc \
  /usr/local/bin/docker-init /usr/local/bin/docker-proxy 2>/dev/null || true

echo "==> Installing docker buildx ${BUILDX_VERSION}"
wget -q --show-progress -O docker-buildx \
  "https://github.com/docker/buildx/releases/download/${BUILDX_VERSION}/buildx-${BUILDX_VERSION}.${buildx_arch}"
mkdir -p /usr/local/lib/docker/cli-plugins
chmod +x docker-buildx
mv docker-buildx /usr/local/lib/docker/cli-plugins/docker-buildx

echo "==> Installing docker compose ${DOCKER_COMPOSE_VERSION}"
compose_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
wget -q --show-progress -O /usr/local/bin/docker-compose \
  "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-${compose_os}-${arch}"
chmod +x /usr/local/bin/docker-compose
mkdir -p /usr/local/lib/docker/cli-plugins
ln -sfn /usr/local/bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose

# Prefer whichever iptables backend actually works on this kernel (DinD / WSL).
configure_iptables() {
  echo "==> Configuring iptables backend"
  if command -v update-alternatives >/dev/null 2>&1; then
    if command -v iptables-legacy >/dev/null 2>&1 && iptables-legacy -nL >/dev/null 2>&1; then
      update-alternatives --set iptables /usr/sbin/iptables-legacy 2>/dev/null || true
      update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy 2>/dev/null || true
      echo "    using iptables-legacy"
    elif command -v iptables-nft >/dev/null 2>&1 && iptables-nft -nL >/dev/null 2>&1; then
      update-alternatives --set iptables /usr/sbin/iptables-nft 2>/dev/null || true
      update-alternatives --set ip6tables /usr/sbin/ip6tables-nft 2>/dev/null || true
      echo "    using iptables-nft"
    else
      echo "    WARN: could not probe iptables backends; leaving system default"
    fi
  fi
  # Best-effort module loads (often blocked in containers — ignore failures).
  if command -v modprobe >/dev/null 2>&1; then
    for m in overlay br_netfilter ip_tables iptable_nat iptable_filter nf_nat nf_conntrack; do
      modprobe "$m" 2>/dev/null || true
    done
  fi
}

write_daemon_json() {
  local storage_driver="$1"
  local iptables_enabled="$2"
  local iptables_py=False
  case "$iptables_enabled" in
    true|True|1|yes|on) iptables_py=True ;;
  esac
  mkdir -p /etc/docker /var/lib/docker /var/run /run
  python3 - <<PY
import json, os
path = "/etc/docker/daemon.json"
try:
    with open(path) as f:
        cfg = json.load(f)
except Exception:
    cfg = {}
cfg.pop("hosts", None)
cfg["log-driver"] = "json-file"
cfg["log-opts"] = {"max-size": "10m", "max-file": "3"}
cfg["storage-driver"] = "$storage_driver"
cfg["iptables"] = $iptables_py
cfg["ip-forward"] = True
cfg["bridge"] = "docker0"
os.makedirs("/etc/docker", exist_ok=True)
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
print("    wrote", path, "storage-driver=$storage_driver", "iptables=$iptables_py")
PY
}

write_systemd_unit() {
  # Type=simple is more reliable than Type=notify inside nested systemd / WSL.
  cat > /etc/systemd/system/docker.service <<'EOF'
[Unit]
Description=Docker Application Container Engine
Documentation=https://docs.docker.com
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/dockerd --host=unix:///var/run/docker.sock
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutStartSec=0
Restart=on-failure
RestartSec=2
StartLimitBurst=0
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
Delegate=yes
KillMode=process
OOMScoreAdjust=-500

[Install]
WantedBy=multi-user.target
EOF
}

dump_docker_failure() {
  echo "==> docker failed — recent logs:" >&2
  if command -v journalctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    journalctl -u docker.service -n 80 --no-pager >&2 || true
    systemctl status docker.service --no-pager -l >&2 || true
  fi
  if [ -f /var/log/dockerd.log ]; then
    echo "==> /var/log/dockerd.log:" >&2
    tail -n 80 /var/log/dockerd.log >&2 || true
  fi
}

has_systemd() {
  command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]
}

docker_api_ready() {
  docker info >/dev/null 2>&1
}

prepare_cgroup_dirs() {
  mkdir -p /sys/fs/cgroup/docker 2>/dev/null || true
  mkdir -p /sys/fs/cgroup/docker/buildx 2>/dev/null || true
}

# Prefer overlay2 when the existing graph already uses it (avoid vfs/overlay2 mismatch).
preferred_storage() {
  if [ -d /var/lib/docker/overlay2 ] && [ -n "$(ls -A /var/lib/docker/overlay2 2>/dev/null || true)" ]; then
    echo "overlay2"
    return
  fi
  echo "overlay2"
}

# True if PID exists and is not a zombie (Z*). PID 1 often fails to reap in direct mode.
pid_alive() {
  local pid="$1"
  [ -n "$pid" ] && [ -d "/proc/$pid" ] || return 1
  local state
  state=$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null) || return 1
  case "$state" in
    Z*) return 1 ;;
  esac
  return 0
}

clear_stale_pidfile() {
  local f="$1"
  [ -f "$f" ] || return 0
  local pid
  pid=$(tr -d ' \n' <"$f" 2>/dev/null) || true
  if ! pid_alive "$pid"; then
    rm -f "$f"
  fi
}

live_dockerd() {
  # pgrep matches zombies; prefer /proc state.
  local p
  for p in $(pgrep -x dockerd 2>/dev/null); do
    if pid_alive "$p"; then
      return 0
    fi
  done
  return 1
}

clear_docker_runtime_pidfiles() {
  clear_stale_pidfile /var/run/docker.pid
  clear_stale_pidfile /run/docker.pid
  clear_stale_pidfile /var/run/docker-direct.pid
  clear_stale_pidfile /var/run/docker-fresh.pid
  clear_stale_pidfile /run/docker/containerd/containerd.pid
  clear_stale_pidfile /var/run/docker/containerd/containerd.pid
  # Stale sockets from a dead daemon confuse clients and restarts.
  if ! live_dockerd; then
    rm -f /var/run/docker.sock /run/docker.sock
    rm -f /run/docker/containerd/containerd.sock \
      /run/docker/containerd/containerd.sock.ttrpc \
      /run/docker/containerd/containerd-debug.sock \
      /var/run/docker/containerd/containerd.sock \
      /var/run/docker/containerd/containerd.sock.ttrpc \
      /var/run/docker/containerd/containerd-debug.sock
  fi
}

start_dockerd_direct() {
  mkdir -p /var/run /run /var/log /usr/local/lib/containerws
  prepare_cgroup_dirs
  if live_dockerd; then
    if docker_api_ready; then
      return 0
    fi
    echo "    dockerd running but API/socket unavailable — restarting"
    pkill -x dockerd 2>/dev/null || true
    pkill -x containerd 2>/dev/null || true
    sleep 1
  fi
  clear_docker_runtime_pidfiles
  # Keep socket on /var/run (same as /run when linked) so clients find it.
  # Use a dedicated pidfile so a zombie holding the classic path cannot block.
  nohup /usr/local/bin/dockerd \
    --host=unix:///var/run/docker.sock \
    --pidfile=/var/run/docker-fresh.pid \
    >>/var/log/dockerd.log 2>&1 &
  echo $! >/var/run/docker-direct.pid 2>/dev/null || true
}

start_dockerd() {
  prepare_cgroup_dirs
  if has_systemd; then
    systemctl daemon-reload || true
    systemctl reset-failed docker.service 2>/dev/null || true
    systemctl enable docker.service >/dev/null 2>&1 || true
    systemctl restart docker.service 2>/dev/null || systemctl start docker.service 2>/dev/null || return 1
  else
    start_dockerd_direct || return 1
  fi
  local i
  for i in $(seq 1 45); do
    if docker_api_ready; then
      return 0
    fi
    if has_systemd && systemctl is-failed docker.service >/dev/null 2>&1; then
      return 1
    fi
    # If the process vanished in direct mode, fail fast (ignore zombies).
    if ! has_systemd && ! live_dockerd && [ "$i" -gt 3 ]; then
      return 1
    fi
    sleep 1
  done
  docker_api_ready
}

stop_dockerd() {
  if has_systemd; then
    systemctl stop docker.service 2>/dev/null || true
    systemctl reset-failed docker.service 2>/dev/null || true
  fi
  pkill -x dockerd 2>/dev/null || true
  pkill -x containerd 2>/dev/null || true
  sleep 1
  clear_docker_runtime_pidfiles
  rm -f /var/run/docker.pid /run/docker.pid /var/run/docker-direct.pid /var/run/docker-fresh.pid
  rm -f /run/docker/containerd/containerd.pid /var/run/docker/containerd/containerd.pid
}

write_ensure_helper() {
  mkdir -p /usr/local/lib/containerws
  cat > /usr/local/lib/containerws/ensure-dockerd.sh <<'EOF'
#!/bin/bash
# Best-effort start for nested / direct-mode (no systemd) containers.
set -e
export PATH="/usr/local/bin:$PATH"
mkdir -p /var/run /run /var/log /sys/fs/cgroup/docker 2>/dev/null || true
if docker info >/dev/null 2>&1; then
  exit 0
fi

pid_alive() {
  local pid="$1"
  [ -n "$pid" ] && [ -d "/proc/$pid" ] || return 1
  local state
  state=$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null) || return 1
  case "$state" in
    Z*) return 1 ;;
  esac
  return 0
}

clear_stale_pidfile() {
  local f="$1"
  [ -f "$f" ] || return 0
  local pid
  pid=$(tr -d ' \n' <"$f" 2>/dev/null) || true
  if ! pid_alive "$pid"; then
    rm -f "$f"
  fi
}

live_dockerd() {
  local p
  for p in $(pgrep -x dockerd 2>/dev/null); do
    if pid_alive "$p"; then
      return 0
    fi
  done
  return 1
}

if live_dockerd; then
  pkill -x dockerd 2>/dev/null || true
  pkill -x containerd 2>/dev/null || true
  sleep 1
fi
clear_stale_pidfile /var/run/docker.pid
clear_stale_pidfile /run/docker.pid
clear_stale_pidfile /var/run/docker-direct.pid
clear_stale_pidfile /var/run/docker-fresh.pid
clear_stale_pidfile /run/docker/containerd/containerd.pid
clear_stale_pidfile /var/run/docker/containerd/containerd.pid
if ! live_dockerd; then
  rm -f /var/run/docker.sock /run/docker.sock
  rm -f /run/docker/containerd/containerd.sock \
    /run/docker/containerd/containerd.sock.ttrpc \
    /run/docker/containerd/containerd-debug.sock \
    /var/run/docker/containerd/containerd.sock \
    /var/run/docker/containerd/containerd.sock.ttrpc \
    /var/run/docker/containerd/containerd-debug.sock
fi
# Nested environments often break nftables; keep iptables off unless already working.
if [ -f /etc/docker/daemon.json ]; then
  python3 - <<'PY' 2>/dev/null || true
import json
p="/etc/docker/daemon.json"
try:
  cfg=json.load(open(p))
except Exception:
  cfg={}
cfg["iptables"]=False
cfg["ip-forward"]=True
if not cfg.get("storage-driver"):
  cfg["storage-driver"]="overlay2"
json.dump(cfg, open(p,"w"), indent=2)
open(p,"a").write("\n")
PY
fi
nohup /usr/local/bin/dockerd \
  --host=unix:///var/run/docker.sock \
  --pidfile=/var/run/docker-fresh.pid \
  >>/var/log/dockerd.log 2>&1 &
for i in $(seq 1 45); do
  if docker info >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done
echo "ensure-dockerd: docker API not ready" >&2
tail -n 40 /var/log/dockerd.log >&2 || true
exit 1
EOF
  chmod 755 /usr/local/lib/containerws/ensure-dockerd.sh
}

configure_iptables
write_systemd_unit
write_ensure_helper

echo "==> Installing systemd unit + ensure-dockerd helper"

# Attempt matrix. Direct mode (no systemd): try iptables=false first — nftables
# often fails in nested containers and leaves dockerd unusable after reboot.
storage_list="overlay2 fuse-overlayfs vfs"
# If overlay2 graph already exists, put it first (already is) and skip wiping.
if has_systemd; then
  ipt_list="true false"
else
  echo "==> Direct mode detected (no systemd) — preferring iptables=false"
  ipt_list="false true"
fi

started=0
for storage in $storage_list; do
  if [ "$storage" = "fuse-overlayfs" ] && ! command -v fuse-overlayfs >/dev/null 2>&1; then
    continue
  fi
  for ipt in $ipt_list; do
    echo "==> Trying storage-driver=${storage} iptables=${ipt}"
    stop_dockerd
    write_daemon_json "$storage" "$ipt"
    if start_dockerd; then
      started=1
      break 2
    fi
    dump_docker_failure || true
  done
done

if [ "$started" -ne 1 ]; then
  echo "ERROR: dockerd failed to start after all fallbacks" >&2
  dump_docker_failure || true
  exit 1
fi

# Warm the ensure helper once so boot hooks can call it safely.
/usr/local/lib/containerws/ensure-dockerd.sh >/dev/null 2>&1 || true

echo "==> Verifying installation"
dockerd --version
docker --version
docker buildx version || true
docker-compose version || docker compose version || true
docker info
echo "==> Docker Engine ${DOCKER_VERSION} is installed and running"
`

const dockerUninstallScript = `#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "==> Stopping Docker"
systemctl stop docker.service 2>/dev/null || true
systemctl disable docker.service 2>/dev/null || true

echo "==> Removing Docker binaries and systemd unit"
rm -f /usr/local/bin/docker /usr/local/bin/dockerd /usr/local/bin/docker-proxy       /usr/local/bin/docker-init /usr/local/bin/containerd /usr/local/bin/containerd-shim-runc-v2       /usr/local/bin/ctr /usr/local/bin/runc /usr/local/bin/docker-compose       /usr/libexec/docker/cli-plugins/docker-buildx 2>/dev/null || true
rm -f /etc/systemd/system/docker.service /etc/systemd/system/docker.socket 2>/dev/null || true
systemctl daemon-reload 2>/dev/null || true

echo "==> Optional: leave /var/lib/docker intact (data). Uncomment to wipe."
# rm -rf /var/lib/docker /var/lib/containerd

echo "==> Docker Engine fully removed"
`

func dockerCatalogItem() catalogItem {
	return catalogItem{
		Software: SoftwareMeta{
			Name:         "Docker Engine",
			Details:      "Install Docker Engine static binaries, Buildx, and Compose, then start the dockerd service.",
			Category:     "Infrastructure",
			SubCategory:  "Containers",
			Tags:         []string{"docker", "containers", "compose", "buildx"},
			ServiceUnits:   []string{"docker.service"},
			CanControl:     true,
			ControlBackend: "docker",
			StartCommand:   "systemctl start docker.service",
			RestartCommand: "systemctl restart docker.service",
			StopCommand:    "systemctl stop docker.service",
			Icon:         "Container",
			Color:        "#2496ED",
			Order:        3,
			IsActive:     true,
		},
		Versions: []VersionMeta{
			{
				Version:       "29.6.2",
				IsLatest:      true,
				InstallScript: strings.TrimSpace(dockerInstallScript) + "\n",
				UninstallScript: strings.TrimSpace(dockerUninstallScript) + "\n",
			},
		},
	}
}
