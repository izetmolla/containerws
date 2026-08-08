package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/:id/status", cc.GetStatusAPI)
	api.Post("/:id/start", cc.StartAPI)
	api.Post("/:id/stop", cc.StopAPI)
	api.Post("/:id/restart", cc.RestartAPI)
	api.Get("/:id/logs", cc.GetLogsAPI)
	api.Get("/:id/logs/stream", cc.StreamLogsAPI)
}

func (cc *controller) GetStatusAPI(c fiber.Ctx) error {
	return cc.respondStatus(c, "")
}

func (cc *controller) StartAPI(c fiber.Ctx) error {
	return cc.respondStatus(c, "start")
}

func (cc *controller) StopAPI(c fiber.Ctx) error {
	return cc.respondStatus(c, "stop")
}

func (cc *controller) RestartAPI(c fiber.Ctx) error {
	return cc.respondStatus(c, "restart")
}

func (cc *controller) respondStatus(c fiber.Ctx, action string) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("software id required")), r.WithStatus(fiber.StatusBadRequest))
	}

	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	units := []string(sw.ServiceUnits)
	canControl := CanControl(sw)
	if action == "" {
		st := ProbeUnits(units)
		return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"software_id":     sw.ID,
			"name":            sw.Name,
			"can_control":     canControl,
			"control_backend": sw.ControlBackend,
			"start_command":   sw.StartCommand,
			"restart_command": sw.RestartCommand,
			"stop_command":    sw.StopCommand,
			"status":          st,
		}))
	}

	if !canControl {
		return r.Api(c, r.WithError(errors.New("software is not marked controllable or has no service units/commands")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorData(fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"can_control": false,
		}))
	}

	st, err := ControlSoftware(action, sw)
	if err != nil {
		status := fiber.StatusBadRequest
		msg := err.Error()
		if strings.Contains(msg, "systemctl not available") ||
			strings.Contains(msg, "systemd not running") {
			status = fiber.StatusServiceUnavailable
		}
		return r.Api(c, r.WithError(err), r.WithStatus(status), r.WithErrorData(fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"can_control": canControl,
			"status":      st,
		}))
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"software_id":     sw.ID,
		"name":            sw.Name,
		"can_control":     canControl,
		"control_backend": sw.ControlBackend,
		"start_command":   sw.StartCommand,
		"restart_command": sw.RestartCommand,
		"stop_command":    sw.StopCommand,
		"status":          st,
	}))
}

func (cc *controller) GetLogsAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("software id required")), r.WithStatus(fiber.StatusBadRequest))
	}
	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	units := []string(sw.ServiceUnits)
	if !CanControl(sw) {
		return r.Api(c, r.WithError(errors.New("software is not marked controllable or has no service units")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorData(fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"can_control": false,
		}))
	}

	n := queryInt(c, "lines", 120)
	lines, lerr := TailLogs(ctx, units, n)
	if lerr != nil {
		status := fiber.StatusBadRequest
		if strings.Contains(lerr.Error(), "journalctl not available") {
			status = fiber.StatusServiceUnavailable
		}
		return r.Api(c, r.WithError(lerr), r.WithStatus(status))
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"software_id": sw.ID,
		"name":        sw.Name,
		"units":       units,
		"lines":       lines,
	}))
}

func (cc *controller) StreamLogsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	ctx := c.Context()

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("software id required")), r.WithStatus(fiber.StatusBadRequest))
	}
	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	units := []string(sw.ServiceUnits)
	if !CanControl(sw) {
		return r.Api(c, r.WithError(errors.New("software is not marked controllable or has no service units")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorData(fiber.Map{
			"software_id": sw.ID,
			"name":        sw.Name,
			"can_control": false,
		}))
	}

	n := queryInt(c, "lines", 120)
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		writeEvent := func(payload any) bool {
			raw, err := json.Marshal(payload)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
				return false
			}
			return w.Flush() == nil
		}

		if !writeEvent(fiber.Map{
			"type":        "start",
			"software_id": sw.ID,
			"name":        sw.Name,
			"units":       units,
			"message":     fmt.Sprintf("Streaming logs for %s (%s)", sw.Name, strings.Join(units, ", ")),
		}) {
			return
		}

		streamErr := StreamLogs(ctx, units, n, func(line LogLine) error {
			if !writeEvent(fiber.Map{
				"type":   "log",
				"unit":   line.Unit,
				"line":   line.Text,
				"at":     line.At,
				"stream": "stdout",
			}) {
				return errors.New("client disconnected")
			}
			return nil
		})
		if streamErr != nil && ctx.Err() == nil {
			_ = writeEvent(fiber.Map{
				"type":    "error",
				"message": streamErr.Error(),
			})
		}
		_ = writeEvent(fiber.Map{
			"type":    "done",
			"message": "log stream ended",
		})
	})
}

func queryInt(c fiber.Ctx, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
