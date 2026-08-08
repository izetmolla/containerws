package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActiveInstallState is a snapshot of the in-progress (or recently finished) setup job.
type ActiveInstallState struct {
	Active       bool                `json:"active"`
	JobID        string              `json:"job_id,omitempty"`
	Status       string              `json:"status"` // idle|running|success|error|cancelled
	Phase        string              `json:"phase,omitempty"` // waiting_queue | installing
	WaitingQueue bool                `json:"waiting_queue,omitempty"`
	Message      string              `json:"message,omitempty"`
	Auto         bool                `json:"auto"`
	StartedAt    *time.Time          `json:"started_at,omitempty"`
	Lines        []ActiveInstallLine `json:"lines,omitempty"`
	Queue        any                 `json:"software_queue,omitempty"`
}

type ActiveInstallLine struct {
	Text   string `json:"text"`
	Stream string `json:"stream"`
	At     int64  `json:"at"`
}

type activeInstall struct {
	mu           sync.RWMutex
	jobID        string
	status       string
	phase        string // waiting_queue | installing
	waitingQueue bool
	message      string
	auto         bool
	startedAt    time.Time
	lines        []ActiveInstallLine
	cancel       context.CancelFunc
}

var (
	activeMu   sync.Mutex
	activeInst *activeInstall
	startOnce  sync.Once
)

// StartAsync probes packages once per process, syncs Option rows, and auto-reinstalls
// when VNC_INSTALLED is true but packages are missing on the host (softwaresync analog).
func StartAsync(db *gorm.DB) {
	if db == nil {
		return
	}
	startOnce.Do(func() {
		go func() {
			time.Sleep(2 * time.Second) // let migrations / other boot work settle
			status := CheckStatus()
			opts := SyncOptionsFromStatus(db, status)
			log.Printf(
				"vncsync: installed=%v present=%v missing=%v ready=%v",
				opts.Installed, opts.Present, opts.Missing, status.Ready,
			)
			if opts.Missing {
				log.Printf("vncsync: VNC marked installed but packages missing — starting reinstall")
				if err := StartBackgroundInstall(db, true); err != nil {
					log.Printf("vncsync: background install: %v", err)
				}
			}
		}()
	})
}

// GetActiveInstall returns a copy of the current/recent background or UI-started install.
func GetActiveInstall() ActiveInstallState {
	activeMu.Lock()
	inst := activeInst
	activeMu.Unlock()
	if inst == nil {
		return ActiveInstallState{Status: "idle"}
	}
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	started := inst.startedAt
	lines := append([]ActiveInstallLine(nil), inst.lines...)
	return ActiveInstallState{
		Active:       inst.status == "running",
		JobID:        inst.jobID,
		Status:       inst.status,
		Phase:        inst.phase,
		WaitingQueue: inst.waitingQueue,
		Message:      inst.message,
		Auto:         inst.auto,
		StartedAt:    &started,
		Lines:        lines,
	}
}

// StartBackgroundInstall runs package setup off the HTTP request (boot reinstall or API).
func StartBackgroundInstall(db *gorm.DB, auto bool) error {
	activeMu.Lock()
	if activeInst != nil {
		activeInst.mu.RLock()
		running := activeInst.status == "running"
		activeInst.mu.RUnlock()
		if running {
			activeMu.Unlock()
			return fmt.Errorf("a VNC setup job is already running")
		}
	}
	activeMu.Unlock()

	plan := DetectHost()
	script, err := BuildSetupScript(plan)
	if err != nil {
		return err
	}

	jobID := uuid.New().String()
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	inst := &activeInstall{
		jobID:     jobID,
		status:    "running",
		phase:     "waiting_queue",
		waitingQueue: true,
		message:   "Waiting for software install queue…",
		auto:      auto,
		startedAt: time.Now().UTC(),
		lines:     nil,
		cancel:    cancel,
	}
	job := &setupJob{
		ID:        jobID,
		Cancel:    cancel,
		CreatedAt: time.Now(),
	}
	registerJob(job)

	activeMu.Lock()
	activeInst = inst
	activeMu.Unlock()

	inst.append("system", inst.message)

	go func() {
		defer cancel()
		defer finishJob(jobID)

		if err := waitForSoftwareQueue(runCtx, db, func(line string) {
			inst.setMessage(line)
			inst.append("system", line)
		}); err != nil {
			msg := err.Error()
			if runCtx.Err() != nil {
				msg = "cancelled while waiting for software queue"
			}
			inst.finish("error", msg)
			_ = SyncOptionsFromStatus(db, CheckStatus())
			return
		}

		inst.setPhase("installing", "Starting VNC/noVNC package setup")
		inst.append("system", "Starting VNC/noVNC package setup")

		cmd := exec.CommandContext(runCtx, "bash", "-lc", script)
		cmd.Env = setupEnv()
		cmd.Dir = "/root"
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			inst.finish("error", err.Error())
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			inst.finish("error", err.Error())
			return
		}
		if err := cmd.Start(); err != nil {
			inst.finish("error", err.Error())
			return
		}

		var wg sync.WaitGroup
		pipe := func(r io.Reader, stream string) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				inst.append(stream, sc.Text())
			}
		}
		wg.Add(2)
		go pipe(stdout, "stdout")
		go pipe(stderr, "stderr")
		wg.Wait()

		runErr := cmd.Wait()
		if job.isCancelled() {
			inst.finish("cancelled", "Installation stopped")
			_ = SyncOptionsFromStatus(db, CheckStatus())
			return
		}
		if runErr != nil {
			msg := runErr.Error()
			if runCtx.Err() == context.DeadlineExceeded {
				msg = "setup timed out"
			}
			inst.finish("error", msg)
			_ = SyncOptionsFromStatus(db, CheckStatus())
			return
		}
		MarkInstalled(db)
		_ = SyncOptionsFromStatus(db, CheckStatus())
		inst.finish("success", "VNC/noVNC packages installed (services not started)")
	}()

	return nil
}

func (a *activeInstall) setMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.message = message
}

func (a *activeInstall) setPhase(phase, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.phase = phase
	a.waitingQueue = phase == "waiting_queue"
	if message != "" {
		a.message = message
	}
}

func (a *activeInstall) append(stream, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lines = append(a.lines, ActiveInstallLine{
		Text:   text,
		Stream: stream,
		At:     time.Now().UnixMilli(),
	})
	// Cap memory for long installs.
	const maxLines = 4000
	if len(a.lines) > maxLines {
		a.lines = a.lines[len(a.lines)-maxLines:]
	}
}

func (a *activeInstall) finish(status, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
	a.message = message
	a.phase = ""
	a.waitingQueue = false
	a.lines = append(a.lines, ActiveInstallLine{
		Text:   message,
		Stream: "system",
		At:     time.Now().UnixMilli(),
	})
}

// BindHTTPJob mirrors a request-scoped stream job into the active install buffer
// so the settings page can keep showing progress after navigation.
func BindHTTPJob(jobID string, auto bool) {
	activeMu.Lock()
	defer activeMu.Unlock()
	if activeInst != nil {
		activeInst.mu.RLock()
		running := activeInst.status == "running"
		activeInst.mu.RUnlock()
		if running {
			return
		}
	}
	activeInst = &activeInstall{
		jobID:        jobID,
		status:       "running",
		phase:        "waiting_queue",
		waitingQueue: true,
		message:      "Waiting for software install queue…",
		auto:         auto,
		startedAt:    time.Now().UTC(),
	}
}

// AppendHTTPLog adds a line for the UI-driven stream job.
func AppendHTTPLog(jobID, stream, text string) {
	activeMu.Lock()
	inst := activeInst
	activeMu.Unlock()
	if inst == nil {
		return
	}
	inst.mu.Lock()
	same := inst.jobID == jobID
	inst.mu.Unlock()
	if !same {
		return
	}
	inst.append(stream, text)
}

// SetHTTPJobPhase updates phase/message for the UI-driven stream job.
func SetHTTPJobPhase(jobID, phase, message string) {
	activeMu.Lock()
	inst := activeInst
	activeMu.Unlock()
	if inst == nil {
		return
	}
	inst.mu.Lock()
	same := inst.jobID == jobID
	inst.mu.Unlock()
	if !same {
		return
	}
	inst.setPhase(phase, message)
}

// FinishHTTPJob marks the UI-driven stream job complete and syncs options.
func FinishHTTPJob(db *gorm.DB, jobID, status, message string) {
	activeMu.Lock()
	inst := activeInst
	activeMu.Unlock()
	if inst == nil {
		return
	}
	inst.mu.Lock()
	same := inst.jobID == jobID
	inst.mu.Unlock()
	if !same {
		return
	}
	if status == "success" {
		MarkInstalled(db)
	}
	_ = SyncOptionsFromStatus(db, CheckStatus())
	inst.finish(status, message)
}
