package models

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Module tracks a standalone module listener (e.g. MCP on a dedicated port).
type Module struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	Name    string `json:"name" gorm:"size:255;uniqueIndex;not null"`
	Address string `json:"address" gorm:"size:255;default:'0.0.0.0'"`
	Port    int    `json:"port" gorm:"not null;index"`

	Status    Status `json:"status" gorm:"size:32;default:'inactive';index"`
	LastError string `json:"last_error" gorm:"type:text"`

	LastStartedAt *time.Time `json:"last_started_at"`
	LastStoppedAt *time.Time `json:"last_stopped_at"`

	Description string   `json:"description" gorm:"type:text"`
	Metadata    JSONBAny `json:"metadata" gorm:"type:text;default:'{}'"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (m *Module) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if strings.TrimSpace(m.Address) == "" {
		m.Address = "0.0.0.0"
	}
	if m.Status == "" {
		m.Status = StatusInactive
	}
	return nil
}

// ListenAddr returns host:port suitable for net.Listen / Fiber Listen.
func (m *Module) ListenAddr() string {
	if m == nil {
		return ""
	}
	host := strings.TrimSpace(m.Address)
	if host == "" {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", m.Port))
}

// SetError records a failure and marks the module inactive.
func (m *Module) SetError(err error) {
	if m == nil {
		return
	}
	if err == nil {
		m.LastError = ""
		return
	}
	m.LastError = err.Error()
	m.Status = StatusError
}

// ClearError clears last_error without changing status.
func (m *Module) ClearError() {
	if m == nil {
		return
	}
	m.LastError = ""
}

func (Module) TableName() string {
	return "modules"
}
