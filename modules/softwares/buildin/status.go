package buildin

import (
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

type toolStatus struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Details     string `json:"details"`
	Category    string `json:"category"`
	SubCategory string `json:"sub_category"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	Binary      string `json:"binary"`
	Installed   bool   `json:"installed"`
	PresentPath string `json:"present_path,omitempty"`
	SoftwareID  string `json:"software_id,omitempty"`
	Version     string `json:"version,omitempty"`
}

func listBuildinAPI(app *config.AppClients) fiber.Handler {
	return func(c fiber.Ctx) error {
		r := app.Render()
		return r.Api(c, r.WithData(fiber.Map{
			"data": CollectToolStatuses(app.DB()),
		}))
	}
}

func toolStatusAPI(app *config.AppClients, key string) fiber.Handler {
	return func(c fiber.Ctx) error {
		r := app.Render()
		st, ok := StatusForKey(app.DB(), key)
		if !ok {
			return r.Api(c, r.WithStatus(fiber.StatusNotFound), r.WithData(fiber.Map{
				"error":   true,
				"message": "build-in tool not found",
				"code":    "NOT_FOUND",
			}))
		}
		return r.Api(c, r.WithData(fiber.Map{"data": st}))
	}
}

// CollectToolStatuses resolves build-in tools against disk probes and the DB catalog.
func CollectToolStatuses(db *gorm.DB) []toolStatus {
	items := Catalog()
	out := make([]toolStatus, 0, len(items))

	var byName map[string]models.Software
	var installedMap map[string]string
	if db != nil {
		byName = map[string]models.Software{}
		var rows []models.Software
		if err := db.Where("name IN ?", Names()).Find(&rows).Error; err == nil {
			for _, row := range rows {
				byName[row.Name] = row
			}
		}
		if m, err := models.InstalledVersionMap(db); err == nil {
			installedMap = m
		}
	}

	for _, item := range items {
		st := toolStatus{
			Key:         item.Key,
			Name:        item.Software.Name,
			Details:     item.Software.Details,
			Category:    item.Software.Category,
			SubCategory: item.Software.SubCategory,
			Icon:        item.Software.Icon,
			Color:       item.Software.Color,
			Binary:      item.Binary,
		}

		probe := softwaresync.ProbeInstalled(item.Software.Name, item.Software.ServiceUnits)
		if !probe.Present && item.Binary != "" {
			if p, err := exec.LookPath(item.Binary); err == nil && strings.TrimSpace(p) != "" {
				probe = softwaresync.ProbeResult{Present: true, Detail: p}
			}
		}
		st.Installed = probe.Present
		st.PresentPath = probe.Detail

		if sw, ok := byName[item.Software.Name]; ok {
			st.SoftwareID = sw.ID
			if installedMap != nil {
				if verID, ok := installedMap[sw.ID]; ok && verID != "" {
					var ver models.SoftwareVersion
					if err := db.Where("id = ?", verID).First(&ver).Error; err == nil {
						st.Version = ver.Version
					}
				}
			}
			if st.Version == "" && len(item.Versions) > 0 {
				for _, v := range item.Versions {
					if v.IsLatest {
						st.Version = v.Version
						break
					}
				}
			}
		}
		// Disk presence is the source of truth for "ready" on the dashboard.
		st.Installed = probe.Present
		out = append(out, st)
	}
	return out
}

// StatusForKey returns one tool status by endpoint key.
func StatusForKey(db *gorm.DB, key string) (toolStatus, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, st := range CollectToolStatuses(db) {
		if st.Key == key {
			return st, true
		}
	}
	return toolStatus{}, false
}
