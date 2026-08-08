package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ShellSessionStatus is the lifecycle of a Cloud Shell / CLI PTY session.
type ShellSessionStatus string

const (
	ShellSessionActive   ShellSessionStatus = "active"
	ShellSessionDetached ShellSessionStatus = "detached"
	ShellSessionClosed   ShellSessionStatus = "closed"
)

// ShellSession stores terminal sessions owned by a logged-in user.
// Live PTY state stays in-process; this table is the durable index for resume/list.
type ShellSession struct {
	ID     string `json:"id" gorm:"primaryKey;type:text"`
	UserID string `json:"user_id" gorm:"type:text;index;not null"`
	User   User   `json:"user" gorm:"foreignKey:UserID;references:ID"`

	Title     string `json:"title" gorm:"size:255"`
	ShellUser string `json:"shell_user" gorm:"size:128"`
	HomeDir   string `json:"home_dir" gorm:"size:512"`
	Shell     string `json:"shell" gorm:"size:255"`
	Cwd       string `json:"cwd" gorm:"size:1024"`

	Cols int `json:"cols" gorm:"default:80"`
	Rows int `json:"rows" gorm:"default:24"`

	Status ShellSessionStatus `json:"status" gorm:"size:32;default:'detached';index"`

	LastActiveAt *time.Time `json:"last_active_at" gorm:"index"`
	ClosedAt     *time.Time `json:"closed_at"`

	Hostname string   `json:"hostname" gorm:"size:255"`
	Metadata JSONBAny `json:"metadata" gorm:"type:text;default:'{}'"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (s *ShellSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = ShellSessionDetached
	}
	if s.Cols <= 0 {
		s.Cols = 80
	}
	if s.Rows <= 0 {
		s.Rows = 24
	}
	now := time.Now().UTC()
	if s.LastActiveAt == nil {
		s.LastActiveAt = &now
	}
	return nil
}

func (ShellSession) TableName() string {
	return "shell_sessions"
}
