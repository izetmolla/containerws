package list

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
)

type codeserverSessionListItem struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	FullName   string `json:"full_name"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Path       string `json:"path"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Pid        int    `json:"pid"`
	Live       bool   `json:"live"`
	ConnectURL string `json:"connect_url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type availableUserItem struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Label    string `json:"label"`
}

func (cc *controller) GetCodeserverSessionsListAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	auth := cc.app.Authorization()
	authUser, err := auth.User(c, ctx, true)
	if err != nil || authUser == nil || authUser.UserID == "" {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized), r.WithErrorCode("UNAUTHORIZED"))
	}
	roles := cc.app.FreshUserRoles(ctx, authUser.UserID, authUser.Roles)
	isAdmin := userHasAdminRole(cc.app, roles)

	mineOnly := strings.EqualFold(strings.TrimSpace(c.Query("mine")), "1") ||
		strings.EqualFold(strings.TrimSpace(c.Query("mine")), "true")

	q := db.WithContext(ctx).Preload("User").Order("updated_at DESC")
	if !isAdmin || mineOnly {
		q = q.Where("user_id = ?", authUser.UserID)
	}

	var rows []models.CodeserverSession
	if err := q.Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out := make([]codeserverSessionListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, toListItem(row))
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":     out,
		"is_admin": isAdmin,
	}))
}

func (cc *controller) GetCodeserverSessionsColumnsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	columns := []fiber.Map{
		{"id": "name", "label": "Workspace", "accessor": "name"},
		{"id": "user", "label": "User", "accessor": "full_name"},
		{"id": "status", "label": "Status", "accessor": "status"},
		{"id": "path", "label": "Folder", "accessor": "path"},
		{"id": "endpoint", "label": "Endpoint", "accessor": "port"},
		{"id": "updated_at", "label": "Updated", "accessor": "updated_at"},
		{"id": "actions", "label": "", "accessor": "actions"},
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": columns,
	}))
}

// GetAvailableUsersAPI returns assignable panel users for admin workspace create.
func (cc *controller) GetAvailableUsersAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	auth := cc.app.Authorization()
	authUser, err := auth.User(c, ctx, true)
	if err != nil || authUser == nil || authUser.UserID == "" {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusUnauthorized), r.WithErrorCode("UNAUTHORIZED"))
	}
	roles := cc.app.FreshUserRoles(ctx, authUser.UserID, authUser.Roles)
	if !userHasAdminRole(cc.app, roles) {
		return r.Api(c, r.WithError(errors.New("admin only")), r.WithStatus(fiber.StatusForbidden), r.WithErrorCode("FORBIDDEN"))
	}

	var users []models.User
	if err := db.WithContext(ctx).Model(&models.User{}).Order("username ASC, email ASC").Find(&users).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out := make([]availableUserItem, 0, len(users))
	for _, u := range users {
		full := fullName(u)
		label := full
		if u.Username != "" {
			if label != "" {
				label = full + " (" + u.Username + ")"
			} else {
				label = u.Username
			}
		} else if u.Email != "" && label == "" {
			label = u.Email
		}
		if label == "" {
			label = u.ID
		}
		out = append(out, availableUserItem{
			ID:       u.ID,
			Username: u.Username,
			Email:    u.Email,
			FullName: full,
			Label:    label,
		})
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
	}))
}

func toListItem(row models.CodeserverSession) codeserverSessionListItem {
	live := row.Port > 0 && adduser.IsLocalPortListening(row.Port)
	return codeserverSessionListItem{
		ID:         row.ID,
		UserID:     row.UserID,
		Username:   row.User.Username,
		Email:      row.User.Email,
		FullName:   fullName(row.User),
		Name:       models.CodeserverWorkspaceName(row.Name, row.Path),
		Status:     row.Status,
		Path:       row.Path,
		Address:    row.Address,
		Port:       row.Port,
		Pid:        row.Pid,
		Live:       live,
		ConnectURL: codeserver.PublicClientURLForFolder(row.ID, row.Path),
		CreatedAt:  row.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:  row.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func fullName(u models.User) string {
	first := u.FirstName
	last := u.LastName
	if first == "" && last == "" {
		return ""
	}
	if first == "" {
		return last
	}
	if last == "" {
		return first
	}
	return first + " " + last
}
