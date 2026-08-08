package environments

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OsEnvironmentSource distinguishes user-defined env vars from core server settings.
type OsEnvironmentSource string

const (
	OsEnvironmentSourceEnv    OsEnvironmentSource = "env"
	OsEnvironmentSourceServer OsEnvironmentSource = "server"
)

// OsEnvironment stores a single process environment variable in the database.
type OsEnvironment struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	Name  string `json:"name" gorm:"size:255;not null;uniqueIndex"`
	Value string `json:"value" gorm:"type:text"`

	ModuleID string `json:"module_id" gorm:"size:255;default:'';index"`

	Group string `json:"group" gorm:"size:255;default:''"`

	Source OsEnvironmentSource `json:"source" gorm:"size:16;not null;default:'env';index"`

	IsCore     bool `json:"is_core" gorm:"not null;default:false;index"`
	IsSecret   bool `json:"is_secret" gorm:"not null;default:false"`
	IsDisabled bool `json:"is_disabled" gorm:"not null;default:false;index"`
	IsTextarea bool `json:"is_textarea" gorm:"not null;default:false"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (row *OsEnvironment) BeforeCreate(_ *gorm.DB) error {
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
	return nil
}

func (OsEnvironment) TableName() string {
	return "os_environments"
}

// OsEnvironmentWatcherModel is an append-only change log. Each row signals that one or
// all environment variables changed so pods reload without scanning the full table.
type OsEnvironmentWatcherModel struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`

	// EnvironmentID is the affected row UUID. Empty means a bulk signal (full reload).
	EnvironmentID string `json:"environment_id" gorm:"type:text;index"`
	// Action is upsert, delete, or signal (legacy bulk invalidation).
	Action string `json:"action" gorm:"size:16;not null;default:signal"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (OsEnvironmentWatcherModel) TableName() string {
	return "os_environment_watchers"
}
