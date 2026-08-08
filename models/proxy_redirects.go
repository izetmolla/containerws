package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProxyRedirect is a host/path redirect rule.
type ProxyRedirect struct {
	ID       string `json:"id" gorm:"primaryKey;type:text"`
	Name     string `json:"name" gorm:"size:255;not null"`
	Enabled  bool   `json:"enabled" gorm:"not null;default:true;index"`
	FromHost string `json:"from_host" gorm:"size:255;not null;index"`
	FromPath string `json:"from_path" gorm:"size:512;not null;default:/"`
	ToURL    string `json:"to_url" gorm:"size:2048;not null"`
	StatusCode int  `json:"status_code" gorm:"not null;default:301"` // 301|302|307|308
	PreservePath bool `json:"preserve_path" gorm:"not null;default:false"`
	OrderNr  int    `json:"order_nr" gorm:"not null;default:0;index"`
	Notes    string `json:"notes" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *ProxyRedirect) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *ProxyRedirect) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (ProxyRedirect) TableName() string {
	return "proxy_redirects"
}

func (b *ProxyRedirect) Normalize() {
	b.Name = strings.TrimSpace(b.Name)
	b.FromHost = strings.ToLower(strings.TrimSpace(b.FromHost))
	b.FromPath = strings.TrimSpace(b.FromPath)
	if b.FromPath == "" {
		b.FromPath = "/"
	}
	if !strings.HasPrefix(b.FromPath, "/") {
		b.FromPath = "/" + b.FromPath
	}
	b.ToURL = strings.TrimSpace(b.ToURL)
	switch b.StatusCode {
	case 301, 302, 307, 308:
	default:
		b.StatusCode = 301
	}
}
