package environments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type Environments struct {
	mu sync.RWMutex

	config          *Config
	shellOverrides  map[string]string
	instanceID      string
	managedKeys     map[string]struct{}
	hooks           Hooks

	lastWatcherEventID int64
	lastRedisRevision  int64
	lastFingerprint    string
	lastFullAudit      time.Time

	watchMu     sync.Mutex
	watchCancel context.CancelFunc
	queueMu     sync.Mutex
	queueCancel context.CancelFunc
	watchWG     sync.WaitGroup
}

func New(cfg *Config) (*Environments, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}
	if cfg.DB() == nil {
		return nil, ErrConfigDBRequired
	}
	if err := AutoMigrate(cfg.DB()); err != nil {
		return nil, fmt.Errorf("auto migrate environments tables: %w", err)
	}

	m := &Environments{
		config:         cfg,
		shellOverrides: copyShellOverrides(cfg.shellOverrides),
		instanceID:     newInstanceID(),
		managedKeys:    make(map[string]struct{}),
	}
	m.debug("boot", "manager created")

	if err := m.Init(context.Background()); err != nil {
		logEnvWarn("init incomplete at boot (watcher will retry): %v", err)
	}
	m.initWatcherState(context.Background())
	m.StartWatcher(context.Background())

	m.debug("boot", "ready")
	return m, nil
}

func (m *Environments) Config() *Config {
	return m.config
}

// Get returns the effective value for an environment variable.
// Shell-pinned values (.env / process overrides) win, then the synced
// process environment. Returns "" when unset or m is nil.
func (m *Environments) Get(name string) string {
	if m == nil {
		return ""
	}
	name = normalizeName(name)
	if name == "" {
		return ""
	}
	if v := strings.TrimSpace(m.shellValue(name)); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv(name))
}

func (m *Environments) Close() {
	if m == nil {
		return
	}
	m.debug("close", "stopping watcher and sync queue")
	m.stopWatcher()
}

func (m *Environments) stopWatcher() {
	m.stopSyncQueueSubscriber()

	m.watchMu.Lock()
	cancel := m.watchCancel
	m.watchCancel = nil
	m.watchMu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.watchWG.Wait()
}

func (m *Environments) db(ctx context.Context) (*gorm.DB, error) {
	if m.config == nil || m.config.DB() == nil {
		return nil, ErrConfigDBRequired
	}
	return m.config.DB().WithContext(ctx), nil
}
