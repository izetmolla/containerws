package softwarespackage

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

func (cc *controller) GetPackageAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("software id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	for i := range versions {
		versions[i].CanUninstall = strings.TrimSpace(versions[i].UninstallScript) != ""
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"software": sw,
			"versions": versions,
		},
	}))
}
