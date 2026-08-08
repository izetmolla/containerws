package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// K8sApplication is a saved Kubernetes application manifest (YAML + namespace).
// Live resource status is always fetched from the cluster using identity refs
// derived from the stored YAML — only name/yaml/namespace are authoritative in SQLite.
type K8sApplication struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	Name      string `json:"name" gorm:"size:255;not null;index"`
	Namespace string `json:"namespace" gorm:"size:255;not null;index"`
	YAML      string `json:"yaml" gorm:"type:text;not null"`
	// Version is the current YAML revision number (1-based).
	Version int `json:"version" gorm:"not null;default:1"`

	// Resources is a compact list of {apiVersion,kind,name} used to query the cluster.
	Resources JSONBArray `json:"resources,omitempty" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (K8sApplication) TableName() string {
	return "k8s_applications"
}

func (a *K8sApplication) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(a.ID) == "" {
		a.ID = uuid.New().String()
	}
	a.Normalize()
	return nil
}

func (a *K8sApplication) BeforeSave(tx *gorm.DB) error {
	a.Normalize()
	return nil
}

func (a *K8sApplication) Normalize() {
	a.Name = strings.TrimSpace(a.Name)
	a.Namespace = strings.TrimSpace(a.Namespace)
	a.YAML = strings.TrimSpace(a.YAML)
}
