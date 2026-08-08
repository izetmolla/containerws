// Package usersync reconciles Linux accounts with panel users in SQLite.
package usersync

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"github.com/izetmolla/goauth"
	"gorm.io/gorm"
)

var startOnce sync.Once

// Result summarizes a Sync pass.
type Result struct {
	LinuxToDBCreated int
	DBToLinuxCreated int
	Skipped          int
	Errors           []string
}

// StartAsync runs Sync once per process in the background (server start).
func StartAsync(db *gorm.DB) {
	if db == nil {
		return
	}
	startOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			res, err := Sync(ctx, db)
			if err != nil {
				log.Printf("usersync: failed: %v", err)
				return
			}
			log.Printf(
				"usersync: done linux→db=%d db→linux=%d skipped=%d errors=%d",
				res.LinuxToDBCreated, res.DBToLinuxCreated, res.Skipped, len(res.Errors),
			)
			for _, e := range res.Errors {
				log.Printf("usersync: %s", e)
			}
		}()
	})
}

// Sync brings Linux login users and panel users into alignment:
//   - Linux account missing in DB → create panel user
//   - Panel user (with username) missing on Linux → create Linux account (+home)
//
// Panel password hashes cannot seed Linux passwords; new Linux accounts are
// created without a login password (set later via the panel).
func Sync(ctx context.Context, db *gorm.DB) (*Result, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	res := &Result{}

	linuxAccounts, err := linuxuser.ListLoginAccounts()
	if err != nil {
		return nil, err
	}

	var panelUsers []models.User
	if err := db.WithContext(ctx).Find(&panelUsers).Error; err != nil {
		return nil, err
	}

	byUsername := map[string]*models.User{}
	byLdap := map[string]*models.User{}
	for i := range panelUsers {
		u := &panelUsers[i]
		if name := strings.TrimSpace(u.Username); name != "" {
			byUsername[strings.ToLower(name)] = u
		}
		if name := strings.TrimSpace(u.LdapUsername); name != "" {
			byLdap[strings.ToLower(name)] = u
		}
	}

	linuxNames := map[string]struct{}{}
	for _, acc := range linuxAccounts {
		name := strings.TrimSpace(acc.Username)
		if name == "" {
			continue
		}
		linuxNames[strings.ToLower(name)] = struct{}{}

		if findPanelUser(byUsername, byLdap, name) != nil {
			continue
		}
		if err := createPanelUserFromLinux(ctx, db, acc); err != nil {
			res.Errors = append(res.Errors, "linux→db "+name+": "+err.Error())
			continue
		}
		res.LinuxToDBCreated++
		log.Printf("usersync: created panel user for linux account %q", name)
	}

	for i := range panelUsers {
		u := &panelUsers[i]
		linuxName := linuxNameForPanel(u)
		if linuxName == "" {
			res.Skipped++
			continue
		}
		if !shouldProvisionLinux(u) {
			res.Skipped++
			continue
		}
		if _, exists := linuxNames[strings.ToLower(linuxName)]; exists {
			continue
		}
		acc, err := linuxuser.Lookup(linuxName)
		if err == nil && acc != nil && acc.Exists {
			linuxNames[strings.ToLower(linuxName)] = struct{}{}
			continue
		}
		if linuxName == "root" {
			res.Skipped++
			continue
		}

		fullName := strings.TrimSpace(u.FirstName + " " + u.LastName)
		if _, err := linuxuser.Create(linuxuser.CreateOptions{
			Username:   linuxName,
			FullName:   fullName,
			Shell:      "/bin/bash",
			CreateHome: true,
			// No password — panel hash cannot be reversed; set via UI later.
		}); err != nil {
			res.Errors = append(res.Errors, "db→linux "+linuxName+": "+err.Error())
			continue
		}
		linuxNames[strings.ToLower(linuxName)] = struct{}{}
		res.DBToLinuxCreated++
		log.Printf("usersync: created linux account for panel user %q", linuxName)
	}

	return res, nil
}

func findPanelUser(byUsername, byLdap map[string]*models.User, linuxName string) *models.User {
	key := strings.ToLower(strings.TrimSpace(linuxName))
	if u := byUsername[key]; u != nil {
		return u
	}
	return byLdap[key]
}

func linuxNameForPanel(u *models.User) string {
	if name := strings.TrimSpace(u.Username); name != "" {
		return name
	}
	return strings.TrimSpace(u.LdapUsername)
}

func shouldProvisionLinux(u *models.User) bool {
	switch u.Status {
	case models.Deleted, models.Disabled, models.Suspended, models.Inactive:
		return false
	default:
		return true
	}
}

func createPanelUserFromLinux(ctx context.Context, db *gorm.DB, acc linuxuser.Account) error {
	linuxName := strings.TrimSpace(acc.Username)
	email := linuxName + "@localhost"

	// Avoid colliding with an existing email-only panel row.
	var existing models.User
	err := db.WithContext(ctx).
		Where("username = ? OR ldap_username = ? OR email = ? OR email = ?",
			linuxName, linuxName, email, linuxName).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	roles := goauth.JSONBArray([]any{"user"})
	firstName := ""
	lastName := ""
	if linuxName == "root" {
		roles = goauth.JSONBArray([]any{"admin"})
		firstName = "Root"
		lastName = "Administrator"
	} else if gecos := strings.TrimSpace(acc.Name); gecos != "" {
		// GECOS is often "Full Name,..." — take the name part.
		gecos, _, _ = strings.Cut(gecos, ",")
		parts := strings.Fields(strings.TrimSpace(gecos))
		if len(parts) > 0 {
			firstName = parts[0]
		}
		if len(parts) > 1 {
			lastName = strings.Join(parts[1:], " ")
		}
	}

	user := models.User{
		Username:    linuxName,
		FirstName:   firstName,
		LastName:    lastName,
		Email:       email,
		Password:    "!", // invalid goauth hash — panel password login disabled until set
		Status:      models.Active,
		IsConfirmed: true,
		Roles:       roles,
	}
	return db.WithContext(ctx).Omit("LdapUsername").Create(&user).Error
}
