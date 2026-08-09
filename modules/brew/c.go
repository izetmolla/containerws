package brew

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /api/brew/* routes.
func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api := router.Group("/brew")
	api.Get("/status", cc.GetStatusAPI)
	api.Post("/bootstrap", cc.PostBootstrapAPI)
	api.Get("/formulae", cc.GetFormulaeAPI)
	api.Get("/formulae/:name", cc.GetFormulaAPI)
	api.Get("/installed", cc.GetInstalledAPI)
	api.Get("/analytics/install/:days", cc.GetAnalyticsAPI)
	api.Post("/actions", cc.PostActionsAPI)
	api.Post("/check-updates", cc.PostCheckUpdatesAPI)
	api.Get("/jobs", cc.GetJobsAPI)
	api.Get("/jobs/:id", cc.GetJobAPI)
	api.Post("/switch", cc.PostSwitchAPI)
}

func (cc *controller) GetStatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	path := ResolveBrewPath()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"module_enabled": models.BrewModuleEnabled(db),
			"binary_present": path != "",
			"brew_path":      path,
			"prefix":         BrewPrefix(path),
			"bootstrap":      BootstrapStatus(),
		},
	}))
}

func (cc *controller) PostBootstrapAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	started := StartBootstrap()
	return r.Api(c, r.WithStatus(fiber.StatusAccepted), r.WithData(fiber.Map{
		"data": fiber.Map{
			"started":   started,
			"bootstrap": BootstrapStatus(),
			"status": fiber.Map{
				"binary_present": ResolveBrewPath() != "",
				"brew_path":      ResolveBrewPath(),
			},
		},
		"message": map[bool]string{true: "Homebrew install started", false: "Bootstrap already running or brew present"}[started],
	}))
}

func (cc *controller) GetFormulaeAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	kindFilter := strings.ToLower(strings.TrimSpace(c.Query("kind", "all")))
	items, err := listCatalogCached(kindFilter)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	category := strings.TrimSpace(c.Query("category"))
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 48)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 48
	}
	if limit > 200 {
		limit = 200
	}

	db := cc.app.DB()
	ownership := brewOwnershipIndex(db)

	filtered := make([]CatalogEntry, 0, len(items))
	cats := map[string]int{}
	for _, f := range items {
		cats[f.Category]++
		if category != "" && !strings.EqualFold(f.Category, category) {
			continue
		}
		if q != "" {
			hay := strings.ToLower(strings.Join([]string{
				f.Name,
				f.DisplayName,
				f.Desc,
				f.Homepage,
				strings.Join(f.Aliases, " "),
			}, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		filtered = append(filtered, f)
	}

	total := len(filtered)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	pageItems := filtered[start:end]

	out := make([]fiber.Map, 0, len(pageItems))
	for _, f := range pageItems {
		row := fiber.Map{
			"kind":         f.Kind,
			"name":         f.Name,
			"display_name": f.DisplayName,
			"full_name":    f.FullName,
			"tap":          f.Tap,
			"desc":         f.Desc,
			"homepage":     f.Homepage,
			"license":      f.License,
			"version":      f.Version,
			"category":     f.Category,
			"aliases":      f.Aliases,
			"icon":         FormulaIconURL(f.Homepage),
			"installed":    false,
			"outdated":     false,
			"ownership":    ownership[strings.ToLower(f.Name)],
		}
		out = append(out, row)
	}

	if ResolveBrewPath() != "" {
		ctx := c.Context()
		installed, _ := listInstalledFormulae(ctx)
		instMap := map[string]map[string]any{}
		for _, it := range installed {
			name, _ := it["name"].(string)
			kind, _ := it["kind"].(string)
			if name == "" {
				continue
			}
			key := strings.ToLower(kind) + ":" + strings.ToLower(name)
			instMap[key] = it
			// Also index by name for older clients.
			if _, exists := instMap[strings.ToLower(name)]; !exists {
				instMap[strings.ToLower(name)] = it
			}
		}
		for i := range out {
			name, _ := out[i]["name"].(string)
			kind, _ := out[i]["kind"].(string)
			st, ok := instMap[strings.ToLower(kind)+":"+strings.ToLower(name)]
			if !ok {
				st, ok = instMap[strings.ToLower(name)]
			}
			if !ok {
				continue
			}
			out[i]["installed"] = true
			out[i]["outdated"] = st["outdated"]
			if v, ok := st["version"].(string); ok && v != "" {
				out[i]["installed_version"] = v
			}
		}
	}

	catList := make([]fiber.Map, 0, len(cats))
	for name, count := range cats {
		catList = append(catList, fiber.Map{"name": name, "count": count})
	}

	totalPages := 1
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
		if totalPages < 1 {
			totalPages = 1
		}
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"items":       out,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
			"categories":  catList,
		},
	}))
}

func (cc *controller) GetFormulaAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	preferred := strings.ToLower(strings.TrimSpace(c.Query("kind")))
	kind := resolvePackageKind(name, preferred)
	if kind == "" {
		// Warm caches then retry once so first hit after boot can succeed.
		_, _ = listCatalogCached("all")
		kind = resolvePackageKind(name, preferred)
	}
	if kind == "cask" {
		return cc.respondCaskDetail(c, name)
	}
	if kind != "formula" {
		return r.Api(c, r.WithError(fiber.ErrNotFound), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}

	f, err := getFormulaCached(name)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	if f == nil {
		return r.Api(c, r.WithError(fiber.ErrNotFound), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}

	detail, detailErr := fetchFormulaDetail(f.Name)
	if detailErr != nil {
		detail = nil
	}

	installed, installedVersions, outdated := formulaInstallDetails(c.Context(), f.Name)
	installedVersion := ""
	if len(installedVersions) > 0 {
		installedVersion = installedVersions[0]
	}

	desc := f.Desc
	homepage := f.Homepage
	license := f.License
	stable := f.Versions.Stable
	aliases := f.Aliases
	tap := f.Tap
	fullName := f.FullName
	var (
		versionedFormulae []string
		oldnames          []string
		deps              []string
		buildDeps         []string
		executables       []string
		revision          int
		headVer           string
		stableURL         string
		analytics30d      int
		analytics90d      int
		analytics365d     int
		deprecated        bool
		disabled          bool
	)
	if detail != nil {
		if strings.TrimSpace(detail.Desc) != "" {
			desc = detail.Desc
		}
		if strings.TrimSpace(detail.Homepage) != "" {
			homepage = detail.Homepage
		}
		if strings.TrimSpace(detail.License) != "" {
			license = detail.License
		}
		if strings.TrimSpace(detail.Versions.Stable) != "" {
			stable = detail.Versions.Stable
		}
		if len(detail.Aliases) > 0 {
			aliases = detail.Aliases
		}
		if strings.TrimSpace(detail.Tap) != "" {
			tap = detail.Tap
		}
		if strings.TrimSpace(detail.FullName) != "" {
			fullName = detail.FullName
		}
		versionedFormulae = detail.VersionedFormulae
		oldnames = detail.Oldnames
		deps = detail.Dependencies
		buildDeps = detail.BuildDependencies
		executables = detail.Executables
		revision = detail.Revision
		stableURL = detail.URLs.Stable.URL
		deprecated = detail.Deprecated
		disabled = detail.Disabled
		if detail.Versions.Head != nil {
			headVer = strings.TrimSpace(*detail.Versions.Head)
		}
		analytics30d = analyticsCount(detail.Analytics.Install.D30, f.Name)
		analytics90d = analyticsCount(detail.Analytics.Install.D90, f.Name)
		analytics365d = analyticsCount(detail.Analytics.Install.D365, f.Name)
	}

	installedSet := make(map[string]bool, len(installedVersions))
	for _, v := range installedVersions {
		installedSet[v] = true
	}

	versions := make([]fiber.Map, 0, 1+len(versionedFormulae)+len(installedVersions))
	versions = append(versions, fiber.Map{
		"formula":   f.Name,
		"version":   stable,
		"kind":      "stable",
		"current":   true,
		"installed": installedSet[stable],
		"href":      "/brew/" + f.Name + "?kind=formula",
	})
	if headVer != "" {
		versions = append(versions, fiber.Map{
			"formula":   f.Name,
			"version":   headVer,
			"kind":      "head",
			"current":   false,
			"installed": false,
			"href":      "/brew/" + f.Name + "?kind=formula",
		})
	}
	for _, sibling := range versionedFormulae {
		sibling = strings.TrimSpace(sibling)
		if sibling == "" || strings.EqualFold(sibling, f.Name) {
			continue
		}
		sibVer := ""
		if sib, _ := getFormulaCached(sibling); sib != nil {
			sibVer = sib.Versions.Stable
		}
		versions = append(versions, fiber.Map{
			"formula":   sibling,
			"version":   sibVer,
			"kind":      "versioned",
			"current":   false,
			"installed": false,
			"href":      "/brew/" + sibling + "?kind=formula",
		})
	}
	for _, v := range installedVersions {
		if v == stable {
			continue
		}
		versions = append(versions, fiber.Map{
			"formula":   f.Name,
			"version":   v,
			"kind":      "installed",
			"current":   false,
			"installed": true,
			"href":      "/brew/" + f.Name + "?kind=formula",
		})
	}

	db := cc.app.DB()
	own := ownershipForToken(db, f.Name)
	sw, _ := FindSoftwareByBrewToken(db, f.Name)

	data := fiber.Map{
		"kind":               "formula",
		"name":               f.Name,
		"display_name":       f.Name,
		"full_name":          fullName,
		"tap":                tap,
		"desc":               desc,
		"homepage":           homepage,
		"license":            license,
		"version":            stable,
		"revision":           revision,
		"head":               headVer,
		"stable_url":         stableURL,
		"category":           f.Category,
		"aliases":            aliases,
		"oldnames":           oldnames,
		"versioned_formulae": versionedFormulae,
		"dependencies":       deps,
		"build_dependencies": buildDeps,
		"executables":        executables,
		"versions":           versions,
		"icon":               FormulaIconURL(homepage),
		"logo":               FormulaLogoURL(homepage),
		"installed":          installed,
		"installed_version":  installedVersion,
		"installed_versions": installedVersions,
		"outdated":           outdated,
		"deprecated":         deprecated,
		"disabled":           disabled,
		"analytics": fiber.Map{
			"install_30d":  analytics30d,
			"install_90d":  analytics90d,
			"install_365d": analytics365d,
		},
		"ownership": own,
	}
	attachSoftwareFields(data, db, sw)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": data}))
}

func (cc *controller) respondCaskDetail(c fiber.Ctx, name string) error {
	r := cc.app.Render()
	ck, err := getCaskCached(name)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	if ck == nil {
		return r.Api(c, r.WithError(fiber.ErrNotFound), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}

	detail, detailErr := fetchCaskDetail(ck.Token)
	if detailErr != nil {
		detail = nil
	}

	desc := ck.Desc
	homepage := ck.Homepage
	version := ck.Version
	tap := firstNonEmpty(ck.Tap, "homebrew/cask")
	display := caskDisplayName(*ck)
	fullName := firstNonEmpty(ck.FullToken, ck.Token)
	deprecated := ck.Deprecated
	disabled := ck.Disabled
	var analytics30d, analytics90d, analytics365d int
	if detail != nil {
		if strings.TrimSpace(detail.Desc) != "" {
			desc = detail.Desc
		}
		if strings.TrimSpace(detail.Homepage) != "" {
			homepage = detail.Homepage
		}
		if strings.TrimSpace(detail.Version) != "" {
			version = detail.Version
		}
		if strings.TrimSpace(detail.Tap) != "" {
			tap = detail.Tap
		}
		if len(detail.Name) > 0 {
			display = caskDisplayName(detail.Cask)
		}
		deprecated = detail.Deprecated
		disabled = detail.Disabled
		analytics30d = analyticsCount(detail.Analytics.Install.D30, ck.Token)
		analytics90d = analyticsCount(detail.Analytics.Install.D90, ck.Token)
		analytics365d = analyticsCount(detail.Analytics.Install.D365, ck.Token)
	}

	installed, installedVersion, outdated := caskInstallDetails(c.Context(), ck.Token)
	versions := []fiber.Map{{
		"formula":   ck.Token,
		"version":   version,
		"kind":      "stable",
		"current":   true,
		"installed": installed && (installedVersion == "" || installedVersion == version),
		"href":      "/brew/" + ck.Token + "?kind=cask",
	}}
	if installed && installedVersion != "" && installedVersion != version {
		versions = append(versions, fiber.Map{
			"formula":   ck.Token,
			"version":   installedVersion,
			"kind":      "installed",
			"current":   false,
			"installed": true,
			"href":      "/brew/" + ck.Token + "?kind=cask",
		})
	}

	db := cc.app.DB()
	own := ownershipForToken(db, ck.Token)
	sw, _ := FindSoftwareByBrewToken(db, ck.Token)

	data := fiber.Map{
		"kind":               "cask",
		"name":               ck.Token,
		"display_name":       display,
		"full_name":          fullName,
		"tap":                tap,
		"desc":               desc,
		"homepage":           homepage,
		"license":            "",
		"version":            version,
		"category":           ck.Category,
		"aliases":            []string{},
		"versions":           versions,
		"icon":               FormulaIconURL(homepage),
		"logo":               FormulaLogoURL(homepage),
		"installed":          installed,
		"installed_version":  installedVersion,
		"installed_versions": []string{},
		"outdated":           outdated,
		"deprecated":         deprecated,
		"disabled":           disabled,
		"analytics": fiber.Map{
			"install_30d":  analytics30d,
			"install_90d":  analytics90d,
			"install_365d": analytics365d,
		},
		"ownership": own,
	}
	if installedVersion != "" {
		data["installed_versions"] = []string{installedVersion}
	}
	attachSoftwareFields(data, db, sw)

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": data}))
}

func attachSoftwareFields(data fiber.Map, db *gorm.DB, sw *models.Software) {
	if sw == nil {
		data["can_switch_to_local"] = false
		data["can_switch_to_brew"] = false
		return
	}
	data["software_id"] = sw.ID
	data["software_name"] = sw.Name
	pm := models.GetSoftwarePackageManager(db, sw.ID)
	data["package_manager"] = pm
	data["can_switch_to_local"] = pm == models.PackageManagerBrew
	data["can_switch_to_brew"] = pm == models.PackageManagerLocal || pm == ""
}

func (cc *controller) GetInstalledAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	// Refresh Softwares ownership from brew CLI installs (throttled).
	SyncHostInstallsThrottled(db)
	if ResolveBrewPath() == "" {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": fiber.Map{"items": []any{}, "brew_missing": true},
		}))
	}
	items, err := listInstalledFormulae(c.Context())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := make([]fiber.Map, 0, len(items))
	for _, it := range items {
		name, _ := it["name"].(string)
		row := fiber.Map{
			"name":      name,
			"version":   it["version"],
			"outdated":  it["outdated"],
			"installed": true,
			"ownership": ownershipForToken(db, name),
		}
		if sw, _ := FindSoftwareByBrewToken(db, name); sw != nil {
			row["software_id"] = sw.ID
			row["software_name"] = sw.Name
			row["package_manager"] = models.GetSoftwarePackageManager(db, sw.ID)
		}
		if f, _ := getFormulaCached(name); f != nil {
			row["desc"] = f.Desc
			row["category"] = f.Category
			row["homepage"] = f.Homepage
			row["icon"] = FormulaIconURL(f.Homepage)
			row["kind"] = "formula"
		} else if ck, _ := getCaskCached(name); ck != nil {
			row["desc"] = ck.Desc
			row["category"] = ck.Category
			row["homepage"] = ck.Homepage
			row["display_name"] = caskDisplayName(*ck)
			row["icon"] = FormulaIconURL(ck.Homepage)
			row["kind"] = "cask"
		}
		if _, ok := row["kind"]; !ok {
			if k, _ := it["kind"].(string); k != "" {
				row["kind"] = k
			} else {
				row["kind"] = "formula"
			}
		}
		out = append(out, row)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"items": out},
	}))
}

func (cc *controller) GetAnalyticsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	days, _ := strconv.Atoi(c.Params("days"))
	names, err := fetchAnalytics(days)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	items := make([]fiber.Map, 0, len(names))
	for i, name := range names {
		row := fiber.Map{"rank": i + 1, "name": name}
		if f, _ := getFormulaCached(name); f != nil {
			row["desc"] = f.Desc
			row["category"] = f.Category
			row["version"] = f.Versions.Stable
			row["homepage"] = f.Homepage
			row["icon"] = FormulaIconURL(f.Homepage)
		}
		items = append(items, row)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"days": days, "items": items},
	}))
}

func (cc *controller) PostActionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body struct {
		Action string   `json:"action"`
		Names  []string `json:"names"`
		Kind   string   `json:"kind"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	// Block brew actions when Softwares owns the token locally.
	db := cc.app.DB()
	for _, name := range body.Names {
		own := ownershipForToken(db, name)
		if own == models.PackageManagerLocal {
			return r.Api(c, r.WithError(fiber.NewError(fiber.StatusConflict, "package is managed by Softwares; switch package manager first")), r.WithStatus(fiber.StatusConflict), r.WithErrorCode("OWNED_BY_SOFTWARES"))
		}
	}

	// Prefer Softwares install queue so brew never overlaps Softwares/VNC jobs.
	if softwaresEnqueue != nil {
		queued, snap, err := softwaresEnqueue(db, body.Action, body.Names, body.Kind)
		if err != nil {
			status := fiber.StatusBadRequest
			if strings.Contains(err.Error(), "already queued") {
				status = fiber.StatusConflict
			}
			return r.Api(c, r.WithError(err), r.WithStatus(status))
		}
		return r.Api(c, r.WithStatus(fiber.StatusAccepted), r.WithData(fiber.Map{
			"data": fiber.Map{
				"queued": queued,
				"queue":  snap,
				"source": "softwares_queue",
			},
			"message": fmt.Sprintf("Queued %d Brew package(s) in Softwares install queue", queued),
		}))
	}

	job, err := startActionJob(body.Action, body.Names, body.Kind)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	// After successful install, mark matching Softwares as brew-owned (async watcher).
	go watchJobOwnership(db, job.ID)

	return r.Api(c, r.WithStatus(fiber.StatusAccepted), r.WithData(fiber.Map{
		"data":    job,
		"message": "Brew job queued",
	}))
}

// PostCheckUpdatesAPI runs `brew update`, lists outdated packages, and optionally
// queues upgrades (default) into the Softwares install queue.
func (cc *controller) PostCheckUpdatesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()

	var body struct {
		Upgrade *bool `json:"upgrade"`
	}
	_ = c.Bind().Body(&body)
	doUpgrade := true
	if body.Upgrade != nil {
		doUpgrade = *body.Upgrade
	}

	if ResolveBrewPath() == "" {
		return r.Api(c, r.WithError(fiber.NewError(fiber.StatusBadRequest, "brew is not installed")), r.WithStatus(fiber.StatusBadRequest))
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Minute)
	defer cancel()

	brewPath := ResolveBrewPath()
	updateOut, updateErr := runBrewCombined(ctx, brewPath, "update")
	if updateErr != nil {
		// Still try to read outdated state — brew update can fail offline.
		AppendBootstrapNote("brew update: " + updateErr.Error())
	}

	items, err := listInstalledFormulae(ctx)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	type outdatedRow struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	outdated := make([]outdatedRow, 0)
	formulae := make([]string, 0)
	casks := make([]string, 0)
	for _, it := range items {
		name, _ := it["name"].(string)
		kind, _ := it["kind"].(string)
		isOut, _ := it["outdated"].(bool)
		if name == "" || !isOut {
			continue
		}
		if ownershipForToken(db, name) == models.PackageManagerLocal {
			continue
		}
		if kind != "cask" {
			kind = "formula"
		}
		outdated = append(outdated, outdatedRow{Name: name, Kind: kind})
		if kind == "cask" {
			casks = append(casks, name)
		} else {
			formulae = append(formulae, name)
		}
	}

	queued := 0
	if doUpgrade && softwaresEnqueue != nil {
		if len(formulae) > 0 {
			n, _, qErr := softwaresEnqueue(db, "upgrade", formulae, "formula")
			if qErr == nil {
				queued += n
			}
		}
		if len(casks) > 0 {
			n, _, qErr := softwaresEnqueue(db, "upgrade", casks, "cask")
			if qErr == nil {
				queued += n
			}
		}
	}

	msg := fmt.Sprintf("Found %d outdated package(s)", len(outdated))
	if doUpgrade {
		msg = fmt.Sprintf("Found %d outdated · queued %d upgrade(s)", len(outdated), queued)
	}
	if updateErr != nil && len(outdated) == 0 {
		msg = "brew update failed: " + updateErr.Error()
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"update_log": truncateLog(updateOut, 4000),
			"update_ok":  updateErr == nil,
			"outdated":   outdated,
			"queued":     queued,
		},
		"message": msg,
	}))
}

func (cc *controller) GetJobsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"items": listRecentJobs(50)},
	}))
}

func (cc *controller) GetJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	job := getJob(c.Params("id"))
	if job == nil {
		return r.Api(c, r.WithError(fiber.ErrNotFound), r.WithStatus(fiber.StatusNotFound))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": job}))
}

func (cc *controller) PostSwitchAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body struct {
		SoftwareID string `json:"software_id"`
		Target     string `json:"target"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	res, err := SwitchPackageManager(c.Context(), cc.app.DB(), body.SoftwareID, body.Target)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": res.Message,
	}))
}

func queryInt(c fiber.Ctx, key string, def int) int {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func brewOwnershipIndex(db *gorm.DB) map[string]string {
	out := map[string]string{}
	if db == nil {
		return out
	}
	rows, err := models.ListSoftwareInstalled(db)
	if err != nil {
		return out
	}
	for i := range rows {
		row := rows[i]
		if row.Uninstalled || row.Software == nil {
			continue
		}
		token := BrewTokenForSoftware(row.Software)
		if token == "" {
			continue
		}
		out[token] = models.NormalizePackageManager(row.PackageManager)
	}
	return out
}

func ownershipForToken(db *gorm.DB, token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" || db == nil {
		return ""
	}
	sw, err := FindSoftwareByBrewToken(db, token)
	if err != nil || sw == nil {
		return ""
	}
	return models.GetSoftwarePackageManager(db, sw.ID)
}

func watchJobOwnership(db *gorm.DB, jobID string) {
	if db == nil || jobID == "" {
		return
	}
	deadline := time.Now().Add(70 * time.Minute)
	for time.Now().Before(deadline) {
		job := getJob(jobID)
		if job == nil {
			return
		}
		switch job.Status {
		case "success":
			ApplyJobOwnership(db, job)
			return
		case "error":
			return
		}
		time.Sleep(time.Second)
	}
}

// ApplyJobOwnership marks matching Softwares rows after a successful brew job.
func ApplyJobOwnership(db *gorm.DB, job *actionJob) {
	if db == nil || job == nil || job.Status != "success" {
		return
	}
	ctx := context.Background()
	for _, name := range job.Names {
		sw, _ := FindSoftwareByBrewToken(db, name)
		if sw == nil {
			continue
		}
		hostVersions, _ := gorm.G[models.SoftwareVersion](db).
			Where("software_id = ?", sw.ID).
			Order("is_latest DESC").
			Find(ctx)
		versionID := ""
		if len(hostVersions) > 0 {
			versionID = hostVersions[0].ID
		}
		if versionID == "" {
			continue
		}
		switch job.Action {
		case "install", "upgrade":
			_ = models.MarkSoftwareInstalledWithManager(db, sw.ID, versionID, models.PackageManagerBrew)
		case "uninstall":
			_ = models.MarkSoftwareUninstalled(db, sw.ID)
		}
	}
}
