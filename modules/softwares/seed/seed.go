package seed

import (
	"log"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/buildin"
)

type SoftwareMeta struct {
	Name           string
	Details        string
	Category       string
	SubCategory    string
	Tags           []string
	ServiceUnits   []string // systemd units managed after install (e.g. docker.service)
	CanControl     bool     // expose Start/Stop/Restart when true + ServiceUnits/commands set
	ControlBackend string   // systemd | docker | ""
	StartCommand   string
	RestartCommand string
	StopCommand    string
	Icon           string
	Image          string
	Color          string
	Order          int
	IsActive       bool
}

type VersionMeta struct {
	Version         string
	IsLatest        bool
	InstallScript   string
	UninstallScript string
	UpgradeScript   string
	CustomScript    string

	OS               string
	OsVersion        string
	Distro           string
	DistroID         string
	DistroVersion    string
	Arch             string
	Platform         string
	PackageFamily    string
	Kernel           string
	Virtualization   string
	ContainerRuntime string
	CloudProvider    string
}

type catalogItem struct {
	Software SoftwareMeta
	Versions []VersionMeta
}

func catalog() []catalogItem {
	items := []catalogItem{
		goCatalogItem(),
		nodeCatalogItem(),
		dockerCatalogItem(),
		chromeCatalogItem(),
		vscodeCatalogItem(),
		vscodeServerCatalogItem(),
		cursoraiCatalogItem(),
	}
	for _, b := range buildin.Catalog() {
		items = append(items, catalogItemFromBuildin(b))
	}
	return items
}

func catalogItemFromBuildin(b buildin.CatalogItem) catalogItem {
	versions := make([]VersionMeta, 0, len(b.Versions))
	for _, v := range b.Versions {
		vm := VersionMeta{
			Version:         v.Version,
			IsLatest:        v.IsLatest,
			InstallScript:   v.InstallScript,
			UninstallScript: v.UninstallScript,
			OS:              v.OS,
			OsVersion:       v.OsVersion,
			Distro:          v.Distro,
			DistroID:        v.DistroID,
			DistroVersion:   v.DistroVersion,
			Arch:            v.Arch,
			Platform:        v.Platform,
			PackageFamily:   v.PackageFamily,
		}
		applyLinuxAptTargets(&vm)
		versions = append(versions, vm)
	}
	return catalogItem{
		Software: SoftwareMeta{
			Name:           b.Software.Name,
			Details:        b.Software.Details,
			Category:       b.Software.Category,
			SubCategory:    b.Software.SubCategory,
			Tags:           b.Software.Tags,
			ServiceUnits:   b.Software.ServiceUnits,
			CanControl:     b.Software.CanControl,
			ControlBackend: b.Software.ControlBackend,
			StartCommand:   b.Software.StartCommand,
			RestartCommand: b.Software.RestartCommand,
			StopCommand:    b.Software.StopCommand,
			Icon:           b.Software.Icon,
			Color:          b.Software.Color,
			Order:          b.Software.Order,
			IsActive:       b.Software.IsActive,
		},
		Versions: versions,
	}
}

// SeedIfEmpty upserts the software catalog and deactivates items not in it.
func SeedIfEmpty(appClients *config.AppClients) {
	db := appClients.DB()
	if db == nil {
		return
	}

	samples := catalog()
	keepNames := make([]string, 0, len(samples))
	for _, sample := range samples {
		keepNames = append(keepNames, sample.Software.Name)
		var existing models.Software
		err := db.Where("name = ?", sample.Software.Name).First(&existing).Error
		if err == nil {
			_ = db.Model(&existing).Updates(map[string]any{
				"details":          sample.Software.Details,
				"category":         sample.Software.Category,
				"sub_category":     sample.Software.SubCategory,
				"tags":             models.JSONBStringArray(sample.Software.Tags),
				"service_units":    models.JSONBStringArray(sample.Software.ServiceUnits),
				"can_control":      sample.Software.CanControl,
				"control_backend":  sample.Software.ControlBackend,
				"start_command":    sample.Software.StartCommand,
				"restart_command":  sample.Software.RestartCommand,
				"stop_command":     sample.Software.StopCommand,
				"icon":             sample.Software.Icon,
				"image":            sample.Software.Image,
				"color":            sample.Software.Color,
				"order":            sample.Software.Order,
				"is_active":        sample.Software.IsActive,
			}).Error

			for _, ver := range sample.Versions {
				applyLinuxAptTargets(&ver)
				var existingVer models.SoftwareVersion
				verr := db.Where("software_id = ? AND version = ?", existing.ID, ver.Version).First(&existingVer).Error
				if verr == nil {
					_ = db.Model(&existingVer).Updates(map[string]any{
						"is_latest":          ver.IsLatest,
						"install_script":     ver.InstallScript,
						"uninstall_script":   ver.UninstallScript,
						"upgrade_script":     ver.UpgradeScript,
						"custom_script":      ver.CustomScript,
						"os":                 ver.OS,
						"os_version":         ver.OsVersion,
						"distro":             ver.Distro,
						"distro_id":          ver.DistroID,
						"distro_version":     ver.DistroVersion,
						"arch":               ver.Arch,
						"platform":           ver.Platform,
						"package_family":     ver.PackageFamily,
						"kernel":             ver.Kernel,
						"virtualization":     ver.Virtualization,
						"container_runtime":  ver.ContainerRuntime,
						"cloud_provider":     ver.CloudProvider,
					}).Error
				} else {
					row := models.SoftwareVersion{
						SoftwareID:         existing.ID,
						Version:            ver.Version,
						IsLatest:           ver.IsLatest,
						InstallScript:      ver.InstallScript,
						UninstallScript:    ver.UninstallScript,
						UpgradeScript:      ver.UpgradeScript,
						CustomScript:       ver.CustomScript,
						OS:                 ver.OS,
						OsVersion:          ver.OsVersion,
						Distro:             ver.Distro,
						DistroID:           ver.DistroID,
						DistroVersion:      ver.DistroVersion,
						Arch:               ver.Arch,
						Platform:           ver.Platform,
						PackageFamily:      ver.PackageFamily,
						Kernel:             ver.Kernel,
						Virtualization:     ver.Virtualization,
						ContainerRuntime:   ver.ContainerRuntime,
						CloudProvider:      ver.CloudProvider,
					}
					if err := db.Omit("Software").Create(&row).Error; err != nil {
						log.Printf("softwares seed: create version %q for %q failed: %v", ver.Version, sample.Software.Name, err)
					}
				}
				if ver.IsLatest {
					_ = db.Model(&models.SoftwareVersion{}).
						Where("software_id = ? AND version <> ?", existing.ID, ver.Version).
						Update("is_latest", false).Error
				}
			}
			continue
		}

		sw := models.Software{
			Name:           sample.Software.Name,
			Details:        sample.Software.Details,
			Category:       sample.Software.Category,
			SubCategory:    sample.Software.SubCategory,
			Tags:           models.JSONBStringArray(sample.Software.Tags),
			ServiceUnits:   models.JSONBStringArray(sample.Software.ServiceUnits),
			CanControl:     sample.Software.CanControl,
			ControlBackend: sample.Software.ControlBackend,
			StartCommand:   sample.Software.StartCommand,
			RestartCommand: sample.Software.RestartCommand,
			StopCommand:    sample.Software.StopCommand,
			Icon:           sample.Software.Icon,
			Image:          sample.Software.Image,
			Color:          sample.Software.Color,
			Order:          sample.Software.Order,
			IsActive:       sample.Software.IsActive,
		}
		if err := db.Create(&sw).Error; err != nil {
			log.Printf("softwares seed: create software %q failed: %v", sw.Name, err)
			continue
		}
		for _, ver := range sample.Versions {
			applyLinuxAptTargets(&ver)
			row := models.SoftwareVersion{
				SoftwareID:       sw.ID,
				Version:          ver.Version,
				IsLatest:         ver.IsLatest,
				InstallScript:    ver.InstallScript,
				UninstallScript:  ver.UninstallScript,
				UpgradeScript:    ver.UpgradeScript,
				CustomScript:     ver.CustomScript,
				OS:               ver.OS,
				OsVersion:        ver.OsVersion,
				Distro:           ver.Distro,
				DistroID:         ver.DistroID,
				DistroVersion:    ver.DistroVersion,
				Arch:             ver.Arch,
				Platform:         ver.Platform,
				PackageFamily:    ver.PackageFamily,
				Kernel:           ver.Kernel,
				Virtualization:   ver.Virtualization,
				ContainerRuntime: ver.ContainerRuntime,
				CloudProvider:    ver.CloudProvider,
			}
			if err := db.Omit("Software").Create(&row).Error; err != nil {
				log.Printf("softwares seed: create version %q for %q failed: %v", ver.Version, sw.Name, err)
			}
		}
		log.Printf("softwares seed: inserted %q", sw.Name)
	}

	if len(keepNames) > 0 {
		_ = db.Model(&models.Software{}).
			Where("name NOT IN ?", keepNames).
			Update("is_active", false).Error
	}
}

// RefreshShortDemoScripts is retained for startup wiring; catalog upsert
// in SeedIfEmpty already keeps install scripts current.
func RefreshShortDemoScripts(appClients *config.AppClients) {}
