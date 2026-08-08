package list

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"github.com/izetmolla/goauth"
)

type userListItem struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	Email        string   `json:"email"`
	FirstName    string   `json:"first_name"`
	LastName     string   `json:"last_name"`
	FullName     string   `json:"full_name"`
	Status       string   `json:"status"`
	Roles        []string `json:"roles"`
	IsConfirmed  bool     `json:"is_confirmed"`
	LinuxExists  bool     `json:"linux_exists"`
	LinuxGroups  []string `json:"linux_groups,omitempty"`
	LinuxShell   string   `json:"linux_shell,omitempty"`
	LinuxHome    string   `json:"linux_home,omitempty"`
	LinuxLocked  bool     `json:"linux_locked,omitempty"`
	VncSessionID string   `json:"vnc_session_id,omitempty"`
	VncStatus    string   `json:"vnc_status,omitempty"`
	VncPort      int      `json:"vnc_port,omitempty"`
	NoVncPort    int      `json:"no_vnc_port,omitempty"`
	HasVnc       bool     `json:"has_vnc"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

func (cc *controller) GetUsersListAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var users []models.User
	if err := db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	var sessions []models.VncSession
	_ = db.WithContext(ctx).Find(&sessions).Error
	byUser := map[string]models.VncSession{}
	for _, s := range sessions {
		byUser[s.UserID] = s
	}

	out := make([]userListItem, 0, len(users))
	for _, u := range users {
		item := toListItem(u)
		if acc, err := linuxuser.Lookup(u.Username); err == nil && acc != nil && acc.Exists {
			item.LinuxExists = true
			item.LinuxGroups = acc.Groups
			item.LinuxShell = acc.Shell
			item.LinuxHome = acc.HomeDir
			item.LinuxLocked = acc.Locked
		}
		if s, ok := byUser[u.ID]; ok {
			item.HasVnc = true
			item.VncSessionID = s.ID
			item.VncStatus = s.Status
			item.VncPort = s.VncPort
			item.NoVncPort = s.NoVncPort
		}
		out = append(out, item)
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
	}))
}

func (cc *controller) GetUsersColumnsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	columns := []fiber.Map{
		{"id": "user", "label": "User", "accessor": "full_name"},
		{"id": "status", "label": "Status", "accessor": "status"},
		{"id": "linux", "label": "Linux", "accessor": "linux_exists"},
		{"id": "groups", "label": "Groups", "accessor": "linux_groups"},
		{"id": "vnc", "label": "VNC", "accessor": "has_vnc"},
		{"id": "roles", "label": "Roles", "accessor": "roles"},
		{"id": "updated_at", "label": "Updated", "accessor": "updated_at"},
		{"id": "actions", "label": "", "accessor": "actions"},
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": columns}))
}

func (cc *controller) GetLinuxGroupsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": linuxuser.FormGroups(),
	}))
}

func (cc *controller) GetUserFormOptionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	system, _ := linuxuser.ListGroups()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"groups":        linuxuser.FormGroups(),
			"system_groups": system,
			"common_groups": linuxuser.CommonGroups,
			"shells":        linuxuser.CommonShells,
			"statuses": []string{
				string(models.Active), string(models.Inactive), string(models.Suspended),
				string(models.Disabled), string(models.New), string(models.Pending),
			},
			"panel_roles": []string{"admin", "user", "guest", "operator"},
		},
	}))
}

func toListItem(u models.User) userListItem {
	return userListItem{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		FullName:    fullName(u),
		Status:      string(u.Status),
		Roles:       rolesSlice(u.Roles),
		IsConfirmed: u.IsConfirmed,
		CreatedAt:   u.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func fullName(u models.User) string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

func rolesSlice(roles goauth.JSONBArray) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		s := strings.TrimSpace(fmt.Sprint(r))
		if s != "" && s != "<nil>" {
			out = append(out, s)
		}
	}
	return out
}
