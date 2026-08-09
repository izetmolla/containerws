package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Package manager ownership for an installed Softwares catalog row.
const (
	PackageManagerLocal = "local"
	PackageManagerBrew  = "brew"
)

// SoftwareInstalled records which catalog version is (or was) installed on this host.
// One row per software (unique software_id).
//
// When Uninstalled is true the user intentionally removed the app: keep the row so it
// stays visible on the Installed list, but softwaresync must not auto-reinstall it.
type SoftwareInstalled struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	SoftwareID string           `json:"software_id" gorm:"size:255;uniqueIndex;not null"`
	Software   *Software        `json:"software,omitempty" gorm:"foreignKey:SoftwareID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	VersionID  string           `json:"version_id" gorm:"size:255;index;not null"`
	Version    *SoftwareVersion `json:"version,omitempty" gorm:"foreignKey:VersionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// PackageManager is local (Softwares scripts) or brew (Homebrew formulae).
	// Brew-owned rows skip softwaresync auto-reinstall and Softwares action buttons.
	PackageManager string `json:"package_manager" gorm:"size:32;not null;default:local;index"`

	// Uninstalled is set after a successful user uninstall. Prevents softwaresync
	// from treating a missing binary as "must reinstall".
	Uninstalled   bool       `json:"uninstalled" gorm:"not null;default:false;index"`
	UninstalledAt *time.Time `json:"uninstalled_at,omitempty"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *SoftwareInstalled) BeforeCreate(_ *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

func (b SoftwareInstalled) TableName() string {
	return "software_installed"
}

// NormalizePackageManager returns local or brew; empty/unknown → local.
func NormalizePackageManager(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case PackageManagerBrew:
		return PackageManagerBrew
	default:
		return PackageManagerLocal
	}
}

// MarkSoftwareInstalled upserts the installed row for softwareID → versionID
// and clears any prior uninstall intent. Sets package_manager to local.
func MarkSoftwareInstalled(db *gorm.DB, softwareID, versionID string) error {
	return MarkSoftwareInstalledWithManager(db, softwareID, versionID, PackageManagerLocal)
}

// MarkSoftwareInstalledWithManager upserts install ownership (local or brew).
func MarkSoftwareInstalledWithManager(db *gorm.DB, softwareID, versionID, packageManager string) error {
	if db == nil || softwareID == "" || versionID == "" {
		return nil
	}
	pm := NormalizePackageManager(packageManager)

	var row SoftwareInstalled
	err := db.Unscoped().Where("software_id = ?", softwareID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&SoftwareInstalled{
			SoftwareID:     softwareID,
			VersionID:      versionID,
			PackageManager: pm,
			Uninstalled:    false,
		}).Error
	}
	if err != nil {
		return err
	}

	row.VersionID = versionID
	row.PackageManager = pm
	row.Uninstalled = false
	row.UninstalledAt = nil
	row.DeletedAt = gorm.DeletedAt{}
	return db.Unscoped().Save(&row).Error
}

// SetSoftwarePackageManager updates ownership without changing version.
func SetSoftwarePackageManager(db *gorm.DB, softwareID, packageManager string) error {
	if db == nil || softwareID == "" {
		return nil
	}
	pm := NormalizePackageManager(packageManager)
	var row SoftwareInstalled
	err := db.Where("software_id = ?", softwareID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return db.Model(&row).Update("package_manager", pm).Error
}

// GetSoftwarePackageManager returns local|brew for an active install; empty if none.
func GetSoftwarePackageManager(db *gorm.DB, softwareID string) string {
	row, err := GetSoftwareInstalled(db, softwareID)
	if err != nil || row == nil || row.Uninstalled {
		return ""
	}
	return NormalizePackageManager(row.PackageManager)
}

// MarkSoftwareUninstalled keeps the install row but marks it as intentionally
// uninstalled so softwaresync will not auto-reinstall.
func MarkSoftwareUninstalled(db *gorm.DB, softwareID string) error {
	if db == nil || softwareID == "" {
		return nil
	}
	now := time.Now()
	var row SoftwareInstalled
	err := db.Unscoped().Where("software_id = ?", softwareID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	row.Uninstalled = true
	row.UninstalledAt = &now
	row.DeletedAt = gorm.DeletedAt{}
	return db.Unscoped().Save(&row).Error
}

// GetSoftwareInstalled returns the install row for a software, or nil if none.
// Includes rows marked Uninstalled (still present, not soft-deleted).
func GetSoftwareInstalled(db *gorm.DB, softwareID string) (*SoftwareInstalled, error) {
	if db == nil || softwareID == "" {
		return nil, nil
	}
	var row SoftwareInstalled
	err := db.Where("software_id = ?", softwareID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListSoftwareInstalled returns all install rows (with Software + Version preloaded).
func ListSoftwareInstalled(db *gorm.DB) ([]SoftwareInstalled, error) {
	if db == nil {
		return nil, nil
	}
	var rows []SoftwareInstalled
	err := db.Preload("Software").Preload("Version").Find(&rows).Error
	return rows, err
}

// InstalledVersionMap returns softwareID → installed versionID for actively
// installed softwares (excludes Uninstalled rows).
func InstalledVersionMap(db *gorm.DB) (map[string]string, error) {
	out := make(map[string]string)
	if db == nil {
		return out, nil
	}
	var rows []SoftwareInstalled
	if err := db.Select("software_id", "version_id", "uninstalled").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Uninstalled {
			continue
		}
		out[row.SoftwareID] = row.VersionID
	}
	return out, nil
}

// PackageManagerMap returns softwareID → package_manager for actively installed rows.
func PackageManagerMap(db *gorm.DB) (map[string]string, error) {
	out := make(map[string]string)
	if db == nil {
		return out, nil
	}
	var rows []SoftwareInstalled
	if err := db.Select("software_id", "package_manager", "uninstalled").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Uninstalled {
			continue
		}
		out[row.SoftwareID] = NormalizePackageManager(row.PackageManager)
	}
	return out, nil
}

// UninstalledVersionMap returns softwareID → last versionID for intentionally
// uninstalled softwares (still shown on the Installed list).
func UninstalledVersionMap(db *gorm.DB) (map[string]string, error) {
	out := make(map[string]string)
	if db == nil {
		return out, nil
	}
	var rows []SoftwareInstalled
	if err := db.Select("software_id", "version_id", "uninstalled").
		Where("uninstalled = ?", true).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SoftwareID] = row.VersionID
	}
	return out, nil
}

// IsSoftwareUninstalled reports whether the user intentionally uninstalled this software.
func IsSoftwareUninstalled(db *gorm.DB, softwareID string) bool {
	if db == nil || softwareID == "" {
		return false
	}
	var row SoftwareInstalled
	err := db.Select("uninstalled").Where("software_id = ?", softwareID).First(&row).Error
	if err != nil {
		return false
	}
	return row.Uninstalled
}

// ClearSoftwareInstalled soft-deletes the install row for softwareID.
// Prefer MarkSoftwareUninstalled for user uninstalls so the app stays on the list
// and softwaresync skips auto-reinstall.
func ClearSoftwareInstalled(db *gorm.DB, softwareID string) error {
	if db == nil || softwareID == "" {
		return nil
	}
	return db.Where("software_id = ?", softwareID).Delete(&SoftwareInstalled{}).Error
}

// HasSoftwareUpdate is true when the installed version is not the latest.
func HasSoftwareUpdate(installedVersionID, latestVersionID string) bool {
	if installedVersionID == "" || latestVersionID == "" {
		return false
	}
	return installedVersionID != latestVersionID
}
