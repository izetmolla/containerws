package authorization

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/machine"
)

// BrandingAPI returns public workspace / OS labels for the auth screens.
func (cc *controller) BrandingAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()

	workspaceName, _, _ := models.GetOption(db, models.OptionWorkspaceName)
	workspaceName = strings.TrimSpace(workspaceName)
	if workspaceName == "" {
		workspaceName = "Container Workspace"
	}

	snap := machine.Detect()
	osName := strings.TrimSpace(snap.Distro)
	if osName == "" {
		osName = strings.TrimSpace(snap.OS)
	}
	if osName == "" {
		osName = workspaceName
	}
	osLabel := osName
	if ver := strings.TrimSpace(snap.DistroVersion); ver != "" && !strings.Contains(osName, ver) {
		osLabel = osName + " " + ver
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"workspace_name":        workspaceName,
			"os_name":               osName,
			"os_label":              osLabel,
			"os_version":            snap.DistroVersion,
			"hostname":              snap.Hostname,
			"localhost_auto_login":  models.LocalhostAutoLoginEnabled(db),
			"localhost_eligible":    c.IsFromLocal() && models.LocalhostAutoLoginEnabled(db),
		},
	}))
}
