package install

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BrewActionRunner runs one brew install/upgrade/uninstall synchronously.
// Wired from modules bootstrap to avoid an import cycle with modules/brew.
type BrewActionRunner func(action, kind string, names []string) (jobID, message string, err error)

var brewActionRunner BrewActionRunner

// SetBrewActionRunner registers the brew runner used by the softwares queue.
func SetBrewActionRunner(fn BrewActionRunner) {
	brewActionRunner = fn
}

// EnqueueBrewActions queues brew formula/cask actions onto the softwares install
// queue so they serialize with Softwares installs (no overlapping package jobs).
// action is install | upgrade | uninstall (upgrade is stored as "update").
func EnqueueBrewActions(db *gorm.DB, action string, names []string, kind string) (int, queueSnapshot, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "install", "upgrade", "uninstall":
	default:
		return 0, queueSnapshot{}, fmt.Errorf("unsupported brew action %q", action)
	}
	queueAction := action
	if action == "upgrade" {
		queueAction = QueueActionUpdate
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "cask" {
		kind = "formula"
	}

	clean := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		clean = append(clean, n)
	}
	if len(clean) == 0 {
		return 0, queueSnapshot{}, fmt.Errorf("at least one package name is required")
	}

	now := time.Now()
	added := make([]*queueItem, 0, len(clean))

	bulkQueue.mu.Lock()
	busy := map[string]struct{}{}
	for _, it := range bulkQueue.items {
		if it == nil {
			continue
		}
		if it.Status != "pending" && it.Status != "running" {
			continue
		}
		key := strings.ToLower(it.BrewName)
		if key == "" {
			key = strings.ToLower(it.SoftwareID)
		}
		if key != "" {
			busy[key] = struct{}{}
		}
	}
	bulkQueue.mu.Unlock()

	for _, name := range clean {
		if _, ok := busy[name]; ok {
			continue
		}
		label := "Install"
		switch queueAction {
		case QueueActionUpdate:
			label = "Update"
		case QueueActionUninstall:
			label = "Uninstall"
		}
		item := &queueItem{
			ID:           uuid.New().String(),
			SoftwareID:   "brew:" + name,
			SoftwareName: name,
			Action:       queueAction,
			Status:       "pending",
			EnqueuedAt:   now,
			Message:      fmt.Sprintf("%s queued (Brew)", label),
			Source:       "brew",
			BrewName:     name,
			BrewKind:     kind,
			Href:         "/brew/" + name + "?kind=" + kind,
			Category:     "Brew",
			Color:        "#FBBF24",
		}
		added = append(added, item)
		busy[name] = struct{}{}
	}
	if len(added) == 0 {
		return 0, snapshotActiveQueue(db), fmt.Errorf("already queued or running")
	}

	bulkQueue.mu.Lock()
	bulkQueue.db = db
	bulkQueue.items = append(bulkQueue.items, added...)
	if len(bulkQueue.items) > 200 {
		bulkQueue.items = bulkQueue.items[len(bulkQueue.items)-200:]
	}
	bulkQueue.mu.Unlock()
	kickQueueWorker()

	return len(added), snapshotActiveQueue(db), nil
}

func runQueuedBrew(db *gorm.DB, item *queueItem) {
	if brewActionRunner == nil {
		finishQueueItem(item, "error", "brew queue runner is not configured", "")
		return
	}
	name := strings.TrimSpace(item.BrewName)
	if name == "" {
		finishQueueItem(item, "error", "missing brew package name", "")
		return
	}
	kind := item.BrewKind
	if kind != "cask" {
		kind = "formula"
	}
	brewAction := item.Action
	switch brewAction {
	case QueueActionUpdate:
		brewAction = "upgrade"
	case QueueActionInstall, QueueActionUninstall:
	default:
		brewAction = "install"
	}

	bulkQueue.mu.Lock()
	item.Message = fmt.Sprintf("Running brew %s %s…", brewAction, name)
	bulkQueue.mu.Unlock()

	jobID, message, err := brewActionRunner(brewAction, kind, []string{name})
	if err != nil {
		msg := err.Error()
		if message != "" {
			msg = message
		}
		finishQueueItem(item, "error", msg, jobID)
		return
	}
	if message == "" {
		message = fmt.Sprintf("brew %s %s completed", brewAction, name)
	}
	finishQueueItem(item, "success", message, jobID)
	_ = db // ownership is applied inside the brew runner
}
