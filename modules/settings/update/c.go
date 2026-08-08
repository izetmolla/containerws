package update

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/version"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /api/settings/update.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.GetStatusAPI)
	api.Get("/status", cc.GetStatusAPI)
	api.Post("/check", cc.CheckAPI)
	api.Get("/releases", cc.ListReleasesAPI)
	api.Post("/apply", cc.ApplyAPI)
}

func (cc *controller) statusPayload() fiber.Map {
	releases, lastCheck, latest, errMsg := globalCache.snapshot()
	binPath, _ := currentBinaryPath()
	var lastCheckStr string
	if !lastCheck.IsZero() {
		lastCheckStr = lastCheck.UTC().Format(time.RFC3339)
	}
	updateAvailable := false
	if latest != "" {
		updateAvailable = isNewer(latest, version.Version)
	}
	return fiber.Map{
		"current_version":  version.Version,
		"commit_sha":       version.CommitSHA,
		"binary_path":      binPath,
		"goos":             runtime.GOOS,
		"goarch":           runtime.GOARCH,
		"repo":             updateRepo(),
		"expected_asset":   expectedAssetName(version.Version),
		"latest_tag":       latest,
		"update_available": updateAvailable,
		"last_check":       lastCheckStr,
		"last_error":       errMsg,
		"releases_count":   len(releases),
	}
}

func (cc *controller) GetStatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	_, lastCheck, _, _ := globalCache.snapshot()
	// Auto-check when never checked or older than 6h.
	if lastCheck.IsZero() || time.Since(lastCheck) > 6*time.Hour {
		ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
		releases, latest, err := fetchReleases(ctx, version.Version)
		cancel()
		if err != nil {
			globalCache.set(nil, "", err.Error())
		} else {
			globalCache.set(releases, latest, "")
		}
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cc.statusPayload(),
	}))
}

func (cc *controller) CheckAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
	defer cancel()
	releases, latest, err := fetchReleases(ctx, version.Version)
	if err != nil {
		globalCache.set(nil, "", err.Error())
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway), r.WithErrorData(fiber.Map{
			"data": cc.statusPayload(),
		}))
	}
	globalCache.set(releases, latest, "")
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.statusPayload(),
		"message": "Checked GitHub releases",
	}))
}

func (cc *controller) ListReleasesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	releases, lastCheck, latest, errMsg := globalCache.snapshot()
	if len(releases) == 0 && lastCheck.IsZero() {
		ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
		var err error
		releases, latest, err = fetchReleases(ctx, version.Version)
		cancel()
		if err != nil {
			globalCache.set(nil, "", err.Error())
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
		}
		globalCache.set(releases, latest, "")
		releases, lastCheck, latest, errMsg = globalCache.snapshot()
	}
	var lastCheckStr string
	if !lastCheck.IsZero() {
		lastCheckStr = lastCheck.UTC().Format(time.RFC3339)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"releases":   releases,
			"latest_tag": latest,
			"last_check": lastCheckStr,
			"last_error": errMsg,
			"repo":       updateRepo(),
			"goos":       runtime.GOOS,
			"goarch":     runtime.GOARCH,
		},
	}))
}

type applyBody struct {
	Version string `json:"version"`
	Force   bool   `json:"force"`
}

func (cc *controller) ApplyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body applyBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	tag := strings.TrimSpace(body.Version)
	if tag == "" {
		return r.Api(c, r.WithError(errors.New("version is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	releases, _, _, _ := globalCache.snapshot()
	var target *Release
	for i := range releases {
		if releases[i].Tag == tag || normalizeVersion(releases[i].Tag) == normalizeVersion(tag) {
			target = &releases[i]
			break
		}
	}
	if target == nil {
		// Fresh fetch if cache miss.
		ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
		list, latest, err := fetchReleases(ctx, version.Version)
		cancel()
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
		}
		globalCache.set(list, latest, "")
		for i := range list {
			if list[i].Tag == tag || normalizeVersion(list[i].Tag) == normalizeVersion(tag) {
				target = &list[i]
				break
			}
		}
	}
	if target == nil {
		return r.Api(c, r.WithError(errors.New("release not found")), r.WithStatus(fiber.StatusNotFound))
	}
	if !target.HasAsset || target.AssetName == "" {
		return r.Api(c, r.WithError(errors.New("no matching binary asset for this OS/arch")), r.WithStatus(fiber.StatusBadRequest))
	}
	if !body.Force && !isNewer(target.Tag, version.Version) && normalizeVersion(target.Tag) != normalizeVersion(version.Version) {
		if compareSemver(target.Tag, version.Version) < 0 {
			return r.Api(c, r.WithError(errors.New("refusing downgrade without force=true")), r.WithStatus(fiber.StatusBadRequest))
		}
	}

	assetURL := strings.TrimSpace(target.AssetURL)
	if assetURL == "" {
		assetURL = releaseDownloadURL(target.Tag, target.AssetName)
	}
	if assetURL == "" {
		return r.Api(c, r.WithError(errors.New("missing download URL for release asset")), r.WithStatus(fiber.StatusBadRequest))
	}

	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Minute)
	defer cancel()
	installed, err := downloadAndInstall(ctx, assetURL, target.AssetName)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	scheduleRestart(installed)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"version":     target.Tag,
			"binary_path": installed,
			"asset_name":  target.AssetName,
			"asset_url":   assetURL,
			"repo":        updateRepo(),
			"restarting":  true,
			"previous":    version.Version,
			"pid":         os.Getpid(),
		},
		"message": "Update installed — restarting with new binary",
	}))
}
