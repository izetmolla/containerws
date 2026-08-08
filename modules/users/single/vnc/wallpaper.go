package vnc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
)

var allowedWallpaperExt = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

func (cc *controller) UploadWallpaperAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if strings.TrimSpace(user.Username) == "" {
		return r.Api(c, r.WithError(errors.New("user has no linux username")), r.WithStatus(fiber.StatusBadRequest))
	}

	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		return r.Api(c, r.WithError(errors.New("file is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	if fh.Size <= 0 || fh.Size > 12<<20 {
		return r.Api(c, r.WithError(errors.New("image must be between 1 byte and 12MB")), r.WithStatus(fiber.StatusBadRequest))
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if _, ok := allowedWallpaperExt[ext]; !ok {
		return r.Api(c, r.WithError(errors.New("allowed types: jpg, jpeg, png, webp")), r.WithStatus(fiber.StatusBadRequest))
	}

	dest := adduser.UserWallpaperPath(user.Username)
	if dest == "" {
		return r.Api(c, r.WithError(errors.New("could not resolve wallpaper path")), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	src, err := fh.Open()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	defer src.Close()

	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = out.Close()
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	_ = db.WithContext(ctx).Model(session).Update("wallpaper_path", dest).Error
	session.WallpaperPath = dest

	warning := ""
	if applyErr := adduser.ApplyWallpaper(user.Username, dest); applyErr != nil {
		warning = "saved; apply on next desktop start: " + applyErr.Error()
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    Payload(*session, user.Username),
		"message": "Desktop wallpaper updated",
		"warning": warning,
	}))
}

func (cc *controller) DeleteWallpaperAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	path := strings.TrimSpace(session.WallpaperPath)
	if path == "" && user.Username != "" {
		path = adduser.UserWallpaperPath(user.Username)
	}
	if path != "" {
		_ = os.Remove(path)
	}
	_ = db.WithContext(ctx).Model(session).Update("wallpaper_path", "").Error
	session.WallpaperPath = ""

	defaultWall := "/usr/share/backgrounds/containerws/desktop.jpg"
	warning := ""
	if user.Username != "" {
		if applyErr := adduser.ApplyWallpaper(user.Username, defaultWall); applyErr != nil {
			warning = "reset saved; apply on next start: " + applyErr.Error()
		}
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    Payload(*session, user.Username),
		"message": "Wallpaper reset to default",
		"warning": warning,
	}))
}

func (cc *controller) GetWallpaperAPI(c fiber.Ctx) error {
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	session, err := cc.loadSession(c, user.ID)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	path := strings.TrimSpace(session.WallpaperPath)
	if path == "" || !fileExists(path) {
		path = "/usr/share/backgrounds/containerws/desktop.jpg"
	}
	if !fileExists(path) {
		return fiber.NewError(fiber.StatusNotFound, "wallpaper not found")
	}
	ext := strings.ToLower(filepath.Ext(path))
	ctype := allowedWallpaperExt[ext]
	if ctype == "" {
		ctype = "image/jpeg"
	}
	c.Set("Content-Type", ctype)
	c.Set("Cache-Control", "private, max-age=60")
	return c.SendFile(path)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func wallpaperURL(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	return fmt.Sprintf("/api/users/single/%s/vnc/wallpaper", userID)
}
