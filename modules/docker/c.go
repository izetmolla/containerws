package docker

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/docker/containers"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/izetmolla/containerws/modules/docker/images"
	"github.com/izetmolla/containerws/modules/docker/networks"
	"github.com/izetmolla/containerws/modules/docker/stacks"
	"github.com/izetmolla/containerws/modules/docker/templates"
	"github.com/izetmolla/containerws/modules/docker/volumes"
	softwareservice "github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"gorm.io/gorm"
)

// SetupRoutesAPI mounts /api/docker (environments, containers, images, networks, volumes, stacks, templates, engine).
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/docker")
	environments.SetupRoutesAPI(api.Group("/environments"), appClients)
	containers.SetupRoutesAPI(api.Group("/containers"), appClients)
	images.SetupRoutesAPI(api.Group("/images"), appClients)
	networks.SetupRoutesAPI(api.Group("/networks"), appClients)
	volumes.SetupRoutesAPI(api.Group("/volumes"), appClients)
	stacks.SetupRoutesAPI(api.Group("/stacks"), appClients)
	templates.SetupRoutesAPI(api.Group("/templates"), appClients)

	cc := &engineController{app: appClients}
	api.Get("/engine/status", cc.StatusAPI)
	api.Post("/engine/start", cc.StartAPI)
	api.Post("/engine/stop", cc.StopAPI)
	api.Post("/engine/restart", cc.RestartAPI)
}

type engineController struct {
	app *config.AppClients
}

func (cc *engineController) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	db := cc.app.DB()
	_ = environments.EnsureDefaultLocal(db)

	out := fiber.Map{
		"reachable": false,
		"sock":      dockerclient.SockPath(),
		"engine":    cc.engineSoftwareStatus(db),
	}

	env, envErr := environments.Resolve(db, c.Query("environment_id"))
	if envErr != nil {
		// Disabled / missing env should still report softwares status so the UI can recover.
		out["error"] = envErr.Error()
		out["env_disabled"] = strings.Contains(strings.ToLower(envErr.Error()), "disabled")
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}

	if env != nil {
		out["environment"] = fiber.Map{
			"id":          env.ID,
			"name":        env.Name,
			"conn_type":   env.ConnType,
			"host_url":    env.HostURL,
			"is_default":  env.IsDefault,
			"is_disabled": env.IsDisabled,
		}
		if env.ConnType == models.DockerConnUnix {
			out["sock"] = env.SocketPath
		}
	}

	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		out["error"] = err.Error()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}
	ping, err := cli.Ping(ctx)
	if err != nil {
		dockerclient.Reset()
		out["error"] = err.Error()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}
	out["reachable"] = true
	out["api_version"] = ping.APIVersion
	out["os_type"] = ping.OSType
	out["experimental"] = ping.Experimental

	info, err := cli.Info(ctx)
	if err == nil {
		out["name"] = info.Name
		out["server_version"] = info.ServerVersion
		out["containers"] = info.Containers
		out["containers_running"] = info.ContainersRunning
		out["images"] = info.Images
		out["driver"] = info.Driver
		out["architecture"] = info.Architecture
		out["ncpu"] = info.NCPU
		out["mem_total"] = info.MemTotal
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

func (cc *engineController) StartAPI(c fiber.Ctx) error {
	return cc.controlEngine(c, "start")
}

func (cc *engineController) StopAPI(c fiber.Ctx) error {
	return cc.controlEngine(c, "stop")
}

func (cc *engineController) RestartAPI(c fiber.Ctx) error {
	return cc.controlEngine(c, "restart")
}

func (cc *engineController) controlEngine(c fiber.Ctx, action string) error {
	r := cc.app.Render()
	db := cc.app.DB()
	sw, err := findDockerEngineSoftware(db)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("Docker Engine is not in the Softwares catalog")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if !softwareservice.CanControl(*sw) {
		return r.Api(c, r.WithError(errors.New("Docker Engine is not marked controllable")), r.WithStatus(fiber.StatusBadRequest))
	}
	st, err := softwareservice.ControlSoftware(action, *sw)
	if err != nil {
		status := fiber.StatusBadRequest
		msg := err.Error()
		if strings.Contains(msg, "systemctl not available") ||
			strings.Contains(msg, "systemd not running") {
			status = fiber.StatusServiceUnavailable
		}
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorData(fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"status":      st,
			"engine":      cc.engineSoftwareStatus(db),
		}))
	}
	dockerclient.Reset()
	_ = environments.EnsureDefaultLocal(db)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"action":      action,
			"status":      st,
			"engine":      cc.engineSoftwareStatus(db),
		},
		"message": "Docker Engine " + action + " requested",
	}))
}

func (cc *engineController) engineSoftwareStatus(db *gorm.DB) fiber.Map {
	out := fiber.Map{
		"binary_present": false,
		"running":        false,
		"installed":      false,
		"can_control":    false,
		"software_id":    "",
		"software_name":  "Docker Engine",
		"service":        softwareservice.Status{Overall: "unmanaged", Managed: false},
	}
	if path, err := exec.LookPath("docker"); err == nil && path != "" {
		out["binary_present"] = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if cli, err := dockerclient.Client(); err == nil {
		if _, err := cli.Ping(ctx); err == nil {
			out["running"] = true
			out["binary_present"] = true
		}
	}
	if db == nil {
		return out
	}
	sw, err := findDockerEngineSoftware(db)
	if err != nil {
		return out
	}
	out["software_id"] = sw.ID
	out["software_name"] = sw.Name
	units := []string(sw.ServiceUnits)
	out["can_control"] = softwareservice.CanControl(*sw)
	out["control_backend"] = sw.ControlBackend
	out["service"] = softwareservice.ProbeUnits(units)
	row, err := models.GetSoftwareInstalled(db, sw.ID)
	if err == nil && row != nil && !row.Uninstalled {
		out["installed"] = true
	}
	if overall, ok := out["service"].(softwareservice.Status); ok && overall.Overall == "running" {
		out["running"] = true
	}
	return out
}

func findDockerEngineSoftware(db *gorm.DB) (*models.Software, error) {
	if db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var sw models.Software
	err := db.Where("name = ? AND is_active = ?", "Docker Engine", true).First(&sw).Error
	if err != nil {
		return nil, err
	}
	return &sw, nil
}
