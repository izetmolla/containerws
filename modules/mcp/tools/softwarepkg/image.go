package softwarepkg

import (
	"context"
	"fmt"
	"strings"

	pkglib "github.com/izetmolla/containerws/packages/softwarepkg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ImageInput struct {
	Name      string `json:"name" jsonschema:"required software / package name"`
	Action    string `json:"action,omitempty" jsonschema:"set | find | generate (default find when image empty, else set)"`
	Image     string `json:"image,omitempty" jsonschema:"https URL or data:image URI when action=set"`
	Domain    string `json:"domain,omitempty" jsonschema:"optional domain hint for find (e.g. nginx.org)"`
	Query     string `json:"query,omitempty" jsonschema:"optional logo search slug (defaults to name)"`
	Color     string `json:"color,omitempty" jsonschema:"hex color for generated SVG (default #0ea5e9)"`
	ApplyLocal *bool `json:"apply_local,omitempty" jsonschema:"update local Softwares.image when row exists (default true)"`
	OutputDir string `json:"output_dir,omitempty" jsonschema:"optional registry root to write package.json image (+ image.svg when generated)"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"allow overwriting image.svg under output_dir"`
}

type ImageOutput struct {
	Name       string   `json:"name"`
	Action     string   `json:"action"`
	Image      string   `json:"image"`
	Source     string   `json:"source"`
	Candidates []string `json:"candidates,omitempty"`
	Applied    bool     `json:"applied"`
	SoftwareID string   `json:"software_id,omitempty"`
	Files      []string `json:"files,omitempty"`
	Message    string   `json:"message"`
}

func (c *Controller) ImageTool(ctx context.Context, _ *mcp.CallToolRequest, input ImageInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	apply := true
	if input.ApplyLocal != nil {
		apply = *input.ApplyLocal
	}

	res, err := pkglib.SetPackageImage(ctx, c.db(), pkglib.SetImageRequest{
		Action:     input.Action,
		Name:       name,
		Image:      input.Image,
		Domain:     input.Domain,
		Query:      input.Query,
		Color:      input.Color,
		ApplyLocal: apply,
		OutputDir:  input.OutputDir,
		Overwrite:  input.Overwrite,
	})
	if err != nil {
		return nil, nil, err
	}
	pkglib.InvalidateCatalogCache()
	return &mcp.CallToolResult{}, ImageOutput{
		Name:       res.Name,
		Action:     res.Action,
		Image:      res.Image,
		Source:     res.Source,
		Candidates: res.Candidates,
		Applied:    res.Applied,
		SoftwareID: res.SoftwareID,
		Files:      res.Files,
		Message:    res.Message,
	}, nil
}
