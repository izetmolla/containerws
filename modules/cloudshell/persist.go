package cloudshell

import (
	"log"
	"os"
	"time"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

var sessionDB *gorm.DB

// SetSessionDB wires GORM for durable shell_sessions rows.
func SetSessionDB(db *gorm.DB) {
	sessionDB = db
}

func persistShellSessionCreate(ms *managedSession, user *cliUserContext) {
	if sessionDB == nil || ms == nil || user == nil {
		return
	}
	now := time.Now().UTC()
	row := models.ShellSession{
		ID:           ms.ID,
		UserID:       user.UserID,
		Title:        ms.Title,
		ShellUser:    user.ShellUser,
		HomeDir:      user.HomeDir,
		Shell:        user.Shell,
		Cwd:          user.Cwd,
		Cols:         defaultSessionCols,
		Rows:         defaultSessionRows,
		Status:       models.ShellSessionActive,
		LastActiveAt: &now,
		Hostname:     hostnameSafe(),
	}
	if err := sessionDB.Create(&row).Error; err != nil {
		log.Printf("shell_sessions create: %v", err)
	}
}

func persistShellSessionStatus(id string, status models.ShellSessionStatus) {
	if sessionDB == nil || id == "" {
		return
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":         status,
		"last_active_at": now,
	}
	if status == models.ShellSessionClosed {
		updates["closed_at"] = now
	}
	if err := sessionDB.Model(&models.ShellSession{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		log.Printf("shell_sessions status %s: %v", id, err)
	}
}

func persistShellSessionTouch(id string) {
	if sessionDB == nil || id == "" {
		return
	}
	now := time.Now().UTC()
	_ = sessionDB.Model(&models.ShellSession{}).Where("id = ?", id).Update("last_active_at", now).Error
}

func persistShellSessionResize(id string, cols, rows uint16) {
	if sessionDB == nil || id == "" {
		return
	}
	_ = sessionDB.Model(&models.ShellSession{}).Where("id = ?", id).Updates(map[string]any{
		"cols":           int(cols),
		"rows":           int(rows),
		"last_active_at": time.Now().UTC(),
	}).Error
}

func listShellSessionsFromDB(userID string) ([]models.ShellSession, error) {
	if sessionDB == nil {
		return nil, nil
	}
	var rows []models.ShellSession
	err := sessionDB.
		Where("user_id = ? AND status != ?", userID, models.ShellSessionClosed).
		Order("last_active_at DESC").
		Find(&rows).Error
	return rows, err
}

func markShellSessionClosed(id, userID string) error {
	if sessionDB == nil {
		return nil
	}
	now := time.Now().UTC()
	return sessionDB.Model(&models.ShellSession{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"status":         models.ShellSessionClosed,
			"closed_at":      now,
			"last_active_at": now,
		}).Error
}

func hostnameSafe() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}
