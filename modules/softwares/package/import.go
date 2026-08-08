package softwarespackage

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwarepkg"
	"gorm.io/gorm"
)

type importBody struct {
	Name      string `json:"name"`
	PackageID string `json:"package_id"`
	Ref       string `json:"ref"`
}

func (cc *controller) ImportSoftwareAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body importBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return r.Api(c, r.WithError(errors.New("name is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	pkg, err := resolveRegistry(db, strings.TrimSpace(body.PackageID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("registry not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	res, err := softwarepkg.Import(ctx, db, *pkg, name, &softwarepkg.ImportOptions{
		Ref: strings.TrimSpace(body.Ref),
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	softwarepkg.InvalidateCatalogCache()

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"software":     res.Software,
			"version":      res.Version,
			"created_sw":   res.CreatedSW,
			"created_ver":  res.CreatedVer,
			"install_url":  res.InstallURL,
			"meta_url":     res.MetaURL,
			"tried_paths":  res.TriedPaths,
		},
		"message": "Imported " + res.Software.Name + " " + res.Version.Version,
	}))
}

func resolveRegistry(db *gorm.DB, packageID string) (*models.SoftwarePackage, error) {
	return ResolveRegistry(db, packageID)
}

// ResolveRegistry returns a SoftwarePackage registry row by id, or the default
// when packageID is empty.
func ResolveRegistry(db *gorm.DB, packageID string) (*models.SoftwarePackage, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
		return nil, err
	}
	if packageID != "" {
		var row models.SoftwarePackage
		if err := db.Where("id = ?", packageID).First(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}
	var rows []models.SoftwarePackage
	if err := db.Order("created_at DESC").Limit(2).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("no software package registry configured — create one via POST /softwares/package/registry")
	}
	if len(rows) > 1 {
		for i := range rows {
			if softwarepkg.SameGitHubRepo(rows[i].PackageURL, softwarepkg.DefaultRegistryURL) {
				return &rows[i], nil
			}
		}
		return nil, errors.New("multiple registries configured — pass package_id")
	}
	return &rows[0], nil
}
