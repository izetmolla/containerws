package proxymanager

import (
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// DefaultConfigDir is the production path on the durable /config volume
	// (same convention as database + SSL under /config/containerws/).
	DefaultConfigDir = "/config/containerws/proxymanager"
	// DevConfigDir is used when ENV=development (mirrors ./tmp/ssl, ./tmp/database.sqlite).
	DevConfigDir = "./tmp/proxymanager"
)

// ResolveConfigDir returns the Proxy Manager generated-config root.
// Override with PROXYMANAGER_CONFIG_DIR. Hosts/certs still live in SQLite;
// this directory only holds generated nginx/traefik/fiber files for apply + Docker mounts.
func ResolveConfigDir() string {
	if dir := strings.TrimSpace(viper.GetString("PROXYMANAGER_CONFIG_DIR")); dir != "" {
		return dir
	}
	if strings.EqualFold(strings.TrimSpace(viper.GetString("ENV")), "development") {
		return DevConfigDir
	}
	return DefaultConfigDir
}

// AbsoluteConfigDir resolves and absolutizes ResolveConfigDir (or override).
func AbsoluteConfigDir(override string) string {
	root := strings.TrimSpace(override)
	if root == "" {
		root = ResolveConfigDir()
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

// IsLegacyDataConfigDir reports whether path is the old CWD-relative data/proxymanager default.
func IsLegacyDataConfigDir(path string) bool {
	p := filepath.ToSlash(strings.TrimSpace(path))
	if p == "" {
		return false
	}
	if p == "data/proxymanager" || strings.HasSuffix(p, "/data/proxymanager") {
		return true
	}
	// Absolutized variants from an earlier boot (…/workspace/…/data/proxymanager).
	base := filepath.Base(filepath.Clean(path))
	parent := filepath.Base(filepath.Dir(filepath.Clean(path)))
	return parent == "data" && base == "proxymanager"
}
