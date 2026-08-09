package brew

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// SyncResult summarizes a brew CLI → Softwares installed reconcile.
type SyncResult struct {
	Imported       int `json:"imported"`
	Updated        int `json:"updated"`
	Uninstalled    int `json:"uninstalled"`
	SkippedLocal   int `json:"skipped_local"`
	Unmatched      int `json:"unmatched"`
	BrewInstalled  int `json:"brew_installed"`
	CatalogMatched int `json:"catalog_matched"`
}

var (
	syncStartOnce sync.Once
	syncThrottle  struct {
		mu   sync.Mutex
		last time.Time
	}
)

const syncMinInterval = 45 * time.Second

// StartSyncAsync imports brew-CLI-installed packages into software_installed
// (package_manager=brew) once per process when brew is present.
func StartSyncAsync(db *gorm.DB) {
	if db == nil {
		return
	}
	syncStartOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			res, err := SyncHostInstalls(ctx, db)
			if err != nil {
				log.Printf("brewsync: failed: %v", err)
				return
			}
			if res == nil {
				log.Printf("brewsync: skipped (brew missing)")
				return
			}
			log.Printf(
				"brewsync: brew=%d matched=%d imported=%d updated=%d uninstalled=%d skipped_local=%d unmatched=%d",
				res.BrewInstalled, res.CatalogMatched, res.Imported, res.Updated, res.Uninstalled, res.SkippedLocal, res.Unmatched,
			)
		}()
	})
}

// SyncHostInstallsThrottled runs SyncHostInstalls at most once per syncMinInterval.
func SyncHostInstallsThrottled(db *gorm.DB) {
	if db == nil || ResolveBrewPath() == "" {
		return
	}
	syncThrottle.mu.Lock()
	if time.Since(syncThrottle.last) < syncMinInterval {
		syncThrottle.mu.Unlock()
		return
	}
	syncThrottle.last = time.Now()
	syncThrottle.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := SyncHostInstalls(ctx, db)
	if err != nil {
		log.Printf("brewsync: throttle sync failed: %v", err)
		return
	}
	if res != nil && (res.Imported > 0 || res.Updated > 0 || res.Uninstalled > 0) {
		log.Printf(
			"brewsync: imported=%d updated=%d uninstalled=%d",
			res.Imported, res.Updated, res.Uninstalled,
		)
	}
}

// SyncHostInstalls reads packages installed via the brew CLI and upserts matching
// Softwares catalog rows into software_installed with package_manager=brew.
// Softwares-owned (local) active installs are never overwritten.
func SyncHostInstalls(ctx context.Context, db *gorm.DB) (*SyncResult, error) {
	if db == nil {
		return nil, nil
	}
	if ResolveBrewPath() == "" {
		return nil, nil
	}

	items, err := listInstalledFormulae(ctx)
	if err != nil {
		return nil, err
	}

	res := &SyncResult{BrewInstalled: len(items)}
	tokenIndex, err := buildBrewTokenIndex(db)
	if err != nil {
		return nil, err
	}

	seenSoftware := map[string]struct{}{}
	for _, it := range items {
		name, _ := it["name"].(string)
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		ver, _ := it["version"].(string)
		sw := tokenIndex[name]
		if sw == nil {
			res.Unmatched++
			continue
		}
		res.CatalogMatched++
		seenSoftware[sw.ID] = struct{}{}

		row, err := models.GetSoftwareInstalled(db, sw.ID)
		if err != nil {
			continue
		}
		// Never steal an active Softwares (local) install.
		if row != nil && !row.Uninstalled && models.NormalizePackageManager(row.PackageManager) == models.PackageManagerLocal {
			res.SkippedLocal++
			continue
		}

		versionID := pickVersionIDForBrew(ctx, db, sw.ID, ver)
		if versionID == "" {
			continue
		}

		wasNew := row == nil || row.Uninstalled || models.NormalizePackageManager(row.PackageManager) != models.PackageManagerBrew
		if err := models.MarkSoftwareInstalledWithManager(db, sw.ID, versionID, models.PackageManagerBrew); err != nil {
			continue
		}
		if wasNew {
			res.Imported++
		} else if row != nil && row.VersionID != versionID {
			res.Updated++
		}
	}

	// Softwares rows marked brew-owned but no longer present via brew CLI → uninstalled.
	rows, err := models.ListSoftwareInstalled(db)
	if err != nil {
		return res, nil
	}
	for i := range rows {
		row := rows[i]
		if row.Uninstalled {
			continue
		}
		if models.NormalizePackageManager(row.PackageManager) != models.PackageManagerBrew {
			continue
		}
		if _, ok := seenSoftware[row.SoftwareID]; ok {
			continue
		}
		if err := models.MarkSoftwareUninstalled(db, row.SoftwareID); err == nil {
			res.Uninstalled++
		}
	}

	return res, nil
}

func buildBrewTokenIndex(db *gorm.DB) (map[string]*models.Software, error) {
	out := map[string]*models.Software{}
	var rows []models.Software
	if err := db.Where("is_active = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		token := BrewTokenForSoftware(&rows[i])
		if token == "" || strings.Contains(token, " ") {
			continue
		}
		// First match wins (stable catalog order).
		if _, exists := out[token]; exists {
			continue
		}
		out[token] = &rows[i]
	}
	return out, nil
}

func pickVersionIDForBrew(ctx context.Context, db *gorm.DB, softwareID, brewVersion string) string {
	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", softwareID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if err != nil || len(versions) == 0 {
		return ""
	}
	brewVersion = strings.TrimSpace(brewVersion)
	if brewVersion != "" {
		for i := range versions {
			v := strings.TrimSpace(versions[i].Version)
			if v == "" {
				continue
			}
			if strings.EqualFold(v, brewVersion) ||
				strings.HasPrefix(strings.ToLower(v), strings.ToLower(brewVersion)) ||
				strings.HasPrefix(strings.ToLower(brewVersion), strings.ToLower(v)) {
				return versions[i].ID
			}
		}
	}
	return versions[0].ID
}
