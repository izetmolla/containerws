package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxReadBytes = 1024 * 1024

type ReadFileInput struct {
	Path   string `json:"path" jsonschema:"required file path to read"`
	Offset int    `json:"offset,omitempty" jsonschema:"optional 1-based line offset to start reading from"`
	Limit  int    `json:"limit,omitempty" jsonschema:"optional max number of lines to return (0 = all from offset)"`
}

type ReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
	LinesFrom int    `json:"lines_from,omitempty"`
	LinesTo   int    `json:"lines_to,omitempty"`
}

func (c *Controller) ReadFileTool(ctx context.Context, _ *mcp.CallToolRequest, input ReadFileInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("path is a directory: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	content := string(data)
	out := ReadFileOutput{
		Path: path,
		Size: info.Size(),
	}

	if input.Offset > 0 || input.Limit > 0 {
		lines := strings.Split(content, "\n")
		start := 0
		if input.Offset > 0 {
			start = input.Offset - 1
		}
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if input.Limit > 0 && start+input.Limit < end {
			end = start + input.Limit
		}
		content = strings.Join(lines[start:end], "\n")
		out.LinesFrom = start + 1
		out.LinesTo = end
	}

	if len(content) > maxReadBytes {
		content = content[:maxReadBytes] + fmt.Sprintf("\n...[truncated to %d bytes]", maxReadBytes)
		out.Truncated = true
	}
	out.Content = content
	return nil, out, nil
}

type WriteFileInput struct {
	Path     string `json:"path" jsonschema:"required file path to write"`
	Content  string `json:"content" jsonschema:"file contents to write"`
	Append   bool   `json:"append,omitempty" jsonschema:"when true, append instead of overwrite"`
	Mode     string `json:"mode,omitempty" jsonschema:"optional octal file mode like 0644 (default 0644)"`
}

type WriteFileOutput struct {
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Created bool   `json:"created"`
	Append  bool   `json:"append,omitempty"`
}

func (c *Controller) WriteFileTool(ctx context.Context, _ *mcp.CallToolRequest, input WriteFileInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureParentDir(path); err != nil {
		return nil, nil, err
	}

	_, statErr := os.Stat(path)
	created := os.IsNotExist(statErr)

	mode := os.FileMode(0o644)
	if m := strings.TrimSpace(input.Mode); m != "" {
		var parsed uint32
		if _, err := fmt.Sscanf(m, "%o", &parsed); err != nil {
			return nil, nil, fmt.Errorf("invalid mode %q: use octal like 0644", m)
		}
		mode = os.FileMode(parsed)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if input.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(path, flag, mode)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	n, err := f.WriteString(input.Content)
	if err != nil {
		return nil, nil, err
	}

	return nil, WriteFileOutput{
		Path:    path,
		Bytes:   n,
		Created: created,
		Append:  input.Append,
	}, nil
}

type EditFileInput struct {
	Path       string `json:"path" jsonschema:"required file path to edit"`
	OldString  string `json:"old_string" jsonschema:"exact text to find"`
	NewString  string `json:"new_string" jsonschema:"replacement text"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"when true, replace every occurrence"`
}

type EditFileOutput struct {
	Path       string `json:"path"`
	Replacements int  `json:"replacements"`
}

func (c *Controller) EditFileTool(ctx context.Context, _ *mcp.CallToolRequest, input EditFileInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	if input.OldString == "" {
		return nil, nil, fmt.Errorf("old_string is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	content := string(data)
	count := strings.Count(content, input.OldString)
	if count == 0 {
		return nil, nil, fmt.Errorf("old_string not found in %s", path)
	}
	if !input.ReplaceAll && count > 1 {
		return nil, nil, fmt.Errorf("old_string found %d times; set replace_all=true or provide a more unique string", count)
	}

	var updated string
	replacements := 1
	if input.ReplaceAll {
		updated = strings.ReplaceAll(content, input.OldString, input.NewString)
		replacements = count
	} else {
		updated = strings.Replace(content, input.OldString, input.NewString, 1)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return nil, nil, err
	}

	return nil, EditFileOutput{Path: path, Replacements: replacements}, nil
}
