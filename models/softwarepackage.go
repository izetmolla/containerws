package models

import (
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

// Server specific settings.
type SoftwarePackage struct {
	ID           string           `json:"id" gorm:"primaryKey;type:text"`


	PackageURL string `json:"package_url" gorm:"type:text;"`
	Token      string `json:"token" gorm:"type:text;default:''"`
	Username   string `json:"username" gorm:"type:text;default:''"`
	Password   string `json:"password" gorm:"type:text;default:''"`


	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *SoftwarePackage) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

func (b SoftwarePackage) TableName() string {
	return "software_packages"
}
