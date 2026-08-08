package environments

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const syncQueueChannel = "environments:sync:queue"

// SyncQueueMessage is broadcast to every container when environment variables change.
type SyncQueueMessage struct {
	SourceID      string        `json:"source_id"`
	EventID       int64         `json:"event_id"`
	Revision      int64         `json:"revision"`
	EnvironmentID string        `json:"environment_id"`
	Action        WatcherAction `json:"action"`
	PublishedAt   time.Time     `json:"published_at"`
}

// EnqueueContainerSync queues a cross-container environment reload.
// It persists a watcher event, bumps the Redis revision, publishes to the sync
// queue, and reloads variables on the local container immediately.
func (m *Environments) EnqueueContainerSync(ctx context.Context, environmentID string, action WatcherAction) error {
	if environmentID == "" {
		return fmt.Errorf("environment id is required")
	}
	switch action {
	case WatcherActionUpsert, WatcherActionDelete:
	default:
		return fmt.Errorf("unsupported sync action %q", action)
	}
	return m.notifyChange(ctx, environmentID, action)
}

func (m *Environments) publishSyncQueue(ctx context.Context, msg SyncQueueMessage) {
	if m.config == nil || m.config.Redis() == nil {
		return
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		logEnvWarn("sync queue marshal failed: %v", err)
		return
	}
	if err := m.config.Redis().Publish(ctx, syncQueueChannel, payload).Err(); err != nil {
		logEnvWarn("sync queue publish failed: %v", err)
		return
	}
	m.debug("queue", "published sync message (environment=%s action=%s revision=%d)", msg.EnvironmentID, msg.Action, msg.Revision)
}

func (m *Environments) startSyncQueueSubscriber(ctx context.Context) {
	if m.config == nil || m.config.Redis() == nil {
		m.debug("queue", "subscriber disabled (redis unavailable)")
		return
	}

	m.queueMu.Lock()
	if m.queueCancel != nil {
		m.queueMu.Unlock()
		return
	}
	queueCtx, cancel := context.WithCancel(ctx)
	m.queueCancel = cancel
	m.queueMu.Unlock()

	sub := m.config.Redis().Subscribe(queueCtx, syncQueueChannel)
	m.watchWG.Go(func() {
		defer sub.Close()
		m.debug("queue", "subscriber started on channel %s", syncQueueChannel)

		ch := sub.Channel()
		for {
			select {
			case <-queueCtx.Done():
				m.debug("queue", "subscriber stopped")
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				m.handleSyncQueueMessage(queueCtx, msg.Payload)
			}
		}
	})
}

func (m *Environments) handleSyncQueueMessage(ctx context.Context, payload string) {
	var event SyncQueueMessage
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		logEnvWarn("sync queue decode failed: %v", err)
		return
	}
	if event.SourceID != "" && event.SourceID == m.instanceID {
		m.debug("queue", "skip local echo (environment=%s)", event.EnvironmentID)
		return
	}

	m.debug("queue", "received sync message (environment=%s action=%s revision=%d)", event.EnvironmentID, event.Action, event.Revision)

	if err := m.Reload(ctx); err != nil {
		logEnvWarn("sync queue reload failed: %v", err)
		return
	}

	m.mu.Lock()
	if event.EventID > m.lastWatcherEventID {
		m.lastWatcherEventID = event.EventID
	}
	if event.Revision > m.lastRedisRevision {
		m.lastRedisRevision = event.Revision
	}
	m.mu.Unlock()
}

func (m *Environments) stopSyncQueueSubscriber() {
	m.queueMu.Lock()
	cancel := m.queueCancel
	m.queueCancel = nil
	m.queueMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func newInstanceID() string {
	return uuid.NewString()
}
