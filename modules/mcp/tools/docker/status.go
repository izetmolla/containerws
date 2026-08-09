package dockermcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EngineStatusInput struct {
	EnvironmentID  string `json:"environment_id,omitempty" jsonschema:"optional Docker environment id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"optional ping timeout (default 5)"`
}

type EngineStatusOutput struct {
	Reachable      bool           `json:"reachable"`
	Environment    map[string]any `json:"environment"`
	APIVersion     string         `json:"api_version,omitempty"`
	ServerVersion  string         `json:"server_version,omitempty"`
	Name           string         `json:"name,omitempty"`
	Containers     int            `json:"containers,omitempty"`
	ContainersRun  int            `json:"containers_running,omitempty"`
	Images         int            `json:"images,omitempty"`
	Driver         string         `json:"driver,omitempty"`
	Architecture   string         `json:"architecture,omitempty"`
	NCPU           int            `json:"ncpu,omitempty"`
	MemTotal       int64          `json:"mem_total,omitempty"`
	Sock           string         `json:"sock,omitempty"`
	Message        string         `json:"message"`
	Error          string         `json:"error,omitempty"`
}

func (c *Controller) EngineStatusTool(ctx context.Context, _ *mcp.CallToolRequest, input EngineStatusInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := c.withTimeout(ctx, input.TimeoutSeconds, 5*time.Second)
	defer cancel()

	env, cli, err := c.resolveClient(input.EnvironmentID)
	out := EngineStatusOutput{Environment: envSummary(env)}
	if env != nil && env.ConnType == models.DockerConnUnix {
		out.Sock = env.SocketPath
	}
	if err != nil {
		out.Message = "Docker client unavailable"
		out.Error = err.Error()
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	ping, err := cli.Ping(runCtx)
	if err != nil {
		out.Message = "Docker Engine is not reachable"
		out.Error = err.Error()
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	out.Reachable = true
	out.APIVersion = ping.APIVersion
	info, err := cli.Info(runCtx)
	if err == nil {
		out.ServerVersion = info.ServerVersion
		out.Name = info.Name
		out.Containers = info.Containers
		out.ContainersRun = info.ContainersRunning
		out.Images = info.Images
		out.Driver = info.Driver
		out.Architecture = info.Architecture
		out.NCPU = info.NCPU
		out.MemTotal = info.MemTotal
	}
	out.Message = fmt.Sprintf("Docker Engine reachable (%s)", strings.TrimSpace(out.ServerVersion))
	return &mcp.CallToolResult{}, out, nil
}

type EnvironmentsListInput struct{}

type EnvironmentsListOutput struct {
	Items   []map[string]any `json:"items"`
	Total   int              `json:"total"`
	Message string           `json:"message"`
}

func (c *Controller) EnvironmentsListTool(ctx context.Context, _ *mcp.CallToolRequest, _ EnvironmentsListInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	db := c.db()
	_ = environments.EnsureDefaultLocal(db)
	var rows []models.DockerEnvironment
	if db != nil {
		_ = db.Order("is_default DESC, name ASC").Find(&rows).Error
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{
			"id":          r.ID,
			"name":        r.Name,
			"conn_type":   r.ConnType,
			"host_url":    r.HostURL,
			"socket_path": r.SocketPath,
			"is_default":  r.IsDefault,
			"is_disabled": r.IsDisabled,
		})
	}
	return &mcp.CallToolResult{}, EnvironmentsListOutput{
		Items:   items,
		Total:   len(items),
		Message: fmt.Sprintf("%d environment(s)", len(items)),
	}, nil
}
