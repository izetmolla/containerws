package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ScaffoldInput struct {
	Name          string   `json:"name" jsonschema:"required software package name (slug)"`
	Details       string   `json:"details,omitempty" jsonschema:"short description"`
	Category      string   `json:"category,omitempty" jsonschema:"category e.g. Web, Tools, Database"`
	SubCategory   string   `json:"sub_category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Icon          string   `json:"icon,omitempty" jsonschema:"icon name e.g. Server, Package"`
	Image         string   `json:"image,omitempty" jsonschema:"optional logo URL (https or data URI)"`
	Color         string   `json:"color,omitempty" jsonschema:"hex color e.g. #009639"`
	Order          int      `json:"order,omitempty"`
	ServiceUnits   []string `json:"service_units,omitempty" jsonschema:"systemd units e.g. nginx.service"`
	CanControl     *bool    `json:"can_control,omitempty" jsonschema:"true to expose Start/Stop/Restart; omit to infer from service_units"`
	ControlBackend string   `json:"control_backend,omitempty" jsonschema:"systemd or docker"`
	StartCommand   string   `json:"start_command,omitempty" jsonschema:"shell command to start the service e.g. systemctl start nginx.service"`
	RestartCommand string   `json:"restart_command,omitempty" jsonschema:"shell command to restart the service"`
	StopCommand    string   `json:"stop_command,omitempty" jsonschema:"shell command to stop the service"`
	Version        string   `json:"version,omitempty" jsonschema:"package version string (default 1.0.0)"`
	Distros       []string `json:"distros,omitempty" jsonschema:"distro ids: ubuntu,debian,fedora,alpine,arch,default"`
	AptPackage    string   `json:"apt_package,omitempty" jsonschema:"Debian/Ubuntu package name if different from name"`
	DnfPackage    string   `json:"dnf_package,omitempty" jsonschema:"Fedora/RHEL package name if different"`
	ApkPackage    string   `json:"apk_package,omitempty" jsonschema:"Alpine package name if different"`
	PacmanPackage string   `json:"pacman_package,omitempty" jsonschema:"Arch package name if different"`
	CustomScript  string   `json:"custom_script,omitempty" jsonschema:"optional bash run after successful install (post-setup)"`
	OutputDir     string   `json:"output_dir" jsonschema:"required path to cws-packages repo root"`
	Overwrite     bool     `json:"overwrite,omitempty" jsonschema:"overwrite existing files"`
	UpdateIndex   *bool    `json:"update_index,omitempty" jsonschema:"merge into softwares/index.json (default true)"`
}

type ScaffoldOutput struct {
	Name      string   `json:"name"`
	Root      string   `json:"root"`
	Files     []string `json:"files"`
	Distros   []string `json:"distros"`
	IndexPath string   `json:"index_path,omitempty"`
	Message   string   `json:"message"`
}

func (c *Controller) ScaffoldTool(_ context.Context, _ *mcp.CallToolRequest, input ScaffoldInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	outDir := strings.TrimSpace(input.OutputDir)
	if outDir == "" {
		return nil, nil, fmt.Errorf("output_dir is required (registry repo root)")
	}

	res, err := pkglib.Scaffold(pkglib.ScaffoldRequest{
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
		Distros:       input.Distros,
		AptPackage:    input.AptPackage,
		DnfPackage:    input.DnfPackage,
		ApkPackage:    input.ApkPackage,
		PacmanPackage: input.PacmanPackage,
		CustomScript:  input.CustomScript,
		OutputDir:     outDir,
		Overwrite:     input.Overwrite,
		UpdateIndex:   input.UpdateIndex,
	})
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{}, ScaffoldOutput{
		Name:      res.Name,
		Root:      res.Root,
		Files:     res.Files,
		Distros:   res.Distros,
		IndexPath: res.IndexPath,
		Message: fmt.Sprintf(
			"Scaffolded %s with %d file(s) for distros [%s]. Commit/push to the registry repo, then softwarepkg_import when ready.",
			res.Name, len(res.Files), strings.Join(res.Distros, ", "),
		),
	}, nil
}
