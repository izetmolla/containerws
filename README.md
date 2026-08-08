# Container Workspace (containerws)

Container Workspace is a full Linux workspace in Docker: systemd (when available), SSH, a web UI + API (`cws` on port **9000**), Softwares catalog, Cloud Shell, MCP tools, nested Docker, and optional desktop apps (XFCE / noVNC, Chrome, VS Code, Cursor).

Images are published as multi-arch (**linux/amd64** + **linux/arm64**) under:

`izetmolla/containerws:<distro>-<version>`

---

## Docker images and tags

### Runtime images (what you run)

| Image | Tag | Base OS | What it runs |
|-------|-----|---------|--------------|
| `izetmolla/containerws` | `ubuntu-26.04` | Ubuntu 26.04 (Resolute) | **Default / recommended.** systemd + OpenSSH + `cws` (UI/API on `:9000`). Softwares catalog, nested Docker volume, desktop-capable. |
| `izetmolla/containerws` | `ubuntu-26.10` | Ubuntu 26.10 | Same stack on newer Ubuntu. |
| `izetmolla/containerws` | `ubuntu-25.10` | Ubuntu 25.10 (Questing) | Same stack on Ubuntu 25.10. |
| `izetmolla/containerws` | `ubuntu-25.04` | Ubuntu 25.04 (Plucky) | Same stack on Ubuntu 25.04. |
| `izetmolla/containerws` | `debian-13` | Debian 13 (Trixie) | Same app stack on Debian 13. |
| `izetmolla/containerws` | `debian-12` | Debian 12 (Bookworm) | Same app stack on Debian 12 (stable-oriented). |
| `izetmolla/containerws` | `kali-rolling` | Kali Linux Rolling | Same app stack on `kalilinux/kali-rolling` (security toolkit base). |
| `izetmolla/containerws` | `fedora-44` | Fedora 44 | Same app stack on Fedora 44 (`dnf` / RPM world). |
| `izetmolla/containerws` | `fedora-43` | Fedora 43 | Same app stack on Fedora 43. |

Every runtime image:

- Starts via `/entrypoint.sh` (`CONTAINERWS_INIT=auto|systemd|direct`)
- Exposes **22** (SSH) and **9000** (Container Workspace web UI / API)
- Persists app data under **`/config`** (SQLite, machine id, SSL, VNC pass, etc.)
- Ships the Softwares catalog install scripts (Go, Node, Docker Engine, XFCE+noVNC, Chrome, VS Code, …)

Pick a tag that matches the distro you want inside the workspace. Behavior of `cws` is the same across tags; package managers and base packages differ.

### Build toolkit (not a workspace)

| Image | Tag | Purpose |
|-------|-----|---------|
| `izetmolla/binoptimization` | `latest` | Build-only toolkit (strip + UPX) imported by OS Dockerfiles. **Do not run as a workspace.** |
| `izetmolla/containerws` | `binoptimization` | Alias of the toolkit above (same purpose). |

Dockerfile sources live under `docker/<os>/<version>/Dockerfile`.

---

## Requirements (Linux / WSL)

- Docker Engine **or** Docker Desktop (WSL2 backend)
- Docker Compose v2 (`docker compose`)
- Recommended: NVIDIA Container Toolkit if you want `gpus: all`
- Treat these containers as a **trusted personal workspace** (`privileged` + unconfined seccomp/apparmor are required for nested Docker and desktop apps)

### WSL / Docker Desktop notes (important)

Do **not** use `cgroup: host` or bind-mount host `/sys/fs/cgroup` into the container on WSL — that breaks systemd (`ef53` / cgroup hierarchy issues).

This repo’s compose files use:

- `cgroup: private`
- **no** `/sys/fs/cgroup` host bind
- `cap_drop: [SYS_MODULE]`
- `CONTAINERWS_INIT=auto` (falls back to direct `cws`+`sshd` if systemd cannot start)

If systemd still fails on your WSL build:

```yaml
environment:
  CONTAINERWS_INIT: direct
```

---

## Quick start — Docker Compose (Linux / WSL)

### 1. Clone and prepare config volume

```bash
git clone https://github.com/izetmolla/containerws.git
cd containerws
mkdir -p tmp/config
```

### 2. Use or adapt compose

**Dev build from this repo** (default `docker-compose.yaml`):

```bash
# Optional: create an external network if your file expects one
docker network create --subnet=10.10.26.0/24 VLAN26 2>/dev/null || true

# Edit docker-compose.yaml:
#   - set ROOT_PWD
#   - adjust networks / IP, or switch to published ports (see below)
#   - keep ./tmp/config:/config

docker compose -f docker-compose.yaml up -d --build
```

**Pull a published image** (no local build) — example service:

```yaml
services:
  containerws:
    image: izetmolla/containerws:ubuntu-26.04
    container_name: containerws
    hostname: containerws
    restart: unless-stopped
    tty: true
    stop_signal: SIGRTMIN+3
    privileged: true
    cgroup: private
    gpus: all          # remove if you have no GPU toolkit
    shm_size: "2gb"
    security_opt:
      - apparmor:unconfined
      - seccomp:unconfined
    cap_drop:
      - SYS_MODULE
    tmpfs:
      - /run:exec,mode=755
      - /run/lock:mode=755
      - /tmp:exec,mode=1777
    environment:
      ENV: production
      container: docker
      CONTAINERWS_INIT: auto
      ROOT_PWD: "CHANGE_ME"
      ROOT_SSH_PUBLIC_KEY: ""
      NVIDIA_VISIBLE_DEVICES: all
      NVIDIA_DRIVER_CAPABILITIES: all
    volumes:
      - ./tmp/config:/config
      - containerws-docker-data:/var/lib/docker
    ports:
      - "9000:9000"
      - "2222:22"
volumes:
  containerws-docker-data:
```

Save as `docker-compose.local.yaml` and run:

```bash
docker compose -f docker-compose.local.yaml up -d
```

### 3. Open the workspace

| Service | URL / command |
|---------|----------------|
| Web UI / API | http://localhost:9000 |
| SSH | `ssh -p 2222 root@localhost` (if you published `2222:22`) |
| Shell into container | `docker exec -it containerws bash` |

Data survives recreate as long as `./tmp/config` (and the nested Docker volume) are kept.

### Compose files in this repo

| File | Audience | Notes |
|------|----------|--------|
| `docker-compose.yaml` | Linux / WSL **dev** | Builds `docker/ubuntu/26.04/Dockerfile`, mounts `./tmp/config`, external `VLAN26` network |
| `docker-compose-windows.yaml` | Windows Docker Desktop example | Pulls `izetmolla/containerws:ubuntu-26.04`, Windows-style host paths — adapt paths before use |
| `docker-compose-example.yaml` | Legacy example | Uses older `cgroup: host` + cgroup bind — **avoid on WSL** |

---

## Quick start — Docker CLI (Linux / WSL)

Pull and run Ubuntu 26.04 (recommended):

```bash
docker pull izetmolla/containerws:ubuntu-26.04

docker volume create containerws-docker-data
mkdir -p "$(pwd)/tmp/config"

docker run -d \
  --name containerws \
  --hostname containerws \
  --restart unless-stopped \
  --privileged \
  --cgroupns=private \
  --gpus all \
  --shm-size 2g \
  --security-opt apparmor=unconfined \
  --security-opt seccomp=unconfined \
  --cap-drop SYS_MODULE \
  --tmpfs /run:exec,mode=755 \
  --tmpfs /run/lock:mode=755 \
  --tmpfs /tmp:exec,mode=1777 \
  -e ENV=production \
  -e container=docker \
  -e CONTAINERWS_INIT=auto \
  -e ROOT_PWD='CHANGE_ME' \
  -e ROOT_SSH_PUBLIC_KEY='' \
  -v "$(pwd)/tmp/config:/config" \
  -v containerws-docker-data:/var/lib/docker \
  -p 9000:9000 \
  -p 2222:22 \
  -t \
  --stop-signal SIGRTMIN+3 \
  izetmolla/containerws:ubuntu-26.04
```

Other distros — only change the image tag:

```bash
docker pull izetmolla/containerws:debian-13
docker pull izetmolla/containerws:fedora-44
# …then docker run … izetmolla/containerws:debian-13
```

Without GPU, omit `--gpus all` and the `NVIDIA_*` env vars.

Useful commands:

```bash
docker logs -f containerws
docker exec -it containerws bash
docker stop containerws && docker start containerws   # update same container; /config keeps identity
docker rm -f containerws                              # remove container (keep volumes/config)
```

---

## Environment variables

| Variable | Default / example | Meaning |
|----------|-------------------|---------|
| `ENV` | `production` | App environment |
| `CONTAINERWS_INIT` | `auto` | `auto` / `systemd` / `direct` — how PID 1 starts `cws` + sshd |
| `ROOT_PWD` | *(set your own)* | Root password configured at first boot |
| `ROOT_SSH_PUBLIC_KEY` | empty | Optional authorized key for root SSH |
| `container` | `docker` | Hint for container-aware scripts |
| `NVIDIA_VISIBLE_DEVICES` | `all` | GPU visibility (with NVIDIA toolkit) |
| `NVIDIA_DRIVER_CAPABILITIES` | `all` | NVIDIA driver caps |
| `MCP_PORT` | unset | If set, starts standalone MCP HTTP listener |
| `MCP_TOKEN` | unset | Optional bootstrap MCP auth token |
| `DATABASE_URL` | `/config/containerws/database/database.sqlite` (prod) | SQLite path |

---

## Ports

| Port | Service |
|------|---------|
| **9000** | Container Workspace web UI + REST API (+ `/api/mcp` when enabled) |
| **22** | OpenSSH |

Publish host ports with `-p 9000:9000 -p 2222:22` (or your compose `ports:` / network IP).

---

## Volumes

| Mount | Purpose |
|-------|---------|
| `/config` | Persistent app state (DB, machine id, SSL, VNC, etc.). **Keep this** across recreate. |
| `/var/lib/docker` | Nested Docker Engine data (named volume recommended) |

---

## Binary releases (GitHub)

Publish archives via GoReleaser (binaries only — no Docker images).

### From GitHub Actions (recommended)

1. Open **Actions → Release → Run workflow**
2. Choose a mode:
   - **bump-and-publish** — build, bump `patch`/`minor`/`major`, tag, push, publish release
   - **publish-tag** — publish an existing `v*` tag (enter tag like `v0.1.0`)
   - **snapshot** — dry-run build only (no GitHub Release)

There is a single **Release** workflow (plus normal **CI** on branch pushes). Do not run two release workflows for the same version.

### Local

```bash
./scripts/release.sh patch   # or: minor | major
# → preflight build, then tags vX.Y.Z and pushes;
#   the tag push runs the Release workflow once
```

Preflight must pass before any tag is created. Use `--skip-build` only when CI already built.

Local build only: `./scripts/build.sh` or `task build`.

Tooling versions match [filebrowser CI](https://github.com/filebrowser/filebrowser/blob/master/.github/workflows/ci.yaml): **Go 1.26.x**, **Node 24.x**, **pnpm 10**.

## Build images locally

Requires Docker Buildx. OS images import the binoptimization toolkit:

```bash
# Toolkit (once)
docker buildx build --platform linux/amd64,linux/arm64 \
  -f tools/binoptimization/Dockerfile \
  -t izetmolla/binoptimization:latest \
  --load .   # or --push

# One OS tag (example)
./deploy.sh ubuntu 26.04 --no-push

# Or raw buildx
docker buildx build --platform linux/amd64 \
  -f docker/ubuntu/26.04/Dockerfile \
  -t izetmolla/containerws:ubuntu-26.04 \
  --load .
```

List discoverable tags:

```bash
./deploy.sh --list
```

---

## Softwares inside the workspace

After the container is up, install catalog tools from the UI (**Softwares**), CLI (`cws software …`), or MCP (`softwares_lookup` → `softwares_install`).

Examples: Go, Node.js, Docker Engine, XFCE + noVNC, Google Chrome, VS Code, VS Code Server, Cursor.

On app start, `softwaresync` checks `software_installed` vs the host and reinstalls missing catalog items in the background.

---

## Security note

These images are intended as a **developer workspace**, not a locked-down multi-tenant sandbox. `privileged`, `seccomp:unconfined`, and `apparmor:unconfined` enable nested Docker and desktop/Electron apps. Do not expose them to untrusted networks without additional controls. Always set your own `ROOT_PWD` / SSH keys.
