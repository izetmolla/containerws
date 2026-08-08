package install

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// QueueViewItem is a public snapshot of one softwares queue / job row.
type QueueViewItem struct {
	ID           string     `json:"id"`
	SoftwareID   string     `json:"software_id"`
	SoftwareName string     `json:"software_name"`
	Action       string     `json:"action"`
	Status       string     `json:"status"`
	JobID        string     `json:"job_id,omitempty"`
	Message      string     `json:"message,omitempty"`
	Icon         string     `json:"icon,omitempty"`
	Image        string     `json:"image,omitempty"`
	Color        string     `json:"color,omitempty"`
	Category     string     `json:"category,omitempty"`
	Version      string     `json:"version,omitempty"`
	EnqueuedAt   time.Time  `json:"enqueued_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// QueueView is a public softwares install queue snapshot.
type QueueView struct {
	Running bool            `json:"running"`
	Pending int             `json:"pending"`
	Items   []QueueViewItem `json:"items"`
}

// packageBusyFn reports whether another host package job (e.g. VNC setup) holds the package manager.
var packageBusyFn func() bool

// SetPackageManagerBusyCheck registers a cross-module gate so the softwares queue
// waits while VNC/noVNC (or similar) package scripts are running.
func SetPackageManagerBusyCheck(fn func() bool) {
	packageBusyFn = fn
}

func packageManagerBusy() bool {
	if packageBusyFn == nil {
		return false
	}
	return packageBusyFn()
}

// ActiveQueue returns the active softwares install queue (pending/running/error).
func ActiveQueue(db *gorm.DB) QueueView {
	return toQueueView(snapshotActiveQueue(db))
}

// IsQueueBusy reports whether any softwares install is pending or running.
// Failed (error) items do not count as busy.
func IsQueueBusy(db *gorm.DB) bool {
	snap := snapshotActiveQueue(db)
	return snap.Pending > 0 || snap.Running
}

// WaitQueueIdle blocks until softwares queue has no pending/running work.
// Failed items are allowed to remain. onTick is optional (called each poll, including idle).
func WaitQueueIdle(ctx context.Context, db *gorm.DB, every time.Duration, onTick func(QueueView)) error {
	if every <= 0 {
		every = time.Second
	}
	for {
		view := ActiveQueue(db)
		busy := view.Pending > 0
		if onTick != nil {
			onTick(view)
		}
		if !busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(every):
		}
	}
}

func toQueueView(snap queueSnapshot) QueueView {
	items := make([]QueueViewItem, 0, len(snap.Items))
	for _, it := range snap.Items {
		if it == nil {
			continue
		}
		items = append(items, QueueViewItem{
			ID:           it.ID,
			SoftwareID:   it.SoftwareID,
			SoftwareName: it.SoftwareName,
			Action:       it.Action,
			Status:       it.Status,
			JobID:        it.JobID,
			Message:      it.Message,
			Icon:         it.Icon,
			Image:        it.Image,
			Color:        it.Color,
			Category:     it.Category,
			Version:      it.Version,
			EnqueuedAt:   it.EnqueuedAt,
			FinishedAt:   it.FinishedAt,
		})
	}
	return QueueView{
		Running: snap.Running,
		Pending: snap.Pending,
		Items:   items,
	}
}
