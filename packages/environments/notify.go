package environments

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NotifyChange signals all pods that environment variables changed.
func (m *Environments) NotifyChange(ctx context.Context) error {
	return m.notifyChange(ctx, "", WatcherActionSignal)
}

// NotifyEnvironmentChange signals that a single environment row changed.
func (m *Environments) NotifyEnvironmentChange(ctx context.Context, environmentID string, action WatcherAction) error {
	if environmentID == "" {
		return fmt.Errorf("environment id is required")
	}
	switch action {
	case WatcherActionUpsert, WatcherActionDelete:
	default:
		return fmt.Errorf("unsupported watcher action %q", action)
	}
	return m.notifyChange(ctx, environmentID, action)
}

func (m *Environments) notifyChange(ctx context.Context, environmentID string, action WatcherAction) error {
	if m.config == nil || m.config.DB() == nil {
		return ErrConfigDBRequired
	}

	m.debug("notify", "change signaled (environment=%s action=%s)", environmentID, action)

	eventID, revision, err := m.bumpRevision(ctx, environmentID, action)
	if err != nil {
		m.debug("notify", "bump revision failed: %v", err)
		return err
	}

	m.publishSyncQueue(ctx, SyncQueueMessage{
		SourceID:      m.instanceID,
		EventID:       eventID,
		Revision:      revision,
		EnvironmentID: environmentID,
		Action:        action,
		PublishedAt:   time.Now().UTC(),
	})

	if err := m.Reload(ctx); err != nil {
		logEnvWarn("reload after notify failed: %v", err)
	}
	return nil
}

func (m *Environments) bumpRevision(ctx context.Context, environmentID string, action WatcherAction) (eventID int64, revision int64, err error) {
	if m.config == nil || m.config.DB() == nil {
		return 0, 0, ErrConfigDBRequired
	}
	if action == "" {
		action = WatcherActionSignal
	}

	if m.config.Redis() != nil {
		if n, incrErr := m.config.Redis().Incr(ctx, revisionKey).Result(); incrErr != nil {
			logEnvWarn("redis revision bump failed: %v", incrErr)
		} else {
			revision = n
		}
	}

	row := &OsEnvironmentWatcherModel{
		EnvironmentID: environmentID,
		Action:        string(action),
	}
	if err := m.config.DB().WithContext(ctx).Create(row).Error; err != nil {
		return 0, revision, fmt.Errorf("create environment watcher event: %w", err)
	}

	m.pruneWatchers(ctx)
	return row.ID, revision, nil
}

func (m *Environments) pruneWatchers(ctx context.Context) {
	if m.config == nil || m.config.DB() == nil {
		return
	}
	_ = m.config.DB().WithContext(ctx).Exec(`
		DELETE FROM os_environment_watchers
		WHERE id NOT IN (
			SELECT id FROM (
				SELECT id FROM os_environment_watchers ORDER BY id DESC LIMIT ?
			) AS recent
		)
	`, maxWatcherRows).Error
}

func (m *Environments) maxWatcherEventID(ctx context.Context) (int64, error) {
	if m.config == nil || m.config.DB() == nil {
		return 0, ErrConfigDBRequired
	}
	// Append-only change log (prune uses hard DELETE). Bypass soft-delete scope so
	// MAX(id) includes pruned tombstones if any remain.
	var maxID int64
	err := m.config.DB().WithContext(ctx).
		Unscoped().
		Raw(`SELECT COALESCE(MAX(id), 0) FROM os_environment_watchers`).
		Scan(&maxID).Error
	return maxID, err
}

func (m *Environments) redisRevision(ctx context.Context) (int64, bool, error) {
	if m.config == nil || m.config.Redis() == nil {
		return 0, false, nil
	}
	rev, err := m.config.Redis().Get(ctx, revisionKey).Int64()
	if err == redis.Nil {
		// Key not created yet — still use Redis path (rev 0) instead of falling back to DB.
		return 0, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	return rev, true, nil
}

func (m *Environments) revisionChanged(ctx context.Context) (bool, error) {
	if rev, ok, err := m.redisRevision(ctx); err != nil {
		m.debug("watcher", "redis revision check failed, falling back to db events: %v", err)
	} else if ok {
		m.mu.RLock()
		changed := rev != m.lastRedisRevision
		m.mu.RUnlock()
		return changed, nil
	}

	maxID, err := m.maxWatcherEventID(ctx)
	if err != nil {
		return false, err
	}

	m.mu.RLock()
	changed := maxID > m.lastWatcherEventID
	m.mu.RUnlock()
	return changed, nil
}

func (m *Environments) updateSyncCursors(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if maxID, err := m.maxWatcherEventID(ctx); err == nil {
		m.lastWatcherEventID = maxID
	}
	if rev, ok, err := m.redisRevision(ctx); err == nil && ok {
		m.lastRedisRevision = rev
	}
}
