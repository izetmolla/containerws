package setup

import (
	"context"
	"fmt"
	"strings"
	"time"

	swinstall "github.com/izetmolla/containerws/modules/softwares/install"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"gorm.io/gorm"
)

// waitForSoftwareQueue blocks until softwaresync boot reconcile finishes and the
// softwares install queue has no pending/running items (failed items are OK).
// logFn receives human-readable status lines for the install terminal / SSE.
func waitForSoftwareQueue(ctx context.Context, db *gorm.DB, logFn func(string)) error {
	if logFn == nil {
		logFn = func(string) {}
	}

	if !softwaresync.Ready() {
		logFn("Waiting for softwares catalog reconcile to finish…")
		if err := softwaresync.WaitReady(ctx); err != nil {
			return fmt.Errorf("waiting for softwaresync: %w", err)
		}
		logFn("Softwares catalog reconcile complete")
	}

	lastPending := -1
	err := swinstall.WaitQueueIdle(ctx, db, time.Second, func(q swinstall.QueueView) {
		if q.Pending == 0 {
			if lastPending > 0 {
				logFn("Software install queue is clear — starting VNC setup")
			}
			lastPending = 0
			return
		}
		if q.Pending == lastPending {
			return
		}
		lastPending = q.Pending
		names := make([]string, 0, 3)
		for _, it := range q.Items {
			if it.Status != "pending" && it.Status != "running" {
				continue
			}
			name := it.SoftwareName
			if name == "" {
				name = it.SoftwareID
			}
			names = append(names, name)
			if len(names) >= 3 {
				break
			}
		}
		msg := fmt.Sprintf("Waiting for software install queue (%d remaining)", q.Pending)
		if len(names) > 0 {
			msg += ": " + joinNames(names, q.Pending)
		}
		logFn(msg)
	})
	if err != nil {
		return fmt.Errorf("waiting for software queue: %w", err)
	}
	return nil
}

func joinNames(names []string, pending int) string {
	var s strings.Builder
	for i, n := range names {
		if i > 0 {
			s.WriteString(", ")
		}
		s.WriteString(n)
	}
	if pending > len(names) {
		s.WriteString(fmt.Sprintf(" (+%d more)", pending-len(names)))
	}
	return s.String()
}

// PackageManagerHeld reports whether VNC setup currently owns the package manager
// (script running — not merely waiting on the softwares queue).
func PackageManagerHeld() bool {
	st := GetActiveInstall()
	return st.Active && st.Phase == "installing"
}
