package softwares

import (
	"context"
	"fmt"
	"strings"

	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type QueueInput struct{}

type QueueOutput struct {
	Running bool                     `json:"running"`
	Pending int                      `json:"pending"`
	Items   []swinstall.QueueViewItem `json:"items"`
	Message string                   `json:"message"`
}

func (c *Controller) QueueTool(ctx context.Context, _ *mcp.CallToolRequest, _ QueueInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	db := c.db()
	view := swinstall.ActiveQueue(db)
	msg := "queue is empty"
	if view.Pending > 0 {
		msg = fmt.Sprintf("%d pending/running job(s)", view.Pending)
	} else if len(view.Items) > 0 {
		msg = fmt.Sprintf("%d item(s) need attention (failed)", len(view.Items))
	}
	return &mcp.CallToolResult{}, QueueOutput{
		Running: view.Running,
		Pending: view.Pending,
		Items:   view.Items,
		Message: msg,
	}, nil
}

type QueueDismissInput struct {
	ID         string `json:"id,omitempty" jsonschema:"queue item id (or job-… for standalone failed jobs)"`
	SoftwareID string `json:"software_id,omitempty" jsonschema:"optional software_id / brew:token to dismiss failed rows"`
}

type QueueDismissOutput struct {
	Removed int    `json:"removed"`
	Message string `json:"message"`
}

func (c *Controller) QueueDismissTool(ctx context.Context, _ *mcp.CallToolRequest, input QueueDismissInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	id := strings.TrimSpace(input.ID)
	softwareID := strings.TrimSpace(input.SoftwareID)
	removed, err := swinstall.DismissFromQueue(c.db(), id, softwareID)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, QueueDismissOutput{Message: err.Error()}, nil
	}
	return &mcp.CallToolResult{}, QueueDismissOutput{
		Removed: removed,
		Message: "Removed from installing queue",
	}, nil
}
