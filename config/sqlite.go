package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (a *AppClients) DB() *gorm.DB {
	return a.db
}

func InitializeDatabase(databaseURL string) (*gorm.DB, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	// SQLite cannot create missing parent dirs; modernc often reports this as
	// "unable to open database file: out of memory (14)" (SQLITE_CANTOPEN).
	if dir := filepath.Dir(databaseURL); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory %q: %w", dir, err)
		}
	}

	config := &gorm.Config{}
	if viper.GetString("ENV") != "development" {
		config.Logger = logger.Default.LogMode(logger.Silent)
	}
	db, err := gorm.Open(sqlite.Open(databaseURL), config)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func AutoMigrateSQLite(db *gorm.DB, models ...any) error {
	if err := db.AutoMigrate(models...); err != nil {
		return err
	}
	if err := migrateVncSessionsIDToUUID(db); err != nil {
		return fmt.Errorf("migrate vnc_sessions id to uuid: %w", err)
	}
	if err := migrateCodeserverSessionsMultiWorkspace(db); err != nil {
		return fmt.Errorf("migrate codeserver_sessions multi-workspace: %w", err)
	}
	return nil
}

// migrateCodeserverSessionsMultiWorkspace drops the legacy unique user_id index
// so a user may own many VS Code workspaces, and backfills empty names.
func migrateCodeserverSessionsMultiWorkspace(db *gorm.DB) error {
	var table string
	if err := db.Raw(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='codeserver_sessions'`,
	).Scan(&table).Error; err != nil {
		return err
	}
	if table == "" {
		return nil
	}

	// GORM historically created a UNIQUE index on user_id; remove it if present.
	_ = db.Exec(`DROP INDEX IF EXISTS idx_codeserver_sessions_user_id`).Error
	if err := db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_codeserver_sessions_user_id ON codeserver_sessions(user_id)`,
	).Error; err != nil {
		return err
	}

	// Ensure name column exists (AutoMigrate adds it); fill blanks with a
	// temporary label — API/list also derive from path when name is empty.
	var hasName int
	if err := db.Raw(
		`SELECT COUNT(*) FROM pragma_table_info('codeserver_sessions') WHERE name = 'name'`,
	).Scan(&hasName).Error; err != nil {
		return err
	}
	if hasName == 0 {
		return nil
	}
	return db.Exec(`
		UPDATE codeserver_sessions
		SET name = 'workspace'
		WHERE name IS NULL OR TRIM(name) = ''
	`).Error
}

// migrateVncSessionsIDToUUID rebuilds vnc_sessions when id is still INTEGER
// (SQLite AutoMigrate cannot change PK type; inserting UUID strings then fails
// with "datatype mismatch").
func migrateVncSessionsIDToUUID(db *gorm.DB) error {
	var table string
	if err := db.Raw(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='vnc_sessions'`,
	).Scan(&table).Error; err != nil {
		return err
	}
	if table == "" {
		return nil
	}

	type colInfo struct {
		CID  int
		Name string
		Type string
	}
	var cols []colInfo
	if err := db.Raw(`PRAGMA table_info(vnc_sessions)`).Scan(&cols).Error; err != nil {
		return err
	}
	idType := ""
	for _, c := range cols {
		if strings.EqualFold(c.Name, "id") {
			idType = strings.ToUpper(strings.TrimSpace(c.Type))
			break
		}
	}
	// Already text/uuid-friendly.
	if idType == "" || idType == "TEXT" || strings.Contains(idType, "CHAR") {
		return nil
	}

	type legacyRow struct {
		ID          int64
		UserID      string
		Status      string
		VncPassword string
		Address     string
		NoVncPort   int
		VncPort     int
		CreatedAt   *string
		UpdatedAt   *string
		DeletedAt   *string
	}
	var rows []legacyRow
	if err := db.Raw(`
		SELECT id, user_id, status, vnc_password, address, no_vnc_port, vnc_port,
		       created_at, updated_at, deleted_at
		FROM vnc_sessions
	`).Scan(&rows).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		stmts := []string{
			`ALTER TABLE vnc_sessions RENAME TO vnc_sessions_legacy_int`,
			`CREATE TABLE vnc_sessions (
				id text PRIMARY KEY,
				user_id text NOT NULL,
				status text DEFAULT 'active',
				vnc_password text DEFAULT '',
				address text DEFAULT '127.0.0.1',
				no_vnc_port integer DEFAULT 0,
				vnc_port integer DEFAULT 0,
				created_at datetime,
				updated_at datetime,
				deleted_at datetime,
				CONSTRAINT fk_vnc_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_vnc_sessions_user_id ON vnc_sessions(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_vnc_sessions_deleted_at ON vnc_sessions(deleted_at)`,
		}
		for _, s := range stmts {
			if err := tx.Exec(s).Error; err != nil {
				return err
			}
		}

		for _, row := range rows {
			id := uuid.New().String()
			if err := tx.Exec(`
				INSERT INTO vnc_sessions
					(id, user_id, status, vnc_password, address, no_vnc_port, vnc_port, created_at, updated_at, deleted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, row.UserID, row.Status, row.VncPassword, row.Address, row.NoVncPort, row.VncPort,
				row.CreatedAt, row.UpdatedAt, row.DeletedAt).Error; err != nil {
				return err
			}
		}

		return tx.Exec(`DROP TABLE vnc_sessions_legacy_int`).Error
	})
}
