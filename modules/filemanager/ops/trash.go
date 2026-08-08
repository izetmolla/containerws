package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/modules/filemanager/fsutil"
)

type trashListItem struct {
	ID           string `json:"id"`
	OriginalPath string `json:"original_path"`
	Name         string `json:"name"`
	IsDir        bool   `json:"is_dir"`
	DeletedAt    string `json:"deleted_at"`
	ExpiresAt    string `json:"expires_at"`
	DaysLeft     int    `json:"days_left"`
	Size         int64  `json:"size,omitempty"`
	TrashPath    string `json:"trash_path"`
}

// TrashAPI POST /filemanager/ops/trash — soft-delete into trash.
func (cc *controller) TrashAPI(c fiber.Ctx) error {
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
	var info fsutil.TrashInfo
	runErr := ctx.Run(func() error {
		_, _ = fsutil.PurgeExpiredTrash(ctx.HomeDir)
		var terr error
		info, terr = fsutil.MoveToTrash(ctx.HomeDir, path)
		return terr
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"id":            info.ID,
		"original_path": info.OriginalPath,
		"name":          info.Name,
		"deleted_at":    info.DeletedAt.Format(time.RFC3339),
	}, "Moved to trash")
}

// ListTrashAPI GET /filemanager/ops/trash
func (cc *controller) ListTrashAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var items []trashListItem
	runErr := ctx.Run(func() error {
		_, _ = fsutil.PurgeExpiredTrash(ctx.HomeDir)
		entries, err := listTrashMeta(ctx.HomeDir)
		if err != nil {
			return err
		}
		items = entries
		return nil
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"path":      fsutil.TrashRoot(ctx.HomeDir),
		"items":     items,
		"retention": "30d",
		"total":     len(items),
	}, "")
}

func listTrashMeta(home string) ([]trashListItem, error) {
	flat := fsutil.ListTrashFlatEntries(home)
	items := make([]trashListItem, 0, len(flat))
	for _, e := range flat {
		id := fsutil.TrashIDFromHint(e.MimeHint)
		if id == "" {
			continue
		}
		info, err := fsutil.ReadTrashInfo(home, id)
		if err != nil {
			continue
		}
		expires, days := fsutil.FormatTrashExpiry(info.DeletedAt)
		items = append(items, trashListItem{
			ID:           info.ID,
			OriginalPath: info.OriginalPath,
			Name:         info.Name,
			IsDir:        info.IsDir,
			DeletedAt:    info.DeletedAt.UTC().Format(time.RFC3339),
			ExpiresAt:    expires,
			DaysLeft:     days,
			Size:         info.Size,
			TrashPath:    e.Path,
		})
	}
	return items, nil
}

type trashIDBody struct {
	ID string `json:"id"`
}

// RestoreTrashAPI POST /filemanager/ops/trash/restore
func (cc *controller) RestoreTrashAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body trashIDBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	id := strings.TrimSpace(body.ID)
	if id == "" {
		return cc.respondErr(c, fmt.Errorf("id is required"))
	}
	var (
		info fsutil.TrashInfo
		dst  string
	)
	runErr := ctx.Run(func() error {
		var rerr error
		info, dst, rerr = fsutil.RestoreFromTrash(ctx.HomeDir, id)
		return rerr
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{
		"id":            info.ID,
		"original_path": info.OriginalPath,
		"restored_to":   dst,
		"name":          info.Name,
	}, "Restored")
}

// DeleteTrashAPI POST /filemanager/ops/trash/delete — permanent delete from trash.
func (cc *controller) DeleteTrashAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var body trashIDBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	id := strings.TrimSpace(body.ID)
	if id == "" {
		return cc.respondErr(c, fmt.Errorf("id is required"))
	}
	runErr := ctx.Run(func() error {
		return fsutil.PermanentlyDeleteTrashItem(ctx.HomeDir, id)
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"id": id, "deleted": true}, "Permanently deleted")
}

// EmptyTrashAPI POST /filemanager/ops/trash/empty
func (cc *controller) EmptyTrashAPI(c fiber.Ctx) error {
	ctx, err := cc.resolve(c)
	if err != nil {
		return cc.authErr(c, err)
	}
	var n int
	runErr := ctx.Run(func() error {
		var eerr error
		n, eerr = fsutil.EmptyTrash(ctx.HomeDir)
		return eerr
	})
	if runErr != nil {
		return cc.respondErr(c, runErr)
	}
	return cc.respond(c, fiber.StatusOK, fiber.Map{"deleted": n}, "Trash emptied")
}
