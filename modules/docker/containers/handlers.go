package containers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/docker/envcli"
	"github.com/izetmolla/containerws/packages/dockerclient"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	list := api.Group("/list")
	list.Get("/", cc.ListAPI)

	single := api.Group("/single")
	single.Post("/", cc.CreateAPI)
	single.Get("/:id", cc.GetAPI)
	single.Put("/:id", cc.UpdateAPI)
	single.Post("/:id/start", cc.StartAPI)
	single.Post("/:id/stop", cc.StopAPI)
	single.Post("/:id/restart", cc.RestartAPI)
	single.Post("/:id/kill", cc.KillAPI)
	single.Post("/:id/pause", cc.PauseAPI)
	single.Post("/:id/resume", cc.UnpauseAPI)
	single.Delete("/:id", cc.RemoveAPI)
	single.Post("/:id/recreate", cc.RecreateAPI)
	single.Post("/:id/restart-policy", cc.RestartPolicyAPI)
	single.Post("/:id/networks/connect", cc.ConnectNetworkAPI)
	single.Post("/:id/networks/disconnect", cc.DisconnectNetworkAPI)
	single.Post("/:id/commit", cc.CommitAPI)
	single.Get("/:id/logs", cc.LogsAPI)
	single.Get("/:id/stats", cc.StatsAPI)
	single.Get("/:id/top", cc.TopAPI)

	single.Use("/:id/exec", func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	single.Get("/:id/exec", websocket.New(cc.HandleExecWS, websocket.Config{
		RecoverHandler: func(conn *websocket.Conn) {
			// Never WriteJSON here — the session may still have an active writer.
			if closer, ok := conn.Locals("exec_ws_close").(func()); ok && closer != nil {
				closer()
				return
			}
			_ = conn.Close()
		},
	}))
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

type portMapping struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type containerRow struct {
	ID         string            `json:"id"`
	ShortID    string            `json:"short_id"`
	Names      []string          `json:"names"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	ImageID    string            `json:"image_id"`
	Command    string            `json:"command"`
	Created    int64             `json:"created"`
	State      string            `json:"state"`
	Status     string            `json:"status"`
	Ports      []portMapping     `json:"ports"`
	Labels     map[string]string `json:"labels,omitempty"`
	Stack      string            `json:"stack,omitempty"`
	IPAddress  string            `json:"ip_address,omitempty"`
	IPAddresses []string         `json:"ip_addresses,omitempty"`
}

func stackFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	for _, key := range []string{
		"com.docker.compose.project",
		"com.docker.stack.namespace",
		"com.docker.compose.project.name",
	} {
		if v := strings.TrimSpace(labels[key]); v != "" {
			return v
		}
	}
	return ""
}

func ipsFromContainer(c types.Container) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || ip == "0.0.0.0" || ip == "::" {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	if c.NetworkSettings != nil {
		for _, nw := range c.NetworkSettings.Networks {
			if nw == nil {
				continue
			}
			add(nw.IPAddress)
			add(nw.GlobalIPv6Address)
		}
	}
	return out
}

func mapListItem(c types.Container) containerRow {
	names := make([]string, 0, len(c.Names))
	name := ""
	for _, n := range c.Names {
		n = strings.TrimPrefix(n, "/")
		names = append(names, n)
		if name == "" {
			name = n
		}
	}
	ports := make([]portMapping, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, portMapping{
			IP:          p.IP,
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		})
	}
	id := c.ID
	short := id
	if len(short) > 12 {
		short = short[:12]
	}
	ips := ipsFromContainer(c)
	ip := ""
	if len(ips) > 0 {
		ip = ips[0]
	}
	return containerRow{
		ID:          id,
		ShortID:     short,
		Names:       names,
		Name:        name,
		Image:       c.Image,
		ImageID:     c.ImageID,
		Command:     c.Command,
		Created:     c.Created,
		State:       c.State,
		Status:      c.Status,
		Ports:       ports,
		Labels:      c.Labels,
		Stack:       stackFromLabels(c.Labels),
		IPAddress:   ip,
		IPAddresses: ips,
	}
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	all := c.Query("all", "1") != "0"
	items, err := cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]containerRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, mapListItem(it))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createBody struct {
	Image          string            `json:"image"`
	Name           string            `json:"name"`
	Cmd            []string          `json:"cmd"`
	Entrypoint     []string          `json:"entrypoint"`
	Env            []string          `json:"env"`
	Ports          []string          `json:"ports"` // host:container[/proto]
	PublishAll     bool              `json:"publish_all"`
	Binds          []string          `json:"binds"`
	Networks       []string          `json:"networks"`
	RestartPolicy  string            `json:"restart_policy"`
	RestartRetries int               `json:"restart_retries"`
	Memory         string            `json:"memory"` // e.g. 512m, 1g
	CPUs           float64           `json:"cpus"`
	WorkingDir     string            `json:"working_dir"`
	User           string            `json:"user"`
	Hostname       string            `json:"hostname"`
	Privileged     bool              `json:"privileged"`
	ReadonlyRootfs bool              `json:"readonly_rootfs"`
	AutoRemove     bool              `json:"auto_remove"`
	Labels         map[string]string `json:"labels"`
	ExtraHosts     []string          `json:"extra_hosts"`
	Devices        []string          `json:"devices"` // host:container[:perms]
	CapAdd         []string          `json:"cap_add"`
	CapDrop        []string          `json:"cap_drop"`
	DNS            []string          `json:"dns"`
	PullIfMissing  bool              `json:"pull_if_missing"`
	AlwaysPull     bool              `json:"always_pull"`
	Start          *bool             `json:"start"`
	Tty            bool              `json:"tty"`
	OpenStdin      bool              `json:"open_stdin"`
	LogDriver      string            `json:"log_driver"`
	LogOpts        map[string]string `json:"log_opts"`
}

func parseMemory(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "g"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		mult = 1024
		s = strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "b"):
		s = strings.TrimSuffix(s, "b")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory: %s", s)
	}
	return int64(v * float64(mult)), nil
}

func parsePortBindings(ports []string) (nat.PortSet, nat.PortMap, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, raw := range ports {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		proto := "tcp"
		parts := strings.Split(raw, "/")
		if len(parts) == 2 {
			proto = strings.ToLower(parts[1])
			raw = parts[0]
		}
		hp := strings.Split(raw, ":")
		var hostIP, hostPort, containerPort string
		switch len(hp) {
		case 1:
			containerPort = hp[0]
		case 2:
			hostPort, containerPort = hp[0], hp[1]
		case 3:
			hostIP, hostPort, containerPort = hp[0], hp[1], hp[2]
		default:
			return nil, nil, fmt.Errorf("invalid port mapping: %s", raw)
		}
		p, err := nat.NewPort(proto, containerPort)
		if err != nil {
			return nil, nil, err
		}
		exposed[p] = struct{}{}
		if hostPort != "" || hostIP != "" {
			bindings[p] = append(bindings[p], nat.PortBinding{HostIP: hostIP, HostPort: hostPort})
		} else {
			bindings[p] = append(bindings[p], nat.PortBinding{})
		}
	}
	return exposed, bindings, nil
}

func parseDevices(devs []string) []container.DeviceMapping {
	out := make([]container.DeviceMapping, 0, len(devs))
	for _, d := range devs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		parts := strings.Split(d, ":")
		m := container.DeviceMapping{CgroupPermissions: "rwm"}
		switch len(parts) {
		case 1:
			m.PathOnHost = parts[0]
			m.PathInContainer = parts[0]
		case 2:
			m.PathOnHost = parts[0]
			m.PathInContainer = parts[1]
		default:
			m.PathOnHost = parts[0]
			m.PathInContainer = parts[1]
			m.CgroupPermissions = parts[2]
		}
		out = append(out, m)
	}
	return out
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()
	insp, err := cc.createFromBody(ctx, cli, body)
	if err != nil {
		if ve, ok := err.(*validationError); ok {
			return r.Api(c, r.WithError(ve), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
		}
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    insp,
		"message": "Container created",
	}))
}

// UpdateAPI replaces an existing container with a new config (stop → remove → create).
func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()

	old, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = strings.TrimPrefix(old.Name, "/")
	}
	_ = cli.ContainerStop(ctx, id, container.StopOptions{})
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return cc.respondErr(c, err)
	}
	insp, err := cc.createFromBody(ctx, cli, body)
	if err != nil {
		if ve, ok := err.(*validationError); ok {
			return r.Api(c, r.WithError(ve), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
		}
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    insp,
		"message": "Container updated",
	}))
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func (cc *controller) createFromBody(ctx context.Context, cli *client.Client, body createBody) (fiber.Map, error) {
	body.Image = strings.TrimSpace(body.Image)
	if body.Image == "" {
		return nil, &validationError{"image is required"}
	}

	_, _, inspErr := cli.ImageInspectWithRaw(ctx, body.Image)
	shouldPull := body.AlwaysPull || (body.PullIfMissing && inspErr != nil)
	if shouldPull {
		rd, err := cli.ImagePull(ctx, body.Image, image.PullOptions{})
		if err != nil {
			return nil, err
		}
		_, _ = io.Copy(io.Discard, rd)
		_ = rd.Close()
	} else if inspErr != nil {
		return nil, &validationError{"image not found locally; enable Always pull or pull the image first"}
	}

	exposed, portBindings, err := parsePortBindings(body.Ports)
	if err != nil {
		return nil, &validationError{err.Error()}
	}

	mem, err := parseMemory(body.Memory)
	if err != nil {
		return nil, &validationError{err.Error()}
	}

	restartName := strings.TrimSpace(body.RestartPolicy)
	if restartName == "" {
		restartName = "no"
	}

	cfg := &container.Config{
		Image:        body.Image,
		Env:          body.Env,
		Cmd:          body.Cmd,
		Entrypoint:   body.Entrypoint,
		WorkingDir:   body.WorkingDir,
		User:         body.User,
		Hostname:     body.Hostname,
		Labels:       body.Labels,
		ExposedPorts: exposed,
		Tty:          body.Tty,
		OpenStdin:    body.OpenStdin,
		AttachStdin:  body.OpenStdin,
		AttachStdout: true,
		AttachStderr: true,
	}

	host := &container.HostConfig{
		Binds:           body.Binds,
		PortBindings:    portBindings,
		PublishAllPorts: body.PublishAll,
		Privileged:      body.Privileged,
		ReadonlyRootfs:  body.ReadonlyRootfs,
		AutoRemove:      body.AutoRemove,
		ExtraHosts:      body.ExtraHosts,
		CapAdd:          body.CapAdd,
		CapDrop:         body.CapDrop,
		DNS:             body.DNS,
		RestartPolicy: container.RestartPolicy{
			Name:              container.RestartPolicyMode(restartName),
			MaximumRetryCount: body.RestartRetries,
		},
		Resources: container.Resources{
			Devices: parseDevices(body.Devices),
		},
	}
	if strings.TrimSpace(body.LogDriver) != "" {
		host.LogConfig = container.LogConfig{
			Type:   strings.TrimSpace(body.LogDriver),
			Config: body.LogOpts,
		}
	}
	if mem > 0 {
		host.Memory = mem
	}
	if body.CPUs > 0 {
		host.NanoCPUs = int64(body.CPUs * 1e9)
	}

	var networking *network.NetworkingConfig
	if len(body.Networks) > 0 {
		endpoints := map[string]*network.EndpointSettings{}
		for i, n := range body.Networks {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			endpoints[n] = &network.EndpointSettings{}
			if i == 0 {
				host.NetworkMode = container.NetworkMode(n)
			}
		}
		networking = &network.NetworkingConfig{EndpointsConfig: endpoints}
	}

	name := strings.TrimSpace(body.Name)
	resp, err := cli.ContainerCreate(ctx, cfg, host, networking, nil, name)
	if err != nil {
		return nil, err
	}

	start := true
	if body.Start != nil {
		start = *body.Start
	}
	if start {
		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return nil, err
		}
	}

	insp, err := cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return fiber.Map{"id": resp.ID, "warnings": resp.Warnings}, nil
	}
	return inspectPayload(insp), nil
}

func inspectPayload(insp types.ContainerJSON) fiber.Map {
	name := strings.TrimPrefix(insp.Name, "/")
	state := ""
	status := ""
	if insp.State != nil {
		state = insp.State.Status
		status = insp.State.Status
		if insp.State.Running {
			status = "running"
		}
	}
	ports := []portMapping{}
	if insp.NetworkSettings != nil {
		for p, bindings := range insp.NetworkSettings.Ports {
			priv, _ := nat.ParsePort(string(p))
			proto := p.Proto()
			if len(bindings) == 0 {
				ports = append(ports, portMapping{PrivatePort: uint16(priv), Type: proto})
				continue
			}
			for _, b := range bindings {
				pub, _ := strconv.Atoi(b.HostPort)
				ports = append(ports, portMapping{
					IP:          b.HostIP,
					PrivatePort: uint16(priv),
					PublicPort:  uint16(pub),
					Type:        proto,
				})
			}
		}
	}
	short := insp.ID
	if len(short) > 12 {
		short = short[:12]
	}
	return fiber.Map{
		"id":       insp.ID,
		"short_id": short,
		"name":     name,
		"image":    insp.Config.Image,
		"state":    state,
		"status":   status,
		"created":  insp.Created,
		"ports":    ports,
		"inspect":  insp,
	}
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": inspectPayload(insp)}))
}

func (cc *controller) StartAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "started", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		return cli.ContainerStart(ctx, id, container.StartOptions{})
	})
}

func (cc *controller) StopAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "stopped", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		timeout := 10
		return cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
	})
}

func (cc *controller) RestartAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "restarted", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		timeout := 10
		return cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
	})
}

func (cc *controller) KillAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "killed", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		return cli.ContainerKill(ctx, id, "SIGKILL")
	})
}

func (cc *controller) PauseAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "paused", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		return cli.ContainerPause(ctx, id)
	})
}

func (cc *controller) UnpauseAPI(c fiber.Ctx) error {
	return cc.simpleAction(c, "resumed", func(ctx context.Context, id string) error {
		cli, err := envcli.Engine(cc.app, c)
		if err != nil {
			return err
		}
		return cli.ContainerUnpause(ctx, id)
	})
}

func (cc *controller) RemoveAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	force := c.Query("force", "1") != "0"
	rmVolumes := c.Query("volumes", "0") == "1"
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force, RemoveVolumes: rmVolumes}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Container removed",
	}))
}

func (cc *controller) RecreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	var body struct {
		Pull bool `json:"pull"`
	}
	_ = c.Bind().Body(&body)
	// Also accept ?pull=1 for simple clients.
	if !body.Pull {
		q := strings.ToLower(c.Query("pull"))
		body.Pull = q == "1" || q == "true" || q == "yes"
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()

	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return cc.respondErr(c, err)
	}
	wasRunning := insp.State != nil && insp.State.Running
	name := strings.TrimPrefix(insp.Name, "/")
	imageRef := ""
	if insp.Config != nil {
		imageRef = strings.TrimSpace(insp.Config.Image)
	}
	if imageRef == "" {
		imageRef = strings.TrimSpace(insp.Image)
	}

	if body.Pull && imageRef != "" {
		rd, pullErr := cli.ImagePull(ctx, imageRef, image.PullOptions{})
		if pullErr != nil {
			return cc.respondErr(c, pullErr)
		}
		_, _ = io.Copy(io.Discard, rd)
		_ = rd.Close()
	}

	_ = cli.ContainerStop(ctx, id, container.StopOptions{})
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return cc.respondErr(c, err)
	}

	var networking *network.NetworkingConfig
	if insp.NetworkSettings != nil && len(insp.NetworkSettings.Networks) > 0 {
		endpoints := map[string]*network.EndpointSettings{}
		maps.Copy(endpoints, insp.NetworkSettings.Networks)
		networking = &network.NetworkingConfig{EndpointsConfig: endpoints}
	}

	resp, err := cli.ContainerCreate(ctx, insp.Config, insp.HostConfig, networking, nil, name)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if wasRunning {
		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return cc.respondErr(c, err)
		}
	}
	out, _ := cli.ContainerInspect(ctx, resp.ID)
	msg := "Container recreated"
	if body.Pull {
		msg = "Container recreated (image re-pulled)"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    inspectPayload(out),
		"message": msg,
	}))
}

func (cc *controller) simpleAction(c fiber.Ctx, verb string, fn func(context.Context, string) error) error {
	r := cc.app.Render()
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	if err := fn(ctx, id); err != nil {
		return cc.respondErr(c, err)
	}
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    fiber.Map{"id": id},
			"message": "Container " + verb,
		}))
	}
	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    fiber.Map{"id": id},
			"message": "Container " + verb,
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    inspectPayload(insp),
		"message": "Container " + verb,
	}))
}

func (cc *controller) LogsAPI(c fiber.Ctx) error {
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	tail := c.Query("tail", "200")
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	reader, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       tail,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	defer reader.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Docker multiplex header is 8 bytes when TTY is false
		if len(line) > 8 && (line[0] == 1 || line[0] == 2) {
			b.Write(line[8:])
		} else {
			b.Write(line)
		}
		b.WriteByte('\n')
	}
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"logs": b.String()},
	}))
}

func (cc *controller) StatsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	stats, err := cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return cc.respondErr(c, err)
	}
	defer stats.Body.Close()
	var raw map[string]any
	if err := json.NewDecoder(stats.Body).Decode(&raw); err != nil {
		return cc.respondErr(c, err)
	}

	out := fiber.Map{
		"read_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if cpuStats, ok := raw["cpu_stats"].(map[string]any); ok {
		if pre, ok := raw["precpu_stats"].(map[string]any); ok {
			out["cpu_percent"] = calcCPUPercent(pre, cpuStats)
		}
	}
	if mem, ok := raw["memory_stats"].(map[string]any); ok {
		usage := memUsageBytes(mem)
		limit, _ := mem["limit"].(float64)
		out["memory_usage"] = usage
		out["memory_limit"] = limit
		if limit > 0 {
			out["memory_percent"] = (usage / limit) * 100
		}
	}

	var netRX, netTX float64
	networks := make([]fiber.Map, 0)
	if nets, ok := raw["networks"].(map[string]any); ok {
		for name, v := range nets {
			nm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			rx, _ := nm["rx_bytes"].(float64)
			tx, _ := nm["tx_bytes"].(float64)
			netRX += rx
			netTX += tx
			networks = append(networks, fiber.Map{
				"name":     name,
				"rx_bytes": rx,
				"tx_bytes": tx,
			})
		}
	}
	out["networks"] = networks
	out["network_rx"] = netRX
	out["network_tx"] = netTX

	readBytes, writeBytes := blkioRW(raw)
	out["blkio_read"] = readBytes
	out["blkio_write"] = writeBytes

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

func (cc *controller) TopAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	top, err := cli.ContainerTop(ctx, id, nil)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"titles":     top.Titles,
			"processes":  top.Processes,
		},
	}))
}

type restartPolicyBody struct {
	Name              string `json:"name"`
	MaximumRetryCount int    `json:"maximum_retry_count"`
}

func (cc *controller) RestartPolicyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body restartPolicyBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "no"
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	_, err = cli.ContainerUpdate(ctx, id, container.UpdateConfig{
		RestartPolicy: container.RestartPolicy{
			Name:              container.RestartPolicyMode(name),
			MaximumRetryCount: body.MaximumRetryCount,
		},
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Restart policy updated"}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    inspectPayload(insp),
		"message": "Restart policy updated",
	}))
}

type networkAttachBody struct {
	NetworkID string `json:"network_id"`
	Force     bool   `json:"force"`
}

func (cc *controller) ConnectNetworkAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body networkAttachBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	nid := strings.TrimSpace(body.NetworkID)
	if nid == "" {
		return r.Api(c, r.WithError(fmt.Errorf("network_id is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.NetworkConnect(ctx, nid, id, &network.EndpointSettings{}); err != nil {
		return cc.respondErr(c, err)
	}
	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Joined network"}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    inspectPayload(insp),
		"message": "Joined network",
	}))
}

func (cc *controller) DisconnectNetworkAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body networkAttachBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	nid := strings.TrimSpace(body.NetworkID)
	if nid == "" {
		return r.Api(c, r.WithError(fmt.Errorf("network_id is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.NetworkDisconnect(ctx, nid, id, body.Force); err != nil {
		return cc.respondErr(c, err)
	}
	insp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Left network"}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    inspectPayload(insp),
		"message": "Left network",
	}))
}

type commitBody struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Pause      *bool  `json:"pause"`
	Message    string `json:"message"`
	Author     string `json:"author"`
}

func (cc *controller) CommitAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body commitBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	repo := strings.TrimSpace(body.Repository)
	if repo == "" {
		return r.Api(c, r.WithError(fmt.Errorf("repository (image name) is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	tag := strings.TrimSpace(body.Tag)
	if tag == "" {
		tag = "latest"
	}
	ref := repo + ":" + tag
	pause := true
	if body.Pause != nil {
		pause = *body.Pause
	}
	id := c.Params("id")
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()
	resp, err := cli.ContainerCommit(ctx, id, container.CommitOptions{
		Reference: ref,
		Comment:   strings.TrimSpace(body.Message),
		Author:    strings.TrimSpace(body.Author),
		Pause:     pause,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": resp.ID, "reference": ref},
		"message": "Image created from container",
	}))
}

func calcCPUPercent(pre, cur map[string]any) float64 {
	cpu, _ := cur["cpu_usage"].(map[string]any)
	precpu, _ := pre["cpu_usage"].(map[string]any)
	if cpu == nil || precpu == nil {
		return 0
	}
	total, _ := cpu["total_usage"].(float64)
	pretotal, _ := precpu["total_usage"].(float64)
	system, _ := cur["system_cpu_usage"].(float64)
	presystem, _ := pre["system_cpu_usage"].(float64)
	online, _ := cur["online_cpus"].(float64)
	if online == 0 {
		if percpu, ok := cpu["percpu_usage"].([]any); ok {
			online = float64(len(percpu))
		}
	}
	cpuDelta := total - pretotal
	sysDelta := system - presystem
	if cpuDelta > 0 && sysDelta > 0 && online > 0 {
		return (cpuDelta / sysDelta) * online * 100.0
	}
	return 0
}

func memUsageBytes(mem map[string]any) float64 {
	usage, _ := mem["usage"].(float64)
	if stats, ok := mem["stats"].(map[string]any); ok {
		// Prefer working set when available (cgroup v1 cache / cgroup v2 inactive_file).
		if cache, ok := stats["total_inactive_file"].(float64); ok && usage >= cache {
			return usage - cache
		}
		if cache, ok := stats["inactive_file"].(float64); ok && usage >= cache {
			return usage - cache
		}
		if cache, ok := stats["cache"].(float64); ok && usage >= cache {
			return usage - cache
		}
	}
	return usage
}

func blkioRW(raw map[string]any) (readBytes, writeBytes float64) {
	blkio, ok := raw["blkio_stats"].(map[string]any)
	if !ok {
		return 0, 0
	}
	entries, ok := blkio["io_service_bytes_recursive"].([]any)
	if !ok || len(entries) == 0 {
		// Some engines expose the non-recursive field instead.
		entries, _ = blkio["io_service_bytes"].([]any)
	}
	for _, e := range entries {
		row, ok := e.(map[string]any)
		if !ok {
			continue
		}
		op, _ := row["op"].(string)
		val, _ := row["value"].(float64)
		switch strings.ToLower(op) {
		case "read":
			readBytes += val
		case "write":
			writeBytes += val
		}
	}
	return readBytes, writeBytes
}
