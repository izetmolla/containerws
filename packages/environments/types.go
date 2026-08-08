package environments

import (
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const revisionKey = "environments:revision"
const maxWatcherRows = 1000
const defaultWatcherInterval = 5 * time.Second
// When Redis is unavailable the watcher polls the DB for MAX(id). Use a longer
// interval so remote-DB RTT does not flood logs every few seconds.
const defaultDBFallbackWatcherInterval = 30 * time.Second
const defaultFullAuditInterval = 5 * time.Minute

// WatcherAction describes what changed in an os_environment_watchers row.
type WatcherAction string

const (
	WatcherActionUpsert WatcherAction = "upsert"
	WatcherActionDelete WatcherAction = "delete"
	WatcherActionSignal WatcherAction = "signal"
)

// ServerConfig holds the core server variables managed in os_environments.
type ServerConfig struct {
	PORT                   string
	ADDRESS                string
	AUTH_URL               string
	JWT_SECRET             string
	ENABLE_HTTPS           string
	SERVER_SSL_CERTIFICATE string
	SERVER_SSL_KEY         string
}

// Hooks runs after the process environment is reconciled from the database.
type Hooks struct {
	OnReload func(ServerConfig)
}

var (
	ErrNotFound         = errors.New("environment variable not found")
	ErrInvalidName      = errors.New("invalid environment variable name")
	ErrNameConflict     = errors.New("environment variable already exists")
	ErrCoreNameReserved = errors.New("name is reserved for core server settings")
	ErrCoreNotDeletable = errors.New("core server settings cannot be deleted")
	ErrConfigRequired   = errors.New("config is required")
	ErrConfigDBRequired = errors.New("config database is required")
)

var coreNames = map[string]struct{}{
	"PORT":                   {},
	"ADDRESS":                {},
	"AUTH_URL":               {},
	"JWT_SECRET":             {},
	"ENABLE_HTTPS":           {},
	"SERVER_SSL_CERTIFICATE": {},
	"SERVER_SSL_KEY":         {},
}

// CreateEnvironmentInput is the payload for CreateEnvironment.
type CreateEnvironmentInput struct {
	Name       string
	Value      string
	Group      string
	IsSecret   bool
	IsDisabled bool
	IsTextarea bool
}

// UpdateEnvironmentInput updates only the fields that are non-nil.
type UpdateEnvironmentInput struct {
	Name       *string
	Value      *string
	Group      *string
	IsSecret   *bool
	IsDisabled *bool
	IsTextarea *bool
}

// CoreNames returns the server variables managed as core settings.
func CoreNames() []string {
	names := make([]string, 0, len(coreNames))
	for name := range coreNames {
		names = append(names, name)
	}
	return names
}

// BuildShellOverrides captures non-empty values for core names from getter.
// Values declared in .env or the process environment must be read before the
// environments manager syncs database rows into os.Setenv.
func BuildShellOverrides(getter func(string) string) map[string]string {
	if getter == nil {
		return nil
	}
	out := make(map[string]string)
	for name := range coreNames {
		if v := getter(name); v != "" {
			out[name] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Config holds the core database and an optional Redis client for cross-pod signals.
type Config struct {
	db              *gorm.DB
	redis           *redis.Client
	shellOverrides  map[string]string
	moduleID        string
	watcherInterval time.Duration
	disableWatcher  bool
	debug           bool
}

// NewConfig creates an environments config. redis is optional.
func NewConfig(db *gorm.DB, redis *redis.Client) *Config {
	return &Config{db: db, redis: redis}
}

func (c *Config) WithWatcherInterval(interval time.Duration) *Config {
	c.watcherInterval = interval
	return c
}

func (c *Config) WithWatcherDisabled() *Config {
	c.disableWatcher = true
	return c
}

func (c *Config) WithDebug(enabled bool) *Config {
	c.debug = enabled
	return c
}

// WithModuleID scopes environment loading to global rows (empty module_id)
// plus rows whose module_id matches id. When unset, all rows are loaded.
func (c *Config) WithModuleID(id string) *Config {
	c.moduleID = strings.TrimSpace(id)
	return c
}

// ModuleID returns the configured module scope, if any.
func (c *Config) ModuleID() string {
	if c == nil {
		return ""
	}
	return c.moduleID
}

// WithShellOverrides pins values declared in .env or the process environment.
// Pinned keys are not overwritten by database sync and are not inserted by EnsureCore.
func (c *Config) WithShellOverrides(overrides map[string]string) *Config {
	if len(overrides) == 0 {
		c.shellOverrides = nil
		return c
	}
	c.shellOverrides = make(map[string]string, len(overrides))
	for name, value := range overrides {
		name = normalizeName(name)
		if name == "" || value == "" {
			continue
		}
		c.shellOverrides[name] = value
	}
	if len(c.shellOverrides) == 0 {
		c.shellOverrides = nil
	}
	return c
}

func (c *Config) DB() *gorm.DB {
	if c == nil {
		return nil
	}
	return c.db
}

func (c *Config) Redis() *redis.Client {
	if c == nil {
		return nil
	}
	return c.redis
}

func IsCoreName(name string) bool {
	_, ok := coreNames[normalizeName(name)]
	return ok
}
