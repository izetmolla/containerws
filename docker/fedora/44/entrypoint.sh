#!/bin/sh
set -e

# CONTAINERWS_INIT:
#   auto   (default) — prepare private cgroup2 and run systemd if usable
#   systemd          — require systemd (fail if cgroup unusable)
#   direct|nosystemd — skip systemd; run setup-user + sshd + cws as PID 1 children
#
# Do NOT bind-mount host /sys/fs/cgroup or use cgroup:host on WSL/Docker Desktop
# (host fstype "ef53" makes systemd exit: Failed to check cgroup hierarchy).
# Prefer cgroup: private + tmpfs /run (see docker-compose.yaml / windows compose).
INIT_MODE="${CONTAINERWS_INIT:-auto}"

resolve_systemd_bin() {
  if [ -x /lib/systemd/systemd ]; then
    echo /lib/systemd/systemd
  elif [ -x /usr/lib/systemd/systemd ]; then
    echo /usr/lib/systemd/systemd
  fi
}

cgroup_fstype() {
  findmnt -n -o FSTYPE /sys/fs/cgroup 2>/dev/null || true
}

# True when /sys/fs/cgroup looks like a usable cgroup2 root for systemd.
cgroup_usable() {
  ft="$(cgroup_fstype)"
  case "$ft" in
    cgroup2|cgroup2fs) ;;
    *) return 1 ;;
  esac
  # WSL host shares often report garbage types (ef53) or are not writable.
  if ! touch /sys/fs/cgroup/.containerws_cgroup_probe 2>/dev/null; then
    return 1
  fi
  rm -f /sys/fs/cgroup/.containerws_cgroup_probe
  return 0
}

# Inside a private cgroup namespace (no host bind), ensure a real cgroup2 mount.
prepare_private_cgroup2() {
  mkdir -p /sys/fs/cgroup
  ft="$(cgroup_fstype)"
  case "$ft" in
    cgroup2|cgroup2fs)
      if cgroup_usable; then
        return 0
      fi
      ;;
  esac

  # Drop a bad/host mount (e.g. WSL ef53) when privileged, then mount our own.
  umount -l /sys/fs/cgroup 2>/dev/null || true
  if mount -t cgroup2 -o rw,nosuid,nodev,noexec,relatime,nsdelegate cgroup2 /sys/fs/cgroup 2>/dev/null \
    || mount -t cgroup2 -o rw,nosuid,nodev,noexec,relatime cgroup2 /sys/fs/cgroup 2>/dev/null; then
    return 0
  fi
  return 1
}

write_credential_files() {
  mkdir -p /etc/containerws
  umask 077
  printf '%s' "${ROOT_PWD-}" > /etc/containerws/root_pwd
  printf '%s' "${ROOT_SSH_PUBLIC_KEY-}" > /etc/containerws/root_ssh_public_key
  chmod 600 /etc/containerws/root_pwd /etc/containerws/root_ssh_public_key
  umask 022
}

# Run without systemd — keeps cws + ssh working when WSL/Docker cgroups break PID 1.
start_direct() {
  echo "containerws: starting in direct mode (no systemd / no host cgroup)" >&2

  if command -v ssh-keygen >/dev/null 2>&1; then
    ssh-keygen -A
  fi
  write_credential_files

  if [ -x /usr/local/lib/containerws/setup_user.sh ]; then
    /usr/local/lib/containerws/setup_user.sh || true
  fi

  mkdir -p /var/run/sshd /run/sshd
  if command -v sshd >/dev/null 2>&1; then
    # Debian/Ubuntu: sshd; Fedora also ships /usr/sbin/sshd
    /usr/sbin/sshd || true
  fi

  exec /usr/local/bin/cws --start
}

systemd_bin="$(resolve_systemd_bin)"

if [ -z "${systemd_bin}" ] || [ ! -f /etc/systemd/system/containerws.service ]; then
  start_direct
fi

case "$INIT_MODE" in
  direct|nosystemd|no-systemd)
    start_direct
    ;;
esac

# Ensure SSH host keys exist before systemd starts ssh/sshd.
if command -v ssh-keygen >/dev/null 2>&1; then
  ssh-keygen -A
fi

write_credential_files

# Journal → /dev/console (compose must set tty: true so console is the docker pty).
mkdir -p /etc/systemd/journald.conf.d /etc/systemd/system.conf.d
printf '%s\n' \
  '[Journal]' \
  'ForwardToConsole=yes' \
  'MaxLevelConsole=info' \
  'TTYPath=/dev/console' \
  'Storage=volatile' \
  > /etc/systemd/journald.conf.d/forward-console.conf
printf '%s\n' \
  '[Manager]' \
  'LogTarget=console' \
  'ShowStatus=yes' \
  > /etc/systemd/system.conf.d/console.conf
chmod 644 \
  /etc/systemd/journald.conf.d/forward-console.conf \
  /etc/systemd/system.conf.d/console.conf

if ! prepare_private_cgroup2 || ! cgroup_usable; then
  echo "containerws: cgroup2 not usable (fstype=$(cgroup_fstype)) — falling back to direct mode" >&2
  if [ "$INIT_MODE" = "systemd" ]; then
    echo "containerws: CONTAINERWS_INIT=systemd requires a working private cgroup2" >&2
    exit 1
  fi
  start_direct
fi

# Drop CAP_SYS_MODULE before PID 1. Privileged containers still get this
# capability, which makes systemd call kmod_setup() and print:
#   "Failed to initialize kmod context: Operation not supported"
# on WSL/container kernels where module loading is unavailable.
systemd_args="--log-target=console --show-status=true"
if command -v setpriv >/dev/null 2>&1; then
  exec setpriv --bounding-set=-sys_module --inh-caps=-sys_module --ambient-caps=-sys_module \
    "$systemd_bin" $systemd_args
fi
if command -v capsh >/dev/null 2>&1; then
  exec capsh --drop=cap_sys_module -- -c "exec \"$systemd_bin\" $systemd_args"
fi

exec "$systemd_bin" $systemd_args
