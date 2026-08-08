package environments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"sort"
)

func fingerprint(rows []OsEnvironment) string {
	if len(rows) == 0 {
		return hex.EncodeToString(sha256.New().Sum(nil))
	}

	sorted := append([]OsEnvironment(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	h := sha256.New()
	for _, row := range sorted {
		h.Write([]byte(row.Name))
		h.Write([]byte{0})
		h.Write([]byte(row.Value))
		h.Write([]byte{0})
		if row.IsDisabled {
			h.Write([]byte("1"))
		} else {
			h.Write([]byte("0"))
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func copyShellOverrides(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	maps.Copy(out, src)
	return out
}

func (m *Environments) isShellPinned(name string) bool {
	if len(m.shellOverrides) == 0 {
		return false
	}
	_, ok := m.shellOverrides[normalizeName(name)]
	return ok
}

func (m *Environments) shellValue(name string) string {
	if len(m.shellOverrides) == 0 {
		return ""
	}
	return m.shellOverrides[normalizeName(name)]
}

func applyCoreValue(cfg *ServerConfig, name, value string) {
	switch normalizeName(name) {
	case "PORT":
		cfg.PORT = value
	case "ADDRESS":
		cfg.ADDRESS = value
	case "AUTH_URL":
		cfg.AUTH_URL = value
	case "JWT_SECRET":
		cfg.JWT_SECRET = value
	case "ENABLE_HTTPS":
		cfg.ENABLE_HTTPS = value
	case "SERVER_SSL_CERTIFICATE":
		cfg.SERVER_SSL_CERTIFICATE = value
	case "SERVER_SSL_KEY":
		cfg.SERVER_SSL_KEY = value
	}
}

func (m *Environments) applyShellPinnedConfig(serverCfg *ServerConfig) {
	for name, value := range m.shellOverrides {
		applyCoreValue(serverCfg, name, value)
	}
}

func (m *Environments) applyShellPinnedEnv() {
	for name, value := range m.shellOverrides {
		os.Setenv(name, value)
	}
}

func (m *Environments) applySync(ctx context.Context, rows []OsEnvironment) error {
	_ = ctx

	desired := make(map[string]string, len(rows))
	disabled := make(map[string]struct{}, len(rows))
	serverCfg := ServerConfig{}

	for _, row := range rows {
		name := normalizeName(row.Name)
		if name == "" {
			continue
		}
		if m.isShellPinned(name) {
			continue
		}
		if row.IsDisabled {
			disabled[name] = struct{}{}
			continue
		}
		desired[name] = row.Value
		if row.IsCore || IsCoreName(name) {
			applyCoreValue(&serverCfg, name, row.Value)
		}
	}

	m.applyShellPinnedConfig(&serverCfg)

	m.mu.Lock()
	previous := m.managedKeys
	if previous == nil {
		previous = make(map[string]struct{})
	}
	nextManaged := make(map[string]struct{}, len(desired))

	for name, value := range desired {
		os.Setenv(name, value)
		nextManaged[name] = struct{}{}
	}
	for name := range disabled {
		if m.isShellPinned(name) {
			continue
		}
		os.Unsetenv(name)
	}
	for name := range previous {
		if m.isShellPinned(name) {
			continue
		}
		if _, ok := desired[name]; ok {
			continue
		}
		if _, off := disabled[name]; off {
			continue
		}
		os.Unsetenv(name)
	}

	m.applyShellPinnedEnv()

	m.managedKeys = nextManaged
	m.lastFingerprint = fingerprint(rows)
	hooks := m.hooks
	m.mu.Unlock()

	m.debug("sync", "applied (%d active key(s), %d disabled)", len(desired), len(disabled))

	if hooks.OnReload != nil {
		hooks.OnReload(serverCfg)
	}
	return nil
}

func (m *Environments) loadFromDB(ctx context.Context) ([]OsEnvironment, error) {
	if m.config == nil || m.config.DB() == nil {
		return nil, ErrConfigDBRequired
	}

	q := m.config.DB().WithContext(ctx).Order("name ASC")
	if moduleID := m.config.ModuleID(); moduleID != "" {
		// Global rows (empty/null module_id) plus this module's rows.
		q = q.Where("module_id = ? OR module_id = '' OR module_id IS NULL", moduleID)
	}

	var rows []OsEnvironment
	err := q.Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load os environments: %w", err)
	}
	if moduleID := m.config.ModuleID(); moduleID != "" {
		m.debug("load", "read %d row(s) from database (module_id=%q + global)", len(rows), moduleID)
	} else {
		m.debug("load", "read %d row(s) from database", len(rows))
	}
	return rows, nil
}

func (m *Environments) Init(ctx context.Context) error {
	m.debug("init", "starting full environment reload")
	rows, err := m.loadFromDB(ctx)
	if err != nil {
		return err
	}
	return m.applySync(ctx, rows)
}

func (m *Environments) Reload(ctx context.Context) error {
	return m.Init(ctx)
}

func (m *Environments) SetHooks(hooks Hooks) {
	m.mu.Lock()
	m.hooks = hooks
	m.mu.Unlock()
}

// GetValue returns the stored value for a variable name, including disabled rows.
func (m *Environments) GetValue(ctx context.Context, name string) (OsEnvironment, error) {
	db, err := m.db(ctx)
	if err != nil {
		return OsEnvironment{}, err
	}
	name = normalizeName(name)
	var row OsEnvironment
	if err := db.Where("name = ?", name).First(&row).Error; err != nil {
		return OsEnvironment{}, mapDBError(err)
	}
	return row, nil
}

// EnsureCore resolves a core server variable: shell/viper value wins, then DB, then default.
func (m *Environments) EnsureCore(ctx context.Context, name, shellValue, defaultValue string) (string, error) {
	name = normalizeName(name)
	if shellValue != "" {
		os.Setenv(name, shellValue)
		return shellValue, nil
	}
	value := defaultValue

	db, err := m.db(ctx)
	if err != nil {
		os.Setenv(name, value)
		return value, err
	}

	var row OsEnvironment
	err = db.Where("name = ?", name).First(&row).Error
	if err != nil {
		if !isRecordNotFound(err) {
			os.Setenv(name, value)
			return value, err
		}
		row = OsEnvironment{
			Name:       name,
			Value:      value,
			ModuleID:   m.config.ModuleID(),
			Source:     OsEnvironmentSourceServer,
			IsCore:     true,
			IsSecret:   name == "JWT_SECRET" || name == "SERVER_SSL_KEY",
			IsDisabled: false,
		}
		if err := db.Create(&row).Error; err != nil {
			os.Setenv(name, value)
			return value, err
		}
		os.Setenv(name, row.Value)
		return row.Value, nil
	}

	if row.IsDisabled {
		os.Unsetenv(name)
		return row.Value, nil
	}
	os.Setenv(name, row.Value)
	return row.Value, nil
}

func (m *Environments) resolveShellValue(name string, shell map[string]string) string {
	if v := m.shellValue(name); v != "" {
		return v
	}
	if shell != nil {
		return shell[name]
	}
	return ""
}

func (m *Environments) ServerConfigFromDB(ctx context.Context, shell map[string]string, defaults map[string]string) (ServerConfig, error) {
	cfg := ServerConfig{}
	var err error

	cfg.PORT, err = m.EnsureCore(ctx, "PORT", m.resolveShellValue("PORT", shell), defaults["PORT"])
	if err != nil {
		return cfg, err
	}
	cfg.ADDRESS, err = m.EnsureCore(ctx, "ADDRESS", m.resolveShellValue("ADDRESS", shell), defaults["ADDRESS"])
	if err != nil {
		return cfg, err
	}
	cfg.AUTH_URL, err = m.EnsureCore(ctx, "AUTH_URL", m.resolveShellValue("AUTH_URL", shell), defaults["AUTH_URL"])
	if err != nil {
		return cfg, err
	}
	cfg.JWT_SECRET, err = m.EnsureCore(ctx, "JWT_SECRET", m.resolveShellValue("JWT_SECRET", shell), defaults["JWT_SECRET"])
	if err != nil {
		return cfg, err
	}
	cfg.ENABLE_HTTPS, err = m.EnsureCore(ctx, "ENABLE_HTTPS", m.resolveShellValue("ENABLE_HTTPS", shell), defaults["ENABLE_HTTPS"])
	if err != nil {
		return cfg, err
	}
	cfg.SERVER_SSL_CERTIFICATE, err = m.EnsureCore(ctx, "SERVER_SSL_CERTIFICATE", m.resolveShellValue("SERVER_SSL_CERTIFICATE", shell), defaults["SERVER_SSL_CERTIFICATE"])
	if err != nil {
		return cfg, err
	}
	cfg.SERVER_SSL_KEY, err = m.EnsureCore(ctx, "SERVER_SSL_KEY", m.resolveShellValue("SERVER_SSL_KEY", shell), defaults["SERVER_SSL_KEY"])
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

// SetCore upserts a core server setting in the database and process environment.
// Shell-pinned values are left unchanged in the process env but the DB row is still updated.
func (m *Environments) SetCore(ctx context.Context, name, value string) error {
	name = normalizeName(name)
	if !IsCoreName(name) {
		return ErrInvalidName
	}
	db, err := m.db(ctx)
	if err != nil {
		return err
	}

	var row OsEnvironment
	err = db.Where("name = ?", name).First(&row).Error
	if err != nil {
		if !isRecordNotFound(err) {
			return err
		}
		row = OsEnvironment{
			Name:       name,
			Value:      value,
			ModuleID:   m.config.ModuleID(),
			Source:     OsEnvironmentSourceServer,
			IsCore:     true,
			IsSecret:   name == "JWT_SECRET" || name == "SERVER_SSL_KEY",
			IsDisabled: false,
		}
		if err := db.Create(&row).Error; err != nil {
			return fmt.Errorf("create core environment %s: %w", name, err)
		}
	} else {
		if err := db.Model(&row).Updates(map[string]any{
			"value":     value,
			"is_core":   true,
			"source":    OsEnvironmentSourceServer,
			"is_secret": name == "JWT_SECRET" || name == "SERVER_SSL_KEY",
		}).Error; err != nil {
			return fmt.Errorf("update core environment %s: %w", name, err)
		}
	}

	if !m.isShellPinned(name) {
		os.Setenv(name, value)
	}
	return nil
}
