package install

import (
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

// markVersionInstalled records a successful install in software_installed.
func markVersionInstalled(db *gorm.DB, softwareID, versionID string) error {
	if err := models.MarkSoftwareInstalled(db, softwareID, versionID); err != nil {
		return err
	}
	softwaresync.ClearOsMissing(softwareID, versionID)
	return nil
}

// CancelPendingInstallsForSoftware drops pending auto-install queue items for
// softwareID so an uninstall is not immediately re-queued.
func CancelPendingInstallsForSoftware(softwareID string) {
	softwareID = strings.TrimSpace(softwareID)
	if softwareID == "" {
		return
	}
	bulkQueue.mu.Lock()
	defer bulkQueue.mu.Unlock()
	next := make([]*queueItem, 0, len(bulkQueue.items))
	for _, it := range bulkQueue.items {
		if it == nil {
			continue
		}
		if it.SoftwareID == softwareID &&
			it.Action == QueueActionInstall &&
			it.Status == "pending" {
			continue
		}
		next = append(next, it)
	}
	bulkQueue.items = next
}

// clearStickyInstallOptions clears host options that would force a background
// reinstall after the user uninstalls related catalog software.
func clearStickyInstallOptions(db *gorm.DB, softwareName string) {
	if db == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(softwareName))
	switch {
	case name == "vs code server" || strings.Contains(name, "vs code server"):
		_ = models.SetOptionBool(db, models.OptionCodeserverInstalled, false)
		_ = models.SetOptionBool(db, models.OptionCodeserverSoftwarePresent, false)
	case strings.Contains(name, "novnc") ||
		(strings.Contains(name, "vnc") && !strings.Contains(name, "vscode")):
		_ = models.SetOptionBool(db, models.OptionVNCInstalled, false)
		_ = models.SetOptionBool(db, models.OptionVNCSoftwarePresent, false)
	}
}

