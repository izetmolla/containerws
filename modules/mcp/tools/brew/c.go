package brewmcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	hostbrew "github.com/izetmolla/containerws/modules/brew"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

func (c *Controller) db() *gorm.DB {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.DB()
}

func LoadTools(server *mcp.Server, app *config.AppClients) {
	controller := NewController(app)

	mcp.AddTool(server, &mcp.Tool{
		Name: "brew_status",
		Description: "Report Homebrew status on this host: module enabled, brew binary path, prefix, and bootstrap state. " +
			"Call before brew_install / brew_search when unsure whether brew is available.",
	}, controller.StatusTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "brew_search",
		Description: "Search Homebrew formulae and/or casks (cached catalogue). " +
			"Returns matching names with kind, version, and Softwares ownership when linked.",
	}, controller.SearchTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "brew_installed",
		Description: "List packages installed via Homebrew on this host (formulae + casks), including outdated flags and Softwares ownership.",
	}, controller.InstalledTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "brew_install",
		Description: "Queue brew install/upgrade/uninstall onto the Softwares install queue (serialized with Softwares/VNC jobs). " +
			"Prefer brew_status first. Softwares-owned (local) tokens are blocked until switched. " +
			"action=install|upgrade|uninstall; kind=formula|cask.",
	}, controller.ActionTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "brew_check_updates",
		Description: "Run brew update, list outdated packages, and optionally queue upgrades (default true) into Softwares → Installing.",
	}, controller.CheckUpdatesTool)
}

type StatusInput struct{}

type StatusOutput struct {
	ModuleEnabled bool           `json:"module_enabled"`
	BinaryPresent bool           `json:"binary_present"`
	BrewPath      string         `json:"brew_path,omitempty"`
	Prefix        string         `json:"prefix,omitempty"`
	Bootstrap     map[string]any `json:"bootstrap,omitempty"`
	Message       string         `json:"message"`
}

func (c *Controller) StatusTool(ctx context.Context, _ *mcp.CallToolRequest, _ StatusInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	hostbrew.EnsureBrewShellPath()
	db := c.db()
	path := hostbrew.ResolveBrewPath()
	out := StatusOutput{
		ModuleEnabled: models.BrewModuleEnabled(db),
		BinaryPresent: path != "",
		BrewPath:      path,
		Prefix:        hostbrew.BrewPrefix(path),
		Bootstrap:     hostbrew.BootstrapStatus(),
	}
	if path == "" {
		out.Message = "Homebrew is not installed — enable Brew Package in Settings → General or bootstrap via the Brew UI"
	} else {
		out.Message = "Homebrew is available"
	}
	return &mcp.CallToolResult{}, out, nil
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"required search string (name, alias, or description substring)"`
	Kind  string `json:"kind,omitempty" jsonschema:"optional filter: all|formula|cask (default all)"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results (default 25, max 100)"`
}

type SearchItem struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Desc        string `json:"desc,omitempty"`
	Version     string `json:"version,omitempty"`
	Category    string `json:"category,omitempty"`
	Ownership   string `json:"ownership,omitempty"`
}

type SearchOutput struct {
	Query   string       `json:"query"`
	Total   int          `json:"total"`
	Items   []SearchItem `json:"items"`
	Message string       `json:"message"`
}

func (c *Controller) SearchTool(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	q := strings.ToLower(strings.TrimSpace(input.Query))
	if q == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = "all"
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	hostbrew.EnsureBrewShellPath()
	items, err := hostbrew.ListCatalog(kind)
	if err != nil && len(items) == 0 {
		return nil, nil, err
	}

	db := c.db()
	out := SearchOutput{Query: input.Query, Items: []SearchItem{}}
	for _, e := range items {
		hay := strings.ToLower(strings.Join([]string{
			e.Name, e.DisplayName, e.Desc, e.Category, strings.Join(e.Aliases, " "),
		}, " "))
		if !strings.Contains(hay, q) {
			continue
		}
		out.Total++
		if len(out.Items) >= limit {
			continue
		}
		out.Items = append(out.Items, SearchItem{
			Kind:        e.Kind,
			Name:        e.Name,
			DisplayName: e.DisplayName,
			Desc:        e.Desc,
			Version:     e.Version,
			Category:    e.Category,
			Ownership:   hostbrew.OwnershipForToken(db, e.Name),
		})
	}
	out.Message = fmt.Sprintf("%d match(es), showing %d", out.Total, len(out.Items))
	return &mcp.CallToolResult{}, out, nil
}

type InstalledInput struct {
	OutdatedOnly bool `json:"outdated_only,omitempty" jsonschema:"when true, only return outdated packages"`
}

type InstalledItem struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Version        string `json:"version,omitempty"`
	Outdated       bool   `json:"outdated"`
	Ownership      string `json:"ownership,omitempty"`
	PackageManager string `json:"package_manager,omitempty"`
	SoftwareID     string `json:"software_id,omitempty"`
}

type InstalledOutput struct {
	Items   []InstalledItem `json:"items"`
	Total   int             `json:"total"`
	Message string          `json:"message"`
}

func (c *Controller) InstalledTool(ctx context.Context, _ *mcp.CallToolRequest, input InstalledInput) (*mcp.CallToolResult, any, error) {
	hostbrew.EnsureBrewShellPath()
	if hostbrew.ResolveBrewPath() == "" {
		return &mcp.CallToolResult{IsError: true}, InstalledOutput{
			Items:   []InstalledItem{},
			Message: "brew is not installed",
		}, nil
	}
	raw, err := hostbrew.ListInstalled(ctx)
	if err != nil {
		return nil, nil, err
	}
	db := c.db()
	out := InstalledOutput{Items: make([]InstalledItem, 0, len(raw))}
	for _, it := range raw {
		name, _ := it["name"].(string)
		kind, _ := it["kind"].(string)
		ver, _ := it["version"].(string)
		outdated, _ := it["outdated"].(bool)
		if name == "" {
			continue
		}
		if input.OutdatedOnly && !outdated {
			continue
		}
		row := InstalledItem{
			Name:      name,
			Kind:      kind,
			Version:   ver,
			Outdated:  outdated,
			Ownership: hostbrew.OwnershipForToken(db, name),
		}
		if sw, _ := hostbrew.FindSoftwareByBrewToken(db, name); sw != nil {
			row.SoftwareID = sw.ID
			row.PackageManager = models.GetSoftwarePackageManager(db, sw.ID)
		}
		out.Items = append(out.Items, row)
	}
	out.Total = len(out.Items)
	out.Message = fmt.Sprintf("%d installed package(s)", out.Total)
	return &mcp.CallToolResult{}, out, nil
}

type ActionInput struct {
	Action string   `json:"action" jsonschema:"required install|upgrade|uninstall"`
	Names  []string `json:"names" jsonschema:"required one or more formula/cask tokens"`
	Kind   string   `json:"kind,omitempty" jsonschema:"formula|cask (default formula); use cask for desktop apps"`
}

type ActionOutput struct {
	Queued  int      `json:"queued"`
	Action  string   `json:"action"`
	Names   []string `json:"names"`
	Kind    string   `json:"kind"`
	Message string   `json:"message"`
}

func (c *Controller) ActionTool(ctx context.Context, _ *mcp.CallToolRequest, input ActionInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	hostbrew.EnsureBrewShellPath()
	action := strings.ToLower(strings.TrimSpace(input.Action))
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind != "cask" {
		kind = "formula"
	}
	db := c.db()
	for _, name := range input.Names {
		if hostbrew.OwnershipForToken(db, name) == models.PackageManagerLocal {
			out := ActionOutput{
				Action:  action,
				Names:   input.Names,
				Kind:    kind,
				Message: fmt.Sprintf("%q is managed by Softwares (local); switch package manager before brew actions", name),
			}
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
	}

	queued, _, err := hostbrew.EnqueueSoftwaresActions(db, action, input.Names, kind)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ActionOutput{
			Action:  action,
			Names:   input.Names,
			Kind:    kind,
			Message: err.Error(),
		}, nil
	}
	return &mcp.CallToolResult{}, ActionOutput{
		Queued:  queued,
		Action:  action,
		Names:   input.Names,
		Kind:    kind,
		Message: fmt.Sprintf("Queued %d Brew %s(s) in Softwares install queue", queued, action),
	}, nil
}

type CheckUpdatesInput struct {
	Upgrade *bool `json:"upgrade,omitempty" jsonschema:"when true (default), queue upgrades for outdated packages"`
}

type outdatedRow struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type CheckUpdatesOutput struct {
	UpdateOK bool          `json:"update_ok"`
	Outdated []outdatedRow `json:"outdated"`
	Queued   int           `json:"queued"`
	Message  string        `json:"message"`
}

func (c *Controller) CheckUpdatesTool(ctx context.Context, _ *mcp.CallToolRequest, input CheckUpdatesInput) (*mcp.CallToolResult, any, error) {
	hostbrew.EnsureBrewShellPath()
	if hostbrew.ResolveBrewPath() == "" {
		return &mcp.CallToolResult{IsError: true}, CheckUpdatesOutput{Message: "brew is not installed"}, nil
	}
	doUpgrade := true
	if input.Upgrade != nil {
		doUpgrade = *input.Upgrade
	}

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	updateErr := hostbrew.UpdateIndex(runCtx)

	items, err := hostbrew.ListInstalled(runCtx)
	if err != nil {
		return nil, nil, err
	}

	db := c.db()
	out := CheckUpdatesOutput{UpdateOK: updateErr == nil, Outdated: []outdatedRow{}}
	formulae := make([]string, 0)
	casks := make([]string, 0)
	for _, it := range items {
		name, _ := it["name"].(string)
		kind, _ := it["kind"].(string)
		isOut, _ := it["outdated"].(bool)
		if name == "" || !isOut {
			continue
		}
		if hostbrew.OwnershipForToken(db, name) == models.PackageManagerLocal {
			continue
		}
		if kind != "cask" {
			kind = "formula"
		}
		out.Outdated = append(out.Outdated, outdatedRow{Name: name, Kind: kind})
		if kind == "cask" {
			casks = append(casks, name)
		} else {
			formulae = append(formulae, name)
		}
	}

	if doUpgrade {
		if len(formulae) > 0 {
			n, _, qErr := hostbrew.EnqueueSoftwaresActions(db, "upgrade", formulae, "formula")
			if qErr == nil {
				out.Queued += n
			}
		}
		if len(casks) > 0 {
			n, _, qErr := hostbrew.EnqueueSoftwaresActions(db, "upgrade", casks, "cask")
			if qErr == nil {
				out.Queued += n
			}
		}
	}

	out.Message = fmt.Sprintf("Found %d outdated", len(out.Outdated))
	if doUpgrade {
		out.Message = fmt.Sprintf("Found %d outdated · queued %d upgrade(s)", len(out.Outdated), out.Queued)
	}
	if updateErr != nil && len(out.Outdated) == 0 {
		out.Message = "brew update failed: " + updateErr.Error()
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	return &mcp.CallToolResult{}, out, nil
}
