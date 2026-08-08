package machine

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/containerws/models"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncCurrentContainer updates the single workspace container row in-place.
// It never creates a second row when one already exists (stop/start or Docker
// recreate with a persisted /config volume). A new row is created only when
// the containers table is empty.
func SyncCurrentContainer(ctx context.Context, db *gorm.DB) (*models.Container, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}

	// Prefer a durable id under /config so recreate does not mint a new machine_id.
	stableID := loadOrCreateStableMachineID()
	snap := Detect()
	if stableID != "" {
		snap.MachineID = stableID
	}
	now := time.Now().UTC()

	row, found, err := findExistingContainer(ctx, db, snap)
	if err != nil {
		return nil, err
	}

	if !found {
		row = models.Container{
			IsMaster: true,
			IsActive: true,
			Icon:     "Container",
		}
		snap.ApplyToContainer(&row)
		row.MachineID = firstNonEmpty(stableID, snap.MachineID)
		row.IsMaster = true
		row.IsActive = true
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, err
		}
		log.Printf("container registered: id=%s name=%s machine_id=%s type=%s",
			row.ID, row.Name, row.MachineID, row.Type)
		return &row, nil
	}

	// Always UPDATE the same row — never insert another on restart/recreate.
	preserveName := strings.TrimSpace(row.Name)
	preserveTitle := strings.TrimSpace(row.Title)
	preserveDesc := strings.TrimSpace(row.Description)
	preserveIcon := strings.TrimSpace(row.Icon)
	preserveID := row.ID

	snap.ApplyToContainer(&row)
	row.ID = preserveID
	row.MachineID = firstNonEmpty(stableID, snap.MachineID, row.MachineID)
	row.IsMaster = true
	row.IsActive = true
	row.BootedAt = &now
	row.LastSeenAt = &now

	if preserveName != "" {
		row.Name = preserveName
	}
	if preserveTitle != "" {
		row.Title = preserveTitle
	}
	if preserveDesc != "" {
		row.Description = preserveDesc
	}
	if preserveIcon != "" {
		row.Icon = preserveIcon
	}

	if err := db.WithContext(ctx).
		Omit(clause.Associations).
		Save(&row).Error; err != nil {
		return nil, err
	}

	// Demote any other rows so this workspace stays the only master.
	_ = db.WithContext(ctx).Model(&models.Container{}).
		Where("id <> ?", row.ID).
		Updates(map[string]any{"is_master": false}).Error

	if viper.GetBool("start") {
		log.Printf("container updated: id=%s name=%s machine_id=%s type=%s virt=%s ips=%v",
			row.ID, row.Name, row.MachineID, row.Type, row.Virtualization, []string(row.IPs))
	}
	return &row, nil
}

// findExistingContainer resolves the workspace row without creating duplicates.
// Order: durable machine_id → master → any active → any row (including soft-deleted restore).
func findExistingContainer(ctx context.Context, db *gorm.DB, snap Snapshot) (models.Container, bool, error) {
	var row models.Container

	if snap.MachineID != "" {
		err := db.WithContext(ctx).Where("machine_id = ?", snap.MachineID).First(&row).Error
		if err == nil {
			return row, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return row, false, err
		}
	}

	err := db.WithContext(ctx).Where("is_master = ?", true).Order("updated_at DESC").First(&row).Error
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, err
	}

	err = db.WithContext(ctx).Where("is_active = ?", true).Order("is_master DESC, updated_at DESC").First(&row).Error
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, err
	}

	// Last resort: any existing row (even soft-deleted) → restore/update, do not create.
	err = db.WithContext(ctx).Unscoped().Order("is_master DESC, updated_at DESC").First(&row).Error
	if err == nil {
		if row.DeletedAt.Valid {
			row.DeletedAt = gorm.DeletedAt{}
		}
		return row, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	return row, false, err
}

func loadOrCreateStableMachineID() string {
	if v := strings.TrimSpace(viper.GetString("CONTAINERWS_MACHINE_ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(viper.GetString("CONTAINER_ID")); v != "" {
		return v
	}

	path := stableMachineIDPath()
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}

	// Seed once from host, then persist under /config so Docker recreate keeps the same id.
	id := strings.TrimSpace(readFirstLine("/etc/machine-id"))
	if id == "" {
		id = strings.TrimSpace(readFirstLine("/var/lib/dbus/machine-id"))
	}
	if id == "" {
		id = uuid.New().String()
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(id+"\n"), 0o644)
	return id
}

func stableMachineIDPath() string {
	if dir := strings.TrimSpace(viper.GetString("CONTAINERWS_DATA_DIR")); dir != "" {
		return filepath.Join(dir, "machine_id")
	}
	// Same volume as the SQLite DB (./tmp/config → /config in compose).
	return "/config/containerws/machine_id"
}
