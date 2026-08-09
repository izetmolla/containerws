package options

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /api/settings/options.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.ListOptionsAPI)
	api.Post("/", cc.CreateOptionAPI)
	api.Get("/:id", cc.GetOptionAPI)
	api.Put("/:id", cc.UpdateOptionAPI)
	api.Delete("/:id", cc.DeleteOptionAPI)
}

type createBody struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type updateBody struct {
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

func optionPublic(row models.Option) fiber.Map {
	return fiber.Map{
		"id":         row.ID,
		"name":       row.Name,
		"value":      row.Value,
		"group":      optionGroup(row.Name),
		"created_at": row.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func optionGroup(name string) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(n, "VNC_"):
		return "vnc"
	case strings.HasPrefix(n, "CODESERVER_"):
		return "codeserver"
	case strings.HasPrefix(n, "MCP_"):
		return "mcp"
	case strings.HasPrefix(n, "WORKSPACE_"):
		return "workspace"
	default:
		return ""
	}
}

func normalizeOptionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > 255 {
		return "", errors.New("name is too long")
	}
	return name, nil
}

func (cc *controller) ensureKnownOptions(db *gorm.DB) {
	known := []struct {
		name  string
		value string
	}{
		{models.OptionVNCInstalled, "false"},
		{models.OptionVNCSoftwarePresent, "false"},
		{models.OptionCodeserverInstalled, "false"},
		{models.OptionCodeserverSoftwarePresent, "false"},
		{models.OptionBrewModuleEnabled, "false"},
		{models.OptionLocalhostAutoLogin, "false"},
	}
	for _, item := range known {
		if _, ok, err := models.GetOption(db, item.name); err == nil && !ok {
			_ = models.SetOption(db, item.name, item.value)
		}
	}
}

func (cc *controller) ListOptionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	cc.ensureKnownOptions(db)

	var rows []models.Option
	if err := db.WithContext(c.Context()).
		Order("name ASC").
		Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out := make([]fiber.Map, 0, len(rows))
	groupsSet := map[string]struct{}{}
	for _, row := range rows {
		out = append(out, optionPublic(row))
		if g := optionGroup(row.Name); g != "" {
			groupsSet[g] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupsSet))
	for g := range groupsSet {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":   out,
		"groups": groups,
	}))
}

func (cc *controller) GetOptionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	var row models.Option
	if err := db.WithContext(c.Context()).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("option not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": optionPublic(row),
	}))
}

func (cc *controller) CreateOptionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name, err := normalizeOptionName(body.Name)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_NAME"))
	}
	var count int64
	if err := db.WithContext(c.Context()).Model(&models.Option{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if count > 0 {
		return r.Api(c, r.WithError(errors.New("option already exists")), r.WithStatus(fiber.StatusConflict), r.WithErrorCode("NAME_CONFLICT"))
	}
	row := models.Option{Name: name, Value: body.Value}
	if err := db.WithContext(c.Context()).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    optionPublic(row),
		"message": "Option created",
	}))
}

func (cc *controller) UpdateOptionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	var row models.Option
	if err := db.WithContext(c.Context()).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("option not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body updateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	updates := map[string]any{}
	if body.Name != nil {
		name, err := normalizeOptionName(*body.Name)
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_NAME"))
		}
		var count int64
		if err := db.WithContext(c.Context()).
			Model(&models.Option{}).
			Where("name = ? AND id <> ?", name, id).
			Count(&count).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		if count > 0 {
			return r.Api(c, r.WithError(errors.New("option already exists")), r.WithStatus(fiber.StatusConflict), r.WithErrorCode("NAME_CONFLICT"))
		}
		updates["name"] = name
	}
	if body.Value != nil {
		updates["value"] = *body.Value
	}
	if len(updates) == 0 {
		return r.Api(c, r.WithError(errors.New("no fields to update")), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := db.WithContext(c.Context()).Model(&row).Updates(updates).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(c.Context()).Where("id = ?", id).First(&row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    optionPublic(row),
		"message": "Option updated",
	}))
}

func (cc *controller) DeleteOptionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	res := db.WithContext(c.Context()).Where("id = ?", id).Delete(&models.Option{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithError(errors.New("option not found")), r.WithStatus(fiber.StatusNotFound))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "Option deleted",
		"data":    fiber.Map{"id": id},
	}))
}
