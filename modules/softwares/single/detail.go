package single

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	softwarespackage "github.com/izetmolla/containerws/modules/softwares/package"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

func (cc *controller) GetSoftwareSingleAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := c.Params("id")
	if id == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("id", "Software id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	// Default: pull latest package.json + install.json from the GitHub registry
	// when this software came from (or can be matched in) a remote registry.
	syncRemote := true
	if v := strings.ToLower(strings.TrimSpace(c.Query("sync"))); v == "0" || v == "false" || v == "no" {
		syncRemote = false
	}

	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	synced := false
	syncErr := ""
	if syncRemote {
		if updated, serr := syncSoftwareFromRemote(ctx, db, &sw); serr != nil {
			// Soft-fail: still return local detail when registry is unreachable.
			log.Printf("softwares single: sync %s: %v", sw.Name, serr)
			syncErr = serr.Error()
		} else if updated {
			synced = true
			if reloaded, rerr := gorm.G[models.Software](db).Where("id = ?", sw.ID).First(ctx); rerr == nil {
				sw = reloaded
			}
		}
	}

	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	installRow, err := models.GetSoftwareInstalled(db, sw.ID)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	var latest *models.SoftwareVersion
	var installed *models.SoftwareVersion
	if len(versions) > 0 {
		v := versions[0]
		latest = &v
	}

	installedVersionID := ""
	userUninstalled := false
	if installRow != nil {
		installedVersionID = installRow.VersionID
		userUninstalled = installRow.Uninstalled
	}

	for i := range versions {
		versions[i].IsInstalled = !userUninstalled && versions[i].ID == installedVersionID && installedVersionID != ""
		if latest != nil {
			versions[i].HasUpdate = versions[i].IsInstalled && models.HasSoftwareUpdate(versions[i].ID, latest.ID)
		} else {
			versions[i].HasUpdate = false
		}
		if versions[i].IsInstalled {
			softwaresync.ApplyOsMissing(&versions[i])
			iv := versions[i]
			installed = &iv
		}
	}

	isInstalled := installRow != nil && !userUninstalled
	hasUpdate := false
	osMissing := false
	if !userUninstalled {
		if installed != nil {
			osMissing = installed.OsMissing
		} else if installRow != nil {
			osMissing = softwaresync.IsOsMissingSoftware(sw.ID) || softwaresync.IsOsMissing(installRow.VersionID)
		}
		if installed != nil && latest != nil {
			hasUpdate = models.HasSoftwareUpdate(installed.ID, latest.ID)
		} else if installRow != nil && latest != nil {
			hasUpdate = models.HasSoftwareUpdate(installRow.VersionID, latest.ID)
		}
	}

	canUninstall := false
	if isInstalled {
		if installed != nil {
			canUninstall = strings.TrimSpace(installed.UninstallScript) != ""
		}
		if !canUninstall {
			for i := range versions {
				if strings.TrimSpace(versions[i].UninstallScript) != "" {
					canUninstall = true
					break
				}
			}
		}
	}

	units := []string(sw.ServiceUnits)
	canControl := service.CanControl(sw) && isInstalled
	var serviceStatus *service.Status
	if canControl {
		st := service.ProbeUnits(units)
		serviceStatus = &st
	}

	fromRemote := strings.TrimSpace(sw.RegistryPackageID) != "" || strings.TrimSpace(sw.RegistrySlug) != ""

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"software":           sw,
			"versions":           versions,
			"latest_version":     latest,
			"installed_version":  installed,
			"is_installed":       isInstalled,
			"uninstalled":        userUninstalled,
			"has_update":         hasUpdate,
			"os_missing":         osMissing,
			"can_uninstall":      canUninstall,
			"can_control":        canControl,
			"service_status":     serviceStatus,
			"from_remote":        fromRemote,
			"synced_from_remote": synced,
			"sync_error":         syncErr,
		},
	}))
}

// syncSoftwareFromRemote re-imports package.json + host install.json into the local DB.
// Skips softwares that are not present in (and not linked to) a remote registry.
func syncSoftwareFromRemote(ctx context.Context, db *gorm.DB, sw *models.Software) (bool, error) {
	if db == nil || sw == nil {
		return false, nil
	}
	pkgID := strings.TrimSpace(sw.RegistryPackageID)
	fetchName := strings.TrimSpace(sw.RegistrySlug)
	if fetchName == "" {
		fetchName = strings.TrimSpace(sw.Name)
	}
	if fetchName == "" {
		return false, nil
	}

	pkg, err := softwarespackage.ResolveRegistry(db, pkgID)
	if err != nil {
		return false, err
	}

	linked := pkgID != "" || strings.TrimSpace(sw.RegistrySlug) != ""
	if !linked {
		items, lerr := softwarepkg.ListRemoteFromPackage(ctx, *pkg, "main", nil)
		if lerr != nil {
			return false, lerr
		}
		found := false
		for _, m := range items {
			if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(sw.Name)) {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	res, err := softwarepkg.Import(ctx, db, *pkg, fetchName, nil)
	if err != nil {
		if alt := strings.TrimSpace(sw.Name); alt != "" && !strings.EqualFold(alt, fetchName) {
			res, err = softwarepkg.Import(ctx, db, *pkg, alt, nil)
		}
		if err != nil {
			return false, err
		}
	}
	if res != nil {
		*sw = res.Software
	}
	softwarepkg.InvalidateCatalogCache()
	return true, nil
}
