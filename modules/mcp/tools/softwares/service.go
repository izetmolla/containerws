package softwares

import (
	"context"
	"fmt"
	"strings"

	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServiceInput struct {
	NameOrID string `json:"name_or_id" jsonschema:"required catalog software id or name — must be listed"`
	Action   string `json:"action,omitempty" jsonschema:"status (default), start, stop, restart, or logs"`
	Lines    int    `json:"lines,omitempty" jsonschema:"for action=logs, number of recent journal lines (default 120)"`
}

type ServiceOutput struct {
	Listed     bool            `json:"listed"`
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Action     string          `json:"action,omitempty"`
	CanControl bool            `json:"can_control"`
	Status     *service.Status `json:"status,omitempty"`
	Units      []string        `json:"units,omitempty"`
	Logs       []service.LogLine `json:"logs,omitempty"`
}

func (c *Controller) ServiceTool(_ context.Context, _ *mcp.CallToolRequest, input ServiceInput) (*mcp.CallToolResult, any, error) {
	c.ensureCatalog()
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}

	query := strings.TrimSpace(input.NameOrID)
	if query == "" {
		return nil, nil, fmt.Errorf("name_or_id is required")
	}

	action := strings.ToLower(strings.TrimSpace(input.Action))
	if action == "" {
		action = "status"
	}
	switch action {
	case "status", "start", "stop", "restart", "logs":
	default:
		return nil, nil, fmt.Errorf("action must be status, start, stop, restart, or logs")
	}

	sw, err := findSoftware(db, query)
	if err != nil {
		return nil, nil, err
	}
	if sw == nil {
		out := ServiceOutput{
			Listed:  false,
			Success: false,
			Message: fmt.Sprintf("%q is not listed in the Softwares catalog", query),
			Action:  action,
		}
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	units := []string(sw.ServiceUnits)
	canControl := service.CanControl(*sw)
	if !canControl {
		out := ServiceOutput{
			Listed:     true,
			Success:    false,
			ID:         sw.ID,
			Name:       sw.Name,
			Action:     action,
			CanControl: false,
			Message:    fmt.Sprintf("%s is not a start/stop typology (can_control + service_units required)", sw.Name),
		}
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	if action == "status" {
		st := service.ProbeUnits(units)
		return &mcp.CallToolResult{}, ServiceOutput{
			Listed:     true,
			Success:    true,
			ID:         sw.ID,
			Name:       sw.Name,
			Action:     action,
			CanControl: true,
			Status:     &st,
			Units:      units,
			Message:    fmt.Sprintf("%s service overall=%s", sw.Name, st.Overall),
		}, nil
	}

	if action == "logs" {
		lines := input.Lines
		if lines <= 0 {
			lines = 120
		}
		logs, lerr := service.TailLogs(context.Background(), units, lines)
		if lerr != nil {
			out := ServiceOutput{
				Listed:     true,
				Success:    false,
				ID:         sw.ID,
				Name:       sw.Name,
				Action:     action,
				CanControl: true,
				Units:      units,
				Message:    lerr.Error(),
			}
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
		return &mcp.CallToolResult{}, ServiceOutput{
			Listed:     true,
			Success:    true,
			ID:         sw.ID,
			Name:       sw.Name,
			Action:     action,
			CanControl: true,
			Units:      units,
			Logs:       logs,
			Message:    fmt.Sprintf("%s: %d log line(s)", sw.Name, len(logs)),
		}, nil
	}

	st, err := service.ControlSoftware(action, *sw)
	if err != nil {
		out := ServiceOutput{
			Listed:     true,
			Success:    false,
			ID:         sw.ID,
			Name:       sw.Name,
			Action:     action,
			CanControl: true,
			Status:     &st,
			Units:      units,
			Message:    err.Error(),
		}
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	return &mcp.CallToolResult{}, ServiceOutput{
		Listed:     true,
		Success:    true,
		ID:         sw.ID,
		Name:       sw.Name,
		Action:     action,
		CanControl: true,
		Status:     &st,
		Units:      units,
		Message:    fmt.Sprintf("%s %s ok (overall=%s)", sw.Name, action, st.Overall),
	}, nil
}
