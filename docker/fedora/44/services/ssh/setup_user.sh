#!/bin/sh
# Configure root SSH credentials from ROOT_PWD / ROOT_SSH_PUBLIC_KEY and
# print connection details to the console (docker logs).
#
# Values may come from the process environment or from files written by
# entrypoint (preferred for keys that contain spaces):
#   /etc/containerws/root_pwd
#   /etc/containerws/root_ssh_public_key
set -eu

STATE_DIR=/config/containerws
mkdir -p "${STATE_DIR}" /root/.ssh
chmod 700 /root/.ssh

if [ -z "${ROOT_PWD:-}" ] && [ -f /etc/containerws/root_pwd ]; then
  ROOT_PWD="$(cat /etc/containerws/root_pwd)"
fi
if [ -z "${ROOT_SSH_PUBLIC_KEY:-}" ] && [ -f /etc/containerws/root_ssh_public_key ]; then
  ROOT_SSH_PUBLIC_KEY="$(cat /etc/containerws/root_ssh_public_key)"
fi

generated=0
if [ -n "${ROOT_PWD:-}" ]; then
  pass="${ROOT_PWD}"
elif [ -f "${STATE_DIR}/root.pass" ]; then
  pass="$(cat "${STATE_DIR}/root.pass")"
else
  # Random password when ROOT_PWD is unset/empty (persisted for restarts).
  pass="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24)"
  generated=1
fi

echo "root:${pass}" | chpasswd
echo "${pass}" > "${STATE_DIR}/root.pass"
chmod 600 "${STATE_DIR}/root.pass"

if [ -n "${ROOT_SSH_PUBLIC_KEY:-}" ]; then
  auth=/root/.ssh/authorized_keys
  touch "${auth}"
  chmod 600 "${auth}"
  # Avoid duplicate entries on restart.
  if ! grep -qxF "${ROOT_SSH_PUBLIC_KEY}" "${auth}" 2>/dev/null; then
    printf '%s\n' "${ROOT_SSH_PUBLIC_KEY}" >> "${auth}"
  fi
fi

# Wait briefly for network addresses (macvlan / DHCP).
ips=""
i=0
while [ "${i}" -lt 15 ]; do
  ips="$(hostname -I 2>/dev/null | tr -s ' ' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [ -n "${ips}" ]; then
    break
  fi
  i=$((i + 1))
  sleep 1
done

# Prefer Port from sshd config drop-ins / main config; default 22.
port="$(
  {
    cat /etc/ssh/sshd_config 2>/dev/null || true
    cat /etc/ssh/sshd_config.d/*.conf 2>/dev/null || true
  } | awk 'tolower($1)=="port" { print $2; exit }'
)"
port="${port:-22}"

# Build IP line: "IP: x" or "IPS: a, b, c"
if [ -n "${ips}" ]; then
  # shellcheck disable=SC2086
  set -- ${ips}
  if [ "$#" -eq 1 ]; then
    ip_line="IP: $1"
  else
    joined="$1"
    shift
    for addr in "$@"; do
      joined="${joined}, ${addr}"
    done
    ip_line="IPS: ${joined}"
  fi
else
  ip_line="IP: (none)"
fi

pass_note=""
if [ "${generated}" -eq 1 ]; then
  pass_note=" (generated)"
fi

banner() {
  echo ""
  echo "========================================"
  echo " SSH access ready"
  echo "========================================"
  echo "${ip_line}"
  echo "Port: ${port}"
  echo "username: root"
  echo "pass: ${pass}${pass_note}"
  if [ -n "${ROOT_SSH_PUBLIC_KEY:-}" ]; then
    echo "pubkey: configured"
  fi
  echo "========================================"
  echo ""
}

banner
