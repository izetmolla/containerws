package softwarespackage

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwarepkg"
	"gorm.io/gorm"
)

type registryBody struct {
	PackageURL string `json:"package_url"`
	Token      string `json:"token"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func (cc *controller) ListRegistriesAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	var rows []models.SoftwarePackage
	if err := db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	// Never echo secrets in list — blank password; mask token presence.
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		out = append(out, registryPublic(row))
	}
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
	}))
}

func (cc *controller) CreateRegistryAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body registryBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	url := strings.TrimSpace(body.PackageURL)
	if url == "" {
		return r.Api(c, r.WithError(errors.New("package_url is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	row := models.SoftwarePackage{
		PackageURL: url,
		Token:      strings.TrimSpace(body.Token),
		Username:   strings.TrimSpace(body.Username),
		Password:   body.Password,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	softwarepkg.InvalidateCatalogCache()
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    registryPublic(row),
		"message": "Registry created",
	}))
}

func (cc *controller) UpdateRegistryAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("registryId"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("registry id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body registryBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	var row models.SoftwarePackage
	if err := db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("registry not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	if u := strings.TrimSpace(body.PackageURL); u != "" {
		row.PackageURL = u
	}
	// Empty token/username leave unchanged; send explicit clear via space? Use pointer-less: always apply when key present.
	// For simplicity: if body fields are non-empty, update; password always applied when non-empty.
	if strings.TrimSpace(body.Token) != "" {
		row.Token = strings.TrimSpace(body.Token)
	}
	if body.Username != "" {
		row.Username = strings.TrimSpace(body.Username)
	}
	if body.Password != "" {
		row.Password = body.Password
	}

	if err := db.WithContext(ctx).Save(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	softwarepkg.InvalidateCatalogCache()
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    registryPublic(row),
		"message": "Registry updated",
	}))
}

func (cc *controller) DeleteRegistryAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("registryId"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("registry id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	res := db.WithContext(ctx).Where("id = ?", id).Delete(&models.SoftwarePackage{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("registry not found")), r.WithStatus(fiber.StatusNotFound))
	}
	softwarepkg.InvalidateCatalogCache()
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "Registry deleted",
	}))
}

func registryPublic(row models.SoftwarePackage) fiber.Map {
	return fiber.Map{
		"id":           row.ID,
		"package_url":  row.PackageURL,
		"username":     row.Username,
		"has_token":    strings.TrimSpace(row.Token) != "",
		"has_password": row.Password != "",
		"created_at":   row.CreatedAt,
		"updated_at":   row.UpdatedAt,
	}
}
