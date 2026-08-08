package environments

import (
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrate creates or updates environments package tables and migrates legacy option rows.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return ErrConfigDBRequired
	}
	if err := db.AutoMigrate(
		&OsEnvironment{},
		&OsEnvironmentWatcherModel{},
	); err != nil {
		return err
	}
	if err := migrateFromOptions(db); err != nil {
		logEnvWarn("legacy options migration failed: %v", err)
	}
	if err := installWatcherTriggers(db); err != nil {
		logEnvWarn("watcher triggers not installed: %v", err)
	}
	return nil
}

func migrateFromOptions(db *gorm.DB) error {
	var count int64
	if err := db.Model(&OsEnvironment{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var options []OsEnvironment
	if err := db.Where("source IN ?", []OsEnvironmentSource{OsEnvironmentSourceEnv, OsEnvironmentSourceServer}).Find(&options).Error; err != nil {
		return fmt.Errorf("load legacy options: %w", err)
	}
	if len(options) == 0 {
		return nil
	}

	rows := make([]OsEnvironment, 0, len(options))
	for _, option := range options {
		name := normalizeName(option.Name)
		if name == "" {
			continue
		}
		row := OsEnvironment{
			ID:         option.ID,
			Name:       name,
			Value:      option.Value,
			Group:      formatGroup(option.Group),
			IsSecret:   option.IsSecret,
			IsDisabled: option.IsDisabled,
		}
		if option.Source == OsEnvironmentSourceServer || IsCoreName(name) {
			row.Source = OsEnvironmentSourceServer
			row.IsCore = IsCoreName(name)
		} else {
			row.Source = OsEnvironmentSourceEnv
		}
		rows = append(rows, row)
	}

	if err := db.Create(&rows).Error; err != nil {
		return fmt.Errorf("migrate options to os_environments: %w", err)
	}
	logEnvWarn("migrated %d legacy option row(s) to os_environments", len(rows))
	return nil
}

func installWatcherTriggers(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	switch db.Dialector.Name() {
	case "postgres":
		return installPostgresWatcherTriggers(db)
	case "sqlite":
		return installSQLiteWatcherTriggers(db)
	default:
		return nil
	}
}

func installPostgresWatcherTriggers(db *gorm.DB) error {
	const fn = `
CREATE OR REPLACE FUNCTION notify_os_environment_watcher()
RETURNS TRIGGER AS $$
DECLARE
    env_uuid uuid;
    change_action text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        env_uuid := OLD.id;
        change_action := 'delete';
    ELSIF TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
        env_uuid := OLD.id;
        change_action := 'delete';
    ELSIF TG_OP = 'UPDATE' AND NEW.is_disabled IS TRUE AND OLD.is_disabled IS DISTINCT FROM TRUE THEN
        env_uuid := NEW.id;
        change_action := 'delete';
    ELSIF TG_OP = 'INSERT' AND NEW.is_disabled IS TRUE THEN
        env_uuid := NEW.id;
        change_action := 'delete';
    ELSE
        env_uuid := NEW.id;
        change_action := 'upsert';
    END IF;

    INSERT INTO os_environment_watchers (environment_id, action, created_at, updated_at)
    VALUES (env_uuid, change_action, NOW(), NOW());

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
`

	const trigger = `
DROP TRIGGER IF EXISTS os_environments_notify_watcher ON os_environments;
CREATE TRIGGER os_environments_notify_watcher
AFTER INSERT OR UPDATE OR DELETE ON os_environments
FOR EACH ROW EXECUTE FUNCTION notify_os_environment_watcher();
`

	if err := db.Exec(fn).Error; err != nil {
		return fmt.Errorf("install os environment watcher function: %w", err)
	}
	if err := db.Exec(trigger).Error; err != nil {
		return fmt.Errorf("install os environment watcher trigger: %w", err)
	}

	_ = db.Exec("DROP TRIGGER IF EXISTS options_notify_watcher ON options").Error
	_ = db.Exec("DROP FUNCTION IF EXISTS notify_option_watcher()").Error

	return nil
}

func installSQLiteWatcherTriggers(db *gorm.DB) error {
	statements := []string{
		`DROP TRIGGER IF EXISTS os_environments_notify_watcher_insert`,
		`DROP TRIGGER IF EXISTS os_environments_notify_watcher_update`,
		`DROP TRIGGER IF EXISTS os_environments_notify_watcher_delete`,
		`
CREATE TRIGGER os_environments_notify_watcher_insert
AFTER INSERT ON os_environments
FOR EACH ROW
BEGIN
	INSERT INTO os_environment_watchers (environment_id, action, created_at, updated_at)
	VALUES (
		NEW.id,
		CASE WHEN NEW.is_disabled = 1 THEN 'delete' ELSE 'upsert' END,
		CURRENT_TIMESTAMP,
		CURRENT_TIMESTAMP
	);
END
`,
		`
CREATE TRIGGER os_environments_notify_watcher_update
AFTER UPDATE ON os_environments
FOR EACH ROW
BEGIN
	INSERT INTO os_environment_watchers (environment_id, action, created_at, updated_at)
	VALUES (
		NEW.id,
		CASE
			WHEN NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN 'delete'
			WHEN NEW.is_disabled = 1 AND IFNULL(OLD.is_disabled, 0) = 0 THEN 'delete'
			ELSE 'upsert'
		END,
		CURRENT_TIMESTAMP,
		CURRENT_TIMESTAMP
	);
END
`,
		`
CREATE TRIGGER os_environments_notify_watcher_delete
AFTER DELETE ON os_environments
FOR EACH ROW
BEGIN
	INSERT INTO os_environment_watchers (environment_id, action, created_at, updated_at)
	VALUES (OLD.id, 'delete', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
END
`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("install sqlite os environment watcher trigger: %w", err)
		}
	}
	return nil
}

// ToResponse maps a row to the admin API shape.
func ToResponse(row OsEnvironment) map[string]any {
	return map[string]any{
		"id":          row.ID,
		"name":        row.Name,
		"value":       row.Value,
		"type":        string(row.Source),
		"group":       row.Group,
		"module_id":   row.ModuleID,
		"is_secret":   row.IsSecret,
		"is_disabled": row.IsDisabled,
		"is_textarea": row.IsTextarea,
		"is_core":     row.IsCore || IsCoreName(row.Name),
		"created_at":  row.CreatedAt,
		"updated_at":  row.UpdatedAt,
	}
}
