package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/seed"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var softwareListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List softwares in the catalog",
	Args:    cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, appClients *config.AppClients) error {
		seed.SeedIfEmpty(appClients)

		includeAll, _ := cmd.Flags().GetBool("all")
		asJSON, _ := cmd.Flags().GetBool("json")

		q := appClients.DB().Model(&models.Software{})
		if !includeAll {
			q = q.Where("is_active = ?", true)
		}

		var rows []models.Software
		if err := q.Find(&rows).Error; err != nil {
			return err
		}

		items := make([]softwareListRow, 0, len(rows))
		installedMap, err := models.InstalledVersionMap(appClients.DB())
		if err != nil {
			return err
		}
		for _, sw := range rows {
			item := softwareListRow{Software: sw}
			versions, verr := loadSoftwareVersions(appClients.DB(), sw.ID)
			if verr != nil {
				return verr
			}
			if len(versions) > 0 {
				latest := versions[0]
				item.LatestVersion = latest.Version
				if installedID, ok := installedMap[sw.ID]; ok {
					item.IsInstalled = true
					item.HasUpdate = models.HasSoftwareUpdate(installedID, latest.ID)
					for i := range versions {
						if versions[i].ID == installedID {
							item.InstalledVersion = versions[i].Version
							break
						}
					}
				}
			}
			if len(sw.ServiceUnits) > 0 {
				st := service.ProbeUnits([]string(sw.ServiceUnits))
				item.ServiceOverall = st.Overall
			}
			items = append(items, item)
		}

		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Order != items[j].Order {
				return items[i].Order < items[j].Order
			}
			return items[i].Name < items[j].Name
		})

		if asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(items)
		}
		printSoftwareList(items)
		return nil
	}),
}

type softwareListRow struct {
	models.Software
	LatestVersion    string `json:"latest_version"`
	InstalledVersion string `json:"installed_version,omitempty"`
	IsInstalled      bool   `json:"is_installed"`
	HasUpdate        bool   `json:"has_update"`
	ServiceOverall   string `json:"service_overall,omitempty"`
}

func loadSoftwareVersions(db *gorm.DB, softwareID string) ([]models.SoftwareVersion, error) {
	var versions []models.SoftwareVersion
	err := db.Where("software_id = ?", softwareID).
		Order("is_latest DESC, created_at DESC").
		Find(&versions).Error
	return versions, err
}

func printSoftwareList(items []softwareListRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tCategory\tLatest\tInstalled\tUpdate\tService\tActive")

	for _, item := range items {
		latest := item.LatestVersion
		if latest == "" {
			latest = "-"
		}
		installed := item.InstalledVersion
		if installed == "" {
			if item.IsInstalled {
				installed = "yes"
			} else {
				installed = "-"
			}
		}
		update := "no"
		if item.HasUpdate {
			update = "yes"
		}
		active := "yes"
		if !item.IsActive {
			active = "no"
		}
		svc := item.ServiceOverall
		if svc == "" {
			svc = "-"
		}
		category := strings.TrimSpace(item.Category)
		if item.SubCategory != "" {
			if category == "" {
				category = item.SubCategory
			} else {
				category = category + "/" + item.SubCategory
			}
		}
		if category == "" {
			category = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Name,
			category,
			latest,
			installed,
			update,
			svc,
			active,
		)
	}
	_ = w.Flush()
}
