package environments

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/packages/environments"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /settings/environments.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.ListEnvironmentsAPI)
	api.Post("/", cc.CreateEnvironmentAPI)
	api.Get("/:id", cc.GetEnvironmentAPI)
	api.Put("/:id", cc.UpdateEnvironmentAPI)
	api.Delete("/:id", cc.DeleteEnvironmentAPI)
}

type createBody struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Group      string `json:"group"`
	IsSecret   bool   `json:"is_secret"`
	IsDisabled bool   `json:"is_disabled"`
	IsTextarea bool   `json:"is_textarea"`
}

type updateBody struct {
	Name       *string `json:"name"`
	Value      *string `json:"value"`
	Group      *string `json:"group"`
	IsSecret   *bool   `json:"is_secret"`
	IsDisabled *bool   `json:"is_disabled"`
	IsTextarea *bool   `json:"is_textarea"`
}

func (cc *controller) envMgr() (*environments.Environments, error) {
	if cc.app == nil || cc.app.Environments() == nil {
		return nil, errors.New("environments manager unavailable")
	}
	return cc.app.Environments(), nil
}

func envPublic(row environments.OsEnvironment, revealSecret bool) fiber.Map {
	value := row.Value
	secretMasked := false
	if row.IsSecret && !revealSecret {
		if value != "" {
			secretMasked = true
		}
		value = ""
	}
	return fiber.Map{
		"id":            row.ID,
		"name":          row.Name,
		"value":         value,
		"group":         row.Group,
		"source":        row.Source,
		"is_core":       row.IsCore,
		"is_secret":     row.IsSecret,
		"is_disabled":   row.IsDisabled,
		"is_textarea":   row.IsTextarea,
		"secret_masked": secretMasked,
		"created_at":    row.CreatedAt,
		"updated_at":    row.UpdatedAt,
	}
}

func mapEnvErr(err error) (int, string) {
	switch {
	case errors.Is(err, environments.ErrNotFound):
		return fiber.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, environments.ErrNameConflict):
		return fiber.StatusConflict, "NAME_CONFLICT"
	case errors.Is(err, environments.ErrCoreNameReserved):
		return fiber.StatusBadRequest, "CORE_RESERVED"
	case errors.Is(err, environments.ErrCoreNotDeletable):
		return fiber.StatusBadRequest, "CORE_NOT_DELETABLE"
	case errors.Is(err, environments.ErrInvalidName):
		return fiber.StatusBadRequest, "INVALID_NAME"
	default:
		return fiber.StatusBadRequest, "ENV_ERROR"
	}
}

func (cc *controller) ListEnvironmentsAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	mgr, err := cc.envMgr()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	group := strings.TrimSpace(c.Query("group"))
	rows, groups, err := mgr.ListEnvironments(ctx, group)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		out = append(out, envPublic(row, false))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":        out,
		"groups":      groups,
		"core_names":  environments.CoreNames(),
		"ungrouped":   environments.UngroupedGroupFilter(),
	}))
}

func (cc *controller) GetEnvironmentAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	mgr, err := cc.envMgr()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	id := strings.TrimSpace(c.Params("id"))
	row, err := mgr.GetEnvironment(ctx, id)
	if err != nil {
		status, code := mapEnvErr(err)
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorCode(code))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": envPublic(row, true),
	}))
}

func (cc *controller) CreateEnvironmentAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	mgr, err := cc.envMgr()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	row, err := mgr.CreateEnvironment(ctx, environments.CreateEnvironmentInput{
		Name:       body.Name,
		Value:      body.Value,
		Group:      body.Group,
		IsSecret:   body.IsSecret,
		IsDisabled: body.IsDisabled,
		IsTextarea: body.IsTextarea,
	})
	if err != nil {
		status, code := mapEnvErr(err)
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorCode(code))
	}
	_ = mgr.NotifyEnvironmentChange(ctx, row.ID, environments.WatcherActionUpsert)
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    envPublic(row, true),
		"message": "Environment variable created",
	}))
}

func (cc *controller) UpdateEnvironmentAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	mgr, err := cc.envMgr()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	id := strings.TrimSpace(c.Params("id"))
	var body updateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	row, err := mgr.UpdateEnvironment(ctx, id, environments.UpdateEnvironmentInput{
		Name:       body.Name,
		Value:      body.Value,
		Group:      body.Group,
		IsSecret:   body.IsSecret,
		IsDisabled: body.IsDisabled,
		IsTextarea: body.IsTextarea,
	})
	if err != nil {
		status, code := mapEnvErr(err)
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorCode(code))
	}
	_ = mgr.NotifyEnvironmentChange(ctx, row.ID, environments.WatcherActionUpsert)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    envPublic(row, true),
		"message": "Environment variable updated",
	}))
}

func (cc *controller) DeleteEnvironmentAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	mgr, err := cc.envMgr()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	id := strings.TrimSpace(c.Params("id"))
	if err := mgr.DeleteEnvironment(ctx, id); err != nil {
		status, code := mapEnvErr(err)
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorCode(code))
	}
	_ = mgr.NotifyEnvironmentChange(ctx, id, environments.WatcherActionDelete)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "Environment variable deleted",
		"data":    fiber.Map{"id": id},
	}))
}
