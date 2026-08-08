package environments

import (
	"context"
	"log"
	"time"
)

func (m *Environments) watcherInterval() time.Duration {
	if m.config != nil && m.config.watcherInterval > 0 {
		return m.config.watcherInterval
	}
	if m.config == nil || m.config.Redis() == nil {
		return defaultDBFallbackWatcherInterval
	}
	return defaultWatcherInterval
}

func (m *Environments) StartWatcher(ctx context.Context) {
	if m.config != nil && m.config.disableWatcher {
		m.debug("watcher", "disabled by config")
		return
	}

	m.watchMu.Lock()
	if m.watchCancel != nil {
		m.watchMu.Unlock()
		m.debug("watcher", "already running")
		return
	}

	watchCtx, cancel := context.WithCancel(ctx)
	m.watchCancel = cancel
	m.watchMu.Unlock()

	interval := m.watcherInterval()
	m.debug("watcher", "started (interval=%s)", interval)

	m.startSyncQueueSubscriber(watchCtx)

	m.watchWG.Go(func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("environments watcher panic: %v", rec)
			}
		}()
		m.runWatcher(watchCtx)
	})
}

func (m *Environments) runWatcher(ctx context.Context) {
	interval := m.watcherInterval()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.debug("watcher", "stopped")
			return
		case <-ticker.C:
			if err := m.poll(ctx); err != nil {
				log.Printf("environments watcher: %v", err)
			}
		}
	}
}

func (m *Environments) poll(ctx context.Context) error {
	now := time.Now()

	changed, err := m.revisionChanged(ctx)
	if err != nil {
		logEnvWarn("watcher revision check failed, skipping poll: %v", err)
		return nil
	}

	m.mu.RLock()
	auditDue := m.shouldRunFullAuditLocked(now)
	lastFingerprint := m.lastFingerprint
	m.mu.RUnlock()

	if !changed {
		if !auditDue {
			return nil
		}
		m.debug("watcher", "periodic full audit")
		return m.pollFull(ctx, lastFingerprint)
	}

	rows, err := m.loadFromDB(ctx)
	if err != nil {
		return err
	}

	fp := fingerprint(rows)
	if fp == lastFingerprint {
		m.updateSyncCursors(ctx)
		return nil
	}

	m.debug("watcher", "poll: reload (%d row(s))", len(rows))
	if err := m.applySync(ctx, rows); err != nil {
		return err
	}
	m.updateSyncCursors(ctx)
	return nil
}

func (m *Environments) pollFull(ctx context.Context, previousFingerprint string) error {
	rows, err := m.loadFromDB(ctx)
	if err != nil {
		logEnvWarn("watcher full audit load failed: %v", err)
		return nil
	}

	fp := fingerprint(rows)
	if fp == previousFingerprint {
		m.mu.Lock()
		m.lastFullAudit = time.Now()
		m.mu.Unlock()
		m.updateSyncCursors(ctx)
		return nil
	}

	m.debug("watcher", "full audit fingerprint changed (prev=%s… new=%s…)", truncHash(previousFingerprint), truncHash(fp))
	if err := m.applySync(ctx, rows); err != nil {
		return err
	}

	m.mu.Lock()
	m.lastFullAudit = time.Now()
	m.mu.Unlock()
	m.updateSyncCursors(ctx)
	return nil
}

func (m *Environments) shouldRunFullAuditLocked(now time.Time) bool {
	if m.lastFullAudit.IsZero() {
		return true
	}
	return now.Sub(m.lastFullAudit) >= defaultFullAuditInterval
}

func (m *Environments) initWatcherState(ctx context.Context) {
	rows, err := m.loadFromDB(ctx)
	if err != nil {
		log.Printf("environments: initial fingerprint failed: %v", err)
		return
	}

	m.mu.Lock()
	m.lastFingerprint = fingerprint(rows)
	if maxID, err := m.maxWatcherEventID(ctx); err == nil {
		m.lastWatcherEventID = maxID
	}
	if rev, ok, err := m.redisRevision(ctx); err == nil && ok {
		m.lastRedisRevision = rev
	}
	m.lastFullAudit = time.Now()
	m.mu.Unlock()
}
