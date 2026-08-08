package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// K8sKey is a stored kubeconfig secret (name + path + YAML content).
type K8sKey struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	Name   string `json:"name" gorm:"size:255;not null"`
	Path   string `json:"path" gorm:"size:1024;not null;index"`
	Secret string `json:"secret" gorm:"type:text;not null"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (K8sKey) TableName() string {
	return "k8s_keys"
}

func (k *K8sKey) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(k.ID) == "" {
		k.ID = uuid.New().String()
	}
	k.Name = strings.TrimSpace(k.Name)
	k.Path = strings.TrimSpace(k.Path)
	if k.Name == "" {
		k.Name = k.ID
	}
	return nil
}

func (k *K8sKey) BeforeSave(tx *gorm.DB) error {
	k.Name = strings.TrimSpace(k.Name)
	k.Path = strings.TrimSpace(k.Path)
	return nil
}
