package proxymanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// ErrSettingsNotFound is returned when the singleton cannot be loaded/created.
var ErrSettingsNotFound = errors.New("proxy settings not found")

// EnsureSettings loads the singleton proxy_settings row, creating defaults if missing.
func EnsureSettings(db *gorm.DB) (*models.ProxySettings, error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	var s models.ProxySettings
	err := db.Where("id = ?", models.ProxySettingsSingletonID).First(&s).Error
	if err == nil {
		s.Normalize()
		if migrateLegacyConfigDir(db, &s) {
			s.Normalize()
		}
		return &s, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	s = *models.NewDefaultProxySettings()
	s.ConfigDir = AbsoluteConfigDir("")
	if err := db.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// migrateLegacyConfigDir moves old data/proxymanager defaults onto the durable
// /config (or ./tmp in development) path and persists the change once.
func migrateLegacyConfigDir(db *gorm.DB, s *models.ProxySettings) bool {
	if s == nil || !IsLegacyDataConfigDir(s.ConfigDir) {
		return false
	}
	next := AbsoluteConfigDir("")
	if next == "" || next == s.ConfigDir {
		return false
	}
	s.ConfigDir = next
	_ = db.Model(s).Update("config_dir", next).Error
	return true
}

// SaveSettings persists settings and marks dirty.
func SaveSettings(db *gorm.DB, s *models.ProxySettings) error {
	if s == nil {
		return errors.New("settings is nil")
	}
	s.ID = models.ProxySettingsSingletonID
	s.Normalize()
	s.Dirty = true
	return db.Save(s).Error
}

// MarkDirty sets dirty=true on the singleton.
func MarkDirty(db *gorm.DB) error {
	s, err := EnsureSettings(db)
	if err != nil {
		return err
	}
	return db.Model(s).Update("dirty", true).Error
}

// ClearDirty clears dirty and records last apply metadata.
func ClearDirty(db *gorm.DB, engine string, applyErr error) error {
	s, err := EnsureSettings(db)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"last_applied_at":   now,
		"last_apply_engine": engine,
	}
	if applyErr == nil {
		updates["dirty"] = false
		updates["last_apply_error"] = ""
	} else {
		updates["dirty"] = true
		updates["last_apply_error"] = applyErr.Error()
	}
	return db.Model(s).Updates(updates).Error
}

// ConfigDirFor returns absolute engine config directory, creating it.
func ConfigDirFor(settings *models.ProxySettings, engine string) (string, error) {
	if settings == nil {
		return "", errors.New("settings is nil")
	}
	root := AbsoluteConfigDir(settings.ConfigDir)
	settings.ConfigDir = root
	dir := filepath.Join(root, engine)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Snapshot loads all enabled proxy metadata for generation.
type Snapshot struct {
	Settings     *models.ProxySettings
	Hosts        []models.ProxyHost
	Redirects    []models.ProxyRedirect
	Certificates []models.ProxyCertificate
}

// LoadSnapshot loads settings + related rows for apply/preview.
func LoadSnapshot(ctx context.Context, db *gorm.DB) (*Snapshot, error) {
	settings, err := EnsureSettings(db)
	if err != nil {
		return nil, err
	}
	var hosts []models.ProxyHost
	if err := db.WithContext(ctx).
		Preload("Locations", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("enabled = ?", true).Order("order_nr asc, path_prefix asc")
		}).
		Where("enabled = ?", true).
		Order("order_nr asc, name asc").
		Find(&hosts).Error; err != nil {
		return nil, err
	}
	var redirects []models.ProxyRedirect
	if err := db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("order_nr asc, from_host asc").
		Find(&redirects).Error; err != nil {
		return nil, err
	}
	var certs []models.ProxyCertificate
	if err := db.WithContext(ctx).Order("name asc").Find(&certs).Error; err != nil {
		return nil, err
	}
	return &Snapshot{
		Settings:     settings,
		Hosts:        hosts,
		Redirects:    redirects,
		Certificates: certs,
	}, nil
}

// CertByID returns a certificate from the snapshot.
func (s *Snapshot) CertByID(id string) *models.ProxyCertificate {
	if id == "" || s == nil {
		return nil
	}
	for i := range s.Certificates {
		if s.Certificates[i].ID == id {
			return &s.Certificates[i]
		}
	}
	return nil
}

// EnsureConfigRoot creates the root config directory.
func EnsureConfigRoot(settings *models.ProxySettings) error {
	if settings == nil {
		return errors.New("settings is nil")
	}
	dir := AbsoluteConfigDir(settings.ConfigDir)
	settings.ConfigDir = dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	return nil
}
