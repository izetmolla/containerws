package redirects

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	list := api.Group("/list")
	list.Get("/", cc.ListAPI)
	single := api.Group("/single")
	single.Post("/", cc.CreateAPI)
	single.Get("/:id", cc.GetAPI)
	single.Put("/:id", cc.UpdateAPI)
	single.Delete("/:id", cc.DeleteAPI)
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var rows []models.ProxyRedirect
	if err := cc.app.DB().WithContext(c.Context()).Order("order_nr asc, from_host asc").Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyRedirect
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": row}))
}

type redirectBody struct {
	Name         string `json:"name"`
	Enabled      *bool  `json:"enabled"`
	FromHost     string `json:"from_host"`
	FromPath     string `json:"from_path"`
	ToURL        string `json:"to_url"`
	StatusCode   int    `json:"status_code"`
	PreservePath bool   `json:"preserve_path"`
	OrderNr      int    `json:"order_nr"`
	Notes        string `json:"notes"`
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body redirectBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.FromHost) == "" || strings.TrimSpace(body.ToURL) == "" {
		return r.Api(c, r.WithError(errors.New("name, from_host and to_url are required")), r.WithStatus(fiber.StatusBadRequest))
	}
	row := models.ProxyRedirect{
		Name:         body.Name,
		Enabled:      true,
		FromHost:     body.FromHost,
		FromPath:     body.FromPath,
		ToURL:        body.ToURL,
		StatusCode:   body.StatusCode,
		PreservePath: body.PreservePath,
		OrderNr:      body.OrderNr,
		Notes:        body.Notes,
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	row.Normalize()
	if err := cc.app.DB().WithContext(c.Context()).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{"data": row, "message": "Redirect created"}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyRedirect
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body redirectBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if body.Name != "" {
		row.Name = body.Name
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.FromHost != "" {
		row.FromHost = body.FromHost
	}
	if body.FromPath != "" {
		row.FromPath = body.FromPath
	}
	if body.ToURL != "" {
		row.ToURL = body.ToURL
	}
	if body.StatusCode != 0 {
		row.StatusCode = body.StatusCode
	}
	row.PreservePath = body.PreservePath
	row.OrderNr = body.OrderNr
	row.Notes = body.Notes
	row.Normalize()
	if err := cc.app.DB().WithContext(c.Context()).Save(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": row, "message": "Redirect updated"}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	res := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).Delete(&models.ProxyRedirect{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Redirect deleted"}))
}
