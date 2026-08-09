package resources

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Setup registers MCP resources and prompts with full agent instructions.
func Setup(server *mcp.Server) {
	registerResources(server)
	registerPrompts(server)
}

func registerResources(server *mcp.Server) {
	docs := map[string]resourceDoc{
		"cws://docs/overview": {
			Name:        "overview",
			Title:       "Container Workspace MCP overview",
			Description: "Master instructions for using bash, filesystem, and browser tools correctly.",
			Body:        overviewDoc,
		},
		"cws://docs/bash": {
			Name:        "bash",
			Title:       "Bash tool guide",
			Description: "How to run shell commands, installs, and package managers safely and effectively.",
			Body:        bashDoc,
		},
		"cws://docs/filesystem": {
			Name:        "filesystem",
			Title:       "Filesystem tools guide",
			Description: "Read/write/edit/delete/search file workflows and selector conventions.",
			Body:        filesystemDoc,
		},
		"cws://docs/browser": {
			Name:        "browser",
			Title:       "Chromium browser automation guide",
			Description: "Open Chromium/Chrome, navigate, click, fill, snapshot, screenshot — full playbook.",
			Body:        browserDoc,
		},
		"cws://docs/softwares": {
			Name:        "softwares",
			Title:       "Softwares catalog tools",
			Description: "List/lookup/install/service Softwares items + install queue dismiss.",
			Body:        softwaresDoc,
		},
		"cws://docs/brew": {
			Name:        "brew",
			Title:       "Homebrew tools",
			Description: "Search/install/upgrade Homebrew formulae and casks via the Softwares queue.",
			Body:        brewDoc,
		},
		"cws://docs/softwarepkg": {
			Name:        "softwarepkg",
			Title:       "Software package authoring",
			Description: "Search local/remote registries and create distro-specific install packages.",
			Body:        softwarepkgDoc,
		},
		"cws://docs/kubernetes": {
			Name:        "kubernetes",
			Title:       "Kubernetes MCP tools",
			Description: "Kubeconfig secrets ↔ clusters, pods, resources, events, nodes, helm — multi-cluster selection.",
			Body:        kubernetesDoc,
		},
		"cws://docs/docker": {
			Name:        "docker",
			Title:       "Docker Engine MCP tools",
			Description: "Containers, images, volumes, networks against workspace Docker environments, plus docker mcp gateway helpers.",
			Body:        dockerDoc,
		},
		"cws://docs/auth": {
			Name:        "auth",
			Title:       "MCP authentication",
			Description: "How API keys / tokens protect the MCP endpoint.",
			Body:        authDoc,
		},
	}

	handler := func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		doc, ok := docs[uri]
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     strings.TrimSpace(doc.Body) + "\n",
			}},
		}, nil
	}

	for uri, doc := range docs {
		server.AddResource(&mcp.Resource{
			URI:         uri,
			Name:        doc.Name,
			Title:       doc.Title,
			Description: doc.Description,
			MIMEType:    "text/markdown",
		}, handler)
	}
}

type resourceDoc struct {
	Name, Title, Description, Body string
}

func registerPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "browser_task",
		Title:       "Browser automation task",
		Description: "Guided workflow to open Chromium (if present) and complete a web UI task with snapshots.",
		Arguments: []*mcp.PromptArgument{
			{Name: "goal", Description: "What to accomplish in the browser", Required: true},
			{Name: "start_url", Description: "Optional starting URL", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		goal := req.Params.Arguments["goal"]
		start := req.Params.Arguments["start_url"]
		if start == "" {
			start = "about:blank"
		}
		text := strings.TrimSpace(`
Follow cws://docs/browser. Complete this browser goal reliably with a low-token loop:

Goal: `+goal+`
Start URL: `+start+`

Required loop:
1. browser_status — confirm Chromium/Chrome exists
2. browser_open with url=`+start+`
3. browser_a11y_snapshot (NOT full browser_snapshot) — note refs like e3
4. Act with ref=eN, role=…, label=, or CSS; batch fields with browser_fill_form
5. Verify with browser_assert or browser_find — avoid re-dumping the whole page
6. browser_wait_for for URL/text/selector instead of fixed sleeps
7. browser_screenshot with path=… only when visual proof is needed (avoid return_image unless required)
8. browser_close when finished (unless the user wants the session kept)

Never invent page state — always assert/snapshot before claiming success.
`) + "\n"
		return &mcp.GetPromptResult{
			Description: "Browser automation playbook for the given goal",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "workspace_setup",
		Title:       "Workspace setup",
		Description: "Install tools / packages and verify with bash + filesystem checks.",
		Arguments: []*mcp.PromptArgument{
			{Name: "packages", Description: "Space-separated package names or setup goal", Required: true},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pkgs := req.Params.Arguments["packages"]
		text := strings.TrimSpace(`
Follow cws://docs/bash, cws://docs/softwares, and cws://docs/overview.

Setup goal / packages: `+pkgs+`

Steps:
1. softwares_list or softwares_lookup for each requested item
2. If listed=true → softwares_install (and softwares_service when units exist)
3. If listed=false → detect distro (cat /etc/os-release) and install via apt-get/dnf with bash
4. Verify binaries with command -v / --version (or softwares_lookup on_host)
5. Report exact versions and any failures with stderr
`) + "\n"
		return &mcp.GetPromptResult{
			Description: "Package/setup workflow",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "create_software_package",
		Title:       "Create software package",
		Description: "Search then create/scaffold a Softwares package with distro-specific scripts.",
		Arguments: []*mcp.PromptArgument{
			{Name: "app", Description: "Application / package name to create (e.g. nginx, redis)", Required: true},
			{Name: "output_dir", Description: "Optional path to cws-packages repo root for scaffolding files", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		app := req.Params.Arguments["app"]
		outDir := req.Params.Arguments["output_dir"]
		extra := ""
		if outDir != "" {
			extra = "\nAlso scaffold registry files under output_dir=" + outDir + " (softwarepkg_scaffold or softwarepkg_create with output_dir)."
		}
		text := strings.TrimSpace(`
Follow cws://docs/softwarepkg and cws://docs/softwares.

Create a package for: `+app+extra+`

Steps:
1. softwarepkg_search query=`+app+`
2. softwarepkg_registries — confirm package_url + token for the GitHub registry
3. If a good remote hit exists → softwarepkg_import (then softwares_install if needed)
4. If missing → softwarepkg_publish name=`+app+` (temp clone → scaffold Hub matrix → commit → push)
   - Use dry_run=true first if unsure; set apt_package/dnf_package when OS names differ
5. softwarepkg_image name=`+app+` action=find (or generate) to set a logo
6. Optional: softwarepkg_test_install against Hub containers
7. softwarepkg_import / softwares_install locally
8. Report commit, files, push status, and install hint
`) + "\n"
		return &mcp.GetPromptResult{
			Description: "Create/scaffold software package workflow",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "kubernetes_cluster_pick",
		Title:       "Pick Kubernetes secret and cluster",
		Description: "Ask the user which kubeconfig secret and context/cluster to use when several exist.",
		Arguments: []*mcp.PromptArgument{
			{Name: "goal", Description: "What Kubernetes work to do after picking a cluster", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		goal := req.Params.Arguments["goal"]
		if goal == "" {
			goal = "(general cluster work)"
		}
		text := strings.TrimSpace(`
Follow cws://docs/kubernetes.

Goal after cluster selection: `+goal+`

Required steps:
1. Call configuration_contexts_list (or targets_list).
2. Present each row as: secret_name (kubeconfig_id) → context → cluster → server.
3. If multiple_secrets or multiple_clusters is true, ASK the user which secret and which cluster to use. Do not guess.
4. Optionally configuration_set_active with their choice, or pass kubeconfig_id + context on every subsequent tool call.
5. Then proceed with the goal using pods_*, resources_*, events_list, namespaces_list, helm_*, etc.

Never dump full configuration_view YAML (contains credentials) unless the user explicitly asked.
`) + "\n"
		return &mcp.GetPromptResult{
			Description: "Ask which kubeconfig secret and cluster to use",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})
}

const overviewDoc = `
# Container Workspace MCP — Overview

You control this container workspace through tools. Read domain guides before complex work:
- cws://docs/bash
- cws://docs/filesystem
- cws://docs/browser
- cws://docs/softwares
- cws://docs/brew
- cws://docs/softwarepkg
- cws://docs/kubernetes
- cws://docs/docker
- cws://docs/auth

## Capabilities
1. **bash** — full shell (installs, services, builds, networking; Homebrew on PATH when present)
2. **filesystem** — structured file CRUD + search + zip/unzip
3. **browser_*** — open Chromium/Chrome (if installed) and automate UI via DevTools
4. **softwares_*** — Softwares catalog (list → lookup listed? → install / service / queue)
5. **brew_*** — Homebrew search/install/updates (queues into Softwares Installing)
6. **softwarepkg_*** — search registries + create/scaffold distro packages
7. **kubernetes_*** / pods_* / resources_* / helm_* — manage clusters via kubeconfig secrets
8. **docker_*** — Docker Engine (containers/images/volumes/networks) + optional docker mcp gateway CLI

## Golden rules for correct results
1. Prefer specialized tools over bash when both work (edit_file > sed; softwares_install / brew_install > raw apt/brew; browser_click > xdotool; pods_list > kubectl via bash; docker_containers_list > docker ps).
2. Observe before acting: softwares_lookup / brew_status / softwarepkg_search / configuration_contexts_list / docker_engine_status / list_directory / read_file / browser_a11y_snapshot / browser_status.
3. After mutations, verify (re-read file, browser_assert / a11y snapshot, check exit_code / on_host).
4. Keep commands non-interactive (DEBIAN_FRONTEND=noninteractive, -y flags).
5. Destructive ops need explicit intent (delete_path recursive, rm -rf, pods_delete, resources_delete).
6. Do not claim UI success without browser_assert, browser_a11y_snapshot, or browser_screenshot.
7. Never call softwares_install unless softwares_lookup returns listed=true.
8. If softwares_lookup package_manager=brew (or ownership brew), use brew_* tools — do not softwares_install.
9. Before inventing a new package, softwarepkg_search; then softwarepkg_create / softwarepkg_scaffold.
10. With multiple kubeconfig secrets/clusters, ASK which secret and cluster before mutating.

## Typical flows
- **Install catalog software**: softwares_lookup → softwares_install → softwares_lookup (on_host / is_installed).
- **Install via Homebrew**: brew_status → brew_search → brew_install → softwares_queue.
- **Check brew updates**: brew_check_updates (queues upgrades) → softwares_queue.
- **Dismiss failed install**: softwares_queue → softwares_queue_dismiss id=…
- **Install ad-hoc package**: softwares_lookup (listed=false) → bash (apt/dnf) or brew_install → verify path.
- **Create package for an app**: softwarepkg_search → softwarepkg_create (+ optional softwarepkg_scaffold) → softwarepkg_image → softwares_install.
- **Edit code**: search_files → read_file → edit_file → bash tests.
- **Zip files**: zip_paths / unzip_path.
- **Web UI task**: browser_status → browser_open → a11y snapshot → click/fill by ref → assert → close.
- **Kubernetes**: configuration_contexts_list → ask user which secret/cluster → pods_list / resources_* / events_list.
`

const bashDoc = `
# Bash tool

Tool: **bash**

## Inputs
- command (required)
- cwd (optional working directory)
- timeout_seconds (default 120, max 1800)
- env (optional map)

## Outputs
- stdout, stderr, exit_code, duration_ms, timed_out, truncated

## Best practices
- Chain with set -euo pipefail for scripts that must stop on error.
- Use timeout_seconds for long installs/builds.
- Prefer absolute paths in cwd.
- Package installs:
  - Debian/Ubuntu: apt-get update && apt-get install -y <pkgs>
  - Fedora: dnf install -y <pkgs>
  - Homebrew: prefer **brew_*** MCP tools; raw ` + "`brew`" + ` works when Linuxbrew is on PATH
- Service control: systemctl start|stop|restart|status <unit>
- Chromium install: Softwares catalog item "Google Chrome" (container-safe wrapper chrome-desktop).

## Homebrew PATH
After Brew Package bootstrap, ` + "`/home/linuxbrew/.linuxbrew/bin`" + ` is prepended for bash -lc sessions
(and written to /etc/profile.d/homebrew.sh). If ` + "`command -v brew`" + ` fails, call brew_status / EnsureBrewShellPath via status tool.

## Interpreting results
- exit_code != 0 is a tool-level error (IsError) — read stderr and recover.
- Huge output is truncated; re-run with more specific commands (head/tail/rg).
`

const filesystemDoc = `
# Filesystem tools

## Tools
- read_file / write_file / edit_file
- list_directory / make_directory
- delete_path / move_path / copy_path
- stat_path / search_files
- zip_paths / unzip_path — File Manager archive parity

## Conventions
- Paths may be absolute or relative to the process CWD.
- edit_file requires a unique old_string unless replace_all=true.
- delete_path on non-empty dirs requires recursive=true.
- search_files: name_glob and/or content_pattern (regex).
- zip_paths: one or more paths → .zip (optional destination).
- unzip_path: extract .zip (optional destination directory).

## Recommended edit loop
1. search_files to locate targets
2. read_file (use offset/limit for large files)
3. edit_file with a unique anchor snippet
4. read_file again or bash tests to verify
`

const browserDoc = `
# Browser automation (Chromium / Chrome)

Automates a real Chromium/Chrome binary via Chrome DevTools Protocol (chromedp).
Uses container-safe flags (--no-sandbox, --disable-dev-shm-usage). Prefers wrappers:
chrome-desktop, chromium-desktop, google-chrome-stable, chromium.

## Low-token playbook (preferred)
1. **browser_a11y_snapshot** — compact tree with refs (e1, e2, …). Default ~4k chars.
2. Act with **ref=eN** (or role=/label=/placeholder=) on click/fill/select/hover.
3. Verify with **browser_assert** or **browser_find** — not a full text dump.
4. Batch forms with **browser_fill_form**; wait with **browser_wait_for** (URL/text/selector).
5. Pull data with **browser_extract**. Save screenshots to a **path** (skip return_image unless vision is needed).

Avoid **browser_snapshot** (full visible text) and inline screenshots unless necessary — they burn tokens.

## Tools
| Tool | Purpose |
|------|---------|
| browser_status | Detect binary + session state |
| browser_open | Launch/reuse session, optional url, headless, restart |
| browser_navigate | Go to URL |
| browser_a11y_snapshot | Compact a11y tree + refs (PREFERRED observe) |
| browser_find | Small list of matches by role/name/text/selector |
| browser_click | Click CSS / xpath= / text= / ref= / role= / label= / placeholder= |
| browser_fill | Clear + type into input (same selector DSL) |
| browser_fill_form | Fill many fields (+ optional submit) in one call |
| browser_type | Type without clear (optional selector, press_enter) |
| browser_press | Key / chord (Enter, Control+a) |
| browser_select | Choose <select> option by value/label |
| browser_hover | Hover element (menus/tooltips) |
| browser_scroll | Scroll element into view or page by x/y |
| browser_wait | Wait selector state or milliseconds |
| browser_wait_for | Wait URL / text / selector condition |
| browser_assert | Cheap ok/fail checks (url/title/text/selector) |
| browser_extract | Structured list of texts/attrs/hrefs |
| browser_snapshot | Full visible text (+ optional HTML) — token-heavy |
| browser_screenshot | PNG to path; return_image only when needed |
| browser_evaluate | Run JS expression |
| browser_close | Tear down session |

## Selectors
- CSS: "#id", ".class", "button[type=submit]"
- Text: "text=Sign in"
- XPath: "xpath=//button[@type='submit']" or "//button[@type='submit']"
- Ref (from a11y snapshot): "ref=e3"
- Role: "role=button" or "role=button[name=\"Sign in\"]"
- Label / placeholder: "label=Email", "placeholder=Search"

Optional on click/fill: exact, nth, observe (returns url/title after action).

## Required workflow for reliable results
1. **browser_status** — if not found, softwares_lookup "Google Chrome"; if listed install via softwares_install, else bash; then retry.
2. **browser_open** — set url when known. headless=false when DISPLAY exists (desktop/VNC).
3. **browser_a11y_snapshot** after load — act with ref=eN.
4. Act (click/fill_form/type/press) → **browser_wait_for** if needed → **browser_assert** or another a11y snapshot.
5. Use **browser_screenshot** path=/tmp/... for visual confirmation; return_image=true only for CAPTCHA/layout.
6. **browser_close** when done.

## Headless vs headed
- DISPLAY/WAYLAND set → default headed (visible on desktop/noVNC).
- No display → default headless.
- Override with browser_open.headless true/false.

## Common failures & fixes
- Binary missing → install Google Chrome software; confirm chrome-desktop on PATH.
- Element not found → a11y_snapshot / browser_find; try role= or label=; wait_for visible/text.
- Stale refs after navigation → take a fresh browser_a11y_snapshot.
- Stale session → browser_open with restart=true.
- Sandbox errors → wrappers already pass --no-sandbox; do not launch raw chrome without flags.

## Example: search the web (low token)
1. browser_open url=https://example.com
2. browser_a11y_snapshot
3. browser_fill selector=ref=e2 value=container workspace  (or label=/placeholder=)
4. browser_press key=Enter
5. browser_wait_for text=Results
6. browser_assert text_contains=Results
7. browser_extract selectors=["h3","a[href]"] attribute=text limit=10
8. browser_screenshot path=/tmp/result.png
`

const softwaresDoc = `
# Softwares catalog tools

Manage packages that exist in the Container Workspace **Softwares** module (same catalog as the UI).

## Tools
| Tool | Purpose |
|------|---------|
| softwares_list | List catalog items (optional query filter) |
| softwares_lookup | Check if name/id is **listed**, plus DB install + host probe + package_manager / brew_available |
| softwares_install | Run catalog install script (requires listed=true; blocked when owned by brew) |
| softwares_service | status/start/stop/restart for items with can_control + service_units |
| softwares_queue | Softwares/Brew install queue (pending/running/failed) |
| softwares_queue_dismiss | Remove a **failed** queue row from Softwares → Installing |

## Always check listed first
1. **softwares_lookup** with name_or_id (e.g. "Go", "Docker", "Chrome").
2. If **listed=false** → do **not** call softwares_install; use **bash** (apt/dnf), **brew_***, or **softwarepkg_create**.
3. If **package_manager=brew** → use **brew_*** tools (or switch ownership in UI); do not softwares_install.
4. If **listed=true** and local → softwares_install (optional version / dry_run).
5. Verify with softwares_lookup again (is_installed, on_host) or softwares_service status.
6. Failed installs: softwares_queue → softwares_queue_dismiss.

## Fields to trust
- listed — present in Softwares catalog
- is_installed — row in software_installed table
- package_manager — local | brew
- brew_available — matching Homebrew formula/cask token exists
- on_host — probe found binary/path on this machine
- has_update — installed version ≠ latest catalog version

## Examples
- softwares_lookup name_or_id=Go
- softwares_install name_or_id="Docker Engine"
- softwares_service name_or_id=docker action=status
- softwares_queue_dismiss id=<queue-item-id>
`

const brewDoc = `
# Homebrew tools (brew_*)

Homebrew formulae/casks on this host. Actions enqueue into the **Softwares install queue**
so they never overlap Softwares/VNC package jobs.

## Tools
| Tool | Purpose |
|------|---------|
| brew_status | Module enabled, binary path, prefix, bootstrap |
| brew_search | Search cached formulae/casks |
| brew_installed | List installed packages (+ outdated) |
| brew_install | Queue install/upgrade/uninstall (kind=formula\|cask) |
| brew_check_updates | brew update + list outdated + queue upgrades (default) |

## Flow
1. brew_status — if binary_present=false, enable Brew in Settings / UI bootstrap first
2. brew_search query=… (or brew_installed)
3. brew_install action=install names=["jq"] kind=formula
4. softwares_queue to watch progress; softwares_queue_dismiss for failures

## Ownership
Tokens owned by Softwares (package_manager=local) are blocked until switched in the UI.
`

const softwarepkgDoc = `
# Software package authoring (softwarepkg_*)

Create and discover GitHub-style Softwares packages with **distro-specific** install/uninstall/upgrade scripts and optional **custom_script** (post-install setup).

Layout (see packages/softwarepkg/DRAFT.md):
- softwares/index.json — catalog listing
- softwares/{name}/package.json — metadata
- softwares/{name}/{distro_id}/{distro_version}/{arch}/install.json — scripts

## Tools
| Tool | Purpose |
|------|---------|
| softwarepkg_search | Search local catalog + remote registries; returns host distro facts |
| softwarepkg_create | Upsert local Softwares entry with scripts for **this** host distro |
| softwarepkg_scaffold | Write multi-distro package tree on disk (apt/dnf/apk/pacman) |
| softwarepkg_hub_distros | List distro/version tags from Docker Hub (izetmolla/containerws) |
| softwarepkg_scaffold_hub | Write install.json for each Hub workspace tag (+ any/default fallbacks) |
| softwarepkg_test_install | Run install scripts inside Hub containers and verify success |
| softwarepkg_publish | Temp-clone GitHub registry → scaffold package → commit → push |
| softwarepkg_import | Import from configured GitHub registry for this host |
| softwarepkg_registries | List configured package_url registries |
| softwarepkg_image | Set / find (web CDNs) / generate SVG logo → Softwares.image + optional package.json |

## Package image (logo)
- Field: "image" on Softwares (local DB) and package.json / index.json (remote)
- Prefer https logo URL; data:image/svg+xml is allowed for generated logos
- softwarepkg_image actions:
  - set — provide image URL
  - find — probe Clearbit / Google favicon / DuckDuckGo / Simple Icons (falls back to generate)
  - generate — SVG initials logo (writes softwares/{name}/image.svg when output_dir set)
- create / scaffold / publish / scaffold_hub accept optional image=

## Recommended flow when user asks to package an app
1. **softwarepkg_search** query=<app>
2. If remote hit → **softwarepkg_import** (then softwares_install)
3. If missing → **softwarepkg_publish** name=<app> (clones registry, adds package, pushes)
   - Or dry_run=true first; needs registry token with repo write access
   - Prefer from_hub=true (default) so every Hub workspace tag gets install.json
4. Optional: **softwarepkg_image** name=<app> action=find (or generate) to set a logo
5. Optional: **softwarepkg_test_install** against Hub containers
6. **softwarepkg_import** on other hosts / softwares_install locally
7. Confirm with softwares_lookup

## softwarepkg_publish details
1. Resolves SoftwarePackage registry (package_url + token)
2. git clone into a temp directory
3. Scaffolds softwares/{name}/… (+ index.json)
4. git add / commit / push to ref (default main)
5. Deletes the temp dir unless keep_work_dir=true or dry_run=true

Requires git on PATH and a GitHub package_url. Token goes in SoftwarePackage.token
(or username/password). Secrets are never returned in tool output.

## Docker Hub matrix
Tags from https://hub.docker.com/r/izetmolla/containerws map to paths:
- ubuntu-26.04 → softwares/{name}/ubuntu/26.04/any/install.json
- debian-13 → softwares/{name}/debian/13/any/install.json
- fedora-44 → softwares/{name}/fedora/44/any/install.json

Non-workspace tags (latest, binoptimization) are skipped by default.

## Distro families generated
- ubuntu / debian / kali → apt-get
- fedora / rhel / centos / rocky / almalinux → dnf
- alpine → apk
- arch / manjaro → pacman
- default → detects apt/dnf/apk/pacman at runtime

## Examples
- softwarepkg_hub_distros
- softwarepkg_publish name=htop details="interactive process viewer" category=Tools
- softwarepkg_publish name=htop dry_run=true keep_work_dir=true
- softwarepkg_scaffold_hub name=htop output_dir=/path/to/cws-packages overwrite=true
- softwarepkg_test_install name=htop package_root=/path/to/cws-packages tags=["ubuntu-26.04"] pull=true
- softwarepkg_search query=redis
- softwarepkg_create name=redis category=Database service_units=["redis-server.service"] can_control=true control_backend=systemd
- softwarepkg_image name=nginx action=find domain=nginx.org
- softwarepkg_image name=nginx action=generate color=#009639
- softwarepkg_image name=nginx action=set image=https://cdn.simpleicons.org/nginx
`

const kubernetesDoc = `
# Kubernetes MCP tools

Inspired by https://github.com/containers/kubernetes-mcp-server — native client-go tools (not kubectl wrappers), plus Helm CLI when present.

## Kubeconfig secrets ↔ clusters (ask first)

Each pasted kubeconfig YAML is a **secret** (credentials file). Each **context** inside it points at a **cluster**.

1. Call **configuration_contexts_list** or **targets_list**.
2. If multiple_secrets or multiple_clusters is true, **ask the user** which secret (kubeconfig_id / secret_name) and which cluster (context) to use.
3. Pass kubeconfig_id + context on tools, or call **configuration_set_active**.
4. Do not dump **configuration_view** YAML unless asked (contains tokens/certs).

UI: Kubernetes → Settings — map each secret to its contexts/clusters and set the active pair.

## Toolsets

### config
| Tool | Purpose |
|------|---------|
| configuration_contexts_list | List secrets + contexts/clusters/servers |
| targets_list | Same targets, pick-oriented |
| configuration_view | Minified (default) or full kubeconfig YAML |
| configuration_set_active | Set workspace default secret + context |

### core
| Tool | Purpose |
|------|---------|
| namespaces_list / projects_list | Namespaces (OpenShift projects when available) |
| events_list | Cluster/namespace events |
| pods_list / pods_list_in_namespace / pods_get / pods_delete | Pod CRUD-ish |
| pods_log / pods_exec / pods_run / pods_top | Logs, exec, run image, metrics |
| nodes_log / nodes_stats_summary / nodes_top | Node logs/stats/metrics |
| resources_list / resources_get / resources_create_or_update / resources_delete / resources_scale | Any apiVersion/kind |

### helm (requires helm on PATH)
| Tool | Purpose |
|------|---------|
| helm_install / helm_list / helm_uninstall | Chart releases against the selected kubeconfig |

## Common args
Most tools accept optional kubeconfig_id and context (ClusterRef). Omit to use the active workspace pair.

## Examples
- configuration_contexts_list
- configuration_set_active kubeconfig_id=<id> context=prod-eu
- pods_list_in_namespace namespace=default
- resources_list apiVersion=apps/v1 kind=Deployment namespace=default
- pods_log name=my-pod namespace=default tail=200
`

const dockerDoc = `
# Docker Engine MCP tools

Native Docker Engine management against Container Workspace **Docker environments** (same backends as the Docker UI).
Gateway helpers probe the optional [docker/mcp-gateway](https://github.com/docker/mcp-gateway) CLI plugin ("docker mcp").

## Environments

1. Call **docker_environments_list** to see local socket / TLS / SSH targets.
2. Pass optional **environment_id** on tools, or omit to use the workspace default.
3. Prefer **docker_engine_status** before mutating.

## Engine tools

| Tool | Purpose |
|------|---------|
| docker_engine_status | Ping Engine; version + container/image counts |
| docker_environments_list | List configured environments |
| docker_containers_list / get / logs | Inspect containers |
| docker_containers_start / stop / restart / remove | Lifecycle |
| docker_containers_run | Create+start (image, ports, volumes, env, network) |
| docker_images_list / pull / remove | Images |
| docker_volumes_list / remove | Volumes |
| docker_networks_list | Networks |

## MCP Gateway CLI (optional)

These do **not** replace Engine tools — they detect/list the Docker MCP Toolkit gateway when the plugin is installed:

| Tool | Purpose |
|------|---------|
| docker_mcp_gateway_status | Detect "docker mcp" plugin / version |
| docker_mcp_tools_list | Run "docker mcp tools ls" (catalog from gateway profiles) |

Install the plugin via Docker Desktop MCP Toolkit or build https://github.com/docker/mcp-gateway.

## Safety

- Confirm **remove** / **force** / privileged **run** with the user when intent is unclear.
- Prefer docker_* tools over raw docker via bash when both work.
`

const authDoc = `
# MCP authentication

Endpoint: **/api/mcp** on the main app port, or the root of **MCP_PORT** when standalone is enabled
(e.g. http://host:9100/).

Provide a valid key via one of:
- Authorization: Bearer <key>
- X-Api-Key: <key>
- X-Auth-Token: <key>
- ?token=<key> or form field token

**Loopback exception:** requests from **127.0.0.1** / **::1** (same host) are allowed
without a key. Remote clients still require a valid key.

Keys are stored in the **mcp_keys** table (models.MCPKey):
- status must be active
- expires_at must be null or in the future
- last_used_at / last_used_ip update on success

Optional bootstrap / standalone override: env **MCP_TOKEN**.
Standalone listener: set **MCP_PORT** (and optional **MCP_ADDRESS**, default 0.0.0.0).

Never embed secrets in resources or tool arguments meant for logging.
`
