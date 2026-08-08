package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/izetmolla/containerws/config"
	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/packages/softwaresync"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/detect", cc.DetectAPI)
	api.Get("/status", cc.StatusAPI)
	api.Post("/stream", cc.StreamSetupAPI)
	api.Post("/jobs/:jobId/cancel", cc.CancelSetupAPI)
	api.Post("/", cc.SetupAPI)
}

type streamEvent struct {
	Type    string `json:"type"`
	JobID   string `json:"job_id,omitempty"`
	Line    string `json:"line,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Message string `json:"message,omitempty"`
	Success *bool  `json:"success,omitempty"`
	Plan    any    `json:"plan,omitempty"`
}

func (cc *controller) DetectAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	plan := DetectHost()
	status := CheckStatus()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"plan": plan, "status": status},
	}))
}

func (cc *controller) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": CheckStatus(),
	}))
}

func (cc *controller) SetupAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	plan := DetectHost()
	script, err := BuildSetupScript(plan)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("UNSUPPORTED_OS"))
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	_ = softwaresync.WaitReady(runCtx)
	_ = swinstall.WaitQueueIdle(runCtx, cc.app.DB(), time.Second, nil)

	cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
	cmd.Env = setupEnv()
	cmd.Dir = "/root"
	out, err := cmd.CombinedOutput()
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithData(fiber.Map{
			"plan": plan, "output": string(out), "message": "RDP setup failed",
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"plan": plan, "output": string(out), "status": CheckStatus(),
		"message": "RDP (xrdp) packages installed",
	}))
}

func (cc *controller) StreamSetupAPI(c fiber.Ctx) error {
	plan := DetectHost()
	reinstall := truthyQuery(c.Query("reinstall"))
	script, err := BuildSetupScriptOpts(plan, reinstall)
	if err != nil {
		r := cc.app.Render()
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("UNSUPPORTED_OS"))
	}

	jobID := uuid.New().String()
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job := &setupJob{ID: jobID, Cancel: cancel, CreatedAt: time.Now()}
	registerJob(job)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	startMsg := "Starting optional RDP (xrdp) package setup"
	if reinstall {
		startMsg = "Starting RDP (xrdp) force reinstall"
	}

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer cancel()
		defer finishJob(jobID)

		var writeMu sync.Mutex
		writeEvent := func(ev streamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}

		if !writeEvent(streamEvent{
			Type: "start", JobID: jobID,
			Message: startMsg,
			Plan:    plan,
		}) {
			return
		}

		_ = writeEvent(streamEvent{
			Type: "log", JobID: jobID, Stream: "system",
			Line: "Waiting for software install queue before RDP setup…",
		})
		_ = softwaresync.WaitReady(runCtx)
		if err := swinstall.WaitQueueIdle(runCtx, cc.app.DB(), time.Second, func(q swinstall.QueueView) {
			if q.Pending == 0 {
				return
			}
			_ = writeEvent(streamEvent{
				Type: "log", JobID: jobID, Stream: "system",
				Line: fmt.Sprintf("Waiting for software install queue (%d remaining)", q.Pending),
			})
		}); err != nil {
			msg := "cancelled while waiting for software queue"
			if runCtx.Err() == nil {
				msg = err.Error()
			}
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: msg})
			return
		}
		_ = writeEvent(streamEvent{
			Type: "log", JobID: jobID, Stream: "system", Line: startMsg,
		})

		cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
		cmd.Env = setupEnv()
		cmd.Dir = "/root"
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}
		if err := cmd.Start(); err != nil {
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}

		var wg sync.WaitGroup
		streamPipe := func(reader io.Reader, stream string) {
			defer wg.Done()
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				if !writeEvent(streamEvent{Type: "log", JobID: jobID, Stream: stream, Line: scanner.Text()}) {
					cancel()
					return
				}
			}
		}
		wg.Add(2)
		go streamPipe(stdout, "stdout")
		go streamPipe(stderr, "stderr")
		wg.Wait()

		runErr := cmd.Wait()
		if job.isCancelled() {
			_ = writeEvent(streamEvent{Type: "cancelled", JobID: jobID, Message: "Installation stopped"})
			return
		}
		ok := runErr == nil
		msg := "RDP (xrdp) packages installed"
		if reinstall && ok {
			msg = "RDP (xrdp) packages reinstalled"
		}
		if !ok {
			msg = runErr.Error()
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				msg = "setup timed out"
			}
		}
		_ = writeEvent(streamEvent{
			Type: "done", JobID: jobID, Success: &ok, Message: msg, Plan: CheckStatus(),
		})
	})
}

func truthyQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (cc *controller) CancelSetupAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	jobID := c.Params("jobId")
	if !cancelJob(jobID) {
		return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("setup job not found or already finished")), r.WithStatus(fiber.StatusNotFound))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"job_id": jobID, "cancelled": true},
	}))
}
