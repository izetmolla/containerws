package softwares

import (
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/softwares/seed"
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
		Name: "softwares_list",
		Description: "List Softwares catalog entries (Container Workspace Softwares module). " +
			"Returns id, name, category, latest/installed versions, and whether each item is listed and installed. " +
			"Call this first to see what can be managed via softwares_install / softwares_service.",
	}, controller.ListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwares_lookup",
		Description: "Check whether a software is listed in the Softwares catalog (by id or name). " +
			"Also reports DB install row and host probe (present/missing). " +
			"Use before install: if listed=false, do not call softwares_install — use bash instead.",
	}, controller.LookupTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwares_install",
		Description: "Install (or reinstall) a Softwares catalog item by running its install script. " +
			"Requires the software to be listed in the catalog (softwares_lookup listed=true). " +
			"Marks software_installed on success. Prefer softwares_lookup first.",
	}, controller.InstallTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwares_service",
		Description: "Manage systemd units for a listed Softwares item (status|start|stop|restart|logs). " +
			"Only works for catalog items with can_control=true and service_units set (e.g. Docker Engine). " +
			"action=logs returns recent journalctl lines for debugging. " +
			"Fails clearly when the software is not listed or has no managed units.",
	}, controller.ServiceTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwares_queue",
		Description: "Show the Softwares install queue (pending/running/failed), including Brew jobs serialized with Softwares installs.",
	}, controller.QueueTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "softwares_queue_dismiss",
		Description: "Remove a failed Softwares/Brew install queue item so it no longer appears under Softwares → Installing. " +
			"Pass id (queue row) and/or software_id. Does not cancel running jobs.",
	}, controller.QueueDismissTool)
}

func (c *Controller) ensureCatalog() {
	if c == nil || c.app == nil {
		return
	}
	seed.SeedIfEmpty(c.app)
}
