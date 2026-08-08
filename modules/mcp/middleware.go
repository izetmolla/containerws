package mcp

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const localsMCPKey = "mcp_key"

// extractToken reads credentials from any HTTP method.
// Order: query ?token=, form field token, Authorization Bearer, X-Api-Key, X-Auth-Token.
func extractToken(c fiber.Ctx) string {
	if t := strings.TrimSpace(c.Query("token")); t != "" {
		return t
	}
	if t := strings.TrimSpace(c.FormValue("token")); t != "" {
		return t
	}
	if auth := strings.TrimSpace(c.Get("Authorization")); auth != "" {
		if len(auth) >= 7 && strings.EqualFold(auth[:7], "bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return auth
	}
	if t := strings.TrimSpace(c.Get("X-Api-Key")); t != "" {
		return t
	}
	return strings.TrimSpace(c.Get("X-Auth-Token"))
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error":   "Missing or invalid credentials",
		"message": "Provide a valid MCP API key via Authorization: Bearer <key>, X-Api-Key, X-Auth-Token header, or token query/form parameter (loopback 127.0.0.1 / ::1 is allowed without a key)",
	})
}

func envBootstrapToken() string {
	return strings.TrimSpace(viper.GetString("MCP_TOKEN"))
}

func (cc *controller) lookupMCPKey(token string) (*models.MCPKey, error) {
	if token == "" || cc.app == nil || cc.app.DB() == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var key models.MCPKey
	err := cc.app.DB().
		Where("key = ? AND status = ?", token, models.StatusActive).
		First(&key).Error
	if err != nil {
		return nil, err
	}
	if !key.IsUsable() {
		return nil, gorm.ErrRecordNotFound
	}
	return &key, nil
}

func (cc *controller) touchMCPKey(key *models.MCPKey, ip string) {
	if key == nil || cc.app == nil || cc.app.DB() == nil {
		return
	}
	now := time.Now().UTC()
	_ = cc.app.DB().Model(key).Updates(map[string]any{
		"last_used_at": now,
		"last_used_ip": ip,
	}).Error
}

func (cc *controller) authenticateMCP(c fiber.Ctx, extraToken string) error {
	// Local callers (127.0.0.1 / ::1) may use MCP without a key — panel-side
	// tooling, health checks, and same-host agents.
	if c.IsFromLocal() {
		return c.Next()
	}

	token := extractToken(c)
	if token == "" {
		return unauthorized(c)
	}

	// Prefer DB-backed keys (mcp_keys).
	if key, err := cc.lookupMCPKey(token); err == nil {
		c.Locals(localsMCPKey, key)
		go cc.touchMCPKey(key, c.IP())
		return c.Next()
	}

	// Standalone override token (MountStandalone argument).
	if extra := strings.TrimSpace(extraToken); extra != "" && token == extra {
		return c.Next()
	}

	// Optional bootstrap: MCP_TOKEN env (useful before any key is created).
	if bootstrap := envBootstrapToken(); bootstrap != "" && token == bootstrap {
		return c.Next()
	}

	return unauthorized(c)
}

func (cc *controller) mcpMiddleware(c fiber.Ctx) error {
	return cc.authenticateMCP(c, "")
}

// MCPKeyFromCtx returns the authenticated MCPKey when middleware validated a DB key.
func MCPKeyFromCtx(c fiber.Ctx) *models.MCPKey {
	if v, ok := c.Locals(localsMCPKey).(*models.MCPKey); ok {
		return v
	}
	return nil
}

// hasValidToken compares the request credential to a single static token.
// Prefer authenticateMCP / DB keys for production; this matches petsprofile standalone helpers.
func hasValidToken(c fiber.Ctx, token string) bool {
	return extractToken(c) == token
}
