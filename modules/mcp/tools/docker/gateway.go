package dockermcp

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GatewayStatusInput struct{}

type GatewayStatusOutput struct {
	CLIPresent bool   `json:"cli_present"`
	Path       string `json:"path,omitempty"`
	Version    string `json:"version,omitempty"`
	HelpHead   string `json:"help_head,omitempty"`
	Message    string `json:"message"`
}

func (c *Controller) GatewayStatusTool(ctx context.Context, _ *mcp.CallToolRequest, _ GatewayStatusInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	path, err := exec.LookPath("docker")
	if err != nil {
		return &mcp.CallToolResult{}, GatewayStatusOutput{
			Message: "docker CLI not found on PATH — install Docker Engine / CLI first",
		}, nil
	}

	// Probe `docker mcp` plugin (https://github.com/docker/mcp-gateway).
	cmd := exec.CommandContext(runCtx, "docker", "mcp", "version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := GatewayStatusOutput{Path: path}

	if runErr != nil {
		// Older plugins may only support --help.
		helpCmd := exec.CommandContext(runCtx, "docker", "mcp", "--help")
		var helpOut bytes.Buffer
		helpCmd.Stdout = &helpOut
		helpCmd.Stderr = &helpOut
		if helpErr := helpCmd.Run(); helpErr != nil {
			out.Message = "docker mcp CLI plugin not available (install Docker Desktop MCP Toolkit or build docker/mcp-gateway)"
			out.HelpHead = truncate(strings.TrimSpace(stderr.String()+"\n"+helpOut.String()), 800)
			return &mcp.CallToolResult{}, out, nil
		}
		out.CLIPresent = true
		out.HelpHead = truncate(helpOut.String(), 1200)
		out.Message = "docker mcp plugin detected"
		return &mcp.CallToolResult{}, out, nil
	}

	out.CLIPresent = true
	out.Version = strings.TrimSpace(stdout.String())
	if out.Version == "" {
		out.Version = strings.TrimSpace(stderr.String())
	}
	out.Message = "docker mcp CLI plugin available"
	return &mcp.CallToolResult{}, out, nil
}

type GatewayToolsListInput struct {
	Format string `json:"format,omitempty" jsonschema:"optional output format: json|table (default json)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"truncate output characters (default 20000)"`
}

type GatewayToolsListOutput struct {
	Output  string `json:"output"`
	Message string `json:"message"`
}

func (c *Controller) GatewayToolsListTool(ctx context.Context, _ *mcp.CallToolRequest, input GatewayToolsListInput) (*mcp.CallToolResult, any, error) {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	format := strings.ToLower(strings.TrimSpace(input.Format))
	if format == "" {
		format = "json"
	}
	args := []string{"mcp", "tools", "ls"}
	if format == "json" {
		args = append(args, "--format=json")
	}

	cmd := exec.CommandContext(runCtx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return &mcp.CallToolResult{IsError: true}, GatewayToolsListOutput{
			Output:  truncate(stdout.String(), 4000),
			Message: "docker mcp tools ls failed: " + msg,
		}, nil
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20000
	}
	text := stdout.String()
	return &mcp.CallToolResult{}, GatewayToolsListOutput{
		Output:  truncate(text, limit),
		Message: fmt.Sprintf("listed gateway tools (%d chars)", len(text)),
	}, nil
}
