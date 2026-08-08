package bash

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
		Name: "bash",
		Description: "Run a bash command with full shell control (pipes, redirects, sudo if available, package installs via apt/dnf, systemd, etc.). " +
			"Returns stdout, stderr, exit_code, and duration. Use for installs, updates, builds, and any shell workflow.",
	}, controller.BashTool)
}
