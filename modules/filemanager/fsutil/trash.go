package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const TrashRetention = 30 * 24 * time.Hour

// TrashInfo is persisted metadata for a soft-deleted item.
type TrashInfo struct {
	ID           string    `json:"id"`
	OriginalPath string    `json:"original_path"`
	Name         string    `json:"name"`
	IsDir        bool      `json:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at"`
	Size         int64     `json:"size,omitempty"`
}

func TrashRoot(home string) string {
	return filepath.Join(home, ".containerws", "trash")
}

func TrashItemDir(home, id string) string {
	return filepath.Join(TrashRoot(home), id)
}

func TrashInfoPath(home, id string) string {
	return filepath.Join(TrashItemDir(home, id), "info.json")
}

func TrashContentPath(home, id, name string) string {
	return filepath.Join(TrashItemDir(home, id), "content", name)
}

func EnsureTrashRoot(home string) error {
	return os.MkdirAll(TrashRoot(home), 0o700)
}

func WriteTrashInfo(home string, info TrashInfo) error {
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(TrashInfoPath(home, info.ID), raw, 0o600)
}

func ReadTrashInfo(home, id string) (TrashInfo, error) {
	var info TrashInfo
	raw, err := os.ReadFile(TrashInfoPath(home, id))
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return info, err
	}
	return info, nil
}

// IsUnderTrash reports whether path is the trash root or inside it.
func IsUnderTrash(home, path string) bool {
	tr := TrashRoot(home)
	path = filepath.Clean(path)
	return path == tr || strings.HasPrefix(path+string(os.PathSeparator), tr+string(os.PathSeparator))
}

func PurgeExpiredTrash(home string) (int, error) {
	root := TrashRoot(home)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	now := time.Now().UTC()
	purged := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		info, err := ReadTrashInfo(home, id)
		if err != nil {
			continue
		}
		if now.Sub(info.DeletedAt.UTC()) >= TrashRetention {
			if err := os.RemoveAll(TrashItemDir(home, id)); err == nil {
				purged++
			}
		}
	}
	return purged, nil
}

func MoveToTrash(home, path string) (TrashInfo, error) {
	infoStat, err := os.Lstat(path)
	if err != nil {
		return TrashInfo{}, err
	}
	if path == "/" {
		return TrashInfo{}, fiber.NewError(fiber.StatusBadRequest, "refusing to trash filesystem root")
	}
	if IsUnderTrash(home, path) {
		return TrashInfo{}, fiber.NewError(fiber.StatusBadRequest, "refusing to trash the trash folder")
	}

	if err := EnsureTrashRoot(home); err != nil {
		return TrashInfo{}, err
	}

	id := uuid.NewString()
	name := filepath.Base(path)
	itemDir := TrashItemDir(home, id)
	contentDir := filepath.Join(itemDir, "content")
	if err := os.MkdirAll(contentDir, 0o700); err != nil {
		return TrashInfo{}, err
	}
	dest := filepath.Join(contentDir, name)

	if err := os.Rename(path, dest); err != nil {
		if infoStat.IsDir() {
			if _, cerr := CopyDir(path, dest); cerr != nil {
				_ = os.RemoveAll(itemDir)
				return TrashInfo{}, cerr
			}
		} else {
			if _, cerr := CopyFile(path, dest, infoStat.Mode()); cerr != nil {
				_ = os.RemoveAll(itemDir)
				return TrashInfo{}, cerr
			}
		}
		if err := os.RemoveAll(path); err != nil {
			_ = os.RemoveAll(itemDir)
			return TrashInfo{}, err
		}
	}

	var size int64
	if !infoStat.IsDir() {
		size = infoStat.Size()
	}

	info := TrashInfo{
		ID:           id,
		OriginalPath: path,
		Name:         name,
		IsDir:        infoStat.IsDir(),
		DeletedAt:    time.Now().UTC(),
		Size:         size,
	}
	if err := WriteTrashInfo(home, info); err != nil {
		_ = os.Rename(dest, path)
		_ = os.RemoveAll(itemDir)
		return TrashInfo{}, err
	}
	return info, nil
}

// ListTrashFlatEntries returns trash items as listing rows (name = original name).
// Entry.Path points at the trash content path; MimeHint carries trash id as "trash:<id>".
func ListTrashFlatEntries(home string) []Entry {
	_, _ = PurgeExpiredTrash(home)
	root := TrashRoot(home)
	dirents, err := os.ReadDir(root)
	if err != nil {
		return []Entry{}
	}
	out := make([]Entry, 0, len(dirents))
	for _, d := range dirents {
		if !d.IsDir() {
			continue
		}
		id := d.Name()
		info, err := ReadTrashInfo(home, id)
		if err != nil {
			continue
		}
		content := TrashContentPath(home, id, info.Name)
		st, err := os.Lstat(content)
		if err != nil {
			continue
		}
		e := BuildEntry(info.Name, content, st)
		e.MimeHint = "trash:" + id
		e.Hidden = false
		out = append(out, e)
	}
	return out
}

func RestoreFromTrash(home, id string) (TrashInfo, string, error) {
	info, err := ReadTrashInfo(home, id)
	if err != nil {
		return TrashInfo{}, "", err
	}
	src := TrashContentPath(home, id, info.Name)
	dst := info.OriginalPath
	if _, err := os.Lstat(dst); err == nil {
		dst, err = UniqueCopyPath(dst)
		if err != nil {
			return TrashInfo{}, "", err
		}
	} else if !os.IsNotExist(err) {
		return TrashInfo{}, "", err
	}
	if err := EnsureParentDir(dst); err != nil {
		return TrashInfo{}, "", err
	}
	if err := os.Rename(src, dst); err != nil {
		st, lerr := os.Lstat(src)
		if lerr != nil {
			return TrashInfo{}, "", err
		}
		if st.IsDir() {
			if _, cerr := CopyDir(src, dst); cerr != nil {
				return TrashInfo{}, "", cerr
			}
		} else {
			if _, cerr := CopyFile(src, dst, st.Mode()); cerr != nil {
				return TrashInfo{}, "", cerr
			}
		}
		if err := os.RemoveAll(src); err != nil {
			return TrashInfo{}, "", err
		}
	}
	_ = os.RemoveAll(TrashItemDir(home, id))
	return info, dst, nil
}

func PermanentlyDeleteTrashItem(home, id string) error {
	return os.RemoveAll(TrashItemDir(home, id))
}

func EmptyTrash(home string) (int, error) {
	root := TrashRoot(home)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err == nil {
			n++
		}
	}
	return n, nil
}

func TrashIDFromHint(hint string) string {
	if strings.HasPrefix(hint, "trash:") {
		return strings.TrimPrefix(hint, "trash:")
	}
	return ""
}

func FormatTrashExpiry(deletedAt time.Time) (expiresAt string, daysLeft int) {
	expires := deletedAt.UTC().Add(TrashRetention)
	daysLeft = int(expires.Sub(time.Now().UTC()).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	return expires.Format(time.RFC3339), daysLeft
}
