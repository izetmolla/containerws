package softwarepkg

import (
	"github.com/izetmolla/containerws/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

func LoadTools(server *mcp.Server, app *config.AppClients) {
	controller := NewController(app)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_search",
		Description: "Search local Softwares catalog and configured GitHub package registries for an app name. " +
			"Use before creating a package to see if it already exists (local / remote / both). " +
			"Also returns current host distro facts for scaffolding.",
	}, controller.SearchTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_scaffold",
		Description: "Create a GitHub-style software package on disk under output_dir: " +
			"softwares/{name}/package.json, distro install.json scripts (apt/dnf/apk/pacman), and softwares/index.json. " +
			"Call softwarepkg_search first. Distros default to ubuntu,debian,fedora,alpine,arch,default.",
	}, controller.ScaffoldTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_create",
		Description: "Create a Softwares catalog entry locally with install/uninstall/upgrade scripts " +
			"matched to the current host distro (apt/dnf/apk/pacman). Optionally also scaffold registry files " +
			"when output_dir is set. Prefer softwarepkg_search first to avoid duplicates.",
	}, controller.CreateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_import",
		Description: "Import a package from a configured SoftwarePackage registry (GitHub raw) into the local catalog " +
			"for this host's distro/arch. Requires registries via softwarepkg_registries.",
	}, controller.ImportTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "softwarepkg_registries",
		Description: "List configured software package registries (package_url). Secrets are never returned.",
	}, controller.RegistriesTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_hub_distros",
		Description: "List distro/version workspace tags from Docker Hub for izetmolla/containerws " +
			"(https://hub.docker.com/r/izetmolla/containerws). Parses tags like ubuntu-26.04, debian-13, fedora-44.",
	}, controller.HubDistrosTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_scaffold_hub",
		Description: "Create install.json files for every workspace tag on Docker Hub (izetmolla/containerws), " +
			"plus optional {distro}/any/any and default fallbacks. Prefer softwarepkg_hub_distros first.",
	}, controller.ScaffoldHubTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_test_install",
		Description: "Test a package by running its install.json script inside each matching izetmolla/containerws " +
			"container tag, then verifying with verify_command (default: command -v <pkg>). " +
			"Requires Docker. Use dry_run=true to only resolve scripts. Set pull=true to docker pull first.",
	}, controller.TestInstallTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_publish",
		Description: "Clone the configured GitHub package registry into a temp folder, scaffold a new/updated package " +
			"(Hub distro matrix by default), commit, and push. Requires SoftwarePackage.package_url and a write token. " +
			"Use dry_run=true to commit locally without pushing. Prefer softwarepkg_search first.",
	}, controller.PublishTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwarepkg_image",
		Description: "Set, find on the web, or generate a logo image for a software package (local Softwares.image and/or " +
			"registry package.json). Actions: set (provide image URL), find (Clearbit/Simple Icons/favicon CDNs; falls back to generate), " +
			"generate (SVG initials logo). Use apply_local=true to update the DB; output_dir to write into a registry tree.",
	}, controller.ImageTool)
}
