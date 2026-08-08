package list

import (
	"log"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// OptionsSnapshot is the persisted codeserver install state from the options table.
type OptionsSnapshot struct {
	Installed bool `json:"installed"`
	Present   bool `json:"present"`
	Missing   bool `json:"missing"` // installed but packages absent on host
}

// ReadOptions loads CODESERVER_INSTALLED / CODESERVER_SOFTWARE_PRESENT.
func ReadOptions(db *gorm.DB) OptionsSnapshot {
	out := OptionsSnapshot{}
	if db == nil {
		return out
	}
	installed, _, _ := models.GetOptionBool(db, models.OptionCodeserverInstalled)
	present, _, _ := models.GetOptionBool(db, models.OptionCodeserverSoftwarePresent)
	out.Installed = installed
	out.Present = present
	out.Missing = installed && !present
	return out
}

// SyncOptionsFromProbe updates option rows from a live CLI probe.
// - Always sets CODESERVER_SOFTWARE_PRESENT to present
// - Sets CODESERVER_INSTALLED=true when the CLI is present
// - Keeps CODESERVER_INSTALLED=true when previously installed even if CLI is now missing
func SyncOptionsFromProbe(db *gorm.DB, present bool) OptionsSnapshot {
	if db == nil {
		return OptionsSnapshot{Present: present}
	}
	_ = models.SetOptionBool(db, models.OptionCodeserverSoftwarePresent, present)

	installed, _, _ := models.GetOptionBool(db, models.OptionCodeserverInstalled)
	if present {
		_ = models.SetOptionBool(db, models.OptionCodeserverInstalled, true)
		installed = true
	}
	return OptionsSnapshot{
		Installed: installed,
		Present:   present,
		Missing:   installed && !present,
	}
}

// MarkInstalled records a successful VS Code Server install.
func MarkInstalled(db *gorm.DB) {
	if db == nil {
		return
	}
	if err := models.SetOptionBool(db, models.OptionCodeserverInstalled, true); err != nil {
		log.Printf("codeserver: set %s: %v", models.OptionCodeserverInstalled, err)
	}
	if err := models.SetOptionBool(db, models.OptionCodeserverSoftwarePresent, true); err != nil {
		log.Printf("codeserver: set %s: %v", models.OptionCodeserverSoftwarePresent, err)
	}
}
