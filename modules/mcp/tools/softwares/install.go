package softwares

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	installDefaultTimeout = 15 * time.Minute
	installMaxTimeout     = 30 * time.Minute
	installMaxOutput      = 256 * 1024
)

type InstallInput struct {
	NameOrID       string `json:"name_or_id" jsonschema:"required catalog software id or name — must be listed (softwares_lookup)"`
	Version        string `json:"version,omitempty" jsonschema:"optional version string; defaults to latest"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"optional timeout in seconds (default 900, max 1800)"`
	DryRun         bool   `json:"dry_run,omitempty" jsonschema:"when true, return the install script without executing"`
}

type InstallOutput struct {
	Listed        bool   `json:"listed"`
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Version       string `json:"version,omitempty"`
	ExitCode      int    `json:"exit_code,omitempty"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	Truncated     bool   `json:"truncated,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	TimedOut      bool   `json:"timed_out,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
	InstallScript string `json:"install_script,omitempty"`
	CustomScript  string `json:"custom_script,omitempty"`
}

func (c *Controller) InstallTool(ctx context.Context, _ *mcp.CallToolRequest, input InstallInput) (*mcp.CallToolResult, any, error) {
	c.ensureCatalog()
	db := c.db()
	if db == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}

	query := strings.TrimSpace(input.NameOrID)
	if query == "" {
		return nil, nil, fmt.Errorf("name_or_id is required")
	}

	sw, err := findSoftware(db, query)
	if err != nil {
		return nil, nil, err
	}
	if sw == nil {
		out := InstallOutput{
			Listed:  false,
			Success: false,
			Message: fmt.Sprintf("%q is not listed in the Softwares catalog — refuse to install via this tool; use softwares_list / softwares_lookup, or bash for ad-hoc packages", query),
		}
		result := &mcp.CallToolResult{IsError: true}
		return result, out, nil
	}

	ver, err := pickVersion(db, sw.ID, input.Version)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(ver.InstallScript) == "" {
		out := InstallOutput{
			Listed:  true,
			Success: false,
			ID:      sw.ID,
			Name:    sw.Name,
			Version: ver.Version,
			Message: fmt.Sprintf("%s %s has no install script", sw.Name, ver.Version),
		}
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	if pm := models.GetSoftwarePackageManager(db, sw.ID); pm == models.PackageManagerBrew {
		out := InstallOutput{
			Listed:  true,
			Success: false,
			ID:      sw.ID,
			Name:    sw.Name,
			Version: ver.Version,
			Message: fmt.Sprintf("%s is owned by Homebrew — use brew_install / brew_check_updates, or switch package manager in Softwares UI", sw.Name),
		}
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	if input.DryRun {
		return &mcp.CallToolResult{}, InstallOutput{
			Listed:        true,
			Success:       true,
			DryRun:        true,
			ID:            sw.ID,
			Name:          sw.Name,
			Version:       ver.Version,
			Message:       "dry_run: install script returned without executing",
			InstallScript: ver.InstallScript,
			CustomScript:  ver.CustomScript,
		}, nil
	}

	timeout := installDefaultTimeout
	if input.TimeoutSeconds > 0 {
		timeout = min(time.Duration(input.TimeoutSeconds)*time.Second, installMaxTimeout)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-lc", ver.InstallScript)
	cmd.Dir = "/root"
	cmd.Env = installEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)

	outStdout, truncOut := truncateBytes(stdout.Bytes(), installMaxOutput)
	outStderr, truncErr := truncateBytes(stderr.Bytes(), installMaxOutput)

	out := InstallOutput{
		Listed:     true,
		ID:         sw.ID,
		Name:       sw.Name,
		Version:    ver.Version,
		Stdout:     outStdout,
		Stderr:     outStderr,
		Truncated:  truncOut || truncErr,
		DurationMs: duration.Milliseconds(),
	}

	timedOut := runCtx.Err() == context.DeadlineExceeded
	out.TimedOut = timedOut
	if timedOut {
		out.ExitCode = 124
		out.Success = false
		out.Message = fmt.Sprintf("install %s %s timed out", sw.Name, ver.Version)
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			out.ExitCode = ee.ExitCode()
		} else {
			out.ExitCode = 1
		}
		out.Success = false
		out.Message = fmt.Sprintf("install %s %s failed: %v", sw.Name, ver.Version, runErr)
		return &mcp.CallToolResult{IsError: true}, out, nil
	}

	if err := models.MarkSoftwareInstalled(db, sw.ID, ver.ID); err != nil {
		out.Success = false
		out.Message = fmt.Sprintf("install succeeded but failed to update software_installed: %v", err)
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	softwaresync.ClearOsMissing(sw.ID, ver.ID)

	if cs := strings.TrimSpace(ver.CustomScript); cs != "" {
		var cStdout, cStderr bytes.Buffer
		ccmd := exec.CommandContext(runCtx, "bash", "-lc", cs)
		ccmd.Dir = "/root"
		ccmd.Env = installEnv()
		ccmd.Stdout = &cStdout
		ccmd.Stderr = &cStderr
		if err := ccmd.Run(); err != nil {
			cOut, _ := truncateBytes(cStdout.Bytes(), installMaxOutput)
			cErrOut, _ := truncateBytes(cStderr.Bytes(), installMaxOutput)
			out.Stdout = joinNonEmpty(out.Stdout, cOut)
			out.Stderr = joinNonEmpty(out.Stderr, cErrOut)
			out.Success = false
			if ee, ok := err.(*exec.ExitError); ok {
				out.ExitCode = ee.ExitCode()
			} else {
				out.ExitCode = 1
			}
			out.Message = fmt.Sprintf("installed %s %s but custom setup failed: %v", sw.Name, ver.Version, err)
			out.CustomScript = cs
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
		cOut, truncC := truncateBytes(cStdout.Bytes(), installMaxOutput)
		out.Stdout = joinNonEmpty(out.Stdout, cOut)
		out.Truncated = out.Truncated || truncC
	}

	out.Success = true
	out.ExitCode = 0
	out.Message = fmt.Sprintf("installed %s %s", sw.Name, ver.Version)
	return &mcp.CallToolResult{}, out, nil
}

func joinNonEmpty(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}
	return b.String()
}

func installEnv() []string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}
	env := os.Environ()
	overrides := map[string]string{
		"HOME":            home,
		"USER":            user,
		"DEBIAN_FRONTEND": "noninteractive",
		"GOCACHE":         home + "/.cache/go-build",
		"GOMODCACHE":      home + "/go/pkg/mod",
		"GOPATH":          home + "/go",
		"GOROOT":          "/usr/local/go",
	}
	seen := make(map[string]bool, len(overrides))
	out := make([]string, 0, len(env)+len(overrides))
	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if val, replace := overrides[key]; replace {
			out = append(out, key+"="+val)
			seen[key] = true
			continue
		}
		out = append(out, kv)
	}
	for key, val := range overrides {
		if !seen[key] {
			out = append(out, key+"="+val)
		}
	}
	return out
}

func truncateBytes(b []byte, limit int) (string, bool) {
	if len(b) <= limit {
		return string(b), false
	}
	msg := fmt.Sprintf("\n...[truncated %d of %d bytes]", limit, len(b))
	return string(b[:limit]) + msg, true
}
