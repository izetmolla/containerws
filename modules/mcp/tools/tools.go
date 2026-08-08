package tools

import (
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/mcp/tools/bash"
	"github.com/izetmolla/containerws/modules/mcp/tools/browser"
	"github.com/izetmolla/containerws/modules/mcp/tools/filesystem"
	"github.com/izetmolla/containerws/modules/mcp/tools/k8s"
	"github.com/izetmolla/containerws/modules/mcp/tools/softwarepkg"
	"github.com/izetmolla/containerws/modules/mcp/tools/softwares"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func SetupTools(server *mcp.Server, appClients *config.AppClients) {
	bash.LoadTools(server, appClients)
	filesystem.LoadTools(server, appClients)
	browser.LoadTools(server, appClients)
	softwares.LoadTools(server, appClients)
	softwarepkg.LoadTools(server, appClients)
	k8s.LoadTools(server, appClients)
}
