package bash

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTimeout = 120 * time.Second
	maxTimeout     = 30 * time.Minute
	maxOutputBytes = 512 * 1024
)

type BashInput struct {
	Command        string            `json:"command" jsonschema:"required bash/shell command to execute"`
	Cwd            string            `json:"cwd,omitempty" jsonschema:"optional working directory (absolute or relative)"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty" jsonschema:"optional timeout in seconds (default 120, max 1800)"`
	Env            map[string]string `json:"env,omitempty" jsonschema:"optional extra environment variables (merged onto process env)"`
}

type BashOutput struct {
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Truncated  bool   `json:"truncated,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out,omitempty"`
}

func (c *Controller) BashTool(ctx context.Context, _ *mcp.CallToolRequest, input BashInput) (*mcp.CallToolResult, any, error) {
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return nil, nil, fmt.Errorf("command is required")
	}

	timeout := defaultTimeout
	if input.TimeoutSeconds > 0 {
		timeout = min(time.Duration(input.TimeoutSeconds)*time.Second, maxTimeout)
	}

	cwd := strings.TrimSpace(input.Cwd)
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	} else {
		if st, err := os.Stat(cwd); err != nil {
			return nil, nil, fmt.Errorf("cwd: %w", err)
		} else if !st.IsDir() {
			return nil, nil, fmt.Errorf("cwd is not a directory: %s", cwd)
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "/bin/bash", "-lc", command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for k, v := range input.Env {
		if strings.TrimSpace(k) == "" {
			continue
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	err := cmd.Run()
	duration := time.Since(started)

	out := BashOutput{
		Command:    command,
		Cwd:        cwd,
		DurationMs: duration.Milliseconds(),
	}

	outStdout, truncOut := truncateBytes(stdout.Bytes(), maxOutputBytes)
	outStderr, truncErr := truncateBytes(stderr.Bytes(), maxOutputBytes)
	out.Stdout = outStdout
	out.Stderr = outStderr
	out.Truncated = truncOut || truncErr

	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	out.TimedOut = timedOut

	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		switch {
		case timedOut:
			exitCode = 124
		case errors.As(err, &ee):
			exitCode = ee.ExitCode()
		default:
			return nil, nil, fmt.Errorf("failed to run command: %w", err)
		}
	}
	out.ExitCode = exitCode

	result := &mcp.CallToolResult{}
	if exitCode != 0 || timedOut {
		result.IsError = true
	}
	return result, out, nil
}

func truncateBytes(b []byte, limit int) (string, bool) {
	if len(b) <= limit {
		return string(b), false
	}
	msg := fmt.Sprintf("\n...[truncated %d of %d bytes]", limit, len(b))
	return string(b[:limit]) + msg, true
}
