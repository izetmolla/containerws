package list

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxuser"
)

type pathRoot struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

type pathEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasChildren bool   `json:"has_children"`
}

type pathBrowseResult struct {
	Path       string      `json:"path"`
	Parent     string      `json:"parent,omitempty"`
	Exists     bool        `json:"exists"`
	IsDir      bool        `json:"is_dir"`
	WillCreate bool        `json:"will_create"`
	Prefix     string      `json:"prefix,omitempty"`
	Entries    []pathEntry `json:"entries"`
	Roots      []pathRoot  `json:"roots"`
	Admin      bool        `json:"admin"`
}

// GetCodeserverPathsAPI lists directories under a typed path for the folder picker.
// Query:
//   - path: current input (default /workspace)
//   - user_id: optional session target user (home root from their Linux account)
func (cc *controller) GetCodeserverPathsAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()

	auth := cc.app.Authorization()
	if auth == nil {
		return r.Api(c, r.WithError(fiber.NewError(fiber.StatusUnauthorized, "unauthorized")), r.WithStatus(fiber.StatusUnauthorized))
	}
	authUser, err := auth.User(c, ctx, true)
	if err != nil || authUser == nil || authUser.UserID == "" {
		return r.Api(c, r.WithError(fiber.NewError(fiber.StatusUnauthorized, "unauthorized")), r.WithStatus(fiber.StatusUnauthorized))
	}

	roles := cc.app.FreshUserRoles(ctx, authUser.UserID, authUser.Roles)
	isAdmin := userHasAdminRole(cc.app, roles)

	targetUserID := strings.TrimSpace(c.Query("user_id"))
	if targetUserID == "" {
		targetUserID = authUser.UserID
	}
	// Non-admins may only browse for themselves.
	if !isAdmin && targetUserID != authUser.UserID {
		return r.Api(c, r.WithError(fiber.NewError(fiber.StatusForbidden, "forbidden")), r.WithStatus(fiber.StatusForbidden))
	}

	home, linuxName := resolveLinuxHome(cc, targetUserID)
	roots := buildPathRoots(isAdmin, home, linuxName)

	raw := strings.TrimSpace(c.Query("path"))
	if raw == "" {
		raw = "/workspace"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	// Collapse // but keep trailing slash meaning "inside this folder".
	trailingSlash := strings.HasSuffix(raw, "/")
	clean := filepath.Clean(raw)
	if clean == "." {
		clean = "/"
	}

	listDir, prefix := splitBrowsePath(clean, trailingSlash)
	if !isPathAllowed(listDir, roots, isAdmin) {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": pathBrowseResult{
				Path:       clean,
				Exists:     false,
				WillCreate: true,
				Prefix:     prefix,
				Entries:    []pathEntry{},
				Roots:      roots,
				Admin:      isAdmin,
			},
		}))
	}

	exists := false
	isDir := false
	if st, err := os.Stat(listDir); err == nil {
		exists = true
		isDir = st.IsDir()
	}

	entries := []pathEntry{}
	if exists && isDir {
		entries = listChildDirs(listDir, prefix, roots, isAdmin)
	} else if !exists {
		// Walk up to the deepest existing ancestor and filter by remaining prefix.
		ancestor, rest := deepestExistingDir(listDir)
		if ancestor != "" && isPathAllowed(ancestor, roots, isAdmin) {
			filter := rest
			if prefix != "" && rest == "" {
				filter = prefix
			} else if prefix != "" {
				filter = rest
			}
			entries = listChildDirs(ancestor, firstSegment(filter), roots, isAdmin)
			listDir = ancestor
		}
	}

	parent := ""
	if listDir != "/" {
		parent = filepath.Dir(listDir)
	}

	// Typed path missing (or not a dir) → EnsureFolder creates it on session start.
	willCreate := true
	if st, err := os.Stat(clean); err == nil && st.IsDir() {
		willCreate = false
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": pathBrowseResult{
			Path:       listDir,
			Parent:     parent,
			Exists:     exists && isDir,
			IsDir:      isDir,
			WillCreate: willCreate,
			Prefix:     prefix,
			Entries:    entries,
			Roots:      roots,
			Admin:      isAdmin,
		},
	}))
}

func resolveLinuxHome(cc *controller, userID string) (home, linuxName string) {
	db := cc.app.DB()
	if db == nil || strings.TrimSpace(userID) == "" {
		return "", ""
	}
	var u models.User
	if err := db.Select("id", "username", "ldap_username").Where("id = ?", userID).First(&u).Error; err != nil {
		return "", ""
	}
	linuxName = strings.TrimSpace(u.Username)
	if linuxName == "" {
		linuxName = strings.TrimSpace(u.LdapUsername)
	}
	if linuxName == "" {
		return "", ""
	}
	acc, err := linuxuser.Lookup(linuxName)
	if err != nil || acc == nil || !acc.Exists {
		// Fallback convention when the Linux account is not provisioned yet.
		return filepath.Join("/home", linuxName), linuxName
	}
	home = strings.TrimSpace(acc.HomeDir)
	if home == "" {
		home = filepath.Join("/home", linuxName)
	}
	return home, linuxName
}

func buildPathRoots(isAdmin bool, home, linuxName string) []pathRoot {
	roots := []pathRoot{
		{Path: "/workspace", Label: "Workspace"},
	}
	if home != "" {
		label := "Home"
		if linuxName != "" {
			label = "Home (" + linuxName + ")"
		}
		roots = append(roots, pathRoot{Path: filepath.Clean(home), Label: label})
	}
	if isAdmin {
		extra := []pathRoot{
			{Path: "/", Label: "Root"},
			{Path: "/home", Label: "Homes"},
			{Path: "/tmp", Label: "Temp"},
			{Path: "/opt", Label: "Opt"},
			{Path: "/var", Label: "Var"},
		}
		seen := map[string]struct{}{}
		for _, r := range roots {
			seen[r.Path] = struct{}{}
		}
		for _, r := range extra {
			if _, ok := seen[r.Path]; ok {
				continue
			}
			roots = append(roots, r)
			seen[r.Path] = struct{}{}
		}
	}
	return roots
}

func isPathAllowed(path string, roots []pathRoot, isAdmin bool) bool {
	path = filepath.Clean(path)
	if path == "" {
		return false
	}
	if isAdmin {
		return strings.HasPrefix(path, "/")
	}
	for _, root := range roots {
		rootPath := filepath.Clean(root.Path)
		if path == rootPath || strings.HasPrefix(path, rootPath+string(os.PathSeparator)) {
			return true
		}
		// Allow listing the parent of a root so the root itself appears as a child suggestion.
		if rootPath != "/" && filepath.Dir(rootPath) == path {
			return true
		}
	}
	return false
}

func splitBrowsePath(clean string, trailingSlash bool) (listDir, prefix string) {
	if trailingSlash || clean == "/" {
		return clean, ""
	}
	base := filepath.Base(clean)
	dir := filepath.Dir(clean)
	if dir == "" {
		dir = "/"
	}
	return dir, base
}

func deepestExistingDir(path string) (ancestor, rest string) {
	path = filepath.Clean(path)
	cur := path
	var missing []string
	for {
		if st, err := os.Stat(cur); err == nil && st.IsDir() {
			rest = strings.Join(reverse(missing), "/")
			return cur, rest
		}
		if cur == "/" {
			return "/", strings.Join(reverse(missing), "/")
		}
		missing = append(missing, filepath.Base(cur))
		next := filepath.Dir(cur)
		if next == cur {
			return "", ""
		}
		cur = next
	}
}

func reverse(in []string) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[len(in)-1-i]
	}
	return out
}

func firstSegment(rest string) string {
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return ""
	}
	if before, _, ok := strings.Cut(rest, "/"); ok {
		return before
	}
	return rest
}

func listChildDirs(dir, prefix string, roots []pathRoot, isAdmin bool) []pathEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []pathEntry{}
	}
	prefixLower := strings.ToLower(prefix)
	out := make([]pathEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !e.IsDir() {
			// Follow symlinks that point to directories.
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target := filepath.Join(dir, name)
			st, err := os.Stat(target)
			if err != nil || !st.IsDir() {
				continue
			}
		}
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(name), prefixLower) {
			continue
		}
		full := filepath.Join(dir, name)
		if !isPathAllowed(full, roots, isAdmin) {
			// When listing a parent of a root, only expose allowed root children.
			allowedChild := false
			for _, root := range roots {
				if filepath.Clean(root.Path) == full {
					allowedChild = true
					break
				}
			}
			if !allowedChild {
				continue
			}
		}
		out = append(out, pathEntry{
			Name:        name,
			Path:        full,
			HasChildren: dirHasChildDirs(full),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func dirHasChildDirs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			return true
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		st, err := os.Stat(filepath.Join(dir, name))
		if err == nil && st.IsDir() {
			return true
		}
	}
	return false
}
