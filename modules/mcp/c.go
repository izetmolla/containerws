package mcp

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/mcp/resources"
	"github.com/izetmolla/containerws/modules/mcp/tools"
	mcplib "github.com/modelcontextprotocol/go-sdk/mcp"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

const mcpServerInstructions = `Container Workspace MCP — bash, filesystem, Chromium browser, Softwares catalog, Homebrew, software packages, Kubernetes, and Docker.

## Always-read resources (use resources/read)
- cws://docs/overview — master rules for correct results
- cws://docs/bash — shell / installs (Homebrew on PATH after panel bootstrap)
- cws://docs/filesystem — file CRUD + search + zip/unzip
- cws://docs/browser — open Chromium and automate UI
- cws://docs/softwares — Softwares catalog + install queue
- cws://docs/brew — Homebrew search / install / updates (queues into Softwares)
- cws://docs/softwarepkg — search / create / scaffold GitHub packages by distro
- cws://docs/kubernetes — Kubernetes MCP (kubeconfig secrets ↔ clusters, pods, resources, helm)
- cws://docs/docker — Docker Engine (containers/images/volumes/networks) + docker mcp gateway helpers
- cws://docs/auth — API key / token auth

## Prompts
- browser_task — guided browser automation loop
- workspace_setup — install/verify packages
- create_software_package — search then scaffold/create a distro package
- kubernetes_cluster_pick — ask which kubeconfig secret and cluster to use

## Softwares (catalog — check listed first)
- softwares_list → softwares_lookup(name) → if listed=true then softwares_install / softwares_service.
- If package_manager=brew, use brew_* tools (do not softwares_install).
- softwares_queue / softwares_queue_dismiss for Installing failures.
- If listed=false, do not invent catalog installs; use bash for ad-hoc packages, brew_search/brew_install, or softwarepkg_create.

## Homebrew
- brew_status → brew_search / brew_installed → brew_install (queues Softwares install jobs).
- brew_check_updates runs brew update and queues upgrades.
- Softwares-owned (local) tokens are blocked until switched in the UI.

## Software packages (authoring)
- softwarepkg_search → if missing, softwarepkg_publish (clone registry → scaffold → push).
- Or local-only: softwarepkg_create / softwarepkg_scaffold_hub + softwarepkg_test_install.
- Scripts are generated per distro family (apt/dnf/apk/pacman). Use apt_package/dnf_package when OS names differ.
- softwarepkg_import pulls from a configured GitHub registry for this host.

## Kubernetes (multi-cluster)
- First call configuration_contexts_list or targets_list.
- If multiple_secrets or multiple_clusters is true, ASK the user which kubeconfig secret (kubeconfig_id) and which cluster (context) to use before mutating.
- Pass kubeconfig_id + context on tools, or configuration_set_active to set the workspace default.
- Prefer k8s tools over raw kubectl via bash. helm_* needs helm CLI on PATH.

## Docker
- docker_engine_status / docker_environments_list first; pass environment_id when not using the default.
- Prefer docker_containers_* / docker_images_* / docker_volumes_* / docker_networks_* over raw docker via bash.
- docker_mcp_gateway_status / docker_mcp_tools_list probe the optional docker mcp CLI plugin (MCP Gateway) — they do not start the gateway.

## Bash
- Use bash for installs, updates, package managers (apt/dnf/brew), services, builds.
- Prefer cwd; set timeout_seconds for long jobs. Non-zero exit_code is recoverable.
- Linuxbrew bin is prepended to PATH when present (/home/linuxbrew/.linuxbrew).

## Filesystem
- Prefer read_file / write_file / edit_file / list_directory / delete_path / move_path / copy_path / make_directory / stat_path / search_files.
- zip_paths / unzip_path for archives (File Manager parity).
- Recursive deletes require recursive=true.

## Browser (Chromium/Chrome)
- Low-token loop: browser_status → browser_open → browser_a11y_snapshot → act with ref=/role= → browser_assert / browser_find → browser_close.
- Prefer browser_a11y_snapshot + ref=eN over browser_snapshot (full text dumps are expensive).
- Prefer browser_fill_form, browser_wait_for, browser_assert, browser_extract over many tiny steps + full snapshots.
- Screenshots: save to path by default; set return_image=true only when pixels are required.
- Selectors: CSS, text=, xpath=, ref=eN, role=button[name="…"], label=, placeholder=.
- Headed when DISPLAY is set; otherwise headless. Requires Chrome/Chromium (Softwares: Google Chrome / chrome-desktop).
- Never claim UI success without browser_assert, a fresh a11y snapshot, or screenshot.

## Safety
- Confirm destructive paths. Prefer edit_file over rewriting large files. Cap large reads.
- Confirm destructive Kubernetes deletes/scales and Docker remove/force/privileged run with the user when intent is unclear.`

func newMCPServer(appClients *config.AppClients) *mcplib.Server {
	server := mcplib.NewServer(
		&mcplib.Implementation{Name: "Container Workspace", Version: "v1.2.0"},
		&mcplib.ServerOptions{Instructions: mcpServerInstructions},
	)
	tools.SetupTools(server, appClients)
	resources.Setup(server)
	return server
}

func mountMCPHandler(router fiber.Router, server *mcplib.Server) {
	router.All("/", adaptor.HTTPHandler(mcplib.NewStreamableHTTPHandler(func(_ *http.Request) *mcplib.Server {
		return server
	}, &mcplib.StreamableHTTPOptions{
		Stateless: true,
	})))
}

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	mcp := router.Group("/mcp", cc.mcpMiddleware)
	mountMCPHandler(mcp, newMCPServer(appClients))
}
