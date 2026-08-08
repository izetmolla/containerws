# syntax=docker/dockerfile:1.7
#
# Multi-stage build for Debian 12 (Bookworm):
#   Stage 1 — frontend (pnpm / Vite → frontend/static)
#   Stage 2 — backend  (Go cross-compile only)
#   Stage 3 — optimize (import izetmolla/binoptimization:latest → strip + UPX)
#   Stage 4 — runtime  (debian:12 + systemd + openssh + containerws unit)
#
# Runtime PID 1 is systemd when a private cgroup2 is available. On Windows/WSL
# Docker Desktop, do NOT use cgroup:host or bind-mount host /sys/fs/cgroup
# (ef53 breaks systemd). See docker-compose-windows.yaml + entrypoint.sh
# (falls back to direct cws+sshd when cgroups are unusable).
#
# Requires the toolkit image (built from tools/binoptimization/Dockerfile):
#   izetmolla/binoptimization:latest
#   (alias: izetmolla/containerws:binoptimization)
#
# Multi-arch (same tag for amd64 + arm64), from repo root:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -f docker/debian/12/Dockerfile \
#     -t izetmolla/containerws:debian-12 --push .
#
# Or: ./deploy.sh debian 12

ARG BINOPT_IMAGE=izetmolla/binoptimization:latest

# ---------------------------------------------------------------------------
# Stage 1: frontend (arch-independent assets — build on host platform)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:22-bookworm AS frontend

WORKDIR /src

RUN corepack enable

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
ENV NODE_ENV=production
RUN pnpm build

# ---------------------------------------------------------------------------
# Stage 2: Go cross-compile on BUILDPLATFORM → TARGETARCH binary (no UPX/strip)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS gobuild

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT_SHA=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY cmd/ ./cmd/
COPY config/ ./config/
COPY models/ ./models/
COPY modules/ ./modules/
COPY packages/ ./packages/
COPY version/ ./version/
COPY frontend/assets.go ./frontend/

COPY --from=frontend /src/static ./frontend/static

# Pure-Go sqlite (glebarez) — static cross-compile, no CGO / QEMU needed.
ENV CGO_ENABLED=0
RUN GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath \
      -ldflags="-s -w -X github.com/izetmolla/containerws/version.Version=${VERSION} -X github.com/izetmolla/containerws/version.CommitSHA=${COMMIT_SHA}" \
      -o /out/cws . && \
    ls -lh /out/cws

# ---------------------------------------------------------------------------
# Stage 3: strip + UPX via imported binoptimization toolkit (BUILDPLATFORM)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${BINOPT_IMAGE} AS optimize

COPY --from=gobuild /out/cws /out/cws
RUN optimize /out/cws

# ---------------------------------------------------------------------------
# Stage 4: Debian 12 runtime (per TARGETPLATFORM)
# ---------------------------------------------------------------------------
FROM debian:12

ARG AUTHOR="Izet Molla <izetmolla@gmail.com>"
ARG TARGETARCH

LABEL org.opencontainers.image.title="containerws" \
      org.opencontainers.image.description="Container Workspace (Debian 12)" \
      org.opencontainers.image.source="https://github.com/izetmolla/containerws" \
      org.opencontainers.image.authors="${AUTHOR}"

ENV ENV=production \
    SYNC_USERS=true \
    DEBIAN_FRONTEND=noninteractive \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt \
    container=docker

COPY --from=optimize /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=optimize /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=optimize /out/cws /usr/local/bin/cws
COPY docker/debian/12/services/containerws/containerws.service /etc/systemd/system/containerws.service
COPY docker/debian/12/services/ssh/setup-user.service /etc/systemd/system/setup-user.service
COPY docker/debian/12/services/ssh/setup_user.sh /usr/local/lib/containerws/setup_user.sh
COPY docker/debian/12/entrypoint.sh /entrypoint.sh

# Install systemd, OpenSSH server, and enable containerws + ssh daemons.
# Entrypoint falls back to `cws --start` if apt/systemd is unavailable.
#
# Runtime env (optional):
#   ROOT_PWD              — root password (random if empty)
#   ROOT_SSH_PUBLIC_KEY   — appended to /root/.ssh/authorized_keys when set
RUN set -eux; \
    sed -i 's/\r$//' /entrypoint.sh /usr/local/lib/containerws/setup_user.sh; \
    chmod +x /usr/local/bin/cws /entrypoint.sh /usr/local/lib/containerws/setup_user.sh; \
    ln -sf cws /usr/local/bin/containerws; \
    mkdir -p /config/containerws/database /config/containerws/ssl /var/run/sshd /var/lib/containerws /etc/containerws; \
    printf '#!/bin/sh\nexit 0\n' > /usr/sbin/policy-rc.d; \
    chmod +x /usr/sbin/policy-rc.d; \
    if apt-get update; then \
      apt-get install --no-install-recommends -y \
        ca-certificates \
        curl \
        dbus \
        git \
        htop \
        iproute2 \
        iputils-ping \
        nano \
        net-tools \
        openssh-server \
        sudo \
        systemd \
        systemd-sysv \
        tzdata \
        util-linux \
        zip \
        unzip \
        wget; \
      apt-get upgrade -y; \
      apt-get autoremove -y; \
      apt-get clean; \
      rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* /tmp/* /var/tmp/*; \
      # SSH daemon: listen on all interfaces, keep root login usable for workspace.
      sed -i \
        -e 's/^#\?PermitRootLogin.*/PermitRootLogin yes/' \
        -e 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' \
        -e 's/^#\?PubkeyAuthentication.*/PubkeyAuthentication yes/' \
        /etc/ssh/sshd_config; \
      mkdir -p /etc/ssh/sshd_config.d; \
      printf '%s\n' \
        'Port 22' \
        'ListenAddress 0.0.0.0' \
        'UsePAM yes' \
        > /etc/ssh/sshd_config.d/containerws.conf; \
      ssh-keygen -A; \
      systemctl enable containerws.service; \
      systemctl enable ssh.service; \
      systemctl enable setup-user.service; \
      # Units that fail or are unsupported inside containers/WSL (no host
      # kernel module loading, no unattended apt upgrades). Mask so boot
      # stays clean in production and does not report [FAILED].
      systemctl mask \
        dev-hugepages.mount \
        sys-fs-fuse-connections.mount \
        systemd-remount-fs.service \
        systemd-modules-load.service \
        systemd-binfmt.service \
        proc-sys-fs-binfmt_misc.automount \
        proc-sys-fs-binfmt_misc.mount \
        getty@.service \
        console-getty.service \
        systemd-logind.service \
        apt-daily.service \
        apt-daily.timer \
        apt-daily-upgrade.service \
        apt-daily-upgrade.timer || true; \
      # Nothing for modules-load to do; also prevents late kmod probes.
      rm -f /etc/modules-load.d/* /usr/lib/modules-load.d/* /lib/modules-load.d/* 2>/dev/null || true; \
    else \
      echo "WARN: apt unavailable for ${TARGETARCH:-unknown}; systemd/ssh not installed" >&2; \
    fi; \
    rm -f /usr/sbin/policy-rc.d

EXPOSE 22 9000
# Only persist /config. Do NOT VOLUME /sys/fs/cgroup — an anonymous volume
# hides Docker's private cgroup mount and breaks systemd on Desktop/WSL.
VOLUME ["/config"]

# systemd expects SIGRTMIN+3 for clean shutdown.
STOPSIGNAL SIGRTMIN+3

WORKDIR /
# Final CRLF scrub + verify (Hub builds sometimes still ship CR from Windows checkouts).
# Invoke via /bin/sh so a bad shebang never becomes "exec /entrypoint.sh: no such file".
RUN set -eux; \
    if [ ! -f /entrypoint.sh ]; then echo "ERROR: /entrypoint.sh missing" >&2; exit 1; fi; \
    sed -i 's/\r$//' /entrypoint.sh /usr/local/lib/containerws/setup_user.sh; \
    printf '%s\n' '#!/bin/sh' > /tmp/entrypoint.shebang; \
    tail -n +2 /entrypoint.sh >> /tmp/entrypoint.shebang; \
    mv /tmp/entrypoint.shebang /entrypoint.sh; \
    chmod 0755 /entrypoint.sh /usr/local/lib/containerws/setup_user.sh; \
    # Fail the build if any CR remains (classic cause of the runtime error).
    ! grep -q $'\r' /entrypoint.sh; \
    /bin/sh -n /entrypoint.sh

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
