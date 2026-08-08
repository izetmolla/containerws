package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/packages/softwaresync"
)

type streamEvent struct {
	Type    string `json:"type"`
	JobID   string `json:"job_id,omitempty"`
	Line    string `json:"line,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Message string `json:"message,omitempty"`
	Success *bool  `json:"success,omitempty"`
	Plan    any    `json:"plan,omitempty"`
	Queue   any    `json:"software_queue,omitempty"`
}

func (cc *controller) DetectAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	plan := DetectHost()
	status := CheckStatus()
	opts := SyncOptionsFromStatus(db, status)
	installState := GetActiveInstall()
	queue := swinstall.ActiveQueue(db)
	installState.Queue = queue
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"plan":               plan,
			"status":             status,
			"options":            opts,
			"install":            installState,
			"software_queue":     queue,
			"softwaresync_ready": softwaresync.Ready(),
		},
	}))
}

func (cc *controller) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	status := CheckStatus()
	opts := SyncOptionsFromStatus(db, status)
	installState := GetActiveInstall()
	queue := swinstall.ActiveQueue(db)
	installState.Queue = queue
	// Flatten StatusReport fields for existing clients that read data.ready.
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"ready":              status.Ready,
			"binaries":           status.Binaries,
			"novnc_roots":        status.NovncRoots,
			"missing":            status.Missing,
			"plan":               status.Plan,
			"options":            opts,
			"install":            installState,
			"software_queue":     queue,
			"softwaresync_ready": softwaresync.Ready(),
		},
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

	if err := waitForSoftwareQueue(runCtx, cc.app.DB(), nil); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusConflict), r.WithData(fiber.Map{
			"message": "software install queue did not become idle",
		}))
	}

	cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
	cmd.Env = setupEnv()
	cmd.Dir = "/root"
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = SyncOptionsFromStatus(cc.app.DB(), CheckStatus())
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithData(fiber.Map{
			"plan":    plan,
			"output":  string(out),
			"message": "setup failed",
		}))
	}
	MarkInstalled(cc.app.DB())
	status := CheckStatus()
	opts := SyncOptionsFromStatus(cc.app.DB(), status)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"plan":    plan,
		"output":  string(out),
		"status":  status,
		"options": opts,
		"message": "VNC/noVNC packages installed (services not started)",
	}))
}

func (cc *controller) StreamSetupAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	plan := DetectHost()
	script, err := BuildSetupScript(plan)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("UNSUPPORTED_OS"))
	}

	jobID := uuid.New().String()
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job := &setupJob{
		ID:        jobID,
		Cancel:    cancel,
		CreatedAt: time.Now(),
	}
	registerJob(job)
	BindHTTPJob(jobID, false)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

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

		queue := swinstall.ActiveQueue(cc.app.DB())
		if !writeEvent(streamEvent{
			Type:    "start",
			JobID:   jobID,
			Message: "Waiting for software install queue before VNC/noVNC setup",
			Plan:    plan,
			Queue:   queue,
		}) {
			return
		}
		AppendHTTPLog(jobID, "system", "Waiting for software install queue before VNC/noVNC setup")
		SetHTTPJobPhase(jobID, "waiting_queue", "Waiting for software install queue…")

		if err := waitForSoftwareQueue(runCtx, cc.app.DB(), func(line string) {
			AppendHTTPLog(jobID, "system", line)
			SetHTTPJobPhase(jobID, "waiting_queue", line)
			_ = writeEvent(streamEvent{
				Type:    "log",
				JobID:   jobID,
				Stream:  "system",
				Line:    line,
				Queue:   swinstall.ActiveQueue(cc.app.DB()),
				Message: line,
			})
		}); err != nil {
			msg := err.Error()
			if runCtx.Err() != nil || job.isCancelled() {
				msg = "cancelled while waiting for software queue"
				FinishHTTPJob(cc.app.DB(), jobID, "cancelled", msg)
				_ = writeEvent(streamEvent{Type: "cancelled", JobID: jobID, Message: msg})
				return
			}
			FinishHTTPJob(cc.app.DB(), jobID, "error", msg)
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: msg})
			return
		}

		SetHTTPJobPhase(jobID, "installing", "Starting VNC/noVNC package setup (no service start)")
		startMsg := "Starting VNC/noVNC package setup (no service start)"
		AppendHTTPLog(jobID, "system", startMsg)
		_ = writeEvent(streamEvent{
			Type:    "log",
			JobID:   jobID,
			Stream:  "system",
			Line:    startMsg,
			Message: startMsg,
		})

		cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
		cmd.Env = setupEnv()
		cmd.Dir = "/root"
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			FinishHTTPJob(cc.app.DB(), jobID, "error", err.Error())
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			FinishHTTPJob(cc.app.DB(), jobID, "error", err.Error())
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}
		if err := cmd.Start(); err != nil {
			FinishHTTPJob(cc.app.DB(), jobID, "error", err.Error())
			_ = writeEvent(streamEvent{Type: "error", JobID: jobID, Message: err.Error()})
			return
		}

		var wg sync.WaitGroup
		streamPipe := func(reader io.Reader, stream string) {
			defer wg.Done()
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				AppendHTTPLog(jobID, stream, line)
				if !writeEvent(streamEvent{
					Type:   "log",
					JobID:  jobID,
					Stream: stream,
					Line:   line,
				}) {
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
			FinishHTTPJob(cc.app.DB(), jobID, "cancelled", "Installation stopped")
			_ = writeEvent(streamEvent{
				Type:    "cancelled",
				JobID:   jobID,
				Message: "Installation stopped",
			})
			return
		}

		ok := runErr == nil
		msg := "VNC/noVNC packages installed (services not started)"
		if !ok {
			msg = runErr.Error()
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				msg = "setup timed out"
			}
			FinishHTTPJob(cc.app.DB(), jobID, "error", msg)
		} else {
			FinishHTTPJob(cc.app.DB(), jobID, "success", msg)
		}
		_ = writeEvent(streamEvent{
			Type:    "done",
			JobID:   jobID,
			Success: &ok,
			Message: msg,
			Plan:    CheckStatus(),
		})
	})
}

func (cc *controller) CancelSetupAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	jobID := c.Params("jobId")
	if jobID == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("jobId", "Job id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	if !cancelJob(jobID) {
		return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("setup job not found or already finished")), r.WithStatus(fiber.StatusNotFound))
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"job_id":    jobID,
			"cancelled": true,
		},
		"message": "Setup cancel requested",
	}))
}

func setupEnv() []string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}
	env := os.Environ()
	overrides := map[string]string{
		"HOME":              home,
		"USER":              user,
		"DEBIAN_FRONTEND":   "noninteractive",
		"containerws_setup": "1",
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(env)+len(overrides))
	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if val, replace := overrides[key]; replace {
			out = append(out, key+"="+val)
			seen[key] = true
			continue
		}
		out = append(out, kv)
	}
	for key, val := range overrides {
		if !seen[key] {
			out = append(out, key+"="+val)
		}
	}
	return out
}
