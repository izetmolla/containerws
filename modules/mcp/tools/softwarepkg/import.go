package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/izetmolla/containerws/models"
	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

type ImportInput struct {
	Name      string `json:"name" jsonschema:"required software name in the registry"`
	PackageID string `json:"package_id,omitempty" jsonschema:"registry id when multiple registries exist"`
	Ref       string `json:"ref,omitempty" jsonschema:"git ref (default main)"`
}

type ImportOutput struct {
	SoftwareID  string   `json:"software_id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	CreatedSW   bool     `json:"created_software"`
	CreatedVer  bool     `json:"created_version"`
	InstallURL  string   `json:"install_url,omitempty"`
	TriedPaths  []string `json:"tried_paths,omitempty"`
	Message     string   `json:"message"`
	InstallHint string   `json:"install_hint"`
}

type RegistriesOutput struct {
	Count      int              `json:"count"`
	Registries []RegistryPublic `json:"registries"`
	Message    string           `json:"message"`
}

type RegistryPublic struct {
	ID         string `json:"id"`
	PackageURL string `json:"package_url"`
	Username   string `json:"username,omitempty"`
	HasToken   bool   `json:"has_token"`
	HasPassword bool  `json:"has_password"`
}

func (c *Controller) ImportTool(ctx context.Context, _ *mcp.CallToolRequest, input ImportInput) (*mcp.CallToolResult, any, error) {
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	pkg, err := resolveRegistry(db, strings.TrimSpace(input.PackageID))
	if err != nil {
		return nil, nil, err
	}
	res, err := pkglib.Import(ctx, db, *pkg, name, &pkglib.ImportOptions{
		Ref: strings.TrimSpace(input.Ref),
	})
	if err != nil {
		return nil, nil, err
	}
	pkglib.InvalidateCatalogCache()
	return &mcp.CallToolResult{}, ImportOutput{
		SoftwareID:  res.Software.ID,
		Name:        res.Software.Name,
		Version:     res.Version.Version,
		CreatedSW:   res.CreatedSW,
		CreatedVer:  res.CreatedVer,
		InstallURL:  res.InstallURL,
		TriedPaths:  res.TriedPaths,
		Message:     fmt.Sprintf("Imported %s %s from registry", res.Software.Name, res.Version.Version),
		InstallHint: fmt.Sprintf("Install via softwares_install name_or_id=%q", res.Software.Name),
	}, nil
}

func (c *Controller) RegistriesTool(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	if _, err := pkglib.EnsureDefaultRegistry(db); err != nil {
		return nil, nil, err
	}
	var rows []models.SoftwarePackage
	if err := db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	out := make([]RegistryPublic, 0, len(rows))
	for _, row := range rows {
		out = append(out, RegistryPublic{
			ID:          row.ID,
			PackageURL:  row.PackageURL,
			Username:    row.Username,
			HasToken:    strings.TrimSpace(row.Token) != "",
			HasPassword: row.Password != "",
		})
	}
	msg := fmt.Sprintf("%d registry(ies) configured", len(out))
	if len(out) == 0 {
		msg = "No registries — default " + pkglib.DefaultRegistryURL + " should have been seeded"
	}
	return &mcp.CallToolResult{}, RegistriesOutput{
		Count:      len(out),
		Registries: out,
		Message:    msg,
	}, nil
}

func resolveRegistry(db *gorm.DB, packageID string) (*models.SoftwarePackage, error) {
	if db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if _, err := pkglib.EnsureDefaultRegistry(db); err != nil {
		return nil, err
	}
	if packageID != "" {
		var row models.SoftwarePackage
		if err := db.Where("id = ?", packageID).First(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}
	var rows []models.SoftwarePackage
	if err := db.Order("created_at DESC").Limit(2).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no software package registry configured")
	}
	if len(rows) > 1 {
		// Prefer the default public registry when several exist.
		for i := range rows {
			if pkglib.SameGitHubRepo(rows[i].PackageURL, pkglib.DefaultRegistryURL) {
				return &rows[i], nil
			}
		}
		return nil, fmt.Errorf("multiple registries — pass package_id")
	}
	return &rows[0], nil
}
