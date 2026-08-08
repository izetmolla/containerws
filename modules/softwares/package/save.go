package softwarespackage

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

type updateSoftwareBody struct {
	Name           *string  `json:"name"`
	Details        *string  `json:"details"`
	Category       *string  `json:"category"`
	SubCategory    *string  `json:"sub_category"`
	Tags           []string `json:"tags"`
	ServiceUnits   []string `json:"service_units"`
	CanControl     *bool    `json:"can_control"`
	ControlBackend *string  `json:"control_backend"`
	StartCommand   *string  `json:"start_command"`
	RestartCommand *string  `json:"restart_command"`
	StopCommand    *string  `json:"stop_command"`
	Icon           *string  `json:"icon"`
	Image          *string  `json:"image"`
	Color          *string  `json:"color"`
	Order          *int     `json:"order"`
	IsActive       *bool    `json:"is_active"`
}

func (cc *controller) UpdateSoftwareAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return r.Api(c, r.WithError(errors.New("software id is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body updateSoftwareBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	sw, err := gorm.G[models.Software](db).Where("id = ?", id).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithErrorCode("NOT_FOUND"), r.WithError(errors.New("software not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return r.Api(c, r.WithError(errors.New("name cannot be empty")), r.WithStatus(fiber.StatusBadRequest))
		}
		sw.Name = name
	}
	if body.Details != nil {
		sw.Details = strings.TrimSpace(*body.Details)
	}
	if body.Category != nil {
		sw.Category = strings.TrimSpace(*body.Category)
	}
	if body.SubCategory != nil {
		sw.SubCategory = strings.TrimSpace(*body.SubCategory)
	}
	if body.Tags != nil {
		sw.Tags = models.JSONBStringArray(trimStringSlice(body.Tags))
	}
	if body.ServiceUnits != nil {
		sw.ServiceUnits = models.JSONBStringArray(trimStringSlice(body.ServiceUnits))
	}
	if body.CanControl != nil {
		sw.CanControl = *body.CanControl
	} else if body.ServiceUnits != nil {
		// Setting units without an explicit flag enables control when units are present.
		sw.CanControl = models.HasSoftwareServiceUnits(sw.ServiceUnits)
	}
	if body.ControlBackend != nil {
		sw.ControlBackend = strings.TrimSpace(*body.ControlBackend)
	}
	if body.StartCommand != nil {
		sw.StartCommand = strings.TrimSpace(*body.StartCommand)
	}
	if body.RestartCommand != nil {
		sw.RestartCommand = strings.TrimSpace(*body.RestartCommand)
	}
	if body.StopCommand != nil {
		sw.StopCommand = strings.TrimSpace(*body.StopCommand)
	}
	if body.Icon != nil {
		sw.Icon = strings.TrimSpace(*body.Icon)
	}
	if body.Image != nil {
		sw.Image = strings.TrimSpace(*body.Image)
	}
	if body.Color != nil {
		sw.Color = strings.TrimSpace(*body.Color)
	}
	if body.Order != nil {
		sw.Order = *body.Order
	}
	if body.IsActive != nil {
		sw.IsActive = *body.IsActive
	}

	if err := db.WithContext(ctx).Save(&sw).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sw,
		"message": "Software updated",
	}))
}

func trimStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
