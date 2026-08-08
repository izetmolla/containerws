package list

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
)

type vncSessionListItem struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	FullName    string `json:"full_name"`
	Status      string `json:"status"`
	Address     string `json:"address"`
	NoVncPort   int    `json:"no_vnc_port"`
	VncPort     int    `json:"vnc_port"`
	HasPassword bool   `json:"has_password"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type availableUserItem struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Label    string `json:"label"`
}

func (cc *controller) GetVncSessionsListAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var rows []models.VncSession
	if err := db.WithContext(ctx).
		Preload("User").
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	out := make([]vncSessionListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, toListItem(row))
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
	}))
}

func (cc *controller) GetVncSessionsColumnsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	columns := []fiber.Map{
		{"id": "user", "label": "User", "accessor": "full_name"},
		{"id": "status", "label": "Status", "accessor": "status"},
		{"id": "address", "label": "Address", "accessor": "address"},
		{"id": "no_vnc_port", "label": "noVNC Port", "accessor": "no_vnc_port"},
		{"id": "vnc_port", "label": "VNC Port", "accessor": "vnc_port"},
		{"id": "updated_at", "label": "Updated", "accessor": "updated_at"},
		{"id": "actions", "label": "", "accessor": "actions"},
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": columns,
	}))
}

// GetAvailableUsersAPI returns users that do not yet have a vnc_session row.
func (cc *controller) GetAvailableUsersAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var taken []string
	if err := db.WithContext(ctx).
		Model(&models.VncSession{}).
		Pluck("user_id", &taken).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	q := db.WithContext(ctx).Model(&models.User{}).Order("username ASC, email ASC")
	if len(taken) > 0 {
		q = q.Where("id NOT IN ?", taken)
	}

	var users []models.User
	if err := q.Find(&users).Error; err != nil {
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

func toListItem(row models.VncSession) vncSessionListItem {
	return vncSessionListItem{
		ID:          row.ID,
		UserID:      row.UserID,
		Username:    row.User.Username,
		Email:       row.User.Email,
		FullName:    fullName(row.User),
		Status:      row.Status,
		Address:     row.Address,
		NoVncPort:   row.NoVncPort,
		VncPort:     row.VncPort,
		HasPassword: row.VncPassword != "",
		CreatedAt:   row.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   row.UpdatedAt.Format("2006-01-02 15:04:05"),
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
