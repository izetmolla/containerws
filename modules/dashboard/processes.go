package dashboard

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/packages/machine"
)

func (cc *controller) GetProcessesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}
	return r.Api(c, r.WithData(fiber.Map{
		"data": machine.CollectProcesses(limit),
	}))
}

func (cc *controller) KillProcessAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	pid, err := strconv.Atoi(strings.TrimSpace(c.Params("pid")))
	if err != nil || pid <= 0 {
		return r.Api(c, r.WithError(errors.New("invalid pid")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_PID"))
	}
	force := c.Query("force") == "1" || c.Query("force") == "true"
	if err := machine.KillProcess(pid, force); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KILL_FAILED"))
	}
	return r.Api(c, r.WithData(fiber.Map{
		"data": fiber.Map{"pid": pid, "killed": true, "force": force},
	}))
}
