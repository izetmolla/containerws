package fsutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Entry is a directory listing row.
type Entry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`
	ModeOctal  string `json:"mode_octal"`
	ModTime    string `json:"mod_time"`
	Owner      string `json:"owner,omitempty"`
	Group      string `json:"group,omitempty"`
	UID        uint32 `json:"uid,omitempty"`
	GID        uint32 `json:"gid,omitempty"`
	Readable   bool   `json:"readable"`
	Writable   bool   `json:"writable"`
	Executable bool   `json:"executable"`
	Hidden     bool   `json:"hidden"`
	MimeHint   string `json:"mime_hint,omitempty"`
}

// Root shortcut shown in the sidebar.
type Root struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	// Group: places | disks | trash — controls sidebar sections.
	Group string `json:"group,omitempty"`
}

func ResolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		path = "/" + path
	}
	clean := filepath.Clean(path)
	if clean == "." {
		clean = "/"
	}
	return clean, nil
}

func EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func EntryType(mode os.FileMode) string {
	switch {
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsRegular():
		return "file"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeDevice != 0:
		return "device"
	default:
		return "other"
	}
}

func ModeOctal(mode os.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}

func BuildEntry(name, full string, info os.FileInfo) Entry {
	mode := info.Mode()
	e := Entry{
		Name:       name,
		Path:       full,
		Type:       EntryType(mode),
		Size:       info.Size(),
		Mode:       mode.String(),
		ModeOctal:  ModeOctal(mode),
		ModTime:    info.ModTime().UTC().Format(time.RFC3339),
		Hidden:     strings.HasPrefix(name, "."),
		Readable:   mode.Perm()&0o444 != 0,
		Writable:   mode.Perm()&0o222 != 0,
		Executable: mode.Perm()&0o111 != 0 || mode.IsDir(),
	}
	if mode.IsRegular() {
		e.MimeHint = mimeHint(name)
	}
	fillOwner(&e, info)
	return e
}

func mimeHint(name string) string {
	base := strings.ToLower(filepath.Base(name))
	switch base {
	case "dockerfile", "makefile", "gnumakefile", "cmakelists.txt",
		"license", "licence", "readme", "changelog", "authors", "copying",
		"gemfile", "rakefile", "procfile", "vagrantfile", ".gitignore",
		".gitattributes", ".dockerignore", ".editorconfig", ".npmrc",
		".nvmrc", ".env", ".env.local", ".env.example", ".bashrc", ".zshrc",
		".profile", ".gitconfig":
		return "text"
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt", ".md", ".markdown", ".rst", ".log", ".json", ".jsonc",
		".yaml", ".yml", ".toml", ".xml", ".csv", ".tsv", ".sql",
		".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py", ".rb",
		".rs", ".c", ".h", ".cpp", ".cc", ".hpp", ".java", ".kt", ".swift",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd",
		".env", ".conf", ".cfg", ".ini", ".properties", ".service", ".timer",
		".html", ".htm", ".css", ".scss", ".less", ".sass", ".svg",
		".vue", ".svelte", ".astro", ".php", ".lua", ".r", ".pl", ".pm",
		".tf", ".hcl", ".proto", ".graphql", ".gql", ".dockerfile",
		".makefile", ".mk", ".cmake", ".gradle", ".mod", ".sum",
		".lock", ".nix", ".ex", ".exs", ".erl", ".hrl", ".clj", ".edn",
		".zig", ".nim", ".dart", ".scala", ".groovy", ".vim", ".el",
		".tex", ".bib", ".org", ".adoc", ".textile", ".wiki",
		".pem", ".crt", ".key", ".pub", ".known_hosts", ".htaccess":
		return "text"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico":
		return "image"
	case ".mp4", ".webm", ".mkv", ".mov":
		return "video"
	case ".mp3", ".wav", ".ogg", ".flac":
		return "audio"
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar":
		return "archive"
	case ".pdf":
		return "pdf"
	default:
		return "binary"
	}
}

func CopyFile(src, dst string, mode os.FileMode) (int64, error) {
	if err := EnsureParentDir(dst); err != nil {
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

func CopyDir(src, dst string) (int64, error) {
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
			info, err := d.Info()
			perm := os.FileMode(0o755)
			if err == nil {
				perm = info.Mode().Perm()
			}
			return os.MkdirAll(target, perm)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		n, err := CopyFile(p, target, info.Mode())
		total += n
		return err
	})
}

func UniqueCopyPath(path string) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	candidate := filepath.Join(dir, name+" copy"+ext)
	if _, err := os.Lstat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s copy %d%s", name, i, ext))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find unique name for %s", path)
}

func ParseMode(raw string, fallback os.FileMode) (os.FileMode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	raw = strings.TrimPrefix(raw, "0o")
	raw = strings.TrimPrefix(raw, "0O")
	var parsed uint32
	if _, err := fmt.Sscanf(raw, "%o", &parsed); err != nil {
		return 0, fmt.Errorf("invalid mode %q: use octal like 0644", raw)
	}
	return os.FileMode(parsed), nil
}

func MapFSError(err error) (int, string) {
	if err == nil {
		return 200, ""
	}
	var fe *fiber.Error
	if errors.As(err, &fe) && fe != nil {
		code := fe.Code
		if code == 0 {
			code = 400
		}
		msg := fe.Message
		if msg == "" {
			msg = fe.Error()
		}
		return code, msg
	}
	if os.IsNotExist(err) {
		return 404, err.Error()
	}
	if os.IsPermission(err) {
		return 403, err.Error()
	}
	if os.IsExist(err) {
		return 409, err.Error()
	}
	return 400, err.Error()
}
