package filesystem

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ZipPathsInput struct {
	Paths       []string `json:"paths" jsonschema:"required one or more files/directories to archive"`
	Destination string   `json:"destination,omitempty" jsonschema:"optional .zip path (default sibling of first path)"`
}

type ZipPathsOutput struct {
	Path    string `json:"path"`
	Count   int    `json:"count"`
	Message string `json:"message"`
}

func (c *Controller) ZipPathsTool(ctx context.Context, _ *mcp.CallToolRequest, input ZipPathsInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	if len(input.Paths) == 0 {
		return nil, nil, fmt.Errorf("paths is required")
	}
	resolved := make([]string, 0, len(input.Paths))
	seen := map[string]struct{}{}
	for _, p := range input.Paths {
		abs, err := resolvePath(p)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		if _, err := os.Lstat(abs); err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, abs)
	}
	if len(resolved) == 0 {
		return nil, nil, fmt.Errorf("paths is required")
	}

	dest := strings.TrimSpace(input.Destination)
	var absDest string
	var err error
	if dest != "" {
		absDest, err = resolvePath(dest)
		if err != nil {
			return nil, nil, err
		}
	} else {
		base := filepath.Base(resolved[0])
		if base == "" || base == "." || base == "/" {
			base = "archive"
		}
		if len(resolved) == 1 {
			absDest = filepath.Join(filepath.Dir(resolved[0]), base+".zip")
		} else {
			absDest = filepath.Join(filepath.Dir(resolved[0]), fmt.Sprintf("archive-%s.zip", time.Now().Format("20060102-150405")))
		}
	}
	if !strings.HasSuffix(strings.ToLower(absDest), ".zip") {
		absDest += ".zip"
	}
	if _, err := os.Lstat(absDest); err == nil {
		return &mcp.CallToolResult{IsError: true}, ZipPathsOutput{Message: "destination already exists: " + absDest}, nil
	}
	if err := ensureParentDir(absDest); err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(absDest, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	if err := writeZipArchive(f, resolved); err != nil {
		_ = os.Remove(absDest)
		return nil, nil, err
	}
	return &mcp.CallToolResult{}, ZipPathsOutput{
		Path:    absDest,
		Count:   len(resolved),
		Message: "Archive created",
	}, nil
}

type UnzipPathInput struct {
	Path        string `json:"path" jsonschema:"required path to a .zip archive"`
	Destination string `json:"destination,omitempty" jsonschema:"optional extract directory (default sibling folder named after the zip)"`
}

type UnzipPathOutput struct {
	Path      string `json:"path"`
	Source    string `json:"source"`
	Extracted int    `json:"extracted"`
	Message   string `json:"message"`
}

func (c *Controller) UnzipPathTool(ctx context.Context, _ *mcp.CallToolRequest, input UnzipPathInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	zipPath, err := resolvePath(input.Path)
	if err != nil {
		return nil, nil, err
	}
	st, err := os.Lstat(zipPath)
	if err != nil {
		return nil, nil, err
	}
	if st.IsDir() {
		return nil, nil, fmt.Errorf("path is a directory")
	}
	if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
		return nil, nil, fmt.Errorf("only .zip archives are supported")
	}

	dest := strings.TrimSpace(input.Destination)
	if dest == "" {
		base := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
		if base == "" {
			base = "extracted"
		}
		dest = filepath.Join(filepath.Dir(zipPath), base)
	}
	destPath, err := resolvePath(dest)
	if err != nil {
		return nil, nil, err
	}

	n, err := extractZip(zipPath, destPath)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{}, UnzipPathOutput{
		Path:      destPath,
		Source:    zipPath,
		Extracted: n,
		Message:   "Archive extracted",
	}, nil
}

func writeZipArchive(w io.Writer, paths []string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	addFile := func(absPath, nameInZip string, info os.FileInfo) error {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(nameInZip)
		header.Method = zip.Deflate
		if info.IsDir() {
			if !strings.HasSuffix(header.Name, "/") {
				header.Name += "/"
			}
			_, err := zw.CreateHeader(header)
			return err
		}
		hw, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(absPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(hw, f)
		return err
	}

	for _, root := range paths {
		info, err := os.Lstat(root)
		if err != nil {
			return err
		}
		base := filepath.Base(root)
		if !info.IsDir() {
			if err := addFile(root, base, info); err != nil {
				return err
			}
			continue
		}
		err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := base
			if rel != "." {
				name = filepath.Join(base, rel)
			}
			if fi.IsDir() {
				return addFile(path, name, fi)
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			return addFile(path, name, fi)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func extractZip(zipPath, destPath string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return 0, err
	}

	count := 0
	destAbs, err := filepath.Abs(destPath)
	if err != nil {
		return 0, err
	}
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(destAbs, name)
		if !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) && target != destAbs {
			return count, fmt.Errorf("refusing to extract outside destination: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return count, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, err
		}
		rc, err := f.Open()
		if err != nil {
			return count, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm())
		if err != nil {
			_ = rc.Close()
			return count, err
		}
		_, copyErr := io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return count, copyErr
		}
		count++
	}
	return count, nil
}
