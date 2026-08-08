package models

import (
	"time"

	"gorm.io/gorm"
)

type Role struct {
	ID          int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string         `json:"name" gorm:"size:50;uniqueIndex;not null"`
	Description string         `json:"description" gorm:"size:255"`
	Status      Status         `json:"status" gorm:"default:active;"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (Role) TableName() string {
	return "roles"
}
