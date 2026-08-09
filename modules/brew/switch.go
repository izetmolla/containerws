package brew

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/install"
	"gorm.io/gorm"
)

// BrewTokenForSoftware returns the exact brew formula token for a Softwares row.
// Prefer registry_slug; otherwise lowercase name.
func BrewTokenForSoftware(sw *models.Software) string {
	if sw == nil {
		return ""
	}
	if slug := strings.ToLower(strings.TrimSpace(sw.RegistrySlug)); slug != "" {
		return slug
	}
	return strings.ToLower(strings.TrimSpace(sw.Name))
}

// FindSoftwareByBrewToken finds an active Softwares row whose registry_slug or name
// equals the brew formula token (case-insensitive exact).
func FindSoftwareByBrewToken(db *gorm.DB, token string) (*models.Software, error) {
	token = strings.ToLower(strings.TrimSpace(token))
	if db == nil || token == "" {
		return nil, nil
	}
	var rows []models.Software
	if err := db.Where("is_active = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if BrewTokenForSoftware(&rows[i]) == token {
			return &rows[i], nil
		}
	}
	return nil, nil
}

type switchResult struct {
	SoftwareID     string `json:"software_id"`
	Token          string `json:"token"`
	Target         string `json:"target"`
	PackageManager string `json:"package_manager"`
	Message        string `json:"message"`
}

// SwitchPackageManager moves ownership between Softwares (local) and Brew.
func SwitchPackageManager(ctx context.Context, db *gorm.DB, softwareID, target string) (*switchResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	softwareID = strings.TrimSpace(softwareID)
	target = models.NormalizePackageManager(target)
	if softwareID == "" {
		return nil, fmt.Errorf("software_id is required")
	}

	var sw models.Software
	if err := db.WithContext(ctx).Where("id = ? AND is_active = ?", softwareID, true).First(&sw).Error; err != nil {
		return nil, fmt.Errorf("software not found")
	}
	token := BrewTokenForSoftware(&sw)
	if token == "" || strings.Contains(token, " ") {
		return nil, fmt.Errorf("software has no exact brew formula token (use registry_slug or single-word name)")
	}
	if !FormulaExists(token) {
		return nil, fmt.Errorf("formula %q not found on Homebrew", token)
	}

	host := install.CurrentHostIdentity()
	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if err != nil {
		return nil, err
	}
	matching := install.MatchingVersion(versions, host, true)
	if matching == nil {
		return nil, fmt.Errorf("no Softwares version compatible with this host")
	}

	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return nil, fmt.Errorf("brew is not installed")
	}

	switch target {
	case models.PackageManagerBrew:
		row, _ := models.GetSoftwareInstalled(db, sw.ID)
		if row != nil && !row.Uninstalled && models.NormalizePackageManager(row.PackageManager) == models.PackageManagerLocal {
			if strings.TrimSpace(matching.UninstallScript) == "" {
				return nil, fmt.Errorf("software has no uninstall script; cannot cleanly leave Softwares")
			}
			install.CancelPendingInstallsForSoftware(sw.ID)
			if n := install.EnqueueActions(db, install.QueueActionUninstall, []string{sw.ID}); n == 0 {
				return nil, fmt.Errorf("failed to enqueue Softwares uninstall")
			}
			if err := waitSoftwareQueueDone(ctx, db, sw.ID, 10*time.Minute); err != nil {
				return nil, fmt.Errorf("softwares uninstall: %w", err)
			}
		}

		out, err := runBrewCombined(ctx, brewPath, "install", "--formula", token)
		if err != nil {
			return nil, fmt.Errorf("brew install failed: %w\n%s", err, out)
		}
		if err := models.MarkSoftwareInstalledWithManager(db, sw.ID, matching.ID, models.PackageManagerBrew); err != nil {
			return nil, err
		}
		return &switchResult{
			SoftwareID:     sw.ID,
			Token:          token,
			Target:         target,
			PackageManager: models.PackageManagerBrew,
			Message:        "Switched to Brew",
		}, nil

	case models.PackageManagerLocal:
		if strings.TrimSpace(matching.InstallScript) == "" {
			return nil, fmt.Errorf("software has no install script; cannot switch to Softwares")
		}
		out, err := runBrewCombined(ctx, brewPath, "uninstall", "--formula", "--force", token)
		if err != nil {
			out2, err2 := runBrewCombined(ctx, brewPath, "uninstall", "--formula", token)
			out = out + "\n" + out2
			if err2 != nil {
				return nil, fmt.Errorf("brew uninstall failed: %w\n%s", err2, out)
			}
		}
		_ = models.MarkSoftwareUninstalled(db, sw.ID)
		install.CancelPendingInstallsForSoftware(sw.ID)
		if n := install.EnqueueActions(db, install.QueueActionInstall, []string{sw.ID}); n == 0 {
			return nil, fmt.Errorf("failed to enqueue Softwares install")
		}
		if err := waitSoftwareQueueDone(ctx, db, sw.ID, 15*time.Minute); err != nil {
			return nil, fmt.Errorf("softwares install: %w", err)
		}
		if err := models.MarkSoftwareInstalledWithManager(db, sw.ID, matching.ID, models.PackageManagerLocal); err != nil {
			return nil, err
		}
		return &switchResult{
			SoftwareID:     sw.ID,
			Token:          token,
			Target:         target,
			PackageManager: models.PackageManagerLocal,
			Message:        "Switched to Softwares (local)",
		}, nil
	default:
		return nil, fmt.Errorf("invalid target")
	}
}

func waitSoftwareQueueDone(ctx context.Context, db *gorm.DB, softwareID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for softwares queue")
		}
		view := install.ActiveQueue(db)
		busy := false
		for _, it := range view.Items {
			if it.SoftwareID != softwareID {
				continue
			}
			switch it.Status {
			case "pending", "running":
				busy = true
			case "error":
				msg := it.Message
				if msg == "" {
					msg = "queue item failed"
				}
				return fmt.Errorf("%s", msg)
			}
		}
		if !busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
}
