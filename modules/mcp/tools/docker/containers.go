package dockermcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ContainersListInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	All           bool   `json:"all,omitempty" jsonschema:"include stopped containers (default false = running only)"`
	Name          string `json:"name,omitempty" jsonschema:"optional name filter substring"`
	Label         string `json:"label,omitempty" jsonschema:"optional label filter key=value"`
	Limit         int    `json:"limit,omitempty" jsonschema:"max containers to return (default 100)"`
}

type containerRow struct {
	ID      string   `json:"id"`
	Names   []string `json:"names"`
	Image   string   `json:"image"`
	Status  string   `json:"status"`
	State   string   `json:"state"`
	Ports   string   `json:"ports,omitempty"`
	Created int64    `json:"created"`
}

type ContainersListOutput struct {
	Items   []containerRow `json:"items"`
	Total   int            `json:"total"`
	Message string         `json:"message"`
}

func (c *Controller) ContainersListTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersListInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := c.withTimeout(ctx, 0, 30*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersListOutput{Message: err.Error()}, nil
	}
	args := container.ListOptions{All: input.All}
	f := filters.NewArgs()
	if n := strings.TrimSpace(input.Name); n != "" {
		f.Add("name", n)
	}
	if l := strings.TrimSpace(input.Label); l != "" {
		f.Add("label", l)
	}
	if f.Len() > 0 {
		args.Filters = f
	}
	list, err := cli.ContainerList(runCtx, args)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersListOutput{Message: err.Error()}, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	items := make([]containerRow, 0, min(len(list), limit))
	for i, it := range list {
		if i >= limit {
			break
		}
		names := make([]string, 0, len(it.Names))
		for _, n := range it.Names {
			names = append(names, strings.TrimPrefix(n, "/"))
		}
		ports := make([]string, 0, len(it.Ports))
		for _, p := range it.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}
		items = append(items, containerRow{
			ID:      it.ID[:min(12, len(it.ID))],
			Names:   names,
			Image:   it.Image,
			Status:  it.Status,
			State:   it.State,
			Ports:   strings.Join(ports, ", "),
			Created: it.Created,
		})
	}
	return &mcp.CallToolResult{}, ContainersListOutput{
		Items:   items,
		Total:   len(list),
		Message: fmt.Sprintf("%d container(s)", len(list)),
	}, nil
}

type ContainersGetInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	IDOrName      string `json:"id_or_name" jsonschema:"required container id or name"`
}

type ContainersGetOutput struct {
	ID      string         `json:"id,omitempty"`
	Name    string         `json:"name,omitempty"`
	Image   string         `json:"image,omitempty"`
	State   string         `json:"state,omitempty"`
	Status  string         `json:"status,omitempty"`
	Inspect map[string]any `json:"inspect,omitempty"`
	Message string         `json:"message"`
}

func (c *Controller) ContainersGetTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersGetInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 30*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersGetOutput{Message: err.Error()}, nil
	}
	insp, err := cli.ContainerInspect(runCtx, id)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersGetOutput{Message: err.Error()}, nil
	}
	raw, _ := json.Marshal(insp)
	var asMap map[string]any
	_ = json.Unmarshal(raw, &asMap)
	state := ""
	status := ""
	if insp.State != nil {
		state = insp.State.Status
	}
	return &mcp.CallToolResult{}, ContainersGetOutput{
		ID:      insp.ID,
		Name:    strings.TrimPrefix(insp.Name, "/"),
		Image:   insp.Config.Image,
		State:   state,
		Status:  status,
		Inspect: asMap,
		Message: "ok",
	}, nil
}

type ContainersLogsInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	IDOrName      string `json:"id_or_name" jsonschema:"required container id or name"`
	Tail          string `json:"tail,omitempty" jsonschema:"number of lines from the end (default 200) or all"`
	Timestamps    bool   `json:"timestamps,omitempty"`
	Stdout        *bool  `json:"stdout,omitempty" jsonschema:"include stdout (default true)"`
	Stderr        *bool  `json:"stderr,omitempty" jsonschema:"include stderr (default true)"`
}

type ContainersLogsOutput struct {
	Logs      string `json:"logs"`
	Truncated bool   `json:"truncated,omitempty"`
	Message   string `json:"message"`
}

func (c *Controller) ContainersLogsTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersLogsInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersLogsOutput{Message: err.Error()}, nil
	}
	tail := strings.TrimSpace(input.Tail)
	if tail == "" {
		tail = "200"
	}
	stdout, stderr := true, true
	if input.Stdout != nil {
		stdout = *input.Stdout
	}
	if input.Stderr != nil {
		stderr = *input.Stderr
	}
	reader, err := cli.ContainerLogs(runCtx, id, container.LogsOptions{
		ShowStdout: stdout,
		ShowStderr: stderr,
		Timestamps: input.Timestamps,
		Tail:       tail,
	})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersLogsOutput{Message: err.Error()}, nil
	}
	defer reader.Close()
	b, err := io.ReadAll(io.LimitReader(reader, 512*1024))
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersLogsOutput{Message: err.Error()}, nil
	}
	text := string(b)
	trunc := len(b) >= 512*1024
	return &mcp.CallToolResult{}, ContainersLogsOutput{
		Logs:      text,
		Truncated: trunc,
		Message:   "ok",
	}, nil
}

type ContainersIDInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	IDOrName      string `json:"id_or_name" jsonschema:"required container id or name"`
	Timeout       *int   `json:"timeout,omitempty" jsonschema:"stop timeout seconds"`
}

type ContainersActionOutput struct {
	ID      string `json:"id_or_name"`
	Action  string `json:"action"`
	Message string `json:"message"`
}

func (c *Controller) ContainersStartTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersIDInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "start", Message: err.Error()}, nil
	}
	if err := cli.ContainerStart(runCtx, id, container.StartOptions{}); err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "start", Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, ContainersActionOutput{ID: id, Action: "start", Message: "started"}, nil
}

func (c *Controller) ContainersStopTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersIDInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "stop", Message: err.Error()}, nil
	}
	var timeout *int
	if input.Timeout != nil {
		timeout = input.Timeout
	}
	if err := cli.ContainerStop(runCtx, id, container.StopOptions{Timeout: timeout}); err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "stop", Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, ContainersActionOutput{ID: id, Action: "stop", Message: "stopped"}, nil
}

func (c *Controller) ContainersRestartTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersIDInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 90*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "restart", Message: err.Error()}, nil
	}
	var timeout *int
	if input.Timeout != nil {
		timeout = input.Timeout
	}
	if err := cli.ContainerRestart(runCtx, id, container.StopOptions{Timeout: timeout}); err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "restart", Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, ContainersActionOutput{ID: id, Action: "restart", Message: "restarted"}, nil
}

type ContainersRemoveInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	IDOrName      string `json:"id_or_name" jsonschema:"required container id or name"`
	Force         bool   `json:"force,omitempty"`
	Volumes       bool   `json:"volumes,omitempty" jsonschema:"remove anonymous volumes"`
}

func (c *Controller) ContainersRemoveTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersRemoveInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrName)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "remove", Message: err.Error()}, nil
	}
	if err := cli.ContainerRemove(runCtx, id, container.RemoveOptions{Force: input.Force, RemoveVolumes: input.Volumes}); err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersActionOutput{ID: id, Action: "remove", Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, ContainersActionOutput{ID: id, Action: "remove", Message: "removed"}, nil
}

type ContainersRunInput struct {
	EnvironmentID string            `json:"environment_id,omitempty"`
	Image         string            `json:"image" jsonschema:"required image reference"`
	Name          string            `json:"name,omitempty"`
	Command       []string          `json:"command,omitempty" jsonschema:"optional command override"`
	Env           map[string]string `json:"env,omitempty"`
	Ports         []string          `json:"ports,omitempty" jsonschema:"host:container[/proto] bindings e.g. 8080:80"`
	Volumes       []string          `json:"volumes,omitempty" jsonschema:"bind mounts host:container[:ro]"`
	Network       string            `json:"network,omitempty"`
	Detach        *bool             `json:"detach,omitempty" jsonschema:"default true — create+start and return id"`
	AutoRemove    bool              `json:"auto_remove,omitempty"`
	Privileged    bool              `json:"privileged,omitempty"`
	Pull          bool              `json:"pull,omitempty" jsonschema:"pull image before create"`
}

type ContainersRunOutput struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Warning string `json:"warning,omitempty"`
	Message string `json:"message"`
}

func (c *Controller) ContainersRunTool(ctx context.Context, _ *mcp.CallToolRequest, input ContainersRunInput) (*mcp.CallToolResult, any, error) {
	img := strings.TrimSpace(input.Image)
	if img == "" {
		return nil, nil, fmt.Errorf("image is required")
	}
	detach := true
	if input.Detach != nil {
		detach = *input.Detach
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 5*time.Minute)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{Message: err.Error()}, nil
	}

	if input.Pull {
		reader, err := cli.ImagePull(runCtx, img, image.PullOptions{})
		if err != nil {
			return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{Message: "pull failed: " + err.Error()}, nil
		}
		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}

	env := make([]string, 0, len(input.Env))
	for k, v := range input.Env {
		if strings.TrimSpace(k) == "" {
			continue
		}
		env = append(env, k+"="+v)
	}

	exposed, portBindings, err := parsePorts(input.Ports)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{Message: err.Error()}, nil
	}
	binds, err := parseVolumes(input.Volumes)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{Message: err.Error()}, nil
	}

	cfg := &container.Config{
		Image:        img,
		Env:          env,
		Cmd:          input.Command,
		ExposedPorts: exposed,
	}
	host := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
		AutoRemove:   input.AutoRemove,
		Privileged:   input.Privileged,
	}
	networking := &network.NetworkingConfig{}
	if n := strings.TrimSpace(input.Network); n != "" {
		networking.EndpointsConfig = map[string]*network.EndpointSettings{n: {}}
	}

	created, err := cli.ContainerCreate(runCtx, cfg, host, networking, nil, strings.TrimSpace(input.Name))
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{Message: err.Error()}, nil
	}
	if err := cli.ContainerStart(runCtx, created.ID, container.StartOptions{}); err != nil {
		return &mcp.CallToolResult{IsError: true}, ContainersRunOutput{
			ID:      created.ID,
			Message: "created but start failed: " + err.Error(),
		}, nil
	}
	msg := "started"
	if !detach {
		msg = "started (detach=false still returns immediately; attach is not supported over MCP)"
	}
	return &mcp.CallToolResult{}, ContainersRunOutput{
		ID:      created.ID,
		Name:    strings.TrimSpace(input.Name),
		Warning: strings.Join(created.Warnings, "; "),
		Message: msg,
	}, nil
}

func parsePorts(ports []string) (nat.PortSet, nat.PortMap, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// host:container[/proto]
		parts := strings.Split(p, ":")
		var hostPort, containerPart string
		switch len(parts) {
		case 1:
			containerPart = parts[0]
		case 2:
			hostPort, containerPart = parts[0], parts[1]
		default:
			return nil, nil, fmt.Errorf("invalid port binding %q (use host:container[/tcp])", p)
		}
		proto := "tcp"
		if i := strings.IndexByte(containerPart, '/'); i >= 0 {
			proto = containerPart[i+1:]
			containerPart = containerPart[:i]
		}
		port, err := nat.NewPort(proto, containerPart)
		if err != nil {
			return nil, nil, err
		}
		exposed[port] = struct{}{}
		if hostPort != "" {
			bindings[port] = append(bindings[port], nat.PortBinding{HostPort: hostPort})
		}
	}
	return exposed, bindings, nil
}

func parseVolumes(vols []string) ([]string, error) {
	out := make([]string, 0, len(vols))
	for _, v := range vols {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !strings.Contains(v, ":") {
			return nil, fmt.Errorf("invalid volume %q (use host:container[:ro])", v)
		}
		out = append(out, v)
	}
	return out, nil
}
