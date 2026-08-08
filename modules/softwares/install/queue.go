package install

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/service"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

// Queue actions from the catalog multi-select bar.
const (
	QueueActionInstall   = "install"
	QueueActionUpdate    = "update"
	QueueActionUninstall = "uninstall"
)

type queueItem struct {
	ID           string     `json:"id"`
	SoftwareID   string     `json:"software_id"`
	SoftwareName string     `json:"software_name"`
	Action       string     `json:"action"`
	Status       string     `json:"status"` // pending | running | success | error | skipped
	JobID        string     `json:"job_id,omitempty"`
	Message      string     `json:"message,omitempty"`
	Icon         string     `json:"icon,omitempty"`
	Image        string     `json:"image,omitempty"`
	Color        string     `json:"color,omitempty"`
	Category     string     `json:"category,omitempty"`
	Version      string     `json:"version,omitempty"`
	VersionID    string     `json:"version_id,omitempty"`
	EnqueuedAt   time.Time  `json:"enqueued_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type queueSnapshot struct {
	Running bool         `json:"running"`
	Pending int          `json:"pending"`
	Items   []*queueItem `json:"items"`
}

var bulkQueue = struct {
	mu      sync.Mutex
	items   []*queueItem
	running bool
	db      *gorm.DB
}{}

type enqueueBody struct {
	Action      string   `json:"action"`
	SoftwareIDs []string `json:"software_ids"`
}

// EnqueueAPI accepts a bulk install/update/uninstall request and processes
// items one-by-one in a background goroutine (no parallel script runs).
func (cc *controller) EnqueueAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()

	var body enqueueBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	action := strings.ToLower(strings.TrimSpace(body.Action))
	switch action {
	case QueueActionInstall, QueueActionUpdate, QueueActionUninstall:
	default:
		return r.Api(c, r.WithError(fmt.Errorf("action must be install, update, or uninstall")), r.WithStatus(fiber.StatusBadRequest))
	}

	ids := uniqueNonEmpty(body.SoftwareIDs)
	if len(ids) == 0 {
		return r.Api(c, r.WithError(errors.New("software_ids required")), r.WithStatus(fiber.StatusBadRequest))
	}

	now := time.Now()
	added := make([]*queueItem, 0, len(ids))
	for _, id := range ids {
		item := &queueItem{
			ID:         uuid.New().String(),
			SoftwareID: id,
			Action:     action,
			Status:     "pending",
			EnqueuedAt: now,
		}
		enrichQueueItemFromSoftware(db, item)
		added = append(added, item)
	}

	bulkQueue.mu.Lock()
	bulkQueue.db = db
	bulkQueue.items = append(bulkQueue.items, added...)
	// Keep a bounded history so GET /queue stays useful.
	if len(bulkQueue.items) > 200 {
		bulkQueue.items = bulkQueue.items[len(bulkQueue.items)-200:]
	}
	bulkQueue.mu.Unlock()

	kickQueueWorker()

	return r.Api(c, r.WithStatus(fiber.StatusAccepted), r.WithData(fiber.Map{
		"data": fiber.Map{
			"queued": len(added),
			"action": action,
			"items":  added,
			"queue":  snapshotActiveQueue(db),
		},
		"message": fmt.Sprintf("Queued %d %s job(s)", len(added), action),
	}))
}

// EnqueueMissingInstalls queues softwaresync-detected OS-missing installs.
// Skips software already pending/running in the queue. PreferVersionID is used
// when set so the exact catalog version is restored.
// Returns how many items were queued.
func EnqueueMissingInstalls(db *gorm.DB, items []softwaresync.MissingInstall) int {
	if db == nil || len(items) == 0 {
		return 0
	}

	now := time.Now()

	bulkQueue.mu.Lock()
	pendingOrRunning := make(map[string]struct{})
	for _, it := range bulkQueue.items {
		if it.Status == "pending" || it.Status == "running" {
			pendingOrRunning[it.SoftwareID] = struct{}{}
		}
	}
	bulkQueue.mu.Unlock()

	candidates := make([]*queueItem, 0, len(items))
	for _, m := range items {
		softwareID := strings.TrimSpace(m.SoftwareID)
		if softwareID == "" {
			continue
		}
		if models.IsSoftwareUninstalled(db, softwareID) {
			continue
		}
		if _, busy := pendingOrRunning[softwareID]; busy {
			continue
		}
		if activeJobForSoftware(softwareID) != nil {
			continue
		}
		item := &queueItem{
			ID:         uuid.New().String(),
			SoftwareID: softwareID,
			VersionID:  strings.TrimSpace(m.VersionID),
			Action:     QueueActionInstall,
			Status:     "pending",
			Message:    "Queued — missing on OS",
			EnqueuedAt: now,
		}
		candidates = append(candidates, item)
		pendingOrRunning[softwareID] = struct{}{}
	}
	if len(candidates) == 0 {
		return 0
	}

	// Enrich metadata in parallel — independent DB lookups per item.
	var enrichWG sync.WaitGroup
	for _, item := range candidates {
		enrichWG.Add(1)
		go func(item *queueItem) {
			defer enrichWG.Done()
			enrichQueueItemFromSoftware(db, item)
		}(item)
	}
	enrichWG.Wait()

	bulkQueue.mu.Lock()
	bulkQueue.db = db
	bulkQueue.items = append(bulkQueue.items, candidates...)
	if len(bulkQueue.items) > 200 {
		bulkQueue.items = bulkQueue.items[len(bulkQueue.items)-200:]
	}
	bulkQueue.mu.Unlock()

	kickQueueWorker()
	return len(candidates)
}

// GetQueueAPI returns the in-memory bulk queue snapshot (active installs only).
func (cc *controller) GetQueueAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": snapshotActiveQueue(cc.app.DB()),
	}))
}

type retryBody struct {
	ID         string `json:"id"`
	SoftwareID string `json:"software_id"`
	Action     string `json:"action"`
}

// RetryQueueAPI re-queues a failed/skipped item (or a software id) at the end of the line.
func (cc *controller) RetryQueueAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()

	var body retryBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	action := strings.ToLower(strings.TrimSpace(body.Action))
	if action == "" {
		action = QueueActionInstall
	}
	switch action {
	case QueueActionInstall, QueueActionUpdate, QueueActionUninstall:
	default:
		return r.Api(c, r.WithError(fmt.Errorf("invalid action")), r.WithStatus(fiber.StatusBadRequest))
	}

	softwareID := strings.TrimSpace(body.SoftwareID)
	itemID := strings.TrimSpace(body.ID)

	if itemID != "" {
		bulkQueue.mu.Lock()
		var found *queueItem
		for _, it := range bulkQueue.items {
			if it.ID == itemID {
				found = it
				break
			}
		}
		if found != nil {
			softwareID = found.SoftwareID
			if found.Action != "" {
				action = found.Action
			}
			// Drop the failed row; a fresh pending item is enqueued below.
			next := make([]*queueItem, 0, len(bulkQueue.items))
			for _, it := range bulkQueue.items {
				if it.ID != itemID {
					next = append(next, it)
				}
			}
			bulkQueue.items = next
		}
		bulkQueue.mu.Unlock()
	}

	if softwareID == "" {
		return r.Api(c, r.WithError(errors.New("id or software_id required")), r.WithStatus(fiber.StatusBadRequest))
	}

	item := &queueItem{
		ID:         uuid.New().String(),
		SoftwareID: softwareID,
		Action:     action,
		Status:     "pending",
		EnqueuedAt: time.Now(),
		Message:    "Retry queued",
	}
	enrichQueueItemFromSoftware(db, item)

	bulkQueue.mu.Lock()
	bulkQueue.db = db
	bulkQueue.items = append(bulkQueue.items, item)
	bulkQueue.mu.Unlock()
	kickQueueWorker()

	return r.Api(c, r.WithStatus(fiber.StatusAccepted), r.WithData(fiber.Map{
		"data": fiber.Map{
			"item":  item,
			"queue": snapshotActiveQueue(db),
		},
		"message": fmt.Sprintf("Retry queued for %s", item.SoftwareName),
	}))
}

func enrichQueueItemFromSoftware(db *gorm.DB, item *queueItem) {
	if db == nil || item == nil || item.SoftwareID == "" {
		return
	}
	sw, err := gorm.G[models.Software](db).Where("id = ?", item.SoftwareID).First(context.Background())
	if err != nil {
		if item.SoftwareName == "" {
			item.SoftwareName = item.SoftwareID
		}
		return
	}
	item.SoftwareName = sw.Name
	item.Icon = sw.Icon
	item.Image = sw.Image
	item.Color = sw.Color
	item.Category = sw.Category

	versions, err := gorm.G[models.SoftwareVersion](db).
		Where("software_id = ?", sw.ID).
		Order("is_latest DESC, created_at DESC").
		Find(context.Background())
	if err == nil && len(versions) > 0 {
		item.Version = versions[0].Version
	}
}

func uniqueNonEmpty(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func snapshotQueue() queueSnapshot {
	bulkQueue.mu.Lock()
	defer bulkQueue.mu.Unlock()
	items := make([]*queueItem, len(bulkQueue.items))
	copy(items, bulkQueue.items)
	pending := 0
	for _, it := range items {
		if it.Status == "pending" || it.Status == "running" {
			pending++
		}
	}
	return queueSnapshot{
		Running: bulkQueue.running,
		Pending: pending,
		Items:   items,
	}
}

// snapshotActiveQueue returns installs still in flight or failed (successes are pruned).
// Also merges standalone in-memory jobs (single-page installs) not already in the queue.
func snapshotActiveQueue(db *gorm.DB) queueSnapshot {
	base := snapshotQueue()
	active := make([]*queueItem, 0, len(base.Items))
	seenSoftware := make(map[string]struct{})

	for _, it := range base.Items {
		switch it.Status {
		case "pending", "running", "error":
			active = append(active, it)
			seenSoftware[it.SoftwareID] = struct{}{}
		}
	}

	for _, snap := range listVisibleJobs() {
		if _, ok := seenSoftware[snap.SoftwareID]; ok {
			continue
		}
		if snap.Status != "running" && snap.Status != "error" {
			continue
		}
		item := &queueItem{
			ID:           "job-" + snap.ID,
			SoftwareID:   snap.SoftwareID,
			SoftwareName: snap.SoftwareName,
			Action:       QueueActionInstall,
			Status:       snap.Status,
			JobID:        snap.ID,
			Message:      snap.Message,
			Version:      snap.Version,
			EnqueuedAt:   snap.StartedAt,
			FinishedAt:   snap.FinishedAt,
		}
		if snap.FailureReason != "" && snap.Status == "error" {
			item.Message = snap.FailureReason
		}
		enrichQueueItemFromSoftware(db, item)
		active = append(active, item)
		seenSoftware[snap.SoftwareID] = struct{}{}
	}

	pending := 0
	for _, it := range active {
		if it.Status == "pending" || it.Status == "running" {
			pending++
		}
	}
	return queueSnapshot{
		Running: base.Running || pending > 0,
		Pending: pending,
		Items:   active,
	}
}

func kickQueueWorker() {
	bulkQueue.mu.Lock()
	if bulkQueue.running {
		bulkQueue.mu.Unlock()
		return
	}
	db := bulkQueue.db
	bulkQueue.running = true
	bulkQueue.mu.Unlock()
	if db == nil {
		bulkQueue.mu.Lock()
		bulkQueue.running = false
		bulkQueue.mu.Unlock()
		return
	}

	go func(db *gorm.DB) {
		defer func() {
			bulkQueue.mu.Lock()
			hasPending := false
			for _, it := range bulkQueue.items {
				if it.Status == "pending" {
					hasPending = true
					break
				}
			}
			bulkQueue.running = false
			bulkQueue.mu.Unlock()
			if hasPending {
				kickQueueWorker()
			}
		}()

		for {
			// Yield the package manager while VNC/noVNC setup scripts are running.
			for packageManagerBusy() {
				time.Sleep(time.Second)
			}
			item := claimNextQueueItem()
			if item == nil {
				return
			}
			runQueueItem(db, item)
		}
	}(db)
}

func claimNextQueueItem() *queueItem {
	bulkQueue.mu.Lock()
	defer bulkQueue.mu.Unlock()
	for _, it := range bulkQueue.items {
		if it.Status == "pending" {
			it.Status = "running"
			it.Message = "Starting…"
			return it
		}
	}
	return nil
}

func finishQueueItem(item *queueItem, status, message, jobID string) {
	now := time.Now()
	bulkQueue.mu.Lock()
	defer bulkQueue.mu.Unlock()
	if jobID != "" {
		item.JobID = jobID
	}
	item.Message = message
	item.FinishedAt = &now

	// Successful / skipped installs leave the Installing list immediately.
	if status == "success" || status == "skipped" {
		next := make([]*queueItem, 0, len(bulkQueue.items))
		for _, it := range bulkQueue.items {
			if it.ID != item.ID {
				next = append(next, it)
			}
		}
		bulkQueue.items = next
		return
	}

	item.Status = status
}

func runQueueItem(db *gorm.DB, item *queueItem) {
	switch item.Action {
	case QueueActionUninstall:
		runQueuedUninstall(db, item)
	default:
		runQueuedInstall(db, item)
	}
}

func runQueuedInstall(db *gorm.DB, item *queueItem) {
	if active := activeJobForSoftware(item.SoftwareID); active != nil {
		finishQueueItem(item, "skipped", "Install already running for this software", active.ID)
		return
	}

	job, script, err := prepareInstallJob(db, item.SoftwareID, item.VersionID)
	if err != nil {
		finishQueueItem(item, "error", err.Error(), "")
		return
	}

	bulkQueue.mu.Lock()
	item.JobID = job.ID
	item.SoftwareName = job.SoftwareName
	item.Version = job.Version
	item.VersionID = job.VersionID
	item.Message = fmt.Sprintf("Installing %s v%s", job.SoftwareName, job.Version)
	bulkQueue.mu.Unlock()
	enrichQueueItemFromSoftware(db, item)

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job.Cancel = cancel
	// Run synchronously so the next queue item waits — no interrupts / overlap.
	runInstallJob(db, job, runCtx, cancel, script)

	snap := job.snapshot()
	finishQueueItem(item, snap.Status, snap.Message, job.ID)
}

func runQueuedUninstall(db *gorm.DB, item *queueItem) {
	if active := activeJobForSoftware(item.SoftwareID); active != nil {
		finishQueueItem(item, "skipped", "Another job is already running for this software", active.ID)
		return
	}

	job, script, err := prepareUninstallJob(db, item.SoftwareID)
	if err != nil {
		finishQueueItem(item, "error", err.Error(), "")
		return
	}

	// Best-effort stop managed units before the uninstall script runs.
	if sw, err := gorm.G[models.Software](db).Where("id = ?", item.SoftwareID).First(context.Background()); err == nil {
		units := []string(sw.ServiceUnits)
		if len(units) > 0 {
			_, _ = service.ControlUnits("stop", units)
		}
	}

	bulkQueue.mu.Lock()
	item.JobID = job.ID
	item.SoftwareName = job.SoftwareName
	item.Version = job.Version
	item.Message = fmt.Sprintf("Uninstalling %s v%s", job.SoftwareName, job.Version)
	bulkQueue.mu.Unlock()
	enrichQueueItemFromSoftware(db, item)

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job.Cancel = cancel
	runUninstallJob(db, job, runCtx, cancel, script)

	snap := job.snapshot()
	finishQueueItem(item, snap.Status, snap.Message, job.ID)
}

// prepareInstallJob creates a registered running install job for softwareID.
// When preferVersionID is set and has an install script, that version is used;
// otherwise the best host-matching latest version is chosen.
func prepareInstallJob(db *gorm.DB, softwareID, preferVersionID string) (*installJob, string, error) {
	ctx := context.Background()
	sw, err := gorm.G[models.Software](db).Where("id = ?", softwareID).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", errors.New("software not found")
		}
		return nil, "", err
	}

	var target *models.SoftwareVersion
	if preferVersionID != "" {
		var preferred models.SoftwareVersion
		if err := db.WithContext(ctx).
			Where("id = ? AND software_id = ?", preferVersionID, sw.ID).
			First(&preferred).Error; err == nil && strings.TrimSpace(preferred.InstallScript) != "" {
			target = &preferred
		}
	}
	if target == nil {
		versions, err := gorm.G[models.SoftwareVersion](db).
			Where("software_id = ?", sw.ID).
			Order("is_latest DESC, created_at DESC").
			Find(ctx)
		if err != nil {
			return nil, "", err
		}
		target = pickBestVersion(versions, true)
	}
	if target == nil {
		return nil, "", errors.New("no version available to install")
	}
	if strings.TrimSpace(target.InstallScript) == "" {
		return nil, "", errors.New("selected version has no install script")
	}

	jobID := uuid.New().String()
	job := &installJob{
		ID:           jobID,
		SoftwareID:   sw.ID,
		SoftwareName: sw.Name,
		VersionID:    target.ID,
		Version:      target.Version,
		CreatedAt:    time.Now(),
		status:       "running",
		subscribers:  make(map[chan streamEvent]struct{}),
	}
	registerJob(job)

	_ = models.UpsertSoftwareInstallJob(db, &models.SoftwareInstallJob{
		ID:           job.ID,
		SoftwareID:   job.SoftwareID,
		VersionID:    job.VersionID,
		VersionLabel: job.Version,
		Status:       "running",
		Message:      fmt.Sprintf("Installing %s v%s", sw.Name, target.Version),
		LogJSON:      "[]",
		StartedAt:    job.CreatedAt,
	})

	return job, target.InstallScript, nil
}

// prepareUninstallJob creates a job that runs the installed version's uninstall script.
func prepareUninstallJob(db *gorm.DB, softwareID string) (*installJob, string, error) {
	ctx := context.Background()
	sw, err := gorm.G[models.Software](db).Where("id = ?", softwareID).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", errors.New("software not found")
		}
		return nil, "", err
	}

	installed, err := models.GetSoftwareInstalled(db, sw.ID)
	if err != nil {
		return nil, "", err
	}
	if installed == nil {
		return nil, "", errors.New("software is not installed")
	}
	if installed.Uninstalled {
		return nil, "", errors.New("software is already uninstalled")
	}

	var ver models.SoftwareVersion
	if err := db.WithContext(ctx).Where("id = ?", installed.VersionID).First(&ver).Error; err != nil {
		// Fall back to best host-matching version that has an uninstall script.
		versions, verr := gorm.G[models.SoftwareVersion](db).
			Where("software_id = ?", sw.ID).
			Order("is_latest DESC, created_at DESC").
			Find(ctx)
		if verr != nil || len(versions) == 0 {
			return nil, "", errors.New("installed version not found")
		}
		best := pickBestVersion(versions, true)
		if best == nil || strings.TrimSpace(best.UninstallScript) == "" {
			return nil, "", errors.New("no uninstall script available for this software")
		}
		ver = *best
	}

	script := strings.TrimSpace(ver.UninstallScript)
	if script == "" {
		// Try any host-matching version with an uninstall script.
		versions, _ := gorm.G[models.SoftwareVersion](db).
			Where("software_id = ? AND uninstall_script <> '' AND uninstall_script IS NOT NULL", sw.ID).
			Order("is_latest DESC, created_at DESC").
			Find(ctx)
		best := pickBestVersion(versions, true)
		if best == nil || strings.TrimSpace(best.UninstallScript) == "" {
			return nil, "", errors.New("no uninstall script available for this software — full remove is not configured")
		}
		ver = *best
		script = strings.TrimSpace(ver.UninstallScript)
	}

	jobID := uuid.New().String()
	job := &installJob{
		ID:           jobID,
		SoftwareID:   sw.ID,
		SoftwareName: sw.Name,
		VersionID:    ver.ID,
		Version:      ver.Version,
		CreatedAt:    time.Now(),
		status:       "running",
		subscribers:  make(map[chan streamEvent]struct{}),
	}
	registerJob(job)

	_ = models.UpsertSoftwareInstallJob(db, &models.SoftwareInstallJob{
		ID:           job.ID,
		SoftwareID:   job.SoftwareID,
		VersionID:    job.VersionID,
		VersionLabel: job.Version,
		Status:       "running",
		Message:      fmt.Sprintf("Uninstalling %s v%s", sw.Name, ver.Version),
		LogJSON:      "[]",
		StartedAt:    job.CreatedAt,
	})

	return job, script, nil
}
