package ops

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/modules/filemanager/fsutil"
)

type pathsBody struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination,omitempty"`
}

type unzipBody struct {
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
}

func uniqueNonEmptyPaths(raw []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		resolved, err := fsutil.ResolvePath(p)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		out = append(out, resolved)
	}
	if len(out) == 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "paths required")
	}
	return out, nil
}

func defaultZipName(paths []string) string {
	if len(paths) == 1 {
		base := filepath.Base(paths[0])
		if base == "" || base == "." || base == "/" {
			base = "archive"
		}
		return base + ".zip"
	}
	return fmt.Sprintf("download-%s.zip", time.Now().Format("20060102-150405"))
}

func zipDestinationPath(paths []string, destination string) (string, error) {
	destination = strings.TrimSpace(destination)
	if destination != "" {
		return fsutil.ResolvePath(destination)
	}
	parent := filepath.Dir(paths[0])
	name := defaultZipName(paths)
	return filepath.Join(parent, name), nil
}

func writeZipArchive(w io.Writer, paths []string) error {
	zw := zip.NewWriter(w)

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
			_, err = zw.CreateHeader(header)
			return err
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(absPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	}

	for _, root := range paths {
		info, err := os.Lstat(root)
		if err != nil {
			_ = zw.Close()
			return err
		}
		baseName := filepath.Base(root)
		if !info.IsDir() {
			if err := addFile(root, baseName, info); err != nil {
				_ = zw.Close()
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
			nameInZip := baseName
			if rel != "." {
				nameInZip = filepath.Join(baseName, rel)
			}
			return addFile(path, nameInZip, fi)
		})
		if err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func safeUnzipPath(destDir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "invalid archive entry")
	}
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", fiber.NewError(fiber.StatusBadRequest, "archive entry escapes destination")
	}
	target := filepath.Join(destDir, clean)
	rel, err := filepath.Rel(destDir, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fiber.NewError(fiber.StatusBadRequest, "archive entry escapes destination")
	}
	return target, nil
}

func extractZip(zipPath, destDir string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, err
	}

	count := 0
	for _, f := range r.File {
		target, err := safeUnzipPath(destDir, f.Name)
		if err != nil {
			return count, err
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
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
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return count, err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return count, copyErr
		}
		if closeErr != nil {
			return count, closeErr
		}
		count++
	}
	return count, nil
}

// ZipAPI POST /filemanager/ops/zip — create a .zip on disk from paths.
func (cc *controller) ZipAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathsBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fiber.NewError(fiber.StatusBadRequest, "invalid body"))
	}
	paths, err := uniqueNonEmptyPaths(body.Paths)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dest, err := zipDestinationPath(paths, body.Destination)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if !strings.HasSuffix(strings.ToLower(dest), ".zip") {
		dest += ".zip"
	}

	var created string
	runErr := ctx.Run(func() error {
		for _, p := range paths {
			if _, err := os.Lstat(p); err != nil {
				return err
			}
		}
		if err := fsutil.EnsureParentDir(dest); err != nil {
			return err
		}
		if _, err := os.Lstat(dest); err == nil {
			return fiber.NewError(fiber.StatusConflict, "destination already exists")
		}
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := writeZipArchive(f, paths); err != nil {
			_ = os.Remove(dest)
			return err
		}
		created = dest
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":  created,
		"name":  filepath.Base(created),
		"count": len(paths),
	}, "Archive created")
}

// UnzipAPI POST /filemanager/ops/unzip — extract a zip archive.
func (cc *controller) UnzipAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body unzipBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fiber.NewError(fiber.StatusBadRequest, "invalid body"))
	}
	zipPath, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dest := strings.TrimSpace(body.Destination)
	if dest == "" {
		base := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
		if base == "" {
			base = "extracted"
		}
		dest = filepath.Join(filepath.Dir(zipPath), base)
	}
	destPath, err := fsutil.ResolvePath(dest)
	if err != nil {
		return cc.respondErr(c, err)
	}

	var extracted int
	runErr := ctx.Run(func() error {
		st, err := os.Lstat(zipPath)
		if err != nil {
			return err
		}
		if st.IsDir() {
			return fiber.NewError(fiber.StatusBadRequest, "path is a directory")
		}
		if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
			return fiber.NewError(fiber.StatusBadRequest, "only .zip archives are supported")
		}
		n, err := extractZip(zipPath, destPath)
		extracted = n
		return err
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":      destPath,
		"extracted": extracted,
		"source":    zipPath,
	}, "Archive extracted")
}

// DownloadArchiveAPI POST /filemanager/ops/download-archive — stream a zip of paths.
func (cc *controller) DownloadArchiveAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathsBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fiber.NewError(fiber.StatusBadRequest, "invalid body"))
	}
	paths, err := uniqueNonEmptyPaths(body.Paths)
	if err != nil {
		return cc.respondErr(c, err)
	}

	filename := defaultZipName(paths)
	var tmp *os.File
	runErr := ctx.Run(func() error {
		for _, p := range paths {
			if _, err := os.Lstat(p); err != nil {
				return err
			}
		}
		f, err := os.CreateTemp("", "cws-dl-*.zip")
		if err != nil {
			return err
		}
		if err := writeZipArchive(f, paths); err != nil {
			name := f.Name()
			_ = f.Close()
			_ = os.Remove(name)
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			name := f.Name()
			_ = f.Close()
			_ = os.Remove(name)
			return err
		}
		tmp = f
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	defer func() {
		name := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(name)
	}()

	info, _ := tmp.Stat()
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set("Content-Type", "application/zip")
	if info != nil {
		c.Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}
	return c.SendStream(tmp)
}
