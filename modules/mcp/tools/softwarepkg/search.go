package softwarepkg

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/izetmolla/containerws/models"
	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

type SearchInput struct {
	Query string `json:"query" jsonschema:"required app name or substring to search"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results (default 20, max 50)"`
}

type SearchHit struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"` // local | remote | both
	ID          string   `json:"id,omitempty"`
	Details     string   `json:"details,omitempty"`
	Category    string   `json:"category,omitempty"`
	IsRemote    bool     `json:"is_remote"`
	PackageID   string   `json:"package_id,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ListedLocal bool     `json:"listed_local"`
}

type SearchOutput struct {
	Query     string            `json:"query"`
	Count     int               `json:"count"`
	Hits      []SearchHit       `json:"hits"`
	Host      pkglib.HostFacts  `json:"host"`
	Message   string            `json:"message"`
	NextSteps []string          `json:"next_steps"`
}

func (c *Controller) db() *gorm.DB {
	if c == nil || c.app == nil {
		return nil
	}
	return c.app.DB()
}

func (c *Controller) SearchTool(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, any, error) {
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	q := strings.TrimSpace(input.Query)
	if q == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	host := pkglib.HostFromMachine()
	hitsByName := map[string]*SearchHit{}

	var locals []models.Software
	like := "%" + strings.ToLower(q) + "%"
	_ = db.WithContext(ctx).
		Where("is_active = ? AND (LOWER(name) LIKE ? OR LOWER(details) LIKE ? OR LOWER(category) LIKE ?)",
			true, like, like, like).
		Order("name ASC").
		Limit(limit).
		Find(&locals).Error
	for _, sw := range locals {
		key := strings.ToLower(strings.TrimSpace(sw.Name))
		hitsByName[key] = &SearchHit{
			Name:        sw.Name,
			Source:      "local",
			ID:          sw.ID,
			Details:     sw.Details,
			Category:    sw.Category,
			Tags:        []string(sw.Tags),
			ListedLocal: true,
			IsRemote:    false,
		}
	}

	var regs []models.SoftwarePackage
	_ = db.WithContext(ctx).Order("created_at ASC").Find(&regs).Error
	client := &pkglib.Client{}
	for _, reg := range regs {
		if strings.TrimSpace(reg.PackageURL) == "" {
			continue
		}
		items, err := pkglib.ListRemoteFromPackage(ctx, reg, "main", client)
		if err != nil {
			continue
		}
		for _, meta := range items {
			if !pkglib.MatchQuery(meta, q) {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(meta.Name))
			if key == "" {
				continue
			}
			if existing, ok := hitsByName[key]; ok {
				existing.Source = "both"
				existing.PackageID = reg.ID
				if existing.Details == "" {
					existing.Details = meta.Details
				}
				if existing.Category == "" {
					existing.Category = meta.Category
				}
				continue
			}
			hitsByName[key] = &SearchHit{
				Name:        meta.Name,
				Source:      "remote",
				ID:          "remote:" + strings.ReplaceAll(key, " ", "-"),
				Details:     meta.Details,
				Category:    meta.Category,
				Tags:        meta.Tags,
				IsRemote:    true,
				PackageID:   reg.ID,
				ListedLocal: false,
			}
		}
	}

	hits := make([]SearchHit, 0, len(hitsByName))
	for _, h := range hitsByName {
		hits = append(hits, *h)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return strings.ToLower(hits[i].Name) < strings.ToLower(hits[j].Name)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}

	next := []string{}
	if len(hits) == 0 {
		next = []string{
			"No existing package found — call softwarepkg_create (local catalog) and/or softwarepkg_scaffold (registry files on disk).",
			fmt.Sprintf("Suggested: softwarepkg_create name=%q with apt/dnf package names if they differ.", q),
		}
	} else {
		next = []string{
			"If a remote-only hit fits, softwarepkg_import name=<name> package_id=<package_id>.",
			"To author a new/updated registry tree, softwarepkg_scaffold output_dir=<cws-packages root>.",
			"To add/update the local Softwares catalog for this host, softwarepkg_create.",
		}
	}

	msg := fmt.Sprintf("Found %d match(es) for %q (host %s %s/%s)",
		len(hits), q, host.DistroID, host.DistroVersion, host.Arch)
	if len(hits) == 0 {
		msg = fmt.Sprintf("No local/remote package for %q — ready to create (host %s %s/%s)",
			q, host.DistroID, host.DistroVersion, host.Arch)
	}

	return &mcp.CallToolResult{}, SearchOutput{
		Query:     q,
		Count:     len(hits),
		Hits:      hits,
		Host:      host,
		Message:   msg,
		NextSteps: next,
	}, nil
}
