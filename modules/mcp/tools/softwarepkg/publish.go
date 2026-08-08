package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PublishInput struct {
	Name          string   `json:"name" jsonschema:"required software package name to add"`
	PackageID     string   `json:"package_id,omitempty" jsonschema:"registry id when multiple registries exist"`
	Ref           string   `json:"ref,omitempty" jsonschema:"git branch (default main)"`
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
	Distros       []string `json:"distros,omitempty" jsonschema:"used when from_hub=false"`
	AptPackage    string   `json:"apt_package,omitempty"`
	DnfPackage    string   `json:"dnf_package,omitempty"`
	ApkPackage    string   `json:"apk_package,omitempty"`
	PacmanPackage string   `json:"pacman_package,omitempty"`
	CustomScript  string   `json:"custom_script,omitempty" jsonschema:"optional bash run after successful install (post-setup)"`
	// FromHub defaults true — scaffold install.json for every Hub workspace tag.
	FromHub       *bool  `json:"from_hub,omitempty" jsonschema:"scaffold from Docker Hub tags (default true)"`
	HubImage      string `json:"hub_image,omitempty"`
	AlsoAny       bool   `json:"also_any,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
	AuthorEmail   string `json:"author_email,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty" jsonschema:"clone+scaffold+commit but do not push"`
	KeepWorkDir   bool   `json:"keep_work_dir,omitempty" jsonschema:"keep temp clone directory"`
}

type PublishOutput struct {
	Name      string   `json:"name"`
	Repo      string   `json:"repo"`
	Ref       string   `json:"ref"`
	WorkDir   string   `json:"work_dir,omitempty"`
	Files     []string `json:"files"`
	Distros   []string `json:"distros"`
	Commit    string   `json:"commit,omitempty"`
	Pushed    bool     `json:"pushed"`
	DryRun    bool     `json:"dry_run,omitempty"`
	Message   string   `json:"message"`
	NextSteps []string `json:"next_steps,omitempty"`
}

func (c *Controller) PublishTool(ctx context.Context, _ *mcp.CallToolRequest, input PublishInput) (*mcp.CallToolResult, any, error) {
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	reg, err := resolveRegistry(db, strings.TrimSpace(input.PackageID))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(reg.PackageURL) == "" {
		return nil, nil, fmt.Errorf("registry package_url is empty")
	}

	fromHub := true
	if input.FromHub != nil {
		fromHub = *input.FromHub
	}
	if !fromHub && len(input.Distros) == 0 {
		input.Distros = pkglib.DefaultDistros()
	}

	res, err := pkglib.Publish(ctx, pkglib.PublishRequest{
		Registry:      *reg,
		Ref:           input.Ref,
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
		FromHub:       fromHub,
		HubImage:      input.HubImage,
		AlsoAny:       input.AlsoAny || fromHub,
		CommitMessage: input.CommitMessage,
		AuthorName:    input.AuthorName,
		AuthorEmail:   input.AuthorEmail,
		DryRun:        input.DryRun,
		KeepWorkDir:   input.KeepWorkDir,
	})
	if err != nil {
		return nil, nil, err
	}

	next := []string{}
	if res.Pushed {
		next = append(next,
			fmt.Sprintf("Import locally: softwarepkg_import name=%q", res.Name),
			fmt.Sprintf("Optional: softwarepkg_test_install name=%q (after cloning or with package_root)", res.Name),
		)
	} else if res.DryRun {
		next = append(next, "Re-run with dry_run=false to push (requires registry token with write access).")
	}

	return &mcp.CallToolResult{}, PublishOutput{
		Name:      res.Name,
		Repo:      res.Repo,
		Ref:       res.Ref,
		WorkDir:   res.WorkDir,
		Files:     res.Files,
		Distros:   res.Distros,
		Commit:    res.Commit,
		Pushed:    res.Pushed,
		DryRun:    res.DryRun,
		Message:   res.Message,
		NextSteps: next,
	}, nil
}
