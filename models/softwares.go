package models

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

// Control backend identifiers for Softwares start/stop/restart.
const (
	SoftwareControlBackendSystemd = "systemd"
	SoftwareControlBackendDocker  = "docker"
)

// Server specific settings.
type Software struct {
	ID           string           `json:"id" gorm:"primaryKey;type:text"`
	Name         string           `json:"name" gorm:"size:255;"`
	Details      string           `json:"details" gorm:"type:text;"`
	Category     string           `json:"category" gorm:"size:255;"`
	SubCategory  string           `json:"sub_category" gorm:"size:255;"`
	Tags         JSONBStringArray `json:"tags" gorm:"type:text;default:'[]';"`
	ServiceUnits JSONBStringArray `json:"service_units" gorm:"type:text;default:'[]';"`
	// CanControl marks Softwares that expose Start / Stop / Restart (+ service logs).
	// Requires service units and/or explicit start/restart/stop commands.
	CanControl bool `json:"can_control" gorm:"default:false;index"`
	// ControlBackend documents how control runs: systemd | docker | "" (auto).
	ControlBackend string `json:"control_backend" gorm:"size:32;default:''"`
	// Explicit shell commands for service control (preferred over inferred systemctl).
	StartCommand   string `json:"start_command" gorm:"type:text;default:''"`
	RestartCommand string `json:"restart_command" gorm:"type:text;default:''"`
	StopCommand    string `json:"stop_command" gorm:"type:text;default:''"`
	Icon           string `json:"icon" gorm:"size:255;"`
	Image          string `json:"image" gorm:"type:text;default:''"` // logo URL or data:image URI
	Color          string `json:"color" gorm:"size:255;"`
	Order          int    `json:"order" gorm:"default:0;"`
	IsActive       bool   `json:"is_active" gorm:"default:true;"`

	// RegistryPackageID is the software_packages row this software was imported from.
	RegistryPackageID string `json:"registry_package_id,omitempty" gorm:"size:255;index;default:''"`
	// RegistrySlug is the softwares/{slug}/ folder in the GitHub registry (may differ from Name).
	RegistrySlug string `json:"registry_slug,omitempty" gorm:"size:255;index;default:''"`

	Versions []SoftwareVersion `json:"versions" gorm:"foreignKey:SoftwareID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *Software) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.NormalizeControl()
	return
}

func (b *Software) BeforeSave(tx *gorm.DB) (err error) {
	b.NormalizeControl()
	return
}

// NormalizeControl fills control_backend and default start/restart/stop commands
// from service units when controllable.
func (b *Software) NormalizeControl() {
	if b == nil {
		return
	}
	b.ControlBackend = strings.ToLower(strings.TrimSpace(b.ControlBackend))
	switch b.ControlBackend {
	case SoftwareControlBackendSystemd, SoftwareControlBackendDocker, "":
	default:
		b.ControlBackend = ""
	}
	b.StartCommand = strings.TrimSpace(b.StartCommand)
	b.RestartCommand = strings.TrimSpace(b.RestartCommand)
	b.StopCommand = strings.TrimSpace(b.StopCommand)
	if b.CanControl && b.ControlBackend == "" && HasSoftwareServiceUnits(b.ServiceUnits) {
		b.ControlBackend = InferSoftwareControlBackend(b.ServiceUnits)
	}
	if b.CanControl && HasSoftwareServiceUnits(b.ServiceUnits) {
		start, restart, stop := DefaultSystemdCommands(b.ServiceUnits)
		if b.StartCommand == "" {
			b.StartCommand = start
		}
		if b.RestartCommand == "" {
			b.RestartCommand = restart
		}
		if b.StopCommand == "" {
			b.StopCommand = stop
		}
	}
}

// IsControllable is true when Start/Stop/Restart should be offered.
func (b *Software) IsControllable() bool {
	if b == nil || !b.CanControl {
		return false
	}
	if HasSoftwareServiceUnits(b.ServiceUnits) {
		return true
	}
	return b.HasControlCommands()
}

// HasControlCommands reports whether any explicit control command is set.
func (b *Software) HasControlCommands() bool {
	if b == nil {
		return false
	}
	return strings.TrimSpace(b.StartCommand) != "" ||
		strings.TrimSpace(b.RestartCommand) != "" ||
		strings.TrimSpace(b.StopCommand) != ""
}

// CommandFor returns the configured shell command for start|stop|restart.
func (b *Software) CommandFor(action string) string {
	if b == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		return strings.TrimSpace(b.StartCommand)
	case "restart":
		return strings.TrimSpace(b.RestartCommand)
	case "stop":
		return strings.TrimSpace(b.StopCommand)
	default:
		return ""
	}
}

// HasSoftwareServiceUnits reports whether any non-blank unit is configured.
func HasSoftwareServiceUnits(units JSONBStringArray) bool {
	for _, u := range units {
		if strings.TrimSpace(u) != "" {
			return true
		}
	}
	return false
}

// DefaultSystemdCommands builds systemctl start/restart/stop lines for units.
func DefaultSystemdCommands(units JSONBStringArray) (start, restart, stop string) {
	clean := make([]string, 0, len(units))
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		return "", "", ""
	}
	joined := strings.Join(clean, " ")
	return "systemctl start " + joined,
		"systemctl restart " + joined,
		"systemctl stop " + joined
}

// InferSoftwareControlBackend picks docker vs systemd from unit names.
func InferSoftwareControlBackend(units JSONBStringArray) string {
	for _, u := range units {
		u = strings.ToLower(strings.TrimSpace(u))
		if u == "docker.service" || u == "docker.socket" || u == "docker" {
			return SoftwareControlBackendDocker
		}
	}
	if HasSoftwareServiceUnits(units) {
		return SoftwareControlBackendSystemd
	}
	return ""
}

func (b Software) TableName() string {
	return "softwares"
}
