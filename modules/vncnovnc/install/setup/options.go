package setup

import (
	"log"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// OptionsSnapshot is the persisted VNC install state from the options table.
type OptionsSnapshot struct {
	Installed bool `json:"installed"`
	Present   bool `json:"present"`
	Missing   bool `json:"missing"` // installed but packages absent on host
}

// ReadOptions loads VNC_INSTALLED / VNC_SOFTWARE_PRESENT.
func ReadOptions(db *gorm.DB) OptionsSnapshot {
	out := OptionsSnapshot{}
	if db == nil {
		return out
	}
	installed, _, _ := models.GetOptionBool(db, models.OptionVNCInstalled)
	present, _, _ := models.GetOptionBool(db, models.OptionVNCSoftwarePresent)
	out.Installed = installed
	out.Present = present
	out.Missing = installed && !present
	return out
}

// SyncOptionsFromStatus updates option rows from a live CheckStatus probe.
// - Always sets VNC_SOFTWARE_PRESENT to status.Ready
// - Sets VNC_INSTALLED=true when packages are ready (successful install / already present)
// - Keeps VNC_INSTALLED=true when previously installed even if packages are now missing
func SyncOptionsFromStatus(db *gorm.DB, status StatusReport) OptionsSnapshot {
	if db == nil {
		return OptionsSnapshot{Present: status.Ready}
	}
	_ = models.SetOptionBool(db, models.OptionVNCSoftwarePresent, status.Ready)

	installed, _, _ := models.GetOptionBool(db, models.OptionVNCInstalled)
	if status.Ready {
		_ = models.SetOptionBool(db, models.OptionVNCInstalled, true)
		installed = true
	}
	return OptionsSnapshot{
		Installed: installed,
		Present:   status.Ready,
		Missing:   installed && !status.Ready,
	}
}

// MarkInstalled records a successful package setup.
func MarkInstalled(db *gorm.DB) {
	if db == nil {
		return
	}
	if err := models.SetOptionBool(db, models.OptionVNCInstalled, true); err != nil {
		log.Printf("vnc setup: set %s: %v", models.OptionVNCInstalled, err)
	}
	if err := models.SetOptionBool(db, models.OptionVNCSoftwarePresent, true); err != nil {
		log.Printf("vnc setup: set %s: %v", models.OptionVNCSoftwarePresent, err)
	}
}
