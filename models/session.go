package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

// User represents a registered user in the system.
// It includes authentication, profile information, and relationships to other entities.
type Session struct {
	ID     string             `json:"id" gorm:"primaryKey;type:text"`
	UserID string             `json:"user_id" gorm:"type:text;default:null"`
	User   User               `json:"user" gorm:"foreignKey:UserID;references:ID"`
	Type   goauth.SessionType `json:"type" gorm:"type:varchar(255);default:'sign_in';"`

	IPAddress string `json:"ip_address" gorm:"size:50;"`
	UserAgent string `json:"user_agent" gorm:"size:255;"`
	Method    string `json:"method" gorm:"size:50;default:'credentials';"`

	Account JSONBAny `json:"account" gorm:"type:text;default:null"`

	ExpiresAt time.Time `json:"expires_at"`
	IsDeleted bool      `json:"is_deleted" gorm:"default:false"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// BeforeCreate sets a UUID for the user before creation.
// This ensures consistent ID generation across different database systems.
func (u *Session) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the custom table name for the User model.
func (Session) TableName() string {
	return "sessions"
}
