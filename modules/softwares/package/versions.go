package softwarespackage

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type versionBody struct {
	Version          string `json:"version"`
	IsLatest         *bool  `json:"is_latest"`
	InstallScript    string `json:"install_script"`
	UninstallScript  string `json:"uninstall_script"`
	UpgradeScript    string `json:"upgrade_script"`
	CustomScript     string `json:"custom_script"`
	OS               string `json:"os"`
	OsVersion        string `json:"os_version"`
	Distro           string `json:"distro"`
	DistroID         string `json:"distro_id"`
	DistroVersion    string `json:"distro_version"`
	Arch             string `json:"arch"`
	Platform         string `json:"platform"`
	PackageFamily    string `json:"package_family"`
	Kernel           string `json:"kernel"`
	Virtualization   string `json:"virtualization"`
	ContainerRuntime string `json:"container_runtime"`
	CloudProvider    string `json:"cloud_provider"`
}

func (cc *controller) CreateVersionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	softwareID := strings.TrimSpace(c.Params("id"))
	if softwareID == "" {
		return r.Api(c, r.WithError(errors.New("software id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body versionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	version := strings.TrimSpace(body.Version)
	if version == "" {
		return r.Api(c, r.WithError(errors.New("version is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	if _, err := gorm.G[models.Software](db).Where("id = ?", softwareID).First(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	isLatest := false
	if body.IsLatest != nil {
		isLatest = *body.IsLatest
	} else {
		var count int64
		_ = db.WithContext(ctx).Model(&models.SoftwareVersion{}).
			Where("software_id = ?", softwareID).
			Count(&count).Error
		isLatest = count == 0
	}

	ver := models.SoftwareVersion{
		SoftwareID:       softwareID,
		Version:          version,
		IsLatest:         isLatest,
		InstallScript:    body.InstallScript,
		UninstallScript:  body.UninstallScript,
		UpgradeScript:    body.UpgradeScript,
		CustomScript:     body.CustomScript,
		OS:               strings.TrimSpace(body.OS),
		OsVersion:        strings.TrimSpace(body.OsVersion),
		Distro:           strings.TrimSpace(body.Distro),
		DistroID:         strings.TrimSpace(body.DistroID),
		DistroVersion:    strings.TrimSpace(body.DistroVersion),
		Arch:             strings.TrimSpace(body.Arch),
		Platform:         strings.TrimSpace(body.Platform),
		PackageFamily:    strings.TrimSpace(body.PackageFamily),
		Kernel:           strings.TrimSpace(body.Kernel),
		Virtualization:   strings.TrimSpace(body.Virtualization),
		ContainerRuntime: strings.TrimSpace(body.ContainerRuntime),
		CloudProvider:    strings.TrimSpace(body.CloudProvider),
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if ver.IsLatest {
			if err := tx.Model(&models.SoftwareVersion{}).
				Where("software_id = ?", softwareID).
				Update("is_latest", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&ver).Error
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	ver.CanUninstall = strings.TrimSpace(ver.UninstallScript) != ""
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":    ver,
		"message": "Version created",
	}))
}

func (cc *controller) UpdateVersionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	softwareID := strings.TrimSpace(c.Params("id"))
	versionID := strings.TrimSpace(c.Params("versionId"))
	if softwareID == "" || versionID == "" {
		return r.Api(c, r.WithError(errors.New("software id and version id are required")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body versionBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	var ver models.SoftwareVersion
	if err := db.WithContext(ctx).
		Where("id = ? AND software_id = ?", versionID, softwareID).
		First(&ver).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("version not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	if v := strings.TrimSpace(body.Version); v != "" {
		ver.Version = v
	}
	// Scripts are always applied from the body (allow clearing).
	ver.InstallScript = body.InstallScript
	ver.UninstallScript = body.UninstallScript
	ver.UpgradeScript = body.UpgradeScript
	ver.CustomScript = body.CustomScript
	ver.OS = strings.TrimSpace(body.OS)
	ver.OsVersion = strings.TrimSpace(body.OsVersion)
	ver.Distro = strings.TrimSpace(body.Distro)
	ver.DistroID = strings.TrimSpace(body.DistroID)
	ver.DistroVersion = strings.TrimSpace(body.DistroVersion)
	ver.Arch = strings.TrimSpace(body.Arch)
	ver.Platform = strings.TrimSpace(body.Platform)
	ver.PackageFamily = strings.TrimSpace(body.PackageFamily)
	ver.Kernel = strings.TrimSpace(body.Kernel)
	ver.Virtualization = strings.TrimSpace(body.Virtualization)
	ver.ContainerRuntime = strings.TrimSpace(body.ContainerRuntime)
	ver.CloudProvider = strings.TrimSpace(body.CloudProvider)

	makeLatest := body.IsLatest != nil && *body.IsLatest
	if body.IsLatest != nil {
		ver.IsLatest = *body.IsLatest
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if makeLatest {
			if err := tx.Model(&models.SoftwareVersion{}).
				Where("software_id = ? AND id <> ?", softwareID, ver.ID).
				Update("is_latest", false).Error; err != nil {
				return err
			}
			ver.IsLatest = true
		}
		return tx.Save(&ver).Error
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	ver.CanUninstall = strings.TrimSpace(ver.UninstallScript) != ""
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    ver,
		"message": "Version updated",
	}))
}

func (cc *controller) DeleteVersionAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	softwareID := strings.TrimSpace(c.Params("id"))
	versionID := strings.TrimSpace(c.Params("versionId"))
	if softwareID == "" || versionID == "" {
		return r.Api(c, r.WithError(errors.New("software id and version id are required")), r.WithStatus(fiber.StatusBadRequest))
	}

	var ver models.SoftwareVersion
	if err := db.WithContext(ctx).
		Where("id = ? AND software_id = ?", versionID, softwareID).
		First(&ver).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("version not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	wasLatest := ver.IsLatest
	if err := db.WithContext(ctx).Delete(&ver).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	if wasLatest {
		var next models.SoftwareVersion
		err := db.WithContext(ctx).
			Where("software_id = ?", softwareID).
			Order("created_at DESC").
			First(&next).Error
		if err == nil {
			_ = db.WithContext(ctx).Model(&next).Update("is_latest", true).Error
		}
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message": "Version deleted",
	}))
}
