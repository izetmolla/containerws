package softwaresync

import (
	"context"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// probeWorkers caps concurrent host probes during startup reconcile.
const maxProbeWorkers = 8

var (
	startOnce sync.Once

	osMissingMu       sync.RWMutex
	osMissingVersions = map[string]struct{}{} // versionID
	osMissingSoftware = map[string]struct{}{} // softwareID

	enqueueMu   sync.Mutex
	enqueueFunc EnqueueMissingFunc

	readyOnce sync.Once
	readyCh   = make(chan struct{})
)

// MissingInstall is a catalog software whose DB install row is absent on the host.
type MissingInstall struct {
	SoftwareID string
	VersionID  string
}

// EnqueueMissingFunc queues missing softwares for (re)install.
// It should return how many items were actually queued.
type EnqueueMissingFunc func(db *gorm.DB, items []MissingInstall) int

// SetEnqueueMissing registers the install-queue hook used after reconcile.
// Typically wired from modules/softwares/install to avoid an import cycle.
func SetEnqueueMissing(fn EnqueueMissingFunc) {
	enqueueMu.Lock()
	defer enqueueMu.Unlock()
	enqueueFunc = fn
}

// StartAsync scans software_installed once per process: sets OsMissing for
// catalog versions absent on the host, then enqueues them for install.
func StartAsync(db *gorm.DB) {
	if db == nil {
		markReady()
		return
	}
	startOnce.Do(func() {
		go func() {
			defer markReady()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := Reconcile(ctx, db); err != nil {
				log.Printf("softwaresync: reconcile failed: %v", err)
			}
		}()
	})
}

func markReady() {
	readyOnce.Do(func() { close(readyCh) })
}

// Ready reports whether the initial softwaresync reconcile has finished (or was skipped).
func Ready() bool {
	select {
	case <-readyCh:
		return true
	default:
		return false
	}
}

// WaitReady blocks until the initial softwaresync reconcile finishes (or ctx ends).
func WaitReady(ctx context.Context) error {
	select {
	case <-readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsOsMissing reports whether the installed version was probed missing on the host.
func IsOsMissing(versionID string) bool {
	if versionID == "" {
		return false
	}
	osMissingMu.RLock()
	defer osMissingMu.RUnlock()
	_, ok := osMissingVersions[versionID]
	return ok
}

// IsOsMissingSoftware reports whether any installed version of the software is missing on the host.
func IsOsMissingSoftware(softwareID string) bool {
	if softwareID == "" {
		return false
	}
	osMissingMu.RLock()
	defer osMissingMu.RUnlock()
	_, ok := osMissingSoftware[softwareID]
	return ok
}

// MarkOsMissing records that a DB-installed version is absent on the OS.
func MarkOsMissing(softwareID, versionID string) {
	osMissingMu.Lock()
	defer osMissingMu.Unlock()
	if versionID != "" {
		osMissingVersions[versionID] = struct{}{}
	}
	if softwareID != "" {
		osMissingSoftware[softwareID] = struct{}{}
	}
}

// ClearOsMissing clears the OS-missing flag after a successful (re)install.
func ClearOsMissing(softwareID, versionID string) {
	osMissingMu.Lock()
	defer osMissingMu.Unlock()
	if versionID != "" {
		delete(osMissingVersions, versionID)
	}
	if softwareID != "" {
		delete(osMissingSoftware, softwareID)
	}
}

// ApplyOsMissing sets v.OsMissing from the startup probe cache.
func ApplyOsMissing(v *models.SoftwareVersion) {
	if v == nil {
		return
	}
	v.OsMissing = IsOsMissing(v.ID)
}

type probeTarget struct {
	softwareID   string
	name         string
	versionID    string
	version      string
	serviceUnits []string
}

type probeOutcome struct {
	target probeTarget
	probe  ProbeResult
}

func probeWorkerCount(n int) int {
	if n <= 1 {
		return 1
	}
	workers := min(min(max(runtime.GOMAXPROCS(0), 2), maxProbeWorkers), n)
	return workers
}

// Reconcile checks every row in software_installed against the host.
// Host probes run concurrently (bounded worker pool). Present → clear OsMissing.
// Missing → set OsMissing and enqueue install. Prints one summary line only.
func Reconcile(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}

	rows, err := models.ListSoftwareInstalled(db)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		log.Printf("softwaresync: installed=0 missing=0 queued=0")
		return nil
	}

	targets := make([]probeTarget, 0, len(rows))
	for i := range rows {
		row := rows[i]
		if row.Uninstalled {
			// User intentionally removed this software — do not auto-reinstall.
			continue
		}
		if models.NormalizePackageManager(row.PackageManager) == models.PackageManagerBrew {
			// Owned by Homebrew — softwaresync must not fight brew's prefix.
			continue
		}
		if row.Software == nil || !row.Software.IsActive {
			continue
		}
		sw := row.Software

		ver := row.Version
		if ver == nil {
			var loaded models.SoftwareVersion
			if err := db.WithContext(ctx).Where("id = ?", row.VersionID).First(&loaded).Error; err != nil {
				continue
			}
			ver = &loaded
		}

		targets = append(targets, probeTarget{
			softwareID:   sw.ID,
			name:         sw.Name,
			versionID:    ver.ID,
			version:      ver.Version,
			serviceUnits: []string(sw.ServiceUnits),
		})
	}
	if len(targets) == 0 {
		log.Printf("softwaresync: installed=0 missing=0 queued=0")
		return nil
	}

	workers := probeWorkerCount(len(targets))
	jobs := make(chan int)
	outcomes := make([]probeOutcome, len(targets))
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for idx := range jobs {
				if ctx.Err() != nil {
					outcomes[idx] = probeOutcome{
						target: targets[idx],
						probe:  ProbeResult{Present: false, Detail: "cancelled"},
					}
					continue
				}
				t := targets[idx]
				outcomes[idx] = probeOutcome{
					target: t,
					probe:  ProbeInstalled(t.name, t.serviceUnits),
				}
			}
		})
	}
	for i := range targets {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}

	installed := 0
	missing := make([]MissingInstall, 0, len(outcomes))
	for _, out := range outcomes {
		t := out.target
		if out.probe.Present {
			ClearOsMissing(t.softwareID, t.versionID)
			installed++
			continue
		}
		MarkOsMissing(t.softwareID, t.versionID)
		missing = append(missing, MissingInstall{
			SoftwareID: t.softwareID,
			VersionID:  t.versionID,
		})
	}

	queued := 0
	if len(missing) > 0 {
		enqueueMu.Lock()
		fn := enqueueFunc
		enqueueMu.Unlock()
		if fn != nil {
			queued = fn(db, missing)
		}
	}

	log.Printf("softwaresync: installed=%d missing=%d queued=%d", installed, len(missing), queued)
	return nil
}

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
		"HOME":            home,
		"USER":            user,
		"DEBIAN_FRONTEND": "noninteractive",
		"GOCACHE":         home + "/.cache/go-build",
		"GOMODCACHE":      home + "/go/pkg/mod",
		"GOPATH":          home + "/go",
		"GOROOT":          "/usr/local/go",
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
