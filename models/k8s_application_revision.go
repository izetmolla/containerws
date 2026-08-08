package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// K8sApplicationRevision is an immutable snapshot of an application's YAML.
type K8sApplicationRevision struct {
	ID            string `json:"id" gorm:"primaryKey;type:text"`
	ApplicationID string `json:"application_id" gorm:"type:text;not null;index;index:idx_k8s_app_rev,priority:1"`

	Version   int    `json:"version" gorm:"not null;index:idx_k8s_app_rev,priority:2"`
	Name      string `json:"name" gorm:"size:255;not null"`
	Namespace string `json:"namespace" gorm:"size:255;not null"`
	YAML      string `json:"yaml" gorm:"type:text;not null"`
	Resources JSONBArray `json:"resources,omitempty" gorm:"type:text"`

	// Source: create | save | apply | restore
	Source string `json:"source" gorm:"size:32;not null;default:save"`
	Note   string `json:"note,omitempty" gorm:"size:255"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (K8sApplicationRevision) TableName() string {
	return "k8s_application_revisions"
}

func (r *K8sApplicationRevision) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(r.ID) == "" {
		r.ID = uuid.New().String()
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Namespace = strings.TrimSpace(r.Namespace)
	r.YAML = strings.TrimSpace(r.YAML)
	r.Source = strings.TrimSpace(r.Source)
	if r.Source == "" {
		r.Source = "save"
	}
	return nil
}
