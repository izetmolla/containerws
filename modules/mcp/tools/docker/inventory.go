package dockermcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ImagesListInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	All           bool   `json:"all,omitempty" jsonschema:"include intermediate images"`
	Reference     string `json:"reference,omitempty" jsonschema:"optional reference filter e.g. nginx*"`
	Dangling      *bool  `json:"dangling,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type imageRow struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
}

type ImagesListOutput struct {
	Items   []imageRow `json:"items"`
	Total   int        `json:"total"`
	Message string     `json:"message"`
}

func (c *Controller) ImagesListTool(ctx context.Context, _ *mcp.CallToolRequest, input ImagesListInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := c.withTimeout(ctx, 0, 30*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesListOutput{Message: err.Error()}, nil
	}
	f := filters.NewArgs()
	if r := strings.TrimSpace(input.Reference); r != "" {
		f.Add("reference", r)
	}
	if input.Dangling != nil {
		if *input.Dangling {
			f.Add("dangling", "true")
		} else {
			f.Add("dangling", "false")
		}
	}
	list, err := cli.ImageList(runCtx, image.ListOptions{All: input.All, Filters: f})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesListOutput{Message: err.Error()}, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	items := make([]imageRow, 0, min(len(list), limit))
	for i, it := range list {
		if i >= limit {
			break
		}
		id := it.ID
		if strings.HasPrefix(id, "sha256:") && len(id) > 19 {
			id = id[7:19]
		}
		items = append(items, imageRow{
			ID:       id,
			RepoTags: it.RepoTags,
			Size:     it.Size,
			Created:  it.Created,
		})
	}
	return &mcp.CallToolResult{}, ImagesListOutput{
		Items:   items,
		Total:   len(list),
		Message: fmt.Sprintf("%d image(s)", len(list)),
	}, nil
}

type ImagesPullInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	Reference     string `json:"reference" jsonschema:"required image reference e.g. nginx:alpine"`
}

type ImagesPullOutput struct {
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

func (c *Controller) ImagesPullTool(ctx context.Context, _ *mcp.CallToolRequest, input ImagesPullInput) (*mcp.CallToolResult, any, error) {
	ref := strings.TrimSpace(input.Reference)
	if ref == "" {
		return nil, nil, fmt.Errorf("reference is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 10*time.Minute)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesPullOutput{Reference: ref, Message: err.Error()}, nil
	}
	reader, err := cli.ImagePull(runCtx, ref, image.PullOptions{})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesPullOutput{Reference: ref, Message: err.Error()}, nil
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return &mcp.CallToolResult{}, ImagesPullOutput{Reference: ref, Message: "pulled"}, nil
}

type ImagesRemoveInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	IDOrRef       string `json:"id_or_ref" jsonschema:"required image id or reference"`
	Force         bool   `json:"force,omitempty"`
	PruneChildren bool   `json:"prune_children,omitempty"`
}

type ImagesRemoveOutput struct {
	Deleted []string `json:"deleted,omitempty"`
	Message string   `json:"message"`
}

func (c *Controller) ImagesRemoveTool(ctx context.Context, _ *mcp.CallToolRequest, input ImagesRemoveInput) (*mcp.CallToolResult, any, error) {
	id := strings.TrimSpace(input.IDOrRef)
	if id == "" {
		return nil, nil, fmt.Errorf("id_or_ref is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesRemoveOutput{Message: err.Error()}, nil
	}
	resp, err := cli.ImageRemove(runCtx, id, image.RemoveOptions{Force: input.Force, PruneChildren: input.PruneChildren})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, ImagesRemoveOutput{Message: err.Error()}, nil
	}
	deleted := make([]string, 0, len(resp))
	for _, r := range resp {
		if r.Deleted != "" {
			deleted = append(deleted, r.Deleted)
		} else if r.Untagged != "" {
			deleted = append(deleted, "untagged:"+r.Untagged)
		}
	}
	return &mcp.CallToolResult{}, ImagesRemoveOutput{Deleted: deleted, Message: "removed"}, nil
}

type VolumesListInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
}

type volumeRow struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint,omitempty"`
	Scope      string `json:"scope,omitempty"`
}

type VolumesListOutput struct {
	Items   []volumeRow `json:"items"`
	Total   int         `json:"total"`
	Message string      `json:"message"`
}

func (c *Controller) VolumesListTool(ctx context.Context, _ *mcp.CallToolRequest, input VolumesListInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := c.withTimeout(ctx, 0, 30*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, VolumesListOutput{Message: err.Error()}, nil
	}
	list, err := cli.VolumeList(runCtx, volume.ListOptions{})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, VolumesListOutput{Message: err.Error()}, nil
	}
	items := make([]volumeRow, 0, len(list.Volumes))
	for _, v := range list.Volumes {
		if v == nil {
			continue
		}
		items = append(items, volumeRow{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Scope:      v.Scope,
		})
	}
	return &mcp.CallToolResult{}, VolumesListOutput{
		Items:   items,
		Total:   len(items),
		Message: fmt.Sprintf("%d volume(s)", len(items)),
	}, nil
}

type VolumesRemoveInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	Name          string `json:"name" jsonschema:"required volume name"`
	Force         bool   `json:"force,omitempty"`
}

type VolumesRemoveOutput struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (c *Controller) VolumesRemoveTool(ctx context.Context, _ *mcp.CallToolRequest, input VolumesRemoveInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	runCtx, cancel := c.withTimeout(ctx, 0, 60*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, VolumesRemoveOutput{Name: name, Message: err.Error()}, nil
	}
	if err := cli.VolumeRemove(runCtx, name, input.Force); err != nil {
		return &mcp.CallToolResult{IsError: true}, VolumesRemoveOutput{Name: name, Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, VolumesRemoveOutput{Name: name, Message: "removed"}, nil
}

type NetworksListInput struct {
	EnvironmentID string `json:"environment_id,omitempty"`
}

type networkRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

type NetworksListOutput struct {
	Items   []networkRow `json:"items"`
	Total   int          `json:"total"`
	Message string       `json:"message"`
}

func (c *Controller) NetworksListTool(ctx context.Context, _ *mcp.CallToolRequest, input NetworksListInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := c.withTimeout(ctx, 0, 30*time.Second)
	defer cancel()
	_, cli, err := c.resolveClient(input.EnvironmentID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, NetworksListOutput{Message: err.Error()}, nil
	}
	list, err := cli.NetworkList(runCtx, network.ListOptions{})
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, NetworksListOutput{Message: err.Error()}, nil
	}
	items := make([]networkRow, 0, len(list))
	for _, n := range list {
		id := n.ID
		if len(id) > 12 {
			id = id[:12]
		}
		items = append(items, networkRow{
			ID:     id,
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}
	return &mcp.CallToolResult{}, NetworksListOutput{
		Items:   items,
		Total:   len(items),
		Message: fmt.Sprintf("%d network(s)", len(items)),
	}, nil
}
