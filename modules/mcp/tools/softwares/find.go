package softwares

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

func (c *Controller) db() *gorm.DB {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.DB()
}

// findSoftware resolves by id, exact name, or unique partial name.
// Returns (nil, nil, false) when not listed; (nil, err, false) on DB/ambiguous errors.
func findSoftware(db *gorm.DB, key string) (*models.Software, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("empty software identifier")
	}
	if db == nil {
		return nil, errors.New("database unavailable")
	}

	var sw models.Software
	err := db.Where("id = ?", key).First(&sw).Error
	if err == nil {
		return &sw, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Where("LOWER(name) = ?", strings.ToLower(key)).First(&sw).Error
	if err == nil {
		return &sw, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var matches []models.Software
	if err := db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(key)+"%").
		Find(&matches).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Order != matches[j].Order {
			return matches[i].Order < matches[j].Order
		}
		return matches[i].Name < matches[j].Name
	})
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous software %q; matches: %s", key, strings.Join(names, ", "))
	}
}

func pickVersion(db *gorm.DB, softwareID, version string) (*models.SoftwareVersion, error) {
	version = strings.TrimSpace(version)
	var ver models.SoftwareVersion
	if version != "" {
		err := db.Where("software_id = ? AND version = ?", softwareID, version).First(&ver).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("version not found: %s", version)
			}
			return nil, err
		}
		return &ver, nil
	}
	err := db.Where("software_id = ?", softwareID).
		Order("is_latest DESC, created_at DESC").
		First(&ver).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no version available to install")
		}
		return nil, err
	}
	return &ver, nil
}

func latestVersion(db *gorm.DB, softwareID string) (*models.SoftwareVersion, error) {
	return pickVersion(db, softwareID, "")
}
