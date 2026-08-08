package install

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
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

type streamEvent struct {
	Type    string `json:"type"`
	JobID   string `json:"job_id,omitempty"`
	Line    string `json:"line,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Message string `json:"message,omitempty"`
	Success *bool  `json:"success,omitempty"`
	Version string `json:"version,omitempty"`
	Name    string `json:"name,omitempty"`
}

func (cc *controller) StreamInstallAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := c.Params("id")
	if id == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("id", "Software id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	if active := activeJobForSoftware(id); active != nil {
		return attachJobSSE(c, active)
	}

	job, script, err := prepareInstallJob(db, id, "")
	if err != nil {
		status := fiber.StatusBadRequest
		code := "INSTALL_PREPARE_FAILED"
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "software not found":
			status = fiber.StatusNotFound
			code = "NOT_FOUND"
		case err.Error() == "no version available to install":
			code = "NO_VERSION"
		case err.Error() == "latest version has no install script":
			code = "NO_SCRIPT"
		}
		return r.Api(c, r.WithErrorCode(code), r.WithError(err), r.WithStatus(status))
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job.Cancel = cancel
	go runInstallJob(db, job, runCtx, cancel, script)

	return attachJobSSE(c, job)
}

func (cc *controller) StreamJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	jobID := c.Params("jobId")
	if jobID == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("jobId", "Job id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	job := getJob(jobID)
	if job == nil {
		// Fall back to persisted snapshot as a one-shot replay via SSE.
		row, err := models.GetSoftwareInstallJob(cc.app.DB(), jobID)
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
		if row == nil {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("install job not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return replayPersistedJobSSE(c, row)
	}
	return attachJobSSE(c, job)
}

func (cc *controller) GetLatestJobAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := c.Params("id")
	if id == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("id", "Software id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	if mem := getLatestJobForSoftware(id); mem != nil {
		snap := mem.snapshot()
		return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": snap,
		}))
	}

	row, err := models.LatestSoftwareInstallJob(db, id)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if row == nil {
		return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data": nil,
		}))
	}

	// DB says running but nothing is in memory → worker died (e.g. server restart).
	if row.Status == "running" {
		reason := "Install interrupted (server restarted). Please retry."
		now := time.Now()
		_ = models.UpsertSoftwareInstallJob(db, &models.SoftwareInstallJob{
			ID:            row.ID,
			SoftwareID:    row.SoftwareID,
			VersionID:     row.VersionID,
			VersionLabel:  row.VersionLabel,
			Status:        "error",
			Message:       "Install interrupted",
			FailureReason: reason,
			ExitCode:      row.ExitCode,
			LogJSON:       row.LogJSON,
			StartedAt:     row.StartedAt,
			FinishedAt:    &now,
		})
		row.Status = "error"
		row.Message = "Install interrupted"
		row.FailureReason = reason
		row.FinishedAt = &now
	}

	var lines []jobLogLine
	if row.LogJSON != "" {
		_ = json.Unmarshal([]byte(row.LogJSON), &lines)
	}
	if lines == nil {
		lines = []jobLogLine{}
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": jobSnapshot{
			ID:            row.ID,
			SoftwareID:    row.SoftwareID,
			VersionID:     row.VersionID,
			Version:       row.VersionLabel,
			Status:        row.Status,
			Message:       row.Message,
			FailureReason: row.FailureReason,
			ExitCode:      row.ExitCode,
			Lines:         lines,
			StartedAt:     row.StartedAt,
			FinishedAt:    row.FinishedAt,
		},
	}))
}

func (cc *controller) CancelInstallAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	jobID := c.Params("jobId")
	if jobID == "" {
		return r.Api(c, r.WithErrorData(r.ErrorFields(
			r.ErrorField("jobId", "Job id is required"),
		)), r.WithStatus(fiber.StatusBadRequest))
	}

	if !cancelJob(jobID) {
		return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("install job not found or already finished")), r.WithStatus(fiber.StatusNotFound))
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"job_id":    jobID,
			"cancelled": true,
		},
		"message": "Installation cancel requested",
	}))
}

func attachJobSSE(c fiber.Ctx, job *installJob) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		writeEvent := func(ev streamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}

		// Snapshot + subscribe under the same lock so no lines are missed.
		job.mu.Lock()
		snapLines := make([]jobLogLine, len(job.lines))
		copy(snapLines, job.lines)
		status := job.status
		message := job.message
		failureReason := job.failureReason
		softwareName := job.SoftwareName
		version := job.Version
		jobID := job.ID
		ch := make(chan streamEvent, 512)
		if job.subscribers == nil {
			job.subscribers = make(map[chan streamEvent]struct{})
		}
		job.subscribers[ch] = struct{}{}
		job.mu.Unlock()
		defer job.unsubscribe(ch)

		if !writeEvent(streamEvent{
			Type:    "start",
			JobID:   jobID,
			Name:    softwareName,
			Version: version,
			Message: fmt.Sprintf("Session %s · %s v%s", shortID(jobID), softwareName, version),
		}) {
			return
		}
		for _, line := range snapLines {
			evType := "log"
			if line.Stream == "system" {
				evType = "system"
			}
			if !writeEvent(streamEvent{
				Type:   evType,
				JobID:  jobID,
				Stream: line.Stream,
				Line:   line.Text,
			}) {
				return
			}
		}

		if status != "running" {
			ok := status == "success"
			if status == "cancelled" {
				_ = writeEvent(streamEvent{
					Type:    "cancelled",
					JobID:   jobID,
					Message: message,
				})
				return
			}
			_ = writeEvent(streamEvent{
				Type:    "done",
				JobID:   jobID,
				Success: &ok,
				Message: message,
				Version: version,
				Name:    softwareName,
			})
			if status == "error" && failureReason != "" {
				_ = writeEvent(streamEvent{
					Type:    "error",
					JobID:   jobID,
					Message: failureReason,
				})
			}
			return
		}

		for ev := range ch {
			if !writeEvent(ev) {
				return
			}
			if ev.Type == "cancelled" {
				return
			}
			if ev.Type == "done" && ev.Success != nil && *ev.Success {
				return
			}
			if ev.Type == "error" {
				return
			}
		}
	})
}

func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

func replayPersistedJobSSE(c fiber.Ctx, row *models.SoftwareInstallJob) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	var lines []jobLogLine
	if row.LogJSON != "" {
		_ = json.Unmarshal([]byte(row.LogJSON), &lines)
	}

	return c.SendStreamWriter(func(w *bufio.Writer) {
		writeEvent := func(ev streamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}

		_ = writeEvent(streamEvent{
			Type:    "start",
			JobID:   row.ID,
			Version: row.VersionLabel,
			Message: row.Message,
		})
		for _, line := range lines {
			evType := "log"
			if line.Stream == "system" {
				evType = "system"
			}
			_ = writeEvent(streamEvent{
				Type:   evType,
				JobID:  row.ID,
				Stream: line.Stream,
				Line:   line.Text,
			})
		}
		ok := row.Status == "success"
		if row.Status == "cancelled" {
			_ = writeEvent(streamEvent{
				Type:    "cancelled",
				JobID:   row.ID,
				Message: row.Message,
			})
			return
		}
		if row.Status == "running" {
			// Orphaned persisted job — no live worker to attach to.
			msg := row.FailureReason
			if msg == "" {
				msg = "Install interrupted (server restarted). Please retry."
			}
			_ = writeEvent(streamEvent{
				Type:    "done",
				JobID:   row.ID,
				Success: &ok,
				Message: "Install interrupted",
				Version: row.VersionLabel,
			})
			_ = writeEvent(streamEvent{
				Type:    "error",
				JobID:   row.ID,
				Message: msg,
			})
			return
		}
		_ = writeEvent(streamEvent{
			Type:    "done",
			JobID:   row.ID,
			Success: &ok,
			Message: row.Message,
			Version: row.VersionLabel,
		})
		if row.Status == "error" && row.FailureReason != "" {
			_ = writeEvent(streamEvent{
				Type:    "error",
				JobID:   row.ID,
				Message: row.FailureReason,
			})
		}
	})
}

func runInstallJob(
	db *gorm.DB,
	job *installJob,
	runCtx context.Context,
	cancel context.CancelFunc,
	script string,
) {
	runSoftwareScript(db, job, runCtx, cancel, script, "install")
}

func runUninstallJob(
	db *gorm.DB,
	job *installJob,
	runCtx context.Context,
	cancel context.CancelFunc,
	script string,
) {
	runSoftwareScript(db, job, runCtx, cancel, script, "uninstall")
}

func runSoftwareScript(
	db *gorm.DB,
	job *installJob,
	runCtx context.Context,
	cancel context.CancelFunc,
	script string,
	mode string,
) {
	// One script at a time — queue + single installs never overlap.
	installSerial.Lock()
	defer installSerial.Unlock()

	defer cancel()
	defer finishJob(job.ID)

	persist := func() {
		snap := job.snapshot()
		_ = models.UpsertSoftwareInstallJob(db, &models.SoftwareInstallJob{
			ID:            snap.ID,
			SoftwareID:    snap.SoftwareID,
			VersionID:     snap.VersionID,
			VersionLabel:  snap.Version,
			Status:        snap.Status,
			Message:       snap.Message,
			FailureReason: snap.FailureReason,
			ExitCode:      snap.ExitCode,
			LogJSON:       job.logJSON(),
			StartedAt:     snap.StartedAt,
			FinishedAt:    snap.FinishedAt,
		})
	}
	defer persist()

	// Persist logs periodically so a page refresh mid-install can restore the terminal.
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				job.mu.Lock()
				done := job.done
				job.mu.Unlock()
				if done {
					return
				}
				persist()
			}
		}
	}()

	verb := "install"
	heredoc := "INSTALL"
	if mode == "uninstall" {
		verb = "uninstall"
		heredoc = "UNINSTALL"
	}

	job.publish(streamEvent{
		Type:    "start",
		Name:    job.SoftwareName,
		Version: job.Version,
		Message: fmt.Sprintf("Starting %s of %s v%s", verb, job.SoftwareName, job.Version),
	})
	job.appendLine("system", fmt.Sprintf("Starting %s of %s v%s", verb, job.SoftwareName, job.Version))

	runErr, cancelled, timedOut, stderrTail, aborted := streamBash(job, runCtx, script, heredoc)
	if aborted {
		return
	}
	if cancelled {
		msg := capitalize(verb) + " cancelled"
		job.appendLine("stderr", msg)
		job.finish("cancelled", msg, "", nil)
		return
	}
	if timedOut {
		msg := capitalize(verb) + " timed out after 30 minutes"
		job.appendLine("stderr", msg)
		job.finish("error", msg, msg, nil)
		return
	}

	if runErr == nil {
		if mode == "uninstall" {
			msg := fmt.Sprintf("Uninstalled %s v%s", job.SoftwareName, job.Version)
			if err := models.MarkSoftwareUninstalled(db, job.SoftwareID); err != nil {
				warn := "warning: uninstall script succeeded but failed to mark uninstalled: " + err.Error()
				job.appendLine("system", warn)
			} else {
				softwaresync.ClearOsMissing(job.SoftwareID, job.VersionID)
				CancelPendingInstallsForSoftware(job.SoftwareID)
				clearStickyInstallOptions(db, job.SoftwareName)
			}
			job.appendLine("system", msg)
			job.finish("success", msg, "", nil)
			return
		}
		msg := fmt.Sprintf("Installed %s v%s", job.SoftwareName, job.Version)
		if err := markVersionInstalled(db, job.SoftwareID, job.VersionID); err != nil {
			warn := "warning: install succeeded but failed to persist installed status: " + err.Error()
			job.appendLine("system", warn)
		}
		job.appendLine("system", msg)

		custom := loadCustomScript(db, job.VersionID)
		if custom != "" {
			job.appendLine("system", "Running custom setup script…")
			cErr, cCancelled, cTimedOut, cTail, cAborted := streamBash(job, runCtx, custom, "CUSTOM")
			if cAborted {
				return
			}
			if cCancelled {
				msg := "Custom setup cancelled after install"
				job.appendLine("stderr", msg)
				job.finish("cancelled", msg, "", nil)
				return
			}
			if cTimedOut {
				msg := "Custom setup timed out after install"
				job.appendLine("stderr", msg)
				job.finish("error", msg, msg, nil)
				return
			}
			if cErr != nil {
				var exitCode *int
				if ee, ok := cErr.(*exec.ExitError); ok {
					code := ee.ExitCode()
					exitCode = &code
				}
				reason := buildFailureReason(cErr, exitCode, cTail)
				failMsg := fmt.Sprintf("Custom setup failed for %s v%s (install completed)", job.SoftwareName, job.Version)
				job.appendLine("stderr", failMsg)
				job.appendLine("stderr", reason)
				job.finish("error", failMsg, reason, exitCode)
				return
			}
			job.appendLine("system", "Custom setup completed")
		}

		job.finish("success", msg, "", nil)
		return
	}

	var exitCode *int
	if ee, ok := runErr.(*exec.ExitError); ok {
		code := ee.ExitCode()
		exitCode = &code
	}

	reason := buildFailureReason(runErr, exitCode, stderrTail)
	failMsg := fmt.Sprintf("%s failed for %s v%s", capitalize(verb), job.SoftwareName, job.Version)
	job.appendLine("stderr", failMsg)
	job.appendLine("stderr", reason)
	job.finish("error", failMsg, reason, exitCode)
}

// streamBash runs script under bash -lc, streaming stdout/stderr into the job log.
// aborted=true means the job was already finished due to a start/pipe failure.
func streamBash(
	job *installJob,
	runCtx context.Context,
	script string,
	heredoc string,
) (waitErr error, cancelled, timedOut bool, stderrTail []string, aborted bool) {
	job.appendLine("system", fmt.Sprintf("$ bash -lc <<'%s'", heredoc))
	for line := range strings.SplitSeq(script, "\n") {
		job.appendLine("system", line)
	}
	job.appendLine("system", heredoc)

	cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
	cmd.Env = installEnv()
	cmd.Dir = "/root"
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.appendLine("stderr", err.Error())
		job.finish("error", "Failed to start script", err.Error(), nil)
		return nil, false, false, nil, true
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		job.appendLine("stderr", err.Error())
		job.finish("error", "Failed to start script", err.Error(), nil)
		return nil, false, false, nil, true
	}

	if err := cmd.Start(); err != nil {
		job.appendLine("stderr", err.Error())
		job.finish("error", "Failed to start script", err.Error(), nil)
		return nil, false, false, nil, true
	}

	var (
		wg       sync.WaitGroup
		tail     []string
		stderrMu sync.Mutex
	)
	streamPipe := func(reader io.Reader, stream string) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			text := scanner.Text()
			job.appendLine(stream, text)
			if stream == "stderr" {
				stderrMu.Lock()
				tail = append(tail, text)
				if len(tail) > 40 {
					tail = tail[len(tail)-40:]
				}
				stderrMu.Unlock()
			}
		}
	}

	wg.Add(2)
	go streamPipe(stdout, "stdout")
	go streamPipe(stderr, "stderr")
	wg.Wait()

	waitErr = cmd.Wait()
	if job.isCancelled() || errors.Is(runCtx.Err(), context.Canceled) {
		return waitErr, true, false, tail, false
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return waitErr, false, true, tail, false
	}
	stderrMu.Lock()
	stderrTail = append([]string(nil), tail...)
	stderrMu.Unlock()
	return waitErr, false, false, stderrTail, false
}

func loadCustomScript(db *gorm.DB, versionID string) string {
	if db == nil || strings.TrimSpace(versionID) == "" {
		return ""
	}
	var ver models.SoftwareVersion
	if err := db.Select("custom_script").Where("id = ?", versionID).First(&ver).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(ver.CustomScript)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func buildFailureReason(runErr error, exitCode *int, stderrTail []string) string {
	var b strings.Builder
	b.WriteString(runErr.Error())
	if exitCode != nil {
		b.WriteString(fmt.Sprintf("\nexit code: %d", *exitCode))
	}
	if len(stderrTail) > 0 {
		b.WriteString("\n\n--- last stderr output ---\n")
		b.WriteString(strings.Join(stderrTail, "\n"))
	}
	return b.String()
}

// installEnv ensures HOME/GOCACHE exist for non-interactive systemd jobs
// where those variables are often unset.
func installEnv() []string {
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
		"HOME":       home,
		"USER":       user,
		"GOCACHE":    home + "/.cache/go-build",
		"GOMODCACHE": home + "/go/pkg/mod",
		"GOPATH":     home + "/go",
		"GOROOT":     "/usr/local/go",
	}
	seen := make(map[string]bool, len(overrides))
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
