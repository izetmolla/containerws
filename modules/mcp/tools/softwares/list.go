package softwares

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListInput struct {
	Query      string `json:"query,omitempty" jsonschema:"optional name filter (case-insensitive substring)"`
	IncludeAll bool   `json:"include_all,omitempty" jsonschema:"when true, include inactive catalog items"`
}

type SoftwareItem struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Details          string          `json:"details,omitempty"`
	Category         string          `json:"category,omitempty"`
	SubCategory      string          `json:"sub_category,omitempty"`
	Listed           bool            `json:"listed"`
	IsActive         bool            `json:"is_active"`
	LatestVersion    string          `json:"latest_version,omitempty"`
	InstalledVersion string          `json:"installed_version,omitempty"`
	IsInstalled      bool            `json:"is_installed"`
	HasUpdate        bool            `json:"has_update"`
	OnHost           bool            `json:"on_host"`
	OnHostDetail     string          `json:"on_host_detail,omitempty"`
	ServiceUnits     []string        `json:"service_units,omitempty"`
	ServiceOverall   string          `json:"service_overall,omitempty"`
	ServiceStatus    *service.Status `json:"service_status,omitempty"`
}

type ListOutput struct {
	Count int            `json:"count"`
	Items []SoftwareItem `json:"items"`
}

func (c *Controller) ListTool(ctx context.Context, _ *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, any, error) {
	c.ensureCatalog()
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}

	q := db.WithContext(ctx).Model(&models.Software{})
	if !input.IncludeAll {
		q = q.Where("is_active = ?", true)
	}
	query := strings.TrimSpace(input.Query)
	if query != "" {
		q = q.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(query)+"%")
	}

	var rows []models.Software
	if err := q.Find(&rows).Error; err != nil {
		return nil, nil, err
	}

	installedMap, err := models.InstalledVersionMap(db)
	if err != nil {
		return nil, nil, err
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Order != rows[j].Order {
			return rows[i].Order < rows[j].Order
		}
		return rows[i].Name < rows[j].Name
	})

	items := make([]SoftwareItem, 0, len(rows))
	for _, sw := range rows {
		item := SoftwareItem{
			ID:           sw.ID,
			Name:         sw.Name,
			Details:      sw.Details,
			Category:     sw.Category,
			SubCategory:  sw.SubCategory,
			Listed:       true,
			IsActive:     sw.IsActive,
			ServiceUnits: []string(sw.ServiceUnits),
		}
		latest, _ := latestVersion(db, sw.ID)
		if latest != nil {
			item.LatestVersion = latest.Version
		}
		if installedID, ok := installedMap[sw.ID]; ok {
			item.IsInstalled = true
			if latest != nil {
				item.HasUpdate = models.HasSoftwareUpdate(installedID, latest.ID)
			}
			var ver models.SoftwareVersion
			if err := db.Where("id = ?", installedID).First(&ver).Error; err == nil {
				item.InstalledVersion = ver.Version
			}
		}
		probe := softwaresync.ProbeInstalled(sw.Name, []string(sw.ServiceUnits))
		item.OnHost = probe.Present
		item.OnHostDetail = probe.Detail
		if len(sw.ServiceUnits) > 0 {
			st := service.ProbeUnits([]string(sw.ServiceUnits))
			item.ServiceStatus = &st
			item.ServiceOverall = st.Overall
		}
		items = append(items, item)
	}

	return &mcp.CallToolResult{}, ListOutput{Count: len(items), Items: items}, nil
}
