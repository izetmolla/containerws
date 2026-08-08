package install

import (
	"encoding/json"
	"sync"
	"time"
)

type jobLogLine struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
	At     int64  `json:"at"`
}

type jobSnapshot struct {
	ID            string       `json:"id"`
	SoftwareID    string       `json:"software_id"`
	SoftwareName  string       `json:"software_name"`
	VersionID     string       `json:"version_id"`
	Version       string       `json:"version"`
	Status        string       `json:"status"`
	Message       string       `json:"message"`
	FailureReason string       `json:"failure_reason,omitempty"`
	ExitCode      *int         `json:"exit_code,omitempty"`
	Lines         []jobLogLine `json:"lines"`
	StartedAt     time.Time    `json:"started_at"`
	FinishedAt    *time.Time   `json:"finished_at,omitempty"`
}

type installJob struct {
	ID           string
	SoftwareID   string
	SoftwareName string
	VersionID    string
	Version      string
	Cancel       func()
	CreatedAt    time.Time

	mu            sync.Mutex
	cancelled     bool
	done          bool
	status        string // running | success | error | cancelled
	message       string
	failureReason string
	exitCode      *int
	finishedAt    *time.Time
	lines         []jobLogLine
	subscribers   map[chan streamEvent]struct{}
}

var jobs = struct {
	mu         sync.Mutex
	byID       map[string]*installJob
	bySoftware map[string]string // softwareID → active/latest job id
}{
	byID:       make(map[string]*installJob),
	bySoftware: make(map[string]string),
}

// installSerial ensures only one install/uninstall script runs at a time.
var installSerial sync.Mutex

func registerJob(job *installJob) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if job.subscribers == nil {
		job.subscribers = make(map[chan streamEvent]struct{})
	}
	if job.status == "" {
		job.status = "running"
	}
	jobs.byID[job.ID] = job
	jobs.bySoftware[job.SoftwareID] = job.ID
}

func getJob(id string) *installJob {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	return jobs.byID[id]
}

func getLatestJobForSoftware(softwareID string) *installJob {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	id := jobs.bySoftware[softwareID]
	if id == "" {
		return nil
	}
	return jobs.byID[id]
}

func listVisibleJobs() []jobSnapshot {
	jobs.mu.Lock()
	list := make([]*installJob, 0, len(jobs.byID))
	for _, j := range jobs.byID {
		list = append(list, j)
	}
	jobs.mu.Unlock()

	out := make([]jobSnapshot, 0, len(list))
	for _, j := range list {
		snap := j.snapshot()
		if snap.Status == "running" || snap.Status == "error" {
			out = append(out, snap)
		}
	}
	return out
}

func activeJobForSoftware(softwareID string) *installJob {
	j := getLatestJobForSoftware(softwareID)
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status == "running" {
		return j
	}
	return nil
}

func (j *installJob) isCancelled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled
}

func (j *installJob) snapshot() jobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	lines := make([]jobLogLine, len(j.lines))
	copy(lines, j.lines)
	var finished *time.Time
	if j.finishedAt != nil {
		t := *j.finishedAt
		finished = &t
	}
	var exitCode *int
	if j.exitCode != nil {
		v := *j.exitCode
		exitCode = &v
	}
	return jobSnapshot{
		ID:            j.ID,
		SoftwareID:    j.SoftwareID,
		SoftwareName:  j.SoftwareName,
		VersionID:     j.VersionID,
		Version:       j.Version,
		Status:        j.status,
		Message:       j.message,
		FailureReason: j.failureReason,
		ExitCode:      exitCode,
		Lines:         lines,
		StartedAt:     j.CreatedAt,
		FinishedAt:    finished,
	}
}

func (j *installJob) logJSON() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	b, err := json.Marshal(j.lines)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (j *installJob) appendLine(stream, text string) {
	line := jobLogLine{
		Stream: stream,
		Text:   text,
		At:     time.Now().UnixMilli(),
	}
	j.mu.Lock()
	j.lines = append(j.lines, line)
	subs := make([]chan streamEvent, 0, len(j.subscribers))
	for ch := range j.subscribers {
		subs = append(subs, ch)
	}
	j.mu.Unlock()

	ev := streamEvent{
		Type:   "log",
		JobID:  j.ID,
		Stream: stream,
		Line:   text,
	}
	if stream == "system" {
		ev.Type = "system"
	}
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// slow subscriber — drop rather than block the install
		}
	}
}

func (j *installJob) publish(ev streamEvent) {
	j.mu.Lock()
	subs := make([]chan streamEvent, 0, len(j.subscribers))
	for ch := range j.subscribers {
		subs = append(subs, ch)
	}
	j.mu.Unlock()
	ev.JobID = j.ID
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (j *installJob) subscribe(buffer int) chan streamEvent {
	if buffer < 64 {
		buffer = 256
	}
	ch := make(chan streamEvent, buffer)
	j.mu.Lock()
	if j.subscribers == nil {
		j.subscribers = make(map[chan streamEvent]struct{})
	}
	j.subscribers[ch] = struct{}{}
	j.mu.Unlock()
	return ch
}

func (j *installJob) unsubscribe(ch chan streamEvent) {
	j.mu.Lock()
	delete(j.subscribers, ch)
	j.mu.Unlock()
	// drain
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func (j *installJob) finish(status, message, failureReason string, exitCode *int) {
	now := time.Now()
	j.mu.Lock()
	j.done = true
	j.status = status
	j.message = message
	j.failureReason = failureReason
	j.exitCode = exitCode
	j.finishedAt = &now
	subs := make([]chan streamEvent, 0, len(j.subscribers))
	for ch := range j.subscribers {
		subs = append(subs, ch)
	}
	j.mu.Unlock()

	ok := status == "success"
	evType := "done"
	if status == "cancelled" {
		evType = "cancelled"
	}
	success := ok
	ev := streamEvent{
		Type:    evType,
		JobID:   j.ID,
		Message: message,
		Success: &success,
		Version: j.Version,
		Name:    j.SoftwareName,
	}
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
	if status == "error" && failureReason != "" {
		errEv := streamEvent{
			Type:    "error",
			JobID:   j.ID,
			Message: failureReason,
		}
		for _, ch := range subs {
			select {
			case ch <- errEv:
			default:
			}
		}
	}

	// Close subscriber channels after a short delay so trailing error events flush.
	go func() {
		time.Sleep(50 * time.Millisecond)
		j.mu.Lock()
		for ch := range j.subscribers {
			close(ch)
			delete(j.subscribers, ch)
		}
		j.mu.Unlock()
	}()
}

func finishJob(id string) {
	// Keep finished jobs in memory for reconnect; prune later.
	go func() {
		time.Sleep(2 * time.Hour)
		jobs.mu.Lock()
		defer jobs.mu.Unlock()
		if j, ok := jobs.byID[id]; ok {
			j.mu.Lock()
			done := j.done
			j.mu.Unlock()
			if done {
				delete(jobs.byID, id)
			}
		}
	}()
}

func cancelJob(id string) bool {
	jobs.mu.Lock()
	j, ok := jobs.byID[id]
	jobs.mu.Unlock()
	if !ok || j == nil {
		return false
	}

	j.mu.Lock()
	if j.done {
		j.mu.Unlock()
		return false
	}
	j.cancelled = true
	cancel := j.Cancel
	j.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return true
}
