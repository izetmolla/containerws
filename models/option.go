package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// OptionVNCInstalled is set true after a successful VNC/noVNC package setup.
	// Survives package removal so startup can detect "installed but missing".
	OptionVNCInstalled = "VNC_INSTALLED"
	// OptionVNCSoftwarePresent mirrors a live host probe (binaries + noVNC roots).
	OptionVNCSoftwarePresent = "VNC_SOFTWARE_PRESENT"

	// OptionCodeserverInstalled is set true after a successful VS Code Server install.
	OptionCodeserverInstalled = "CODESERVER_INSTALLED"
	// OptionCodeserverSoftwarePresent mirrors a live host probe for the VS Code CLI.
	OptionCodeserverSoftwarePresent = "CODESERVER_SOFTWARE_PRESENT"

	// General workspace settings.
	OptionWorkspaceName        = "WORKSPACE_NAME"
	OptionWorkspaceDescription = "WORKSPACE_DESCRIPTION"

	// Module sidebar visibility (General settings). Missing → disabled (opt-in).
	// EnsureModuleSidebarDefaults seeds explicit false on first install.
	OptionDockerModuleEnabled       = "DOCKER_MODULE_ENABLED"
	OptionKubernetesModuleEnabled   = "KUBERNETES_MODULE_ENABLED"
	OptionProxymanagerModuleEnabled = "PROXYMANAGER_MODULE_ENABLED"
	OptionBrewModuleEnabled         = "BREW_MODULE_ENABLED"
	// OptionModuleSidebarDefaultsSeeded marks that first-boot module defaults were applied.
	OptionModuleSidebarDefaultsSeeded = "MODULE_SIDEBAR_DEFAULTS_SEEDED"

	// LocalhostAutoLogin signs in as the panel process Linux user when the
	// TCP peer is loopback (127.0.0.1 / ::1). Missing → disabled (opt-in).
	OptionLocalhostAutoLogin = "LOCALHOST_AUTO_LOGIN"

	// Kubernetes — kubeconfig location / context / managed file registry.
	// Cluster state is always read from the Kubernetes API.
	OptionKubeconfigPath     = "KUBECONFIG_PATH"
	OptionKubeconfigContext  = "KUBECONFIG_CONTEXT"
	OptionKubeconfigActiveID = "KUBECONFIG_ACTIVE_ID"
	OptionKubeconfigFiles    = "KUBECONFIG_FILES"

	// MCP standalone listener (persisted; overrides env when managed from Settings).
	OptionMCPStandaloneEnabled = "MCP_STANDALONE_ENABLED"
	OptionMCPStandaloneAddress = "MCP_STANDALONE_ADDRESS"
	OptionMCPStandalonePort    = "MCP_STANDALONE_PORT"
)

// Option is a key/value settings row.
type Option struct {
	ID        string         `json:"id" gorm:"primaryKey;type:text"`
	Name      string         `json:"name" gorm:"type:text;uniqueIndex;not null"`
	Value     string         `json:"value" gorm:"type:text;not null"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (Option) TableName() string {
	return "options"
}

func (s *Option) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(s.ID) == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// GetOption returns the stored value for name. ok is false when missing.
func GetOption(db *gorm.DB, name string) (value string, ok bool, err error) {
	if db == nil {
		return "", false, errors.New("database unavailable")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false, errors.New("option name is required")
	}
	var row Option
	if err := db.Where("name = ?", name).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return row.Value, true, nil
}

// SetOption upserts a named option value.
func SetOption(db *gorm.DB, name, value string) error {
	if db == nil {
		return errors.New("database unavailable")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("option name is required")
	}
	var row Option
	err := db.Where("name = ?", name).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&Option{Name: name, Value: value}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&row).Update("value", value).Error
}

// GetOptionBool parses "true"/"1"/"yes" as true; missing → false, ok=false.
func GetOptionBool(db *gorm.DB, name string) (value bool, ok bool, err error) {
	raw, found, err := GetOption(db, name)
	if err != nil || !found {
		return false, found, err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, true, nil
	default:
		return false, true, nil
	}
}

// ModuleEnabled returns whether a module sidebar entry should be shown.
// Missing option defaults to false (opt-in). Call EnsureModuleSidebarDefaults
// on startup so existing installs keep menus and new installs stay off.
func ModuleEnabled(db *gorm.DB, name string) bool {
	v, ok, err := GetOptionBool(db, name)
	if err != nil || !ok {
		return false
	}
	return v
}

// BrewModuleEnabled reports whether the Brew Package module is on.
// Missing option defaults to false (opt-in).
func BrewModuleEnabled(db *gorm.DB) bool {
	v, ok, err := GetOptionBool(db, OptionBrewModuleEnabled)
	if err != nil || !ok {
		return false
	}
	return v
}

// LocalhostAutoLoginEnabled reports whether loopback auto-login is on.
// Missing option defaults to false (opt-in).
func LocalhostAutoLoginEnabled(db *gorm.DB) bool {
	v, ok, err := GetOptionBool(db, OptionLocalhostAutoLogin)
	if err != nil || !ok {
		return false
	}
	return v
}

// EnsureModuleSidebarDefaults seeds Docker / Kubernetes / Proxy Manager / Brew toggles.
// Brand-new installs (no users yet) always get false (off).
// Existing installs that already have users keep previously-visible modules on when
// the keys were never stored (one-time upgrade). Brew is always seeded off when missing.
func EnsureModuleSidebarDefaults(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if _, seeded, err := GetOption(db, OptionModuleSidebarDefaultsSeeded); err != nil {
		return err
	} else if seeded {
		// Still ensure Brew exists as off if somehow missing after an older seed.
		if _, found, err := GetOption(db, OptionBrewModuleEnabled); err != nil {
			return err
		} else if !found {
			if err := SetOptionBool(db, OptionBrewModuleEnabled, false); err != nil {
				return err
			}
		}
		return nil
	}

	var userCount int64
	_ = db.Model(&User{}).Count(&userCount).Error
	// Only treat as upgrade when real users already exist — not merely because
	// other option rows were seeded (that wrongly flipped modules on for fresh DBs).
	legacyDefaultOn := userCount > 0

	keys := []string{
		OptionDockerModuleEnabled,
		OptionKubernetesModuleEnabled,
		OptionProxymanagerModuleEnabled,
		OptionBrewModuleEnabled,
	}
	for _, key := range keys {
		if _, found, err := GetOption(db, key); err != nil {
			return err
		} else if found {
			continue
		}
		value := legacyDefaultOn
		// Brew stays opt-in even for upgrades (never auto-enable).
		if key == OptionBrewModuleEnabled {
			value = false
		}
		if err := SetOptionBool(db, key, value); err != nil {
			return err
		}
	}
	return SetOption(db, OptionModuleSidebarDefaultsSeeded, "1")
}

// SetOptionBool stores a boolean option as "true" or "false".
func SetOptionBool(db *gorm.DB, name string, value bool) error {
	if value {
		return SetOption(db, name, "true")
	}
	return SetOption(db, name, "false")
}
