package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Docker stack lifecycle states.
const (
	DockerStackStatusCreated  = "created"
	DockerStackStatusRunning  = "running"
	DockerStackStatusStopped  = "stopped"
	DockerStackStatusError    = "error"
	DockerStackStatusRemoving = "removing"
)

// DockerStack is a Compose stack managed by Container Workspace (Portainer-style).
type DockerStack struct {
	ID            string `json:"id" gorm:"primaryKey;type:text"`
	Name          string `json:"name" gorm:"size:255;not null;index"`
	EnvironmentID string `json:"environment_id" gorm:"size:36;not null;index"`
	ComposeYAML   string `json:"compose_yaml" gorm:"type:text;not null"`
	EnvFile       string `json:"env_file" gorm:"type:text"`
	Status        string `json:"status" gorm:"size:32;not null;default:created;index"`
	Message       string `json:"message" gorm:"type:text"`
	TemplateID    *int   `json:"template_id,omitempty" gorm:"index"`
	TemplateTitle string `json:"template_title,omitempty" gorm:"size:255"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *DockerStack) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *DockerStack) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (DockerStack) TableName() string {
	return "docker_stacks"
}

func (b *DockerStack) Normalize() {
	b.Name = strings.TrimSpace(b.Name)
	b.EnvironmentID = strings.TrimSpace(b.EnvironmentID)
	if b.Status == "" {
		b.Status = DockerStackStatusCreated
	}
}
