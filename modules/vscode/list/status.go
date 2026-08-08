package list

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

const vscodeServerSoftwareName = "VS Code Server"

// GetCodeserverStatusAPI reports whether the Microsoft VS Code Server CLI is
// present on this host and returns the catalog software id for install.
func (cc *controller) GetCodeserverStatusAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	probe := softwaresync.ProbeInstalled(vscodeServerSoftwareName, nil)
	opts := SyncOptionsFromProbe(db, probe.Present)
	queue := swinstall.ActiveQueue(db)

	var sw models.Software
	err := db.WithContext(ctx).
		Where("LOWER(name) = ? AND is_active = ?", strings.ToLower(vscodeServerSoftwareName), true).
		First(&sw).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	softwareID := ""
	softwareName := vscodeServerSoftwareName
	if err == nil {
		softwareID = sw.ID
		if strings.TrimSpace(sw.Name) != "" {
			softwareName = sw.Name
		}
	}

	cli := `cws software install "VS Code Server"`

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"installed":          probe.Present,
			"option_installed":   opts.Installed,
			"present":            opts.Present,
			"missing":            opts.Missing,
			"detail":             probe.Detail,
			"software_id":        softwareID,
			"software_name":      softwareName,
			"cli_command":        cli,
			"software_queue":     queue,
			"softwaresync_ready": softwaresync.Ready(),
		},
	}))
}
