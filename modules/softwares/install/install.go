package install

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

func (cc *controller) InstallSoftwareAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := c.Params("id")
	if id == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("id", "Software id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(ctx)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if len(versions) == 0 {
		return r.Api(c, r.WithErrorCode("NO_VERSION"), r.WithError(errors.New("no version available to install")), r.WithStatus(fiber.StatusBadRequest))
	}

	latest := versions[0]
	if latest.InstallScript == "" {
		return r.Api(c, r.WithErrorCode("NO_SCRIPT"), r.WithError(errors.New("latest version has no install script")), r.WithStatus(fiber.StatusBadRequest))
	}

	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-lc", latest.InstallScript)
	cmd.Env = installEnv()
	cmd.Dir = "/root"
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	result := fiber.Map{
		"software":       sw,
		"latest_version": latest,
		"stdout":         stdout.String(),
		"stderr":         stderr.String(),
	}

	if runErr != nil {
		result["success"] = false
		result["error"] = runErr.Error()
		return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    result,
			"message": "Install finished with errors",
		}))
	}

	_ = markVersionInstalled(db, sw.ID, latest.ID)

	customOut := ""
	customErr := ""
	if cs := strings.TrimSpace(latest.CustomScript); cs != "" {
		var cStdout, cStderr bytes.Buffer
		ccmd := exec.CommandContext(runCtx, "bash", "-lc", cs)
		ccmd.Env = installEnv()
		ccmd.Dir = "/root"
		ccmd.Stdout = &cStdout
		ccmd.Stderr = &cStderr
		if err := ccmd.Run(); err != nil {
			customErr = err.Error()
			result["custom_stdout"] = cStdout.String()
			result["custom_stderr"] = cStderr.String()
			result["custom_error"] = customErr
			result["success"] = false
			result["error"] = "custom setup failed: " + customErr
			return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
				"data":    result,
				"message": "Installed but custom setup failed",
			}))
		}
		customOut = cStdout.String()
		result["custom_stdout"] = customOut
		result["custom_stderr"] = cStderr.String()
	}

	latest.IsInstalled = true
	latest.HasUpdate = false
	result["latest_version"] = latest
	result["success"] = true
	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    result,
		"message": "Installed " + sw.Name + " " + latest.Version,
	}))
}
