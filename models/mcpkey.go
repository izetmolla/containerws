package models

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPKey authenticates clients to the MCP streamable HTTP endpoint.
type MCPKey struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	// Display
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// Secret presented by the client (Bearer / X-Api-Key / ?token=).
	Key       string `json:"key" gorm:"size:128;uniqueIndex;not null"`
	KeyPrefix string `json:"key_prefix" gorm:"size:16;index"` // first chars for UI lists (never show full key twice)

	Status Status `json:"status" gorm:"size:32;default:'active';index"`

	ExpiresAt  *time.Time `json:"expires_at" gorm:"index"`
	LastUsedAt *time.Time `json:"last_used_at"`
	LastUsedIP string     `json:"last_used_ip" gorm:"size:64"`

	CreatedBy string   `json:"created_by" gorm:"type:text;index"` // optional users.id
	Metadata  JSONBAny `json:"metadata" gorm:"type:text;default:'{}'"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (k *MCPKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	if strings.TrimSpace(k.Key) == "" {
		secret, err := generateMCPSecret()
		if err != nil {
			return err
		}
		k.Key = secret
	}
	if k.KeyPrefix == "" {
		k.KeyPrefix = mcpKeyPrefix(k.Key)
	}
	if k.Status == "" {
		k.Status = StatusActive
	}
	return nil
}

func (k *MCPKey) BeforeSave(tx *gorm.DB) error {
	if k.Key != "" && k.KeyPrefix == "" {
		k.KeyPrefix = mcpKeyPrefix(k.Key)
	}
	return nil
}

// IsUsable reports whether the key may authenticate an MCP request right now.
func (k *MCPKey) IsUsable() bool {
	if k == nil {
		return false
	}
	if k.Status != StatusActive {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(time.Now().UTC()) {
		return false
	}
	return true
}

func (MCPKey) TableName() string {
	return "mcp_keys"
}

func mcpKeyPrefix(key string) string {
	const n = 16
	if len(key) <= n {
		return key
	}
	return key[:n]
}

func mcpKeySuffix(key string) string {
	const n = 4
	if len(key) <= n {
		return key
	}
	return key[len(key)-n:]
}

// MCPKeyPrefix returns the leading hint stored/shown in the UI.
func MCPKeyPrefix(key string) string {
	return mcpKeyPrefix(key)
}

// MCPKeySuffix returns the trailing hint shown in the UI (not for auth).
func MCPKeySuffix(key string) string {
	return mcpKeySuffix(key)
}

func generateMCPSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "cws_mcp_" + hex.EncodeToString(b), nil
}
