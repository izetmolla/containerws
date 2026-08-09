package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/filemanager/fsutil"
	"github.com/izetmolla/containerws/modules/filemanager/identity"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Post("/mkdir", cc.MkdirAPI)
	api.Post("/create", cc.CreateFileAPI)
	api.Post("/rename", cc.RenameAPI)
	api.Post("/move", cc.MoveAPI)
	api.Post("/copy", cc.CopyAPI)
	api.Post("/duplicate", cc.DuplicateAPI)
	api.Post("/delete", cc.DeleteAPI)
	api.Post("/chmod", cc.ChmodAPI)
	api.Post("/upload", cc.UploadAPI)
	api.Get("/download", cc.DownloadAPI)
	api.Post("/download-archive", cc.DownloadArchiveAPI)
	api.Post("/zip", cc.ZipAPI)
	api.Post("/unzip", cc.UnzipAPI)
	api.Get("/read", cc.ReadAPI)
	api.Post("/write", cc.WriteAPI)

	api.Post("/trash", cc.TrashAPI)
	api.Get("/trash", cc.ListTrashAPI)
	api.Post("/trash/restore", cc.RestoreTrashAPI)
	api.Post("/trash/delete", cc.DeleteTrashAPI)
	api.Post("/trash/empty", cc.EmptyTrashAPI)
}

func (cc *controller) respond(c fiber.Ctx, status int, data any, message string) error {
	r := cc.app.Render()
	payload := fiber.Map{"data": data}
	if message != "" {
		payload["message"] = message
	}
	return r.Api(c, r.WithStatus(status), r.WithData(payload))
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := fsutil.MapFSError(err)
	return r.Api(c, r.WithError(fiber.NewError(code, msg)), r.WithStatus(code))
}

func (cc *controller) authErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	if err == fiber.ErrUnauthorized {
		return r.Api(c, r.WithError(fiber.NewError(fiber.StatusUnauthorized, "unauthorized")), r.WithStatus(fiber.StatusUnauthorized))
	}
	if e, ok := err.(*fiber.Error); ok {
		return r.Api(c, r.WithError(e), r.WithStatus(e.Code))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}

func (cc *controller) resolve(c fiber.Ctx) (*identity.Context, error) {
	return identity.Resolve(c, cc.app)
}

type pathBody struct {
	Path     string `json:"path"`
	Mode     string `json:"mode,omitempty"`
	Content  string `json:"content,omitempty"`
	Recursive bool  `json:"recursive,omitempty"`
}

type twoPathBody struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Recursive   bool   `json:"recursive,omitempty"`
}

type chmodBody struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type writeBody struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// MkdirAPI POST /filemanager/ops/mkdir
func (cc *controller) MkdirAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	path, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	mode, err := fsutil.ParseMode(body.Mode, 0o755)
	if err != nil {
		return cc.respondErr(c, err)
	}
	created := false
	runErr := ctx.Run(func() error {
		_, err := os.Lstat(path)
		created = os.IsNotExist(err)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.MkdirAll(path, mode)
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"path": path, "created": created}, "Folder created")
}

// CreateFileAPI POST /filemanager/ops/create
func (cc *controller) CreateFileAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	path, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	mode, err := fsutil.ParseMode(body.Mode, 0o644)
	if err != nil {
		return cc.respondErr(c, err)
	}
	created := false
	runErr := ctx.Run(func() error {
		if _, err := os.Lstat(path); err == nil {
			return fiber.NewError(fiber.StatusConflict, "file already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := fsutil.EnsureParentDir(path); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		defer f.Close()
		if body.Content != "" {
			if _, err := f.WriteString(body.Content); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"path": path, "created": created}, "File created")
}

// RenameAPI POST /filemanager/ops/rename
func (cc *controller) RenameAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body twoPathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	src, err := fsutil.ResolvePath(body.Source)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dstName := strings.TrimSpace(body.Destination)
	if dstName == "" {
		return cc.respondErr(c, fmt.Errorf("destination is required"))
	}
	var dst string
	if strings.Contains(dstName, "/") {
		dst, err = fsutil.ResolvePath(dstName)
		if err != nil {
			return cc.respondErr(c, err)
		}
	} else {
		dst = filepath.Join(filepath.Dir(src), filepath.Base(dstName))
	}
	runErr := ctx.Run(func() error {
		if _, err := os.Lstat(dst); err == nil {
			return fiber.NewError(fiber.StatusConflict, "destination already exists")
		}
		return os.Rename(src, dst)
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"source": src, "destination": dst}, "Renamed")
}

// MoveAPI POST /filemanager/ops/move
func (cc *controller) MoveAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body twoPathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	src, err := fsutil.ResolvePath(body.Source)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dst, err := fsutil.ResolvePath(body.Destination)
	if err != nil {
		return cc.respondErr(c, err)
	}
	finalDst := dst
	runErr := ctx.Run(func() error {
		d := dst
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			d = filepath.Join(d, filepath.Base(src))
		} else if os.IsNotExist(err) {
			// Destination folder missing — create silently, then place source inside.
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			d = filepath.Join(dst, filepath.Base(src))
		} else if err != nil {
			return err
		}
		if err := fsutil.EnsureParentDir(d); err != nil {
			return err
		}
		if err := os.Rename(src, d); err != nil {
			info, lerr := os.Lstat(src)
			if lerr != nil {
				return err
			}
			if info.IsDir() {
				if _, cerr := fsutil.CopyDir(src, d); cerr != nil {
					return cerr
				}
			} else {
				if _, cerr := fsutil.CopyFile(src, d, info.Mode()); cerr != nil {
					return cerr
				}
			}
			if err := os.RemoveAll(src); err != nil {
				return err
			}
		}
		finalDst = d
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"source": src, "destination": finalDst}, "Moved")
}

// CopyAPI POST /filemanager/ops/copy
func (cc *controller) CopyAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body twoPathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	src, err := fsutil.ResolvePath(body.Source)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dst, err := fsutil.ResolvePath(body.Destination)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var bytesCopied int64
	finalDst := dst
	runErr := ctx.Run(func() error {
		info, err := os.Lstat(src)
		if err != nil {
			return err
		}
		d := dst
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			d = filepath.Join(d, filepath.Base(src))
		} else if os.IsNotExist(err) {
			// Destination folder missing — create silently, then place source inside.
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			d = filepath.Join(dst, filepath.Base(src))
		} else if err != nil {
			return err
		}
		if info.IsDir() {
			n, err := fsutil.CopyDir(src, d)
			bytesCopied = n
			finalDst = d
			return err
		}
		n, err := fsutil.CopyFile(src, d, info.Mode())
		bytesCopied = n
		finalDst = d
		return err
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"source":      src,
		"destination": finalDst,
		"bytes":       bytesCopied,
	}, "Copied")
}

// DuplicateAPI POST /filemanager/ops/duplicate
func (cc *controller) DuplicateAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	src, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var dst string
	var bytesCopied int64
	runErr := ctx.Run(func() error {
		info, err := os.Lstat(src)
		if err != nil {
			return err
		}
		dst, err = fsutil.UniqueCopyPath(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			n, err := fsutil.CopyDir(src, dst)
			bytesCopied = n
			return err
		}
		n, err := fsutil.CopyFile(src, dst, info.Mode())
		bytesCopied = n
		return err
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"source":      src,
		"destination": dst,
		"bytes":       bytesCopied,
	}, "Duplicated")
}

// DeleteAPI POST /filemanager/ops/delete
func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body pathBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	path, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if path == "/" {
		return cc.respondErr(c, fiber.NewError(fiber.StatusBadRequest, "refusing to delete filesystem root"))
	}
	runErr := ctx.Run(func() error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if !body.Recursive {
				entries, err := os.ReadDir(path)
				if err != nil {
					return err
				}
				if len(entries) > 0 {
					return fiber.NewError(fiber.StatusBadRequest, "directory not empty; set recursive=true")
				}
				return os.Remove(path)
			}
			return os.RemoveAll(path)
		}
		return os.Remove(path)
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"path": path, "deleted": true}, "Deleted")
}

// ChmodAPI POST /filemanager/ops/chmod
func (cc *controller) ChmodAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body chmodBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	path, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	mode, err := fsutil.ParseMode(body.Mode, 0)
	if err != nil {
		return cc.respondErr(c, err)
	}
	runErr := ctx.Run(func() error {
		return os.Chmod(path, mode)
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":       path,
		"mode":       mode.String(),
		"mode_octal": fsutil.ModeOctal(mode),
	}, "Permissions updated")
}

// UploadAPI POST /filemanager/ops/upload (multipart: file + body JSON {path: dir})
func (cc *controller) UploadAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return cc.respondErr(c, fmt.Errorf("file is required"))
	}
	destDir := strings.TrimSpace(c.FormValue("path"))
	if destDir == "" {
		if raw := strings.TrimSpace(c.FormValue("body")); raw != "" {
			var b pathBody
			if err := json.Unmarshal([]byte(raw), &b); err == nil {
				destDir = strings.TrimSpace(b.Path)
			}
		}
	}
	if destDir == "" {
		destDir = ctx.HomeDir
	}
	dir, err := fsutil.ResolvePath(destDir)
	if err != nil {
		return cc.respondErr(c, err)
	}
	name := filepath.Base(fh.Filename)
	if name == "" || name == "." || name == ".." {
		return cc.respondErr(c, fmt.Errorf("invalid filename"))
	}
	dest := filepath.Join(dir, name)

	src, err := fh.Open()
	if err != nil {
		return cc.respondErr(c, err)
	}
	defer src.Close()

	var written int64
	runErr := ctx.Run(func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		n, err := io.Copy(out, src)
		written = n
		return err
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":  dest,
		"bytes": written,
		"name":  name,
	}, "Uploaded")
}

// DownloadAPI GET /filemanager/ops/download?path=
func (cc *controller) DownloadAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	path, err := fsutil.ResolvePath(c.Query("path"))
	if err != nil {
		return cc.respondErr(c, err)
	}
	var f *os.File
	var info os.FileInfo
	runErr := ctx.Run(func() error {
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if st.IsDir() {
			return fiber.NewError(fiber.StatusBadRequest, "cannot download a directory")
		}
		info = st
		opened, err := os.Open(path)
		if err != nil {
			return err
		}
		f = opened
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	defer f.Close()

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	return c.SendStream(f)
}

const maxReadBytes = 2 * 1024 * 1024

// ReadAPI GET /filemanager/ops/read?path= — text preview (capped).
func (cc *controller) ReadAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	path, err := fsutil.ResolvePath(c.Query("path"))
	if err != nil {
		return cc.respondErr(c, err)
	}
	var (
		content   string
		size      int64
		truncated bool
		mode      string
	)
	runErr := ctx.Run(func() error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fiber.NewError(fiber.StatusBadRequest, "path is a directory")
		}
		size = info.Size()
		mode = info.Mode().String()
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > maxReadBytes {
			data = data[:maxReadBytes]
			truncated = true
		}
		content = string(data)
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":      path,
		"content":   content,
		"size":      size,
		"truncated": truncated,
		"mode":      mode,
	}, "")
}

// WriteAPI POST /filemanager/ops/write
func (cc *controller) WriteAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body writeBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	path, err := fsutil.ResolvePath(body.Path)
	if err != nil {
		return cc.respondErr(c, err)
	}
	mode, err := fsutil.ParseMode(body.Mode, 0o644)
	if err != nil {
		return cc.respondErr(c, err)
	}
	var bytesWritten int
	runErr := ctx.Run(func() error {
		if err := fsutil.EnsureParentDir(path); err != nil {
			return err
		}
		flag := os.O_CREATE | os.O_WRONLY
		if body.Append {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		// Preserve existing mode when overwriting without explicit mode.
		if body.Mode == "" {
			if st, err := os.Lstat(path); err == nil {
				mode = st.Mode().Perm()
			}
		}
		f, err := os.OpenFile(path, flag, mode)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := f.WriteString(body.Content)
		bytesWritten = n
		return err
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":  path,
		"bytes": bytesWritten,
	}, "Saved")
}
