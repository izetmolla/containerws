# softwarepkg — GitHub software package registry (draft)

Design draft for a GitHub-hosted package manager used by Container Workspace.
The Go API in this package will resolve host identity → raw GitHub paths →
upsert `Software` + `SoftwareVersion`.

Related models:

- `models.SoftwarePackage` — registry base URL + optional credentials
- `models.Software` / `models.SoftwareVersion` — local catalog after import
- `packages/machine.Detect()` — host facts for path resolution

---

## How the URL identifies the host

Host facts come from `packages/machine.Detect()` and `models.NormalizeArch`:

| Path segment | Source | Example |
|---|---|---|
| `distro_id` | `/etc/os-release` `ID` → `Snapshot.DistroID` | `ubuntu` |
| `distro_version` | `VERSION_ID` → `Snapshot.DistroVersion` | `26.10` / `24.04` |
| `arch` | `NormalizeArch(GOARCH)` | `amd64`, `arm64`, `arm` |

Do **not** put `linux` in every path (almost all targets are Linux). Keep OS in
the JSON payload (`"os": "linux"`) so Darwin/Windows can be added later under a
different top-level if needed.

**Canonical path:**

```text
softwares/{software}/{distro_id}/{distro_version}/{arch}/install.json
# softwares/nginx/ubuntu/26.10/arm64/install.json
```

Normalize arch in both repo folders and resolver:

| Runtime / uname | Catalog folder |
|---|---|
| `x86_64`, `amd64`, `x64` | `amd64` |
| `aarch64`, `arm64` | `arm64` |
| `armv7l`, `armhf`, `arm` | `arm` |
| `i386`, `i686` | `386` |

---

## Recommended GitHub repo layout

```text
cws-packages/                          # git repo
  README.md
  softwares/
    index.json                         # catalog listing (required for merge)
    nginx/
      package.json                     # software metadata (shared)
      ubuntu/
        24.04/
          amd64/install.json
          arm64/install.json
        26.10/
          amd64/install.json
          arm64/install.json
        any/                           # any Ubuntu VERSION_ID
          amd64/install.json
          any/install.json             # any arch on Ubuntu
      debian/
        bookworm/
          amd64/install.json
        any/
          any/install.json
      default/
        install.json                   # last-resort fallback
```

### Catalog index: `softwares/index.json`

The softwares list merges local DB rows with remotes discovered from this file
(cached ~2 minutes). Search / pagination / filters run on the merged set.

```json
{
  "softwares": [
    {
      "name": "nginx",
      "details": "High-performance HTTP server",
      "category": "Web",
      "sub_category": "Servers",
      "tags": ["web", "proxy"],
      "icon": "Server",
      "color": "#009639",
      "order": 10,
      "service_units": ["nginx.service"],
      "can_control": true,
      "control_backend": "systemd",
      "start_command": "systemctl start nginx.service",
      "restart_command": "systemctl restart nginx.service",
      "stop_command": "systemctl stop nginx.service"
    }
  ]
}
```

Raw URL:

```text
https://raw.githubusercontent.com/{owner}/{repo}/{ref}/softwares/index.json
```

Names already in the local DB are marked `source: "both"`; registry-only entries
get synthetic ids `remote:{slug}` and `is_remote: true` until imported via
`POST /api/softwares/package/import`.

**Raw URL** (public):

```text
https://raw.githubusercontent.com/{owner}/{repo}/{ref}/softwares/nginx/ubuntu/26.10/arm64/install.json
```

`SoftwarePackage.PackageURL` stores the repo base, e.g.
`https://github.com/izetmolla/containerwspkg` (default) or an already-raw base.
`Token` / `Username` / `Password` attach as HTTP auth for private repos.

On app start (and on registry list / import / MCP resolve),
`softwarepkg.EnsureDefaultRegistry` inserts
`https://github.com/izetmolla/containerwspkg` if no matching row exists.

### Example: `softwares/nginx/package.json`

```json
{
  "name": "nginx",
  "details": "High-performance HTTP server",
  "category": "Web",
  "sub_category": "Servers",
  "tags": ["web", "proxy"],
  "icon": "Server",
  "color": "#009639",
  "order": 10,
  "service_units": ["nginx.service"],
  "can_control": true,
  "control_backend": "systemd",
  "start_command": "systemctl start nginx.service",
  "restart_command": "systemctl restart nginx.service",
  "stop_command": "systemctl stop nginx.service"
}
```

### Example: `softwares/nginx/ubuntu/26.10/arm64/install.json`

```json
{
  "version": "1.26.2",
  "is_latest": true,
  "os": "linux",
  "distro_id": "ubuntu",
  "distro_version": "26.10",
  "arch": "arm64",
  "package_family": "apt",
  "install_script": "#!/usr/bin/env bash\nset -euo pipefail\napt-get update\napt-get install -y nginx\n",
  "uninstall_script": "#!/usr/bin/env bash\nset -euo pipefail\napt-get remove -y nginx\n",
  "upgrade_script": "",
  "custom_script": ""
}
```

`custom_script` (optional) runs **after a successful install** when non-empty —
use it for configuration / post-setup. Leave empty to skip.

Different distros = different folders + different scripts (apt vs dnf vs
pacman). Shared name/details live only in `package.json`.

---

## Resolution algorithm

When importing `nginx` for the current host:

```text
machine.Detect
  → normalize distro_id, distro_version, arch
  → GET .../ubuntu/26.10/arm64/install.json
      404 → .../ubuntu/26.10/any/install.json
      404 → .../ubuntu/any/arm64/install.json
      404 → .../ubuntu/any/any/install.json
      404 → .../default/install.json
  → upsert Software + SoftwareVersion
```

Candidate paths (in order):

1. `softwares/{name}/{distro_id}/{distro_version}/{arch}/install.json`
2. `softwares/{name}/{distro_id}/{distro_version}/any/install.json`
3. `softwares/{name}/{distro_id}/any/{arch}/install.json`
4. `softwares/{name}/{distro_id}/any/any/install.json`
5. `softwares/{name}/default/install.json`

Also `GET softwares/{name}/package.json` once for metadata (required).

### URL builder sketch

```go
// base: https://raw.githubusercontent.com/acme/cws-packages/main
path := fmt.Sprintf("softwares/%s/%s/%s/%s/install.json",
  name, distroID, distroVersion, arch)
```

If `PackageURL` is `https://github.com/owner/repo`, convert to
`https://raw.githubusercontent.com/owner/repo/{ref}/…` (default ref `main`).

---

## Docker Hub workspace matrix

Published images: https://hub.docker.com/r/izetmolla/containerws

Tag → path:

| Hub tag | install.json path |
|---|---|
| `ubuntu-26.04` | `softwares/{name}/ubuntu/26.04/any/install.json` |
| `debian-13` | `softwares/{name}/debian/13/any/install.json` |
| `fedora-44` | `softwares/{name}/fedora/44/any/install.json` |

Go helpers:

- `ListHubTags` — fetch + parse workspace tags
- `Scaffold(... FromHub: true)` — write install.json per tag (+ optional any/default)
- `TestInstall` — `docker run --entrypoint bash <image> -lc '<install_script>; <verify>'`

Skip non-workspace tags: `latest`, `binoptimization`, and `*-<extra>` variants.

---

## Publish to GitHub registry

`Publish` / MCP `softwarepkg_publish`:

1. Resolve `SoftwarePackage` (`package_url` + optional token)
2. `git clone` into a temp directory (authenticated HTTPS)
3. Scaffold package files (Hub matrix by default)
4. `git add softwares && git commit && git push`
5. Remove temp dir (unless `keep_work_dir` / `dry_run`)

---

## Planned Go API (this package)

| Function | Role |
|---|---|
| `ResolveInstallPaths(name, host) []string` | Build fallback-relative paths |
| `RawBaseURL(packageURL, ref) (string, error)` | GitHub → raw.githubusercontent.com |
| `FetchJSON(ctx, client, url, auth)` | HTTP GET with registry credentials |
| `Import(ctx, db, pkg, softwareName)` | Detect host → fetch → upsert DB |

Wire from `modules/softwares/package` later, e.g.:

```http
POST /api/softwares/package/import
{ "name": "nginx", "package_id": "<software_packages.id>" }
```

---

## Mapping into the local DB

| GitHub | DB |
|---|---|
| `package.json` | `models.Software` (name, details, category, tags, icon, …) |
| `install.json` | `models.SoftwareVersion` (version, scripts, os/distro_id/arch/package_family, is_latest) |

One GitHub variant folder → one `SoftwareVersion` row with host targeting filled
so existing `MatchesHost` / `pickBestVersion` keep working after import.

---

## Checklist: publish nginx on GitHub

1. Create repo `cws-packages` (public or private)
2. Add `softwares/nginx/package.json`
3. Add per distro/arch `install.json` files, plus `ubuntu/any/any` and `default` fallbacks
4. Insert a `software_packages` row pointing at the repo (+ token if private)
5. Call `softwarepkg.Import(..., "nginx")` on the target machine → scripts land in SQLite

---

## Implementation status

- [x] Package scaffold + this draft
- [x] Path resolve + arch normalize + GitHub raw base parse
- [x] HTTP fetch with auth + fallback chain
- [x] Upsert Software / SoftwareVersion
- [x] API endpoints under `modules/softwares/package`
  - `GET/POST /api/softwares/package/registry`
  - `PUT/DELETE /api/softwares/package/registry/:registryId`
  - `POST /api/softwares/package/import` `{ "name": "nginx", "package_id": "…", "ref": "main" }`

