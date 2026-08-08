package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CreateInput struct {
	Name          string   `json:"name" jsonschema:"required software name"`
	Details       string   `json:"details,omitempty"`
	Category      string   `json:"category,omitempty"`
	SubCategory   string   `json:"sub_category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Image         string   `json:"image,omitempty" jsonschema:"optional logo URL (https or data URI)"`
	Color         string   `json:"color,omitempty"`
	Order         int      `json:"order,omitempty"`
	ServiceUnits  []string `json:"service_units,omitempty" jsonschema:"systemd units e.g. nginx.service or docker.service"`
	CanControl    *bool    `json:"can_control,omitempty" jsonschema:"true to expose Start/Stop/Restart; omit to infer from service_units"`
	ControlBackend string  `json:"control_backend,omitempty" jsonschema:"systemd or docker (auto when empty)"`
	StartCommand   string  `json:"start_command,omitempty" jsonschema:"shell command to start e.g. systemctl start nginx.service"`
	RestartCommand string  `json:"restart_command,omitempty" jsonschema:"shell command to restart"`
	StopCommand    string  `json:"stop_command,omitempty" jsonschema:"shell command to stop"`
	Version       string   `json:"version,omitempty"`
	AptPackage    string   `json:"apt_package,omitempty"`
	DnfPackage    string   `json:"dnf_package,omitempty"`
	ApkPackage    string   `json:"apk_package,omitempty"`
	PacmanPackage string   `json:"pacman_package,omitempty"`
	CustomScript  string   `json:"custom_script,omitempty" jsonschema:"optional bash run after successful install (post-setup)"`
	// Optional: also write registry files under output_dir.
	OutputDir string   `json:"output_dir,omitempty" jsonschema:"optional cws-packages root to also scaffold files"`
	Distros   []string `json:"distros,omitempty" jsonschema:"distros for scaffold when output_dir is set"`
	Overwrite bool     `json:"overwrite,omitempty"`
}

type CreateOutput struct {
	SoftwareID   string   `json:"software_id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	CreatedSW    bool     `json:"created_software"`
	CreatedVer   bool     `json:"created_version"`
	DistroID     string   `json:"distro_id"`
	Family       string   `json:"package_family"`
	PkgName      string   `json:"os_package"`
	Scaffolded   bool     `json:"scaffolded"`
	Files        []string `json:"files,omitempty"`
	Message      string   `json:"message"`
	InstallHint  string   `json:"install_hint"`
}

func (c *Controller) CreateTool(ctx context.Context, _ *mcp.CallToolRequest, input CreateInput) (*mcp.CallToolResult, any, error) {
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}

	local, err := pkglib.CreateLocal(ctx, db, pkglib.CreateLocalRequest{
		Name:          name,
		Details:       input.Details,
		Category:      input.Category,
		SubCategory:   input.SubCategory,
		Tags:          input.Tags,
		Icon:          input.Icon,
		Image:         input.Image,
		Color:         input.Color,
		Order:          input.Order,
		ServiceUnits:   input.ServiceUnits,
		CanControl:     input.CanControl,
		ControlBackend: input.ControlBackend,
		StartCommand:   input.StartCommand,
		RestartCommand: input.RestartCommand,
		StopCommand:    input.StopCommand,
		Version:        input.Version,
		AptPackage:    input.AptPackage,
		DnfPackage:    input.DnfPackage,
		ApkPackage:    input.ApkPackage,
		PacmanPackage: input.PacmanPackage,
		CustomScript:  input.CustomScript,
	})
	if err != nil {
		return nil, nil, err
	}

	out := CreateOutput{
		SoftwareID:  local.Software.ID,
		Name:        local.Software.Name,
		Version:     local.Version.Version,
		CreatedSW:   local.CreatedSW,
		CreatedVer:  local.CreatedVer,
		DistroID:    local.DistroID,
		Family:      local.Family,
		PkgName:     local.PkgName,
		InstallHint: fmt.Sprintf("Install via softwares_install name_or_id=%q (or Softwares UI).", local.Software.Name),
		Message: fmt.Sprintf(
			"Upserted local catalog entry %s@%s (%s/%s → %s package %q)",
			local.Software.Name, local.Version.Version, local.DistroID, local.Family, local.Family, local.PkgName,
		),
	}

	if dir := strings.TrimSpace(input.OutputDir); dir != "" {
		res, serr := pkglib.Scaffold(pkglib.ScaffoldRequest{
			Name:           name,
			Details:        input.Details,
			Category:       input.Category,
			SubCategory:    input.SubCategory,
			Tags:           input.Tags,
			Icon:           input.Icon,
			Image:          input.Image,
			Color:          input.Color,
			Order:          input.Order,
			ServiceUnits:   input.ServiceUnits,
			CanControl:     input.CanControl,
			ControlBackend: input.ControlBackend,
			StartCommand:   input.StartCommand,
			RestartCommand: input.RestartCommand,
			StopCommand:    input.StopCommand,
			Version:        input.Version,
			Distros:        input.Distros,
			AptPackage:     input.AptPackage,
			DnfPackage:     input.DnfPackage,
			ApkPackage:     input.ApkPackage,
			PacmanPackage:  input.PacmanPackage,
			CustomScript:   input.CustomScript,
			OutputDir:      dir,
			Overwrite:      input.Overwrite,
		})
		if serr != nil {
			return nil, nil, fmt.Errorf("local create ok, scaffold failed: %w", serr)
		}
		out.Scaffolded = true
		out.Files = res.Files
		out.Message += fmt.Sprintf("; scaffolded %d file(s) under %s", len(res.Files), res.Root)
	}

	pkglib.InvalidateCatalogCache()
	return &mcp.CallToolResult{}, out, nil
}
