package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HubDistrosInput struct {
	Image               string `json:"image,omitempty" jsonschema:"Docker Hub image namespace/name (default izetmolla/containerws)"`
	IncludeNonWorkspace bool   `json:"include_non_workspace,omitempty" jsonschema:"include latest/binoptimization/unparsed tags"`
}

type HubDistrosOutput struct {
	Image   string          `json:"image"`
	Count   int             `json:"count"`
	Tags    []pkglib.HubTag `json:"tags"`
	Message string          `json:"message"`
}

func (c *Controller) HubDistrosTool(ctx context.Context, _ *mcp.CallToolRequest, input HubDistrosInput) (*mcp.CallToolResult, any, error) {
	image := strings.TrimSpace(input.Image)
	if image == "" {
		image = pkglib.DefaultHubImage
	}
	tags, err := pkglib.ListHubTags(ctx, &pkglib.ListHubTagsOptions{
		Image:               image,
		IncludeNonWorkspace: input.IncludeNonWorkspace,
	})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{}, HubDistrosOutput{
		Image:   image,
		Count:   len(tags),
		Tags:    tags,
		Message: fmt.Sprintf("Found %d tag(s) on https://hub.docker.com/r/%s", len(tags), image),
	}, nil
}

type ScaffoldHubInput struct {
	Name          string   `json:"name" jsonschema:"required software package name"`
	Details       string   `json:"details,omitempty"`
	Category      string   `json:"category,omitempty"`
	SubCategory   string   `json:"sub_category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Image         string   `json:"image,omitempty" jsonschema:"optional logo URL (https or data URI)"`
	Color         string   `json:"color,omitempty"`
	Order          int      `json:"order,omitempty"`
	ServiceUnits   []string `json:"service_units,omitempty"`
	CanControl     *bool    `json:"can_control,omitempty" jsonschema:"true to expose Start/Stop/Restart"`
	ControlBackend string   `json:"control_backend,omitempty" jsonschema:"systemd or docker"`
	StartCommand   string   `json:"start_command,omitempty" jsonschema:"shell command to start the service"`
	RestartCommand string   `json:"restart_command,omitempty" jsonschema:"shell command to restart the service"`
	StopCommand    string   `json:"stop_command,omitempty" jsonschema:"shell command to stop the service"`
	Version        string   `json:"version,omitempty"`
	AptPackage    string   `json:"apt_package,omitempty"`
	DnfPackage    string   `json:"dnf_package,omitempty"`
	ApkPackage    string   `json:"apk_package,omitempty"`
	PacmanPackage string   `json:"pacman_package,omitempty"`
	CustomScript  string   `json:"custom_script,omitempty" jsonschema:"optional bash run after successful install (post-setup)"`
	OutputDir     string   `json:"output_dir" jsonschema:"required cws-packages repo root"`
	HubImage      string   `json:"hub_image,omitempty" jsonschema:"default izetmolla/containerws"`
	AlsoAny       *bool    `json:"also_any,omitempty" jsonschema:"also write {distro}/any/any/install.json (default true)"`
	AlsoDefault   *bool    `json:"also_default,omitempty" jsonschema:"also write default/install.json (default true)"`
	Overwrite     bool     `json:"overwrite,omitempty"`
}

type ScaffoldHubOutput struct {
	Name      string   `json:"name"`
	Root      string   `json:"root"`
	Files     []string `json:"files"`
	Distros   []string `json:"distros"`
	IndexPath string   `json:"index_path,omitempty"`
	Message   string   `json:"message"`
	NextStep  string   `json:"next_step"`
}

func (c *Controller) ScaffoldHubTool(ctx context.Context, _ *mcp.CallToolRequest, input ScaffoldHubInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	outDir := strings.TrimSpace(input.OutputDir)
	if outDir == "" {
		return nil, nil, fmt.Errorf("output_dir is required")
	}
	alsoAny := true
	if input.AlsoAny != nil {
		alsoAny = *input.AlsoAny
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
		AptPackage:    input.AptPackage,
		DnfPackage:    input.DnfPackage,
		ApkPackage:    input.ApkPackage,
		PacmanPackage: input.PacmanPackage,
		CustomScript:  input.CustomScript,
		OutputDir:     outDir,
		Overwrite:     input.Overwrite,
		FromHub:       true,
		HubImage:      input.HubImage,
		AlsoAny:       alsoAny,
		AlsoDefault:   input.AlsoDefault,
	})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{}, ScaffoldHubOutput{
		Name:      res.Name,
		Root:      res.Root,
		Files:     res.Files,
		Distros:   res.Distros,
		IndexPath: res.IndexPath,
		Message: fmt.Sprintf(
			"Scaffolded %s from Docker Hub tags (%d files, distros: %s)",
			res.Name, len(res.Files), strings.Join(res.Distros, ", "),
		),
		NextStep: "Call softwarepkg_test_install with the same name + package_root to verify install scripts in containers.",
	}, nil
}

type TestInstallInput struct {
	Name           string   `json:"name" jsonschema:"required software name under softwares/{name}"`
	PackageRoot    string   `json:"package_root" jsonschema:"required registry repo root with softwares/"`
	HubImage       string   `json:"hub_image,omitempty"`
	Tags           []string `json:"tags,omitempty" jsonschema:"optional subset e.g. ubuntu-26.04,debian-13"`
	VerifyCommand  string   `json:"verify_command,omitempty" jsonschema:"shell check after install (default: command -v <pkg>)"`
	Pull           bool     `json:"pull,omitempty" jsonschema:"docker pull before each run"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"per-tag timeout (default 600)"`
	DryRun         bool     `json:"dry_run,omitempty" jsonschema:"resolve scripts only; do not run docker"`
}

func (c *Controller) TestInstallTool(ctx context.Context, _ *mcp.CallToolRequest, input TestInstallInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	root := strings.TrimSpace(input.PackageRoot)
	if name == "" || root == "" {
		return nil, nil, fmt.Errorf("name and package_root are required")
	}
	res, err := pkglib.TestInstall(ctx, pkglib.TestInstallRequest{
		Name:           name,
		PackageRoot:    root,
		HubImage:       input.HubImage,
		Tags:           input.Tags,
		VerifyCommand:  input.VerifyCommand,
		Pull:           input.Pull,
		TimeoutSeconds: input.TimeoutSeconds,
		DryRun:         input.DryRun,
	})
	if err != nil {
		return nil, nil, err
	}
	result := &mcp.CallToolResult{}
	if res.Failed > 0 {
		result.IsError = true
	}
	return result, res, nil
}
