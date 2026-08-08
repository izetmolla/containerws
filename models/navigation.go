package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Navigation specific settings.
type Navigation struct {
	ID       string `json:"id" gorm:"primaryKey;type:text"`
	IdNumber int64  `json:"id_number" gorm:"uniqueIndex;autoIncrement"`

	ContainerID string    `json:"container_id" gorm:"type:text;default:null;index:idx_app_navigation_container_id"`
	Container   Container `json:"container" gorm:"foreignKey:ContainerID;references:ID"`
	ParentID    string    `json:"parent_id" gorm:"type:text;default:null;index:idx_app_navigation_parent_id"`

	Title       string     `json:"title" gorm:"size:255;"`
	Icon        string     `json:"icon" gorm:"size:255;"`
	To          string     `json:"to" gorm:"type:text;"`
	IsNew       bool       `json:"isNew" gorm:"column:isNew;default:false;"`
	IsComing    bool       `json:"isComing" gorm:"column:isComing;default:false;"`
	IsDataBadge string     `json:"isDataBadge" gorm:"column:isDataBadge;size:255;"`
	NewTab      bool       `json:"newTab" gorm:"column:newTab;default:false;"`
	IsExternal  bool       `json:"isExternal" gorm:"column:isExternal;default:false;"`
	Roles       JSONBArray `json:"roles" gorm:"type:text;default:'[]';"`

	Children []Navigation `json:"children,omitempty" gorm:"foreignKey:ParentID;references:ID"`

	OrderNr  int64 `json:"order_nr" gorm:"column:order_nr;default:0;"`
	IsActive bool  `json:"is_active" gorm:"column:is_active;default:true;"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *Navigation) BeforeCreate(_ *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

func (b Navigation) TableName() string {
	return "navigations"
}
