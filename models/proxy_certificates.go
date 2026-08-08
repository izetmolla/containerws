package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Certificate source kinds.
const (
	ProxyCertUpload      = "upload"
	ProxyCertPath        = "path"
	ProxyCertLetsEncrypt = "letsencrypt" // stub for MVP
)

// ProxyCertificate stores TLS material or filesystem paths for proxy hosts.
type ProxyCertificate struct {
	ID      string `json:"id" gorm:"primaryKey;type:text"`
	Name    string `json:"name" gorm:"size:255;not null"`
	Domains string `json:"domains" gorm:"type:text"` // comma-separated
	Source  string `json:"source" gorm:"size:32;not null;default:upload;index"`

	// upload / pasted PEM
	CertPEM string `json:"cert_pem,omitempty" gorm:"type:text"`
	KeyPEM  string `json:"key_pem,omitempty" gorm:"type:text"`

	// path-based
	CertPath string `json:"cert_path" gorm:"size:1024"`
	KeyPath  string `json:"key_path" gorm:"size:1024"`

	// letsencrypt stub fields
	LetsEncryptEmail  string `json:"letsencrypt_email" gorm:"size:255"`
	LetsEncryptStatus string `json:"letsencrypt_status" gorm:"size:64"`

	ExpiresAt *time.Time `json:"expires_at"`
	Notes     string     `json:"notes" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *ProxyCertificate) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *ProxyCertificate) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (ProxyCertificate) TableName() string {
	return "proxy_certificates"
}

func (b *ProxyCertificate) Normalize() {
	b.Name = strings.TrimSpace(b.Name)
	b.Domains = strings.TrimSpace(b.Domains)
	b.Source = strings.ToLower(strings.TrimSpace(b.Source))
	switch b.Source {
	case ProxyCertUpload, ProxyCertPath, ProxyCertLetsEncrypt:
	default:
		b.Source = ProxyCertUpload
	}
	b.CertPath = strings.TrimSpace(b.CertPath)
	b.KeyPath = strings.TrimSpace(b.KeyPath)
	b.LetsEncryptEmail = strings.TrimSpace(b.LetsEncryptEmail)
}

// DomainList returns trimmed domains.
func (b *ProxyCertificate) DomainList() []string {
	parts := strings.Split(b.Domains, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
