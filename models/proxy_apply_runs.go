package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Apply run statuses.
const (
	ProxyApplyPending = "pending"
	ProxyApplyRunning = "running"
	ProxyApplySuccess = "success"
	ProxyApplyFailed  = "failed"
)

// ProxyApplyRun records one Apply invocation for audit / status UI.
type ProxyApplyRun struct {
	ID     string `json:"id" gorm:"primaryKey;type:text"`
	Engine string `json:"engine" gorm:"size:16;not null;index"`
	Status string `json:"status" gorm:"size:16;not null;default:pending;index"`

	StartedAt  time.Time  `json:"started_at" gorm:"not null"`
	FinishedAt *time.Time `json:"finished_at"`
	LogText    string     `json:"log_text" gorm:"type:text"`
	FilesJSON  JSONBArray `json:"files_json" gorm:"type:text"` // list of generated file paths
	ErrorText  string     `json:"error_text" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (b *ProxyApplyRun) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if b.StartedAt.IsZero() {
		b.StartedAt = time.Now().UTC()
	}
	if b.Status == "" {
		b.Status = ProxyApplyPending
	}
	if b.FilesJSON == nil {
		b.FilesJSON = JSONBArray{}
	}
	return
}

func (ProxyApplyRun) TableName() string {
	return "proxy_apply_runs"
}
