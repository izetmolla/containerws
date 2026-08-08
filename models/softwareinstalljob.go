package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SoftwareInstallJob persists install/update/reinstall sessions and their logs
// so the UI can resume after refresh or reconnect after a network drop.
type SoftwareInstallJob struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	SoftwareID string    `json:"software_id" gorm:"size:255;index;not null"`
	Software   *Software `json:"software,omitempty" gorm:"foreignKey:SoftwareID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	VersionID string           `json:"version_id" gorm:"size:255;index"`
	Version   *SoftwareVersion `json:"version,omitempty" gorm:"foreignKey:VersionID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	VersionLabel string `json:"version_label" gorm:"size:64"`

	Status  string `json:"status" gorm:"size:32;index;not null"` // running | success | error | cancelled
	Message string `json:"message" gorm:"type:text"`
	// FailureReason is a longer human-readable summary for failed jobs.
	FailureReason string `json:"failure_reason" gorm:"type:text"`
	ExitCode      *int  `json:"exit_code,omitempty"`

	// LogJSON is a JSON array of {stream,text,at} lines.
	LogJSON string `json:"log_json" gorm:"type:text"`

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (b *SoftwareInstallJob) BeforeCreate(_ *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

func (b SoftwareInstallJob) TableName() string {
	return "software_install_jobs"
}

// GetSoftwareInstallJob returns a job by id, or nil.
func GetSoftwareInstallJob(db *gorm.DB, id string) (*SoftwareInstallJob, error) {
	if db == nil || id == "" {
		return nil, nil
	}
	var row SoftwareInstallJob
	err := db.Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// LatestSoftwareInstallJob returns the most recent job for a software id.
func LatestSoftwareInstallJob(db *gorm.DB, softwareID string) (*SoftwareInstallJob, error) {
	if db == nil || softwareID == "" {
		return nil, nil
	}
	var row SoftwareInstallJob
	err := db.Where("software_id = ?", softwareID).
		Order("started_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpsertSoftwareInstallJob creates or updates a persisted install job row.
func UpsertSoftwareInstallJob(db *gorm.DB, row *SoftwareInstallJob) error {
	if db == nil || row == nil || row.ID == "" {
		return nil
	}
	var existing SoftwareInstallJob
	err := db.Where("id = ?", row.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(row).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&existing).Updates(map[string]any{
		"status":         row.Status,
		"message":        row.Message,
		"failure_reason": row.FailureReason,
		"exit_code":      row.ExitCode,
		"log_json":       row.LogJSON,
		"finished_at":    row.FinishedAt,
		"version_id":     row.VersionID,
		"version_label":  row.VersionLabel,
	}).Error
}
