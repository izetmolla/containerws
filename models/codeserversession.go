package models

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	CodeserverSessionStatusActive   = "active"
	CodeserverSessionStatusInactive = "inactive"
)

// CodeserverSession tracks a Microsoft VS Code Server (serve-web) workspace.
// A user may own many sessions (one per folder/workspace).
type CodeserverSession struct {
	ID     string `json:"id" gorm:"primaryKey;type:text"`
	UserID string `json:"user_id" gorm:"size:50;index;not null"`
	User   User   `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// Name is a short display title (defaults to folder basename).
	Name   string `json:"name" gorm:"type:text;default:''"`
	Status string `json:"status" gorm:"type:text;default:active"`
	Path   string `json:"path" gorm:"type:text;default:'';index"`

	Address string `json:"address" gorm:"type:text;default:127.0.0.1"`
	Port    int    `json:"port" gorm:"default:0"`
	Pid     int    `json:"pid" gorm:"default:0"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (CodeserverSession) TableName() string {
	return "codeserver_sessions"
}

func (s *CodeserverSession) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(s.ID) == "" {
		s.ID = uuid.New().String()
	}
	if strings.TrimSpace(s.Name) == "" {
		s.Name = CodeserverWorkspaceName("", s.Path)
	}
	return nil
}

// CodeserverWorkspaceName returns a display name from an explicit name or path basename.
func CodeserverWorkspaceName(name, path string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "workspace"
	}
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	if base == "" || base == "." || base == "/" {
		return "workspace"
	}
	return base
}

// UpstreamHostPort returns host:port for the serve-web HTTP endpoint.
func (s CodeserverSession) UpstreamHostPort() string {
	addr := strings.TrimSpace(s.Address)
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := s.Port
	if port <= 0 {
		port = 8443
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

// UpstreamBaseURL is http://address:port (no trailing slash).
func (s CodeserverSession) UpstreamBaseURL() string {
	return "http://" + s.UpstreamHostPort()
}
