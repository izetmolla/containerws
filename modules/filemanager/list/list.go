package list

import (
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
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

func (cc *controller) respond(c fiber.Ctx, status int, data any) error {
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(status), r.WithData(fiber.Map{"data": data}))
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := fsutil.MapFSError(err)
	return r.Api(c, r.WithError(fiber.NewError(code, msg)), r.WithStatus(code))
}

type listResult struct {
	Path       string         `json:"path"`
	Parent     string         `json:"parent,omitempty"`
	Exists     bool           `json:"exists"`
	IsDir      bool           `json:"is_dir"`
	Entries    []fsutil.Entry `json:"entries"`
	Total      int            `json:"total"`
	Truncated  bool           `json:"truncated,omitempty"`
	Roots      []fsutil.Root  `json:"roots"`
	User       *identity.Context `json:"user"`
	ShowHidden bool           `json:"show_hidden"`
}

// ListAPI GET /filemanager/list?path=&show_hidden=1&as_user=
func (cc *controller) ListAPI(c fiber.Ctx) error {
	ctx, err := identity.Resolve(c, cc.app)
	if err != nil {
		return cc.authErr(c, err)
	}

	raw := strings.TrimSpace(c.Query("path"))
	if raw == "" {
		raw = ctx.HomeDir
	}
	path, err := fsutil.ResolvePath(raw)
	if err != nil {
		return cc.respondErr(c, err)
	}

	showHidden := c.Query("show_hidden") == "1" || strings.EqualFold(c.Query("show_hidden"), "true")
	limit := 2000
	trashDir := fsutil.TrashRoot(ctx.HomeDir)

	var (
		entries   []fsutil.Entry
		exists    bool
		isDir     bool
		truncated bool
		listErr   error
	)

	runErr := ctx.Run(func() error {
		if path == trashDir || fsutil.IsUnderTrash(ctx.HomeDir, path) {
			_, _ = fsutil.PurgeExpiredTrash(ctx.HomeDir)
		}

		st, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Auto-create empty trash so the Places shortcut works.
				if path == trashDir {
					if mkErr := fsutil.EnsureTrashRoot(ctx.HomeDir); mkErr != nil {
						listErr = mkErr
						return mkErr
					}
					exists = true
					isDir = true
					entries = []fsutil.Entry{}
					return nil
				}
				exists = false
				return nil
			}
			listErr = err
			return err
		}
		exists = true
		isDir = st.IsDir()
		if !isDir {
			// Prefer lstat details for the leaf file/symlink itself.
			if lst, lerr := os.Lstat(path); lerr == nil {
				st = lst
			}
			e := fsutil.BuildEntry(filepath.Base(path), path, st)
			resolveOwnerNames(&e)
			entries = []fsutil.Entry{e}
			return nil
		}

		// Flat trash view: show deleted items by original name.
		if path == trashDir {
			entries = fsutil.ListTrashFlatEntries(ctx.HomeDir)
			return nil
		}

		dirents, err := os.ReadDir(path)
		if err != nil {
			listErr = err
			return err
		}
		entries = make([]fsutil.Entry, 0, len(dirents))
		for _, d := range dirents {
			name := d.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}
			if len(entries) >= limit {
				truncated = true
				break
			}
			info, err := d.Info()
			if err != nil {
				continue
			}
			full := filepath.Join(path, name)
			e := fsutil.BuildEntry(name, full, info)
			resolveOwnerNames(&e)
			entries = append(entries, e)
		}
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	if listErr != nil {
		return cc.respondErr(c, listErr)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Type == "directory" && entries[j].Type != "directory" {
			return true
		}
		if entries[i].Type != "directory" && entries[j].Type == "directory" {
			return false
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := ""
	if path != "/" {
		parent = filepath.Dir(path)
	}

	return cc.respond(c, fiber.StatusOK, listResult{
		Path:       path,
		Parent:     parent,
		Exists:     exists,
		IsDir:      isDir,
		Entries:    entries,
		Total:      len(entries),
		Truncated:  truncated,
		Roots:      buildRoots(ctx),
		User:       ctx,
		ShowHidden: showHidden,
	})
}

type statResult struct {
	Entry  *fsutil.Entry    `json:"entry,omitempty"`
	Exists bool             `json:"exists"`
	User   *identity.Context `json:"user"`
}

// StatAPI GET /filemanager/list/stat?path=
func (cc *controller) StatAPI(c fiber.Ctx) error {
	ctx, err := identity.Resolve(c, cc.app)
	if err != nil {
		return cc.authErr(c, err)
	}
	path, err := fsutil.ResolvePath(c.Query("path"))
	if err != nil {
		return cc.respondErr(c, err)
	}

	var entry *fsutil.Entry
	exists := false
	runErr := ctx.Run(func() error {
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		exists = true
		e := fsutil.BuildEntry(filepath.Base(path), path, info)
		resolveOwnerNames(&e)
		entry = &e
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, statResult{Entry: entry, Exists: exists, User: ctx})
}

// RootsAPI GET /filemanager/list/roots
func (cc *controller) RootsAPI(c fiber.Ctx) error {
	ctx, err := identity.Resolve(c, cc.app)
	if err != nil {
		return cc.authErr(c, err)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"roots": buildRoots(ctx),
		"user":  ctx,
	})
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

func buildRoots(ctx *identity.Context) []fsutil.Root {
	seen := map[string]struct{}{}
	out := make([]fsutil.Root, 0, 12)

	add := func(r fsutil.Root) {
		clean := filepath.Clean(r.Path)
		if clean == "" {
			return
		}
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		r.Path = clean
		if r.Group == "" {
			r.Group = "places"
		}
		out = append(out, r)
	}

	add(fsutil.Root{Path: ctx.HomeDir, Label: "Home", Icon: "Home", Group: "places"})
	if ctx.IsAdmin || ctx.IsRootLinux {
		add(fsutil.Root{Path: "/", Label: "Root", Icon: "HardDrive", Group: "places"})
		add(fsutil.Root{Path: "/home", Label: "Homes", Icon: "Users", Group: "places"})
	}

	for _, d := range fsutil.ListMountedDisks() {
		add(d)
	}

	add(fsutil.Root{
		Path:  fsutil.TrashRoot(ctx.HomeDir),
		Label: "Trash",
		Icon:  "Trash2",
		Group: "trash",
	})
	return out
}

func resolveOwnerNames(e *fsutil.Entry) {
	if e == nil {
		return
	}
	if u, err := user.LookupId(strconv.FormatUint(uint64(e.UID), 10)); err == nil {
		e.Owner = u.Username
	} else if e.Owner == "" {
		e.Owner = strconv.FormatUint(uint64(e.UID), 10)
	}
	if g, err := user.LookupGroupId(strconv.FormatUint(uint64(e.GID), 10)); err == nil {
		e.Group = g.Name
	} else if e.Group == "" {
		e.Group = strconv.FormatUint(uint64(e.GID), 10)
	}
}
