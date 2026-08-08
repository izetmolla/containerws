package filesystem

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListDirectoryInput struct {
	Path      string `json:"path" jsonschema:"directory path to list (default .)"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"when true, walk nested directories"`
	MaxDepth  int    `json:"max_depth,omitempty" jsonschema:"optional max recursion depth (0 = unlimited when recursive)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"optional max entries to return (default 500)"`
}

type DirEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Size    int64  `json:"size,omitempty"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

type ListDirectoryOutput struct {
	Path      string     `json:"path"`
	Entries   []DirEntry `json:"entries"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated,omitempty"`
}

func (c *Controller) ListDirectoryTool(ctx context.Context, _ *mcp.CallToolRequest, input ListDirectoryInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "."
	}
	abs, err := resolvePath(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("path is not a directory: %s", abs)
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 500
	}

	out := ListDirectoryOutput{Path: abs, Entries: []DirEntry{}}

	if !input.Recursive {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range entries {
			if len(out.Entries) >= limit {
				out.Truncated = true
				break
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			out.Entries = append(out.Entries, DirEntry{
				Name:    e.Name(),
				Path:    filepath.Join(abs, e.Name()),
				Type:    entryType(fi.Mode()),
				Size:    fi.Size(),
				Mode:    fileModeString(fi.Mode()),
				ModTime: fi.ModTime().UTC().Format(time.RFC3339),
			})
		}
		out.Total = len(out.Entries)
		if out.Truncated {
			out.Total = len(entries)
		}
		return nil, out, nil
	}

	maxDepth := input.MaxDepth
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if p == abs {
			return nil
		}
		if maxDepth > 0 {
			rel, _ := filepath.Rel(abs, p)
			depth := strings.Count(rel, string(os.PathSeparator)) + 1
			if depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if len(out.Entries) >= limit {
			out.Truncated = true
			return fs.SkipAll
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		out.Entries = append(out.Entries, DirEntry{
			Name:    d.Name(),
			Path:    p,
			Type:    entryType(fi.Mode()),
			Size:    fi.Size(),
			Mode:    fileModeString(fi.Mode()),
			ModTime: fi.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	out.Total = len(out.Entries)
	return nil, out, nil
}

type MakeDirectoryInput struct {
	Path string `json:"path" jsonschema:"required directory path to create"`
	Mode string `json:"mode,omitempty" jsonschema:"optional octal mode like 0755 (default 0755)"`
}

type MakeDirectoryOutput struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

func (c *Controller) MakeDirectoryTool(ctx context.Context, _ *mcp.CallToolRequest, input MakeDirectoryInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	mode := os.FileMode(0o755)
	if m := strings.TrimSpace(input.Mode); m != "" {
		var parsed uint32
		if _, err := fmt.Sscanf(m, "%o", &parsed); err != nil {
			return nil, nil, fmt.Errorf("invalid mode %q: use octal like 0755", m)
		}
		mode = os.FileMode(parsed)
	}

	_, err = os.Stat(path)
	created := os.IsNotExist(err)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return nil, nil, err
	}
	return nil, MakeDirectoryOutput{Path: path, Created: created}, nil
}

type DeletePathInput struct {
	Path      string `json:"path" jsonschema:"required path to delete"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"required true to delete non-empty directories"`
}

type DeletePathOutput struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

func (c *Controller) DeletePathTool(ctx context.Context, _ *mcp.CallToolRequest, input DeletePathInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, DeletePathOutput{Path: path, Deleted: false}, nil
		}
		return nil, nil, err
	}
	if info.IsDir() && !input.Recursive {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil, nil, readErr
		}
		if len(entries) > 0 {
			return nil, nil, fmt.Errorf("directory not empty; set recursive=true to delete %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, nil, err
		}
		return nil, DeletePathOutput{Path: path, Deleted: true}, nil
	}
	if input.Recursive {
		if err := os.RemoveAll(path); err != nil {
			return nil, nil, err
		}
	} else {
		if err := os.Remove(path); err != nil {
			return nil, nil, err
		}
	}
	return nil, DeletePathOutput{Path: path, Deleted: true}, nil
}

type MovePathInput struct {
	Source      string `json:"source" jsonschema:"required source path"`
	Destination string `json:"destination" jsonschema:"required destination path"`
}

type MovePathOutput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func (c *Controller) MovePathTool(ctx context.Context, _ *mcp.CallToolRequest, input MovePathInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	src, err := resolvePath(input.Source)
	if err != nil {
		return nil, nil, err
	}
	dst, err := resolvePath(input.Destination)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureParentDir(dst); err != nil {
		return nil, nil, err
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, nil, err
	}
	return nil, MovePathOutput{Source: src, Destination: dst}, nil
}

type CopyPathInput struct {
	Source      string `json:"source" jsonschema:"required source path"`
	Destination string `json:"destination" jsonschema:"required destination path"`
	Recursive   bool   `json:"recursive,omitempty" jsonschema:"required true when copying directories"`
}

type CopyPathOutput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Bytes       int64  `json:"bytes,omitempty"`
}

func (c *Controller) CopyPathTool(ctx context.Context, _ *mcp.CallToolRequest, input CopyPathInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	src, err := resolvePath(input.Source)
	if err != nil {
		return nil, nil, err
	}
	dst, err := resolvePath(input.Destination)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(src)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		if !input.Recursive {
			return nil, nil, fmt.Errorf("source is a directory; set recursive=true")
		}
		n, err := copyDir(src, dst)
		if err != nil {
			return nil, nil, err
		}
		return nil, CopyPathOutput{Source: src, Destination: dst, Bytes: n}, nil
	}
	n, err := copyFile(src, dst, info.Mode())
	if err != nil {
		return nil, nil, err
	}
	return nil, CopyPathOutput{Source: src, Destination: dst, Bytes: n}, nil
}

func copyFile(src, dst string, mode os.FileMode) (int64, error) {
	if err := ensureParentDir(dst); err != nil {
		return 0, err
	}
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, in)
}

func copyDir(src, dst string) (int64, error) {
	var total int64
	return total, filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		n, err := copyFile(p, target, info.Mode())
		total += n
		return err
	})
}

type StatPathInput struct {
	Path string `json:"path" jsonschema:"required path to inspect"`
}

type StatPathOutput struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	Exists  bool   `json:"exists"`
}

func (c *Controller) StatPathTool(ctx context.Context, _ *mcp.CallToolRequest, input StatPathInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	path, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, StatPathOutput{Path: path, Exists: false}, nil
		}
		return nil, nil, err
	}
	return nil, StatPathOutput{
		Path:    path,
		Type:    entryType(info.Mode()),
		Size:    info.Size(),
		Mode:    fileModeString(info.Mode()),
		ModTime: info.ModTime().UTC().Format(time.RFC3339),
		Exists:  true,
	}, nil
}
