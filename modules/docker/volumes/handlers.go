package volumes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/volume"
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
	single.Get("/:id", cc.InspectAPI)
	single.Delete("/:id", cc.RemoveAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

type volumeRow struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at,omitempty"`
	Scope      string            `json:"scope"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	resp, err := cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]volumeRow, 0, len(resp.Volumes))
	for _, it := range resp.Volumes {
		if it == nil {
			continue
		}
		rows = append(rows, volumeRow{
			Name:       it.Name,
			Driver:     it.Driver,
			Mountpoint: it.Mountpoint,
			CreatedAt:  it.CreatedAt,
			Scope:      it.Scope,
			Labels:     it.Labels,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createBody struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Labels map[string]string `json:"labels"`
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
	opts := volume.CreateOptions{
		Name:   strings.TrimSpace(body.Name),
		Driver: strings.TrimSpace(body.Driver),
		Labels: body.Labels,
	}
	if opts.Driver == "" {
		opts.Driver = "local"
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	vol, err := cli.VolumeCreate(ctx, opts)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    vol,
		"message": "Volume created",
	}))
}

func (cc *controller) InspectAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	vol, err := cli.VolumeInspect(ctx, c.Params("id"))
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": vol}))
}

func (cc *controller) RemoveAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	force := c.Query("force", "0") == "1"
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.VolumeRemove(ctx, id, force); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"name": id},
		"message": "Volume removed",
	}))
}
