package dockermcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// Controller hosts Docker Engine MCP tools for Container Workspace.
// Inspired by Docker MCP Toolkit patterns (https://github.com/docker/mcp-gateway)
// but exposes Engine management (containers/images/volumes/networks) against the
// workspace Docker environments — same backend as the Docker UI module.
type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

func (c *Controller) db() *gorm.DB {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.DB()
}

func LoadTools(server *mcp.Server, app *config.AppClients) {
	ctrl := NewController(app)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_engine_status",
		Description: "Ping Docker Engine for the selected (or default) environment. " +
			"Returns reachability, server version, container/image counts. Prefer this before other docker_* tools.",
	}, ctrl.EngineStatusTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_environments_list",
		Description: "List Docker environments configured in Container Workspace (local socket, TLS, SSH).",
	}, ctrl.EnvironmentsListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_list",
		Description: "List Docker containers (all=true includes stopped). Optional name/label filters.",
	}, ctrl.ContainersListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_get",
		Description: "Inspect a Docker container by id or name.",
	}, ctrl.ContainersGetTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_logs",
		Description: "Fetch recent container logs (tail lines, optional timestamps/stderr).",
	}, ctrl.ContainersLogsTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_start",
		Description: "Start a stopped Docker container.",
	}, ctrl.ContainersStartTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_stop",
		Description: "Stop a running Docker container (optional timeout seconds).",
	}, ctrl.ContainersStopTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_restart",
		Description: "Restart a Docker container.",
	}, ctrl.ContainersRestartTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_remove",
		Description: "Remove a Docker container. force=true to remove a running container; volumes=true removes anonymous volumes.",
	}, ctrl.ContainersRemoveTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_containers_run",
		Description: "Create and start a container (image required). Optional name, command, env, ports, volumes, network, detach (default true).",
	}, ctrl.ContainersRunTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_images_list",
		Description: "List Docker images (optional dangling/reference filters).",
	}, ctrl.ImagesListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_images_pull",
		Description: "Pull a Docker image by reference (e.g. nginx:alpine).",
	}, ctrl.ImagesPullTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_images_remove",
		Description: "Remove a Docker image by id or reference. force=true to ignore dependents.",
	}, ctrl.ImagesRemoveTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_volumes_list",
		Description: "List Docker volumes.",
	}, ctrl.VolumesListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_volumes_remove",
		Description: "Remove a Docker volume by name. force=true to force removal.",
	}, ctrl.VolumesRemoveTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_networks_list",
		Description: "List Docker networks.",
	}, ctrl.NetworksListTool)

	// Optional Docker MCP Gateway CLI helpers (https://github.com/docker/mcp-gateway).
	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_mcp_gateway_status",
		Description: "Detect whether the docker mcp CLI plugin (MCP Gateway) is installed and summarize version/help. " +
			"Does not start the gateway — use for availability checks.",
	}, ctrl.GatewayStatusTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "docker_mcp_tools_list",
		Description: "If docker mcp CLI is available, run `docker mcp tools ls` (optional --format=json). " +
			"Lists tools exposed by the Docker MCP Gateway/profile catalog.",
	}, ctrl.GatewayToolsListTool)
}

func (c *Controller) resolveClient(environmentID string) (*models.DockerEnvironment, *client.Client, error) {
	db := c.db()
	_ = environments.EnsureDefaultLocal(db)
	env, err := environments.Resolve(db, strings.TrimSpace(environmentID))
	if err != nil {
		return nil, nil, err
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return env, nil, err
	}
	return env, cli, nil
}

func (c *Controller) withTimeout(parent context.Context, seconds int, def time.Duration) (context.Context, context.CancelFunc) {
	d := def
	if seconds > 0 {
		d = time.Duration(seconds) * time.Second
	}
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return context.WithTimeout(parent, d)
}

func envSummary(env *models.DockerEnvironment) map[string]any {
	if env == nil {
		return map[string]any{"id": "", "name": "local", "conn_type": "unix"}
	}
	return map[string]any{
		"id":          env.ID,
		"name":        env.Name,
		"conn_type":   env.ConnType,
		"is_default":  env.IsDefault,
		"is_disabled": env.IsDisabled,
	}
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("\n...[truncated %d of %d chars]", n, len(s))
}
