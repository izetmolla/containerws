package softwares

import (
	"context"
	"fmt"
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

type LookupInput struct {
	NameOrID string `json:"name_or_id" jsonschema:"required software id or name (exact or unique substring)"`
}

type LookupOutput struct {
	Listed           bool            `json:"listed"`
	Query            string          `json:"query"`
	Message          string          `json:"message"`
	ID               string          `json:"id,omitempty"`
	Name             string          `json:"name,omitempty"`
	Details          string          `json:"details,omitempty"`
	Category         string          `json:"category,omitempty"`
	IsActive         bool            `json:"is_active,omitempty"`
	LatestVersion    string          `json:"latest_version,omitempty"`
	InstalledVersion string          `json:"installed_version,omitempty"`
	IsInstalled      bool            `json:"is_installed"`
	HasUpdate        bool            `json:"has_update"`
	OnHost           bool            `json:"on_host"`
	OnHostDetail     string          `json:"on_host_detail,omitempty"`
	HasInstallScript bool            `json:"has_install_script,omitempty"`
	HasCustomScript  bool            `json:"has_custom_script,omitempty"`
	ServiceUnits     []string        `json:"service_units,omitempty"`
	ServiceOverall   string          `json:"service_overall,omitempty"`
	ServiceStatus    *service.Status `json:"service_status,omitempty"`
	Suggestions      []string        `json:"suggestions,omitempty"`
}

func (c *Controller) LookupTool(ctx context.Context, _ *mcp.CallToolRequest, input LookupInput) (*mcp.CallToolResult, any, error) {
	c.ensureCatalog()
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}

	query := strings.TrimSpace(input.NameOrID)
	if query == "" {
		return nil, nil, fmt.Errorf("name_or_id is required")
	}

	sw, err := findSoftware(db, query)
	if err != nil {
		return nil, nil, err
	}
	if sw == nil {
		out := LookupOutput{
			Listed:      false,
			Query:       query,
			Message:     fmt.Sprintf("%q is not listed in the Softwares catalog — use bash for ad-hoc installs, or softwares_list to see catalog items", query),
			Suggestions: suggestNames(db, query, 8),
		}
		return &mcp.CallToolResult{}, out, nil
	}

	out := LookupOutput{
		Listed:       true,
		Query:        query,
		Message:      fmt.Sprintf("%q is listed in the Softwares catalog", sw.Name),
		ID:           sw.ID,
		Name:         sw.Name,
		Details:      sw.Details,
		Category:     sw.Category,
		IsActive:     sw.IsActive,
		ServiceUnits: []string(sw.ServiceUnits),
	}

	latest, _ := latestVersion(db, sw.ID)
	if latest != nil {
		out.LatestVersion = latest.Version
		out.HasInstallScript = strings.TrimSpace(latest.InstallScript) != ""
		out.HasCustomScript = strings.TrimSpace(latest.CustomScript) != ""
	}

	installRow, err := models.GetSoftwareInstalled(db, sw.ID)
	if err != nil {
		return nil, nil, err
	}
	if installRow != nil {
		out.IsInstalled = true
		var ver models.SoftwareVersion
		if err := db.WithContext(ctx).Where("id = ?", installRow.VersionID).First(&ver).Error; err == nil {
			out.InstalledVersion = ver.Version
		}
		if latest != nil {
			out.HasUpdate = models.HasSoftwareUpdate(installRow.VersionID, latest.ID)
		}
	}

	probe := softwaresync.ProbeInstalled(sw.Name, []string(sw.ServiceUnits))
	out.OnHost = probe.Present
	out.OnHostDetail = probe.Detail

	if len(sw.ServiceUnits) > 0 {
		st := service.ProbeUnits([]string(sw.ServiceUnits))
		out.ServiceStatus = &st
		out.ServiceOverall = st.Overall
	}

	return &mcp.CallToolResult{}, out, nil
}

func suggestNames(db *gorm.DB, query string, limit int) []string {
	if db == nil || limit <= 0 {
		return nil
	}
	var rows []models.Software
	_ = db.Where("is_active = ? AND LOWER(name) LIKE ?", true, "%"+strings.ToLower(query)+"%").
		Order("name ASC").
		Limit(limit).
		Find(&rows).Error
	if len(rows) == 0 {
		_ = db.Where("is_active = ?", true).
			Order("name ASC").
			Limit(limit).
			Find(&rows).Error
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}
