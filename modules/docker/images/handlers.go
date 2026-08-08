package images

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
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
	single.Post("/", cc.PullAPI)
	single.Post("/prune", cc.PruneAPI)
	single.Get("/:id", cc.InspectAPI)
	single.Delete("/:id", cc.RemoveAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

type imageRow struct {
	ID          string            `json:"id"`
	ShortID     string            `json:"short_id"`
	RepoTags    []string          `json:"repo_tags"`
	RepoDigests []string          `json:"repo_digests,omitempty"`
	Created     int64             `json:"created"`
	Size        int64             `json:"size"`
	Containers  int64             `json:"containers"`
	InUse       bool              `json:"in_use"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	items, err := cli.ImageList(ctx, image.ListOptions{All: true, SharedSize: true})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]imageRow, 0, len(items))
	for _, it := range items {
		id := it.ID
		short := strings.TrimPrefix(id, "sha256:")
		if len(short) > 12 {
			short = short[:12]
		}
		tags := it.RepoTags
		if len(tags) == 0 {
			tags = []string{"<none>:<none>"}
		}
		containers := max(it.Containers, 0)
		rows = append(rows, imageRow{
			ID:          id,
			ShortID:     short,
			RepoTags:    tags,
			RepoDigests: it.RepoDigests,
			Created:     it.Created,
			Size:        it.Size,
			Containers:  containers,
			InUse:       containers > 0,
			Labels:      it.Labels,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type pullBody struct {
	Image  string `json:"image"`
	Tag    string `json:"tag"`
	Force  bool   `json:"force"`   // re-pull from registry even if present locally
	RePull bool   `json:"re_pull"` // alias of force
}

func (cc *controller) PullAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var body pullBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	ref := strings.TrimSpace(body.Image)
	if ref == "" {
		return r.Api(c, r.WithError(fmt.Errorf("image is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	tag := strings.TrimSpace(body.Tag)
	if tag != "" && !strings.Contains(ref, ":") && !strings.Contains(ref, "@") {
		ref = ref + ":" + tag
	}
	force := body.Force || body.RePull
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Minute)
	defer cancel()

	if !force {
		if insp, _, err := cli.ImageInspectWithRaw(ctx, ref); err == nil {
			return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
				"data":    insp,
				"message": "Image already present locally",
				"skipped": true,
			}))
		}
	}

	rd, err := cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	defer rd.Close()
	dec := json.NewDecoder(rd)
	var last map[string]any
	for {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		last = msg
		if errMsg, ok := msg["error"].(string); ok && errMsg != "" {
			return cc.respondErr(c, fmt.Errorf("%s", errMsg))
		}
	}
	msg := "Image pulled"
	if force {
		msg = "Image re-pulled from registry"
	}
	insp, _, err := cli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    fiber.Map{"ref": ref, "last": last},
			"message": msg,
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    insp,
		"message": msg,
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
	insp, _, err := cli.ImageInspectWithRaw(ctx, c.Params("id"))
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": insp}))
}

func (cc *controller) RemoveAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	id := c.Params("id")
	force := c.Query("force", "0") == "1"
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	items, err := cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    items,
		"message": "Image removed",
	}))
}

func (cc *controller) PruneAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, err := envcli.Engine(cc.app, c)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Minute)
	defer cancel()
	report, err := cli.ImagesPrune(ctx, filters.NewArgs())
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    report,
		"message": "Unused images pruned",
	}))
}
