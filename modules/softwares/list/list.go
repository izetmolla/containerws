package list

import (
	"context"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/brew"
	"github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

type softwareListItem struct {
	models.Software
	LatestVersion    *models.SoftwareVersion `json:"latest_version"`
	InstalledVersion *models.SoftwareVersion `json:"installed_version,omitempty"`
	IsInstalled      bool                    `json:"is_installed"`
	Uninstalled      bool                    `json:"uninstalled"`
	HasUpdate        bool                    `json:"has_update"`
	CanUninstall     bool                    `json:"can_uninstall"`
	CanControl       bool                    `json:"can_control"`
	OsMissing        bool                    `json:"os_missing"`
	ServiceStatus    *service.Status         `json:"service_status,omitempty"`
	// Source is local | remote | both (present in DB and registry index).
	Source string `json:"source"`
	// IsRemote is true when the item exists only in the GitHub registry (not imported yet).
	IsRemote bool `json:"is_remote"`
	// PackageID is the software_packages row used for remote-only items (for import).
	PackageID string `json:"package_id,omitempty"`
	// PackageManager is local | brew when installed (empty when not installed).
	PackageManager string `json:"package_manager,omitempty"`
	// BrewAvailable is true when an exact Homebrew formula token matches this software.
	BrewAvailable bool `json:"brew_available"`
	// CanSwitchToBrew / CanSwitchToLocal for Softwares ↔ Brew ownership switch.
	CanSwitchToBrew  bool `json:"can_switch_to_brew"`
	CanSwitchToLocal bool `json:"can_switch_to_local"`
}

type categoryFacet struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type listFacets struct {
	Categories  []categoryFacet `json:"categories"`
	UpdateCount int             `json:"update_count"`
	TotalActive int             `json:"total_active"`
	RemoteCount int             `json:"remote_count"`
}

func (cc *controller) GetSoftwaresListAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 12)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 12
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(c.Query("q"))
	category := strings.TrimSpace(c.Query("category"))
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	sortKey := strings.ToLower(strings.TrimSpace(c.Query("sort")))
	if sortKey == "" {
		sortKey = "order"
	}
	sourceFilter := strings.ToLower(strings.TrimSpace(c.Query("source"))) // all | local | remote
	if sourceFilter == "" {
		sourceFilter = "local"
	}

	host := install.CurrentHostIdentity()
	hostFacts := softwarepkg.HostFacts{
		DistroID:      host.DistroID,
		DistroVersion: host.DistroVersion,
		Arch:          host.Arch,
	}

	query := db.WithContext(ctx).Model(&models.Software{}).Where("is_active = ?", true)
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		query = query.Where(
			"(LOWER(name) LIKE ? OR LOWER(details) LIKE ? OR LOWER(category) LIKE ? OR LOWER(sub_category) LIKE ? OR LOWER(tags) LIKE ?)",
			like, like, like, like, like,
		)
	}

	var rows []models.Software
	if err := query.Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	installedMap, err := models.InstalledVersionMap(db)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	uninstalledMap, err := models.UninstalledVersionMap(db)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	pmMap, err := models.PackageManagerMap(db)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out := make([]softwareListItem, 0, len(rows))
	localByName := map[string]int{} // lower name → index in out
	for _, sw := range rows {
		item, compatible := enrichSoftwareForHost(c, db, sw, installedMap, uninstalledMap, host)
		applyPackageManagerFields(&item, sw, pmMap)
		// Hide catalog entries that have no installable version for this machine,
		// unless already installed or intentionally uninstalled (so the user can still manage).
		if !compatible && !item.IsInstalled && !item.Uninstalled {
			continue
		}
		item.Source = "local"
		item.IsRemote = false
		if pid := strings.TrimSpace(sw.RegistryPackageID); pid != "" {
			item.PackageID = pid
		}
		localByName[strings.ToLower(strings.TrimSpace(sw.Name))] = len(out)
		out = append(out, item)
	}

	// Remote GitHub catalogs are expensive — only load when the user searches or
	// explicitly asks for all/remote sources. Pass refresh=1 (or search/remote) to
	// drop the in-memory index cache and re-fetch from registries.
	wantRemote := q != "" || sourceFilter == "all" || sourceFilter == "remote"
	refreshRemote := queryBool(c, "refresh") || q != "" || sourceFilter == "remote"
	remoteCount := 0
	if wantRemote {
		if refreshRemote {
			softwarepkg.InvalidateCatalogCache()
		}
		if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
			log.Printf("softwares list: default registry: %v", err)
		}
		if remotes, rerr := loadRemoteCatalog(ctx, db); rerr != nil {
			log.Printf("softwares list: remote catalog: %v", rerr)
		} else {
			compatibleRemotes := filterRemotesForHost(ctx, db, remotes, hostFacts)
			for _, entry := range compatibleRemotes {
				if q != "" && !softwarepkg.MatchQuery(entry.Meta, q) {
					continue
				}
				key := strings.ToLower(strings.TrimSpace(entry.Meta.Name))
				if idx, ok := localByName[key]; ok {
					out[idx].Source = "both"
					if out[idx].PackageID == "" {
						out[idx].PackageID = entry.PackageID
					}
					continue
				}
				out = append(out, remoteListItem(entry))
				remoteCount++
			}
		}
	}

	// Facets from host+search-filtered set (before category/status) so chips stay useful.
	facets := buildFacets(out)
	facets.RemoteCount = remoteCount

	if category != "" && !strings.EqualFold(category, "all") {
		filtered := make([]softwareListItem, 0, len(out))
		for _, item := range out {
			if strings.EqualFold(strings.TrimSpace(item.Category), category) {
				filtered = append(filtered, item)
			}
		}
		out = filtered
	}

	// source=local with an active search still includes matching remotes (wantRemote).
	if sourceFilter == "local" && q == "" {
		filtered := make([]softwareListItem, 0, len(out))
		for _, item := range out {
			if !item.IsRemote {
				filtered = append(filtered, item)
			}
		}
		out = filtered
	} else if sourceFilter == "remote" {
		filtered := make([]softwareListItem, 0, len(out))
		for _, item := range out {
			if item.IsRemote || item.Source == "both" {
				filtered = append(filtered, item)
			}
		}
		out = filtered
	}

	if status != "" && status != "all" {
		filtered := make([]softwareListItem, 0, len(out))
		for _, item := range out {
			switch status {
			case "installed":
				if item.IsInstalled && !item.Uninstalled {
					filtered = append(filtered, item)
				}
			case "uninstalled":
				if item.Uninstalled {
					filtered = append(filtered, item)
				}
			case "update_available":
				if item.HasUpdate && !item.Uninstalled {
					filtered = append(filtered, item)
				}
			case "not_installed":
				if !item.IsInstalled && !item.Uninstalled {
					filtered = append(filtered, item)
				}
			default:
				filtered = append(filtered, item)
			}
		}
		out = filtered
	}

	sortSoftwareItems(out, sortKey)

	total := len(out)
	totalPages := max(int(math.Ceil(float64(total)/float64(limit))), 1)
	if page > totalPages {
		page = totalPages
	}
	start := min((page-1)*limit, total)
	end := min(start+limit, total)
	pageItems := out[start:end]

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": pageItems,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"pageCount":   totalPages,
		},
		"facets": facets,
		"host": fiber.Map{
			"os":             host.OS,
			"distro":         host.Distro,
			"distro_id":      host.DistroID,
			"distro_version": host.DistroVersion,
			"arch":           host.Arch,
			"platform":       host.Platform,
			"package_family": host.PackageFamily,
		},
	}))
}

type remoteCatalogEntry struct {
	Meta      softwarepkg.PackageMeta
	PackageID string
}

func loadRemoteCatalog(ctx context.Context, db *gorm.DB) ([]remoteCatalogEntry, error) {
	if db == nil {
		return nil, nil
	}
	var regs []models.SoftwarePackage
	if err := db.WithContext(ctx).Order("created_at ASC").Find(&regs).Error; err != nil {
		return nil, err
	}
	if len(regs) == 0 {
		return nil, nil
	}
	// Merge catalogs from all registries; first occurrence of a name wins.
	seen := map[string]struct{}{}
	merged := make([]remoteCatalogEntry, 0)
	client := &softwarepkg.Client{}
	for _, reg := range regs {
		if strings.TrimSpace(reg.PackageURL) == "" {
			continue
		}
		items, err := softwarepkg.ListRemoteFromPackage(ctx, reg, "main", client)
		if err != nil {
			log.Printf("softwares list: registry %s: %v", reg.ID, err)
			continue
		}
		for _, m := range items {
			key := strings.ToLower(strings.TrimSpace(m.Name))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, remoteCatalogEntry{Meta: m, PackageID: reg.ID})
		}
	}
	return merged, nil
}

func remoteListItem(entry remoteCatalogEntry) softwareListItem {
	meta := entry.Meta
	active := true
	if meta.IsActive != nil {
		active = *meta.IsActive
	}
	canControl, backend := meta.ResolveControlFields()
	slug := strings.ToLower(strings.TrimSpace(meta.Name))
	slug = strings.ReplaceAll(slug, " ", "-")
	return softwareListItem{
		Software: models.Software{
			ID:             "remote:" + slug,
			Name:           meta.Name,
			Details:        meta.Details,
			Category:       meta.Category,
			SubCategory:    meta.SubCategory,
			Tags:           models.JSONBStringArray(meta.Tags),
			ServiceUnits:   models.JSONBStringArray(meta.ServiceUnits),
			CanControl:     canControl,
			ControlBackend: backend,
			Icon:           meta.Icon,
			Image:          meta.Image,
			Color:          meta.Color,
			Order:          meta.Order,
			IsActive:       active,
		},
		LatestVersion: nil,
		IsInstalled:   false,
		CanControl:    canControl,
		Source:        "remote",
		IsRemote:      true,
		PackageID:     entry.PackageID,
	}
}

func enrichSoftware(
	c fiber.Ctx,
	db *gorm.DB,
	sw models.Software,
	installedMap map[string]string,
) softwareListItem {
	uninstalledMap, _ := models.UninstalledVersionMap(db)
	item, _ := enrichSoftwareForHost(
		c, db, sw, installedMap, uninstalledMap, install.CurrentHostIdentity(),
	)
	return item
}

// enrichSoftwareForHost attaches install state and picks the best host-matching version.
// compatible is true when at least one SoftwareVersion targets this machine.
func enrichSoftwareForHost(
	c fiber.Ctx,
	db *gorm.DB,
	sw models.Software,
	installedMap map[string]string,
	uninstalledMap map[string]string,
	host models.HostIdentity,
) (softwareListItem, bool) {
	ctx := c.Context()
	item := softwareListItem{Software: sw}
	versions, verr := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if verr != nil {
		return item, false
	}

	matching := install.MatchingVersion(versions, host, true)
	compatible := matching != nil
	if matching != nil {
		latest := *matching
		latest.CanUninstall = strings.TrimSpace(latest.UninstallScript) != ""
		item.LatestVersion = &latest
	}

	if versionID, ok := uninstalledMap[sw.ID]; ok {
		item.Uninstalled = true
		item.IsInstalled = false
		item.OsMissing = false
		item.HasUpdate = false
		for i := range versions {
			if versions[i].ID == versionID {
				prev := versions[i]
				prev.IsInstalled = false
				prev.CanUninstall = strings.TrimSpace(prev.UninstallScript) != ""
				item.InstalledVersion = &prev
				break
			}
		}
		for i := range versions {
			if strings.TrimSpace(versions[i].UninstallScript) != "" {
				item.CanUninstall = false // already uninstalled
				break
			}
		}
		if sw.IsControllable() {
			st := service.ProbeUnits([]string(sw.ServiceUnits))
			item.ServiceStatus = &st
		}
		item.CanControl = false
		return item, compatible
	}

	if installedID, ok := installedMap[sw.ID]; ok {
		for i := range versions {
			if versions[i].ID == installedID {
				installed := versions[i]
				installed.IsInstalled = true
				if item.LatestVersion != nil {
					installed.HasUpdate = models.HasSoftwareUpdate(installed.ID, item.LatestVersion.ID)
				}
				installed.CanUninstall = strings.TrimSpace(installed.UninstallScript) != ""
				softwaresync.ApplyOsMissing(&installed)
				item.InstalledVersion = &installed
				item.IsInstalled = true
				item.HasUpdate = installed.HasUpdate
				item.CanUninstall = installed.CanUninstall
				item.OsMissing = installed.OsMissing
				break
			}
		}
		if !item.IsInstalled {
			item.IsInstalled = true
			if item.LatestVersion != nil {
				item.HasUpdate = models.HasSoftwareUpdate(installedID, item.LatestVersion.ID)
			}
			item.OsMissing = softwaresync.IsOsMissingSoftware(sw.ID) || softwaresync.IsOsMissing(installedID)
			for i := range versions {
				if strings.TrimSpace(versions[i].UninstallScript) != "" {
					item.CanUninstall = true
					break
				}
			}
		}
	}
	if sw.IsControllable() {
		st := service.ProbeUnits([]string(sw.ServiceUnits))
		item.ServiceStatus = &st
	}
	item.CanControl = service.CanControl(sw)
	return item, compatible
}

func applyPackageManagerFields(item *softwareListItem, sw models.Software, pmMap map[string]string) {
	if item == nil {
		return
	}
	token := brew.BrewTokenForSoftware(&sw)
	brewOK := token != "" && !strings.Contains(token, " ") && brew.FormulaExists(token)
	item.BrewAvailable = brewOK

	pm := ""
	if item.IsInstalled {
		pm = models.NormalizePackageManager(pmMap[sw.ID])
		item.PackageManager = pm
	}

	// Brew-owned: Softwares must not manage install/update/uninstall/service.
	if pm == models.PackageManagerBrew {
		item.CanUninstall = false
		item.CanControl = false
		item.HasUpdate = false
		item.CanSwitchToLocal = brewOK && item.LatestVersion != nil && strings.TrimSpace(item.LatestVersion.InstallScript) != ""
		item.CanSwitchToBrew = false
		return
	}

	item.CanSwitchToBrew = brewOK && item.IsInstalled && pm == models.PackageManagerLocal
	item.CanSwitchToLocal = false
}

// filterRemotesForHost keeps registry packages that have an install.json for this host.
func filterRemotesForHost(
	ctx context.Context,
	db *gorm.DB,
	remotes []remoteCatalogEntry,
	host softwarepkg.HostFacts,
) []remoteCatalogEntry {
	if len(remotes) == 0 {
		return remotes
	}

	var regs []models.SoftwarePackage
	_ = db.WithContext(ctx).Find(&regs).Error
	regByID := map[string]models.SoftwarePackage{}
	for _, reg := range regs {
		regByID[reg.ID] = reg
	}

	client := &softwarepkg.Client{}
	type result struct {
		idx int
		ok  bool
	}
	outCh := make(chan result, len(remotes))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i, entry := range remotes {
		reg, ok := regByID[entry.PackageID]
		if !ok || strings.TrimSpace(reg.PackageURL) == "" {
			continue
		}
		wg.Add(1)
		go func(i int, entry remoteCatalogEntry, reg models.SoftwarePackage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rawBase, err := softwarepkg.RawBaseURL(reg.PackageURL, "main")
			if err != nil {
				return
			}
			ok, err := client.HasInstallForHost(ctx, rawBase, entry.Meta.Name, host, softwarepkg.AuthFromPackage(reg))
			if err != nil {
				// Fail open on transient network errors so the catalog is not emptied.
				log.Printf("softwares list: probe %s: %v", entry.Meta.Name, err)
				outCh <- result{idx: i, ok: true}
				return
			}
			outCh <- result{idx: i, ok: ok}
		}(i, entry, reg)
	}
	wg.Wait()
	close(outCh)

	keep := make([]bool, len(remotes))
	for res := range outCh {
		keep[res.idx] = res.ok
	}
	filtered := make([]remoteCatalogEntry, 0, len(remotes))
	for i, entry := range remotes {
		if keep[i] {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func buildFacets(items []softwareListItem) listFacets {
	counts := map[string]int{}
	updateCount := 0
	remoteCount := 0
	for _, item := range items {
		cat := strings.TrimSpace(item.Category)
		if cat == "" {
			cat = "Other"
		}
		counts[cat]++
		if item.HasUpdate {
			updateCount++
		}
		if item.IsRemote {
			remoteCount++
		}
	}
	cats := make([]categoryFacet, 0, len(counts))
	for name, count := range counts {
		cats = append(cats, categoryFacet{Name: name, Count: count})
	}
	sort.SliceStable(cats, func(i, j int) bool {
		return strings.ToLower(cats[i].Name) < strings.ToLower(cats[j].Name)
	})
	return listFacets{
		Categories:  cats,
		UpdateCount: updateCount,
		TotalActive: len(items),
		RemoteCount: remoteCount,
	}
}

func sortSoftwareItems(items []softwareListItem, sortKey string) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch sortKey {
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "category":
			if a.Category != b.Category {
				return strings.ToLower(a.Category) < strings.ToLower(b.Category)
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "recent":
			at, bt := "", ""
			if a.LatestVersion != nil {
				at = a.LatestVersion.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if b.LatestVersion != nil {
				bt = b.LatestVersion.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if at != bt {
				return at > bt
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default: // order
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

func queryInt(c fiber.Ctx, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func queryBool(c fiber.Ctx, key string) bool {
	raw := strings.ToLower(strings.TrimSpace(c.Query(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
