package setup

import (
	"sync"
	"time"
)

type setupJob struct {
	ID        string
	Cancel    func()
	CreatedAt time.Time

	mu        sync.Mutex
	cancelled bool
	done      bool
}

var jobs = struct {
	mu   sync.Mutex
	byID map[string]*setupJob
}{
	byID: make(map[string]*setupJob),
}

func registerJob(job *setupJob) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.byID[job.ID] = job
}

func finishJob(id string) {
	jobs.mu.Lock()
	if j, ok := jobs.byID[id]; ok {
		j.mu.Lock()
		j.done = true
		j.mu.Unlock()
	}
	jobs.mu.Unlock()
	go func() {
		time.Sleep(5 * time.Minute)
		jobs.mu.Lock()
		defer jobs.mu.Unlock()
		delete(jobs.byID, id)
	}()
}

func (j *setupJob) isCancelled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled
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
