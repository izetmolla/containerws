package tools

import (
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/mcp/tools/bash"
	brewmcp "github.com/izetmolla/containerws/modules/mcp/tools/brew"
	"github.com/izetmolla/containerws/modules/mcp/tools/browser"
	dockermcp "github.com/izetmolla/containerws/modules/mcp/tools/docker"
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
	brewmcp.LoadTools(server, appClients)
	softwarepkg.LoadTools(server, appClients)
	k8s.LoadTools(server, appClients)
	dockermcp.LoadTools(server, appClients)
}
