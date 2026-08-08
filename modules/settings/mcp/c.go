package mcp

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	mcppkg "github.com/izetmolla/containerws/modules/mcp"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /api/settings/mcp.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/standalone", cc.GetStandaloneAPI)
	api.Put("/standalone", cc.UpdateStandaloneAPI)
	api.Get("/addresses", cc.ListBindAddressesAPI)

	api.Get("/keys", cc.ListKeysAPI)
	api.Post("/keys", cc.CreateKeyAPI)
	api.Get("/keys/:id", cc.GetKeyAPI)
	api.Put("/keys/:id", cc.UpdateKeyAPI)
	api.Post("/keys/:id/revoke", cc.RevokeKeyAPI)
	api.Delete("/keys/:id", cc.DeleteKeyAPI)
}

type standaloneBody struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

func (cc *controller) GetStandaloneAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	st := mcppkg.GetStandaloneStatus(cc.app)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": st}))
}

func (cc *controller) ListBindAddressesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	addrs := mcppkg.ListBindAddresses()
	// Prefer the currently configured address so a saved IP stays selectable.
	st := mcppkg.GetStandaloneStatus(cc.app)
	addrs = mcppkg.EnsureBindAddressOption(addrs, st.Address)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": addrs,
	}))
}

func (cc *controller) UpdateStandaloneAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body standaloneBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	st, err := mcppkg.ApplyStandaloneConfig(cc.app, body.Enabled, body.Address, body.Port)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithData(fiber.Map{
			"data": st,
		}))
	}
	msg := "Standalone MCP disabled"
	if st.Enabled && st.Running {
		msg = "Standalone MCP listening on " + st.PublicURL
	} else if st.Enabled {
		msg = "Standalone MCP enabled"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": msg,
	}))
}

type createKeyBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ExpiresIn   int    `json:"expires_in_days"` // 0 = never
}

type updateKeyBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

func keyPublic(row models.MCPKey, includeSecret bool) fiber.Map {
	prefix := row.KeyPrefix
	suffix := ""
	if row.Key != "" {
		prefix = models.MCPKeyPrefix(row.Key)
		suffix = models.MCPKeySuffix(row.Key)
	}
	out := fiber.Map{
		"id":           row.ID,
		"name":         row.Name,
		"description":  row.Description,
		"key_prefix":   prefix,
		"key_suffix":   suffix,
		"status":       row.Status,
		"expires_at":   nil,
		"last_used_at": nil,
		"last_used_ip": row.LastUsedIP,
		"created_by":   row.CreatedBy,
		"created_at":   row.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":   row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.ExpiresAt != nil {
		out["expires_at"] = row.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if row.LastUsedAt != nil {
		out["last_used_at"] = row.LastUsedAt.UTC().Format(time.RFC3339)
	}
	if includeSecret {
		out["key"] = row.Key
	}
	return out
}

func (cc *controller) ListKeysAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var rows []models.MCPKey
	if err := db.WithContext(c.Context()).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		out = append(out, keyPublic(row, false))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

// GetKeyAPI returns one key including the full secret (for copy/reveal in Settings).
func (cc *controller) GetKeyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("id required")), r.WithStatus(fiber.StatusBadRequest))
	}
	var row models.MCPKey
	if err := db.WithContext(c.Context()).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("key not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": keyPublic(row, true),
	}))
}

func (cc *controller) CreateKeyAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body createKeyBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "MCP key"
	}
	row := models.MCPKey{
		Name:        name,
		Description: strings.TrimSpace(body.Description),
		Status:      models.StatusActive,
	}
	if body.ExpiresIn > 0 {
		t := time.Now().UTC().AddDate(0, 0, body.ExpiresIn)
		row.ExpiresAt = &t
	}
	if auth := cc.app.Authorization(); auth != nil {
		if u, err := auth.User(c, ctx, false); err == nil && u != nil {
			row.CreatedBy = u.UserID
		}
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    keyPublic(row, true),
		"message": "MCP key created — copy and store it securely",
	}))
}

func (cc *controller) UpdateKeyAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("id required")), r.WithStatus(fiber.StatusBadRequest))
	}
	var row models.MCPKey
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("key not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body updateKeyBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	updates := map[string]any{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			name = row.Name
		}
		updates["name"] = name
	}
	if body.Description != nil {
		updates["description"] = strings.TrimSpace(*body.Description)
	}
	if body.Status != nil {
		st := models.Status(strings.TrimSpace(*body.Status))
		switch st {
		case models.StatusActive, models.StatusInactive:
			updates["status"] = st
		default:
			return r.Api(c, r.WithError(errors.New("status must be active or inactive")), r.WithStatus(fiber.StatusBadRequest))
		}
	}
	if len(updates) == 0 {
		return r.Api(c, r.WithError(errors.New("no fields to update")), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Where("id = ?", id).First(&row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    keyPublic(row, false),
		"message": "MCP key updated",
	}))
}

func (cc *controller) RevokeKeyAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	var row models.MCPKey
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("key not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := db.WithContext(ctx).Model(&row).Update("status", models.StatusInactive).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).Where("id = ?", id).First(&row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    keyPublic(row, false),
		"message": "MCP key revoked",
	}))
}

func (cc *controller) DeleteKeyAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	res := db.WithContext(ctx).Where("id = ?", id).Delete(&models.MCPKey{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithError(errors.New("key not found")), r.WithStatus(fiber.StatusNotFound))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "MCP key deleted",
		"data":    fiber.Map{"id": id},
	}))
}
