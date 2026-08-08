package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/setup"
	"gorm.io/gorm"
)

type SessionRuntime struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	Live      bool   `json:"live"`
	Address   string `json:"address"`
	VncPort   int    `json:"vnc_port"`
	NoVncPort int    `json:"no_vnc_port"`
	LogPath   string `json:"log_path,omitempty"`
}

type StatusReport struct {
	PackagesReady  bool             `json:"packages_ready"`
	Desired        string           `json:"desired"`
	Running        bool             `json:"running"`
	LiveSessions   int              `json:"live_sessions"`
	ActiveSessions int              `json:"active_sessions"`
	Sessions       []SessionRuntime `json:"sessions"`
	ServiceLog     string           `json:"service_log"`
	CheckedAt      time.Time        `json:"checked_at"`
}

func CollectStatus(db *gorm.DB) StatusReport {
	pkg := setup.CheckStatus()
	desired := loadDesired()
	report := StatusReport{
		PackagesReady: pkg.Ready,
		Desired:       desired,
		Sessions:      []SessionRuntime{},
		ServiceLog:    serviceLogPath(),
		CheckedAt:     time.Now().UTC(),
	}
	if db == nil {
		report.Running = desired == DesiredRunning
		return report
	}

	var rows []models.VncSession
	_ = db.Preload("User").Find(&rows).Error
	for _, row := range rows {
		username := strings.TrimSpace(row.User.Username)
		live := adduser.IsSessionLive(username, row.VncPort, row.NoVncPort)
		if strings.EqualFold(row.Status, models.VncSessionStatusActive) {
			report.ActiveSessions++
		}
		if live {
			report.LiveSessions++
		}
		item := SessionRuntime{
			ID:        row.ID,
			UserID:    row.UserID,
			Username:  username,
			Status:    row.Status,
			Live:      live,
			Address:   row.Address,
			VncPort:   row.VncPort,
			NoVncPort: row.NoVncPort,
		}
		if username != "" {
			item.LogPath = adduser.SessionLogPath(username)
		}
		report.Sessions = append(report.Sessions, item)
	}

	// Start/Stop button: live sessions win; desired=running keeps "running"
	// even while sessions are still coming up.
	report.Running = report.LiveSessions > 0 || desired == DesiredRunning
	return report
}

type actionResult struct {
	Started  int              `json:"started"`
	Stopped  int              `json:"stopped"`
	Skipped  int              `json:"skipped"`
	Errors   []string         `json:"errors,omitempty"`
	Sessions []SessionRuntime `json:"sessions"`
}

func StartAll(db *gorm.DB) (actionResult, error) {
	res := actionResult{Sessions: []SessionRuntime{}}
	if db == nil {
		return res, fmt.Errorf("database unavailable")
	}
	_ = saveDesired(DesiredRunning)
	appendServiceLog("service start requested")

	var rows []models.VncSession
	if err := db.Preload("User").Where("status = ?", models.VncSessionStatusActive).Find(&rows).Error; err != nil {
		return res, err
	}
	for _, row := range rows {
		username := strings.TrimSpace(row.User.Username)
		password := strings.TrimSpace(row.VncPassword)
		if username == "" || password == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: missing username or vnc password", row.ID))
			continue
		}
		started, err := adduser.StartUserSession(adduser.StartOptions{
			Username:  username,
			Password:  password,
			VncPort:   row.VncPort,
			NoVncPort: row.NoVncPort,
		})
		if err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("%s (%s): %v", username, row.ID, err))
			appendServiceLog(fmt.Sprintf("start failed %s: %v", username, err))
			continue
		}
		_ = db.Model(&row).Updates(map[string]any{
			"address":     started.Address,
			"vnc_port":    started.VncPort,
			"no_vnc_port": started.NoVncPort,
		}).Error
		res.Started++
		appendServiceLog(fmt.Sprintf("started %s vnc=%d novnc=%d", username, started.VncPort, started.NoVncPort))
		res.Sessions = append(res.Sessions, SessionRuntime{
			ID:        row.ID,
			UserID:    row.UserID,
			Username:  username,
			Status:    models.VncSessionStatusActive,
			Live:      true,
			Address:   started.Address,
			VncPort:   started.VncPort,
			NoVncPort: started.NoVncPort,
			LogPath:   adduser.SessionLogPath(username),
		})
	}
	appendServiceLog(fmt.Sprintf("service start finished started=%d skipped=%d", res.Started, res.Skipped))
	return res, nil
}

func StopAll(db *gorm.DB) (actionResult, error) {
	res := actionResult{Sessions: []SessionRuntime{}}
	if db == nil {
		return res, fmt.Errorf("database unavailable")
	}
	_ = saveDesired(DesiredStopped)
	appendServiceLog("service stop requested")

	var rows []models.VncSession
	if err := db.Preload("User").Find(&rows).Error; err != nil {
		return res, err
	}
	for _, row := range rows {
		username := strings.TrimSpace(row.User.Username)
		if username == "" {
			res.Skipped++
			continue
		}
		if err := adduser.StopUserSession(username, row.VncPort, row.NoVncPort); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", username, err))
			appendServiceLog(fmt.Sprintf("stop error %s: %v", username, err))
		} else {
			res.Stopped++
			appendServiceLog(fmt.Sprintf("stopped %s", username))
		}
		res.Sessions = append(res.Sessions, SessionRuntime{
			ID:        row.ID,
			UserID:    row.UserID,
			Username:  username,
			Status:    row.Status,
			Live:      false,
			Address:   row.Address,
			VncPort:   row.VncPort,
			NoVncPort: row.NoVncPort,
			LogPath:   adduser.SessionLogPath(username),
		})
	}
	appendServiceLog(fmt.Sprintf("service stop finished stopped=%d", res.Stopped))
	return res, nil
}

func collectLogSources(db *gorm.DB) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	add(serviceLogPath())
	if db != nil {
		var rows []models.VncSession
		_ = db.Preload("User").Find(&rows).Error
		for _, row := range rows {
			username := strings.TrimSpace(row.User.Username)
			if username == "" {
				continue
			}
			add(adduser.SessionLogPath(username))
			// TigerVNC session logs under the user home when present.
			if uHome := linuxHome(username); uHome != "" {
				entries, _ := filepath.Glob(filepath.Join(uHome, ".config", "tigervnc", "*.log"))
				for _, e := range entries {
					add(e)
				}
			}
		}
	}
	return paths
}

func linuxHome(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	prefix := username + ":"
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 6 {
			return parts[5]
		}
	}
	return ""
}
