package buildin

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/softwares/buildin/gpustat"
	"github.com/izetmolla/containerws/modules/softwares/buildin/htop"
	"github.com/izetmolla/containerws/modules/softwares/buildin/iftop"
	"github.com/izetmolla/containerws/modules/softwares/buildin/intelgputools"
	"github.com/izetmolla/containerws/modules/softwares/buildin/nvtop"
	"github.com/izetmolla/containerws/modules/softwares/buildin/radeontop"
	"github.com/izetmolla/containerws/modules/softwares/buildin/sysstat"
)

// Catalog returns every built-in monitoring/tool software definition.
func Catalog() []CatalogItem {
	htopCat, htopSub := htop.Category()
	sysCat, sysSub := sysstat.Category()
	iftopCat, iftopSub := iftop.Category()
	nvtopCat, nvtopSub := nvtop.Category()
	igtCat, igtSub := intelgputools.Category()
	radeonCat, radeonSub := radeontop.Category()
	gpustatCat, gpustatSub := gpustat.Category()
	return []CatalogItem{
		itemFrom(
			htop.Key, htop.Binary, htop.Name,
			htop.Details(), htop.Icon(), htop.Color(), htop.Order(),
			htop.Tags(), htop.InstallScript(), htop.UninstallScript(),
			htopCat, htopSub,
		),
		itemFrom(
			sysstat.Key, sysstat.Binary, sysstat.Name,
			sysstat.Details(), sysstat.Icon(), sysstat.Color(), sysstat.Order(),
			sysstat.Tags(), sysstat.InstallScript(), sysstat.UninstallScript(),
			sysCat, sysSub,
		),
		itemFrom(
			iftop.Key, iftop.Binary, iftop.Name,
			iftop.Details(), iftop.Icon(), iftop.Color(), iftop.Order(),
			iftop.Tags(), iftop.InstallScript(), iftop.UninstallScript(),
			iftopCat, iftopSub,
		),
		itemFrom(
			nvtop.Key, nvtop.Binary, nvtop.Name,
			nvtop.Details(), nvtop.Icon(), nvtop.Color(), nvtop.Order(),
			nvtop.Tags(), nvtop.InstallScript(), nvtop.UninstallScript(),
			nvtopCat, nvtopSub,
		),
		itemFrom(
			intelgputools.Key, intelgputools.Binary, intelgputools.Name,
			intelgputools.Details(), intelgputools.Icon(), intelgputools.Color(), intelgputools.Order(),
			intelgputools.Tags(), intelgputools.InstallScript(), intelgputools.UninstallScript(),
			igtCat, igtSub,
		),
		itemFrom(
			radeontop.Key, radeontop.Binary, radeontop.Name,
			radeontop.Details(), radeontop.Icon(), radeontop.Color(), radeontop.Order(),
			radeontop.Tags(), radeontop.InstallScript(), radeontop.UninstallScript(),
			radeonCat, radeonSub,
		),
		itemFrom(
			gpustat.Key, gpustat.Binary, gpustat.Name,
			gpustat.Details(), gpustat.Icon(), gpustat.Color(), gpustat.Order(),
			gpustat.Tags(), gpustat.InstallScript(), gpustat.UninstallScript(),
			gpustatCat, gpustatSub,
		),
	}
}

func itemFrom(
	key, binary, name, details, icon, color string,
	order int,
	tags []string,
	installScript, uninstallScript string,
	category, subCategory string,
) CatalogItem {
	return CatalogItem{
		Key:    key,
		Binary: binary,
		Software: SoftwareMeta{
			Name:        name,
			Details:     details,
			Category:    category,
			SubCategory: subCategory,
			Tags:        tags,
			Icon:        icon,
			Color:       color,
			Order:       order,
			IsActive:    true,
		},
		Versions: []VersionMeta{{
			Version:         "apt",
			IsLatest:        true,
			InstallScript:   installScript,
			UninstallScript: uninstallScript,
			OS:              "linux",
			PackageFamily:   "apt",
		}},
	}
}

// Names returns catalog software names in registration order.
func Names() []string {
	items := Catalog()
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Software.Name)
	}
	return out
}

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	api := router.Group("/buildin")
	api.Get("/", listBuildinAPI(appClients))

	// One status endpoint per tool folder key.
	for _, key := range []string{
		htop.Key,
		sysstat.Key,
		iftop.Key,
		nvtop.Key,
		intelgputools.Key,
		radeontop.Key,
		gpustat.Key,
	} {
		k := key
		api.Get("/"+k, toolStatusAPI(appClients, k))
	}
}
