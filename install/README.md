# Container Workspace — native installer

Installs the **released `containerws` binary** on the host, prepares config directories, links the `cws` CLI, and installs an OS daemon that runs:

```bash
cws --start
```

This is for **bare-metal / VM** installs of the Go binary built by [`.goreleaser.yml`](../.goreleaser.yml). It is not the Docker image wizard.

## Quick install

```bash
curl -sSL https://raw.githubusercontent.com/izetmolla/containerws/main/install/install.sh | bash
```

Or download first:

```bash
curl -sSL https://raw.githubusercontent.com/izetmolla/containerws/main/install/install.sh -o /tmp/cws-install.sh
sudo bash /tmp/cws-install.sh
```

## Options

| Flag | Meaning |
|------|---------|
| `--version vX.Y.Z` | Install a specific GitHub Release tag (default: latest) |
| `--binary ./containerws` | Install a local binary instead of downloading |
| `--repo owner/name` | Override GitHub repo (default: `izetmolla/containerws`) |
| `--no-start` | Install + enable daemon, but do not start it yet |
| `--uninstall` | Stop daemon and remove binaries/links (keeps `/config` data) |

```bash
sudo bash install.sh --version v0.1.0
sudo bash install.sh --binary ./dist/containerws_linux_amd64/containerws
sudo bash install.sh --uninstall
```

## What it does

1. **Root** — re-execs with `sudo` when needed  
2. **OS packages** — ensures `curl`, `ca-certificates`, `tar`, `gzip` via apt/dnf/apk/pacman/zypper/pkg/brew  
3. **Directories**
   - `/usr/local/lib/containerws/bin/` — binary home  
   - `/usr/local/bin/cws` and `/usr/local/bin/containerws` — CLI symlinks  
   - `/config/containerws/{database,ssl,vnc-sessions}` — app data (matches production defaults)  
   - `/etc/containerws/environment` — daemon env (`ENV=production`)  
   - `/var/lib/containerws`, `/var/log/containerws`  
4. **Binary** — downloads `containerws_<ver>_<os>_<arch>.tar.gz` from GitHub Releases (same layout as GoReleaser), or copies `--binary`  
5. **Daemon** — installs and starts a service that runs `cws --start`:

| OS / init | Unit |
|-----------|------|
| Linux systemd | `/etc/systemd/system/containerws.service` |
| macOS launchd | `/Library/LaunchDaemons/com.izetmolla.containerws.plist` |
| OpenRC | `/etc/init.d/containerws` |
| FreeBSD | `/usr/local/etc/rc.d/containerws` |

## After install

```bash
cws version
systemctl status containerws          # Linux systemd
journalctl -u containerws -f

# Web UI
open http://127.0.0.1:9000
```

Edit `/etc/containerws/environment` for `DATABASE_URL`, `MCP_PORT`, `ENABLE_HTTPS`, etc., then restart the service.

## Uninstall

```bash
sudo bash install.sh --uninstall
```

Removes the binary, CLI links, and daemon unit. **Keeps** `/config/containerws` and `/etc/containerws` so your database and SSL material survive.

## Related

- App CLI: `cws --start` (see `cmd/root.go`)
- Releases: https://github.com/izetmolla/containerws/releases
- Docker images (separate path): see the repo [README](../README.md)
