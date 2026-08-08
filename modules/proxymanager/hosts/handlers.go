package hosts

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
	var rows []models.ProxyHost
	if err := cc.app.DB().WithContext(c.Context()).
		Preload("Locations", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("order_nr asc, path_prefix asc")
		}).
		Order("order_nr asc, name asc").
		Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyHost
	if err := cc.app.DB().WithContext(c.Context()).
		Preload("Locations").
		Where("id = ?", c.Params("id")).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": row}))
}

type locationBody struct {
	ID             string          `json:"id"`
	PathPrefix     string          `json:"path_prefix"`
	UpstreamType   string          `json:"upstream_type"`
	UpstreamTarget string          `json:"upstream_target"`
	StripPrefix    bool            `json:"strip_prefix"`
	Websocket      bool            `json:"websocket"`
	Extras         models.JSONBAny `json:"extras"`
	OrderNr        int             `json:"order_nr"`
	Enabled        *bool           `json:"enabled"`
}

type hostBody struct {
	Name           string          `json:"name"`
	Domains        string          `json:"domains"`
	Enabled        *bool           `json:"enabled"`
	ListenScheme   string          `json:"listen_scheme"`
	ForwardScheme  string          `json:"forward_scheme"`
	ForwardHost    string          `json:"forward_host"`
	ForwardPort    *int            `json:"forward_port"`
	UpstreamType   string          `json:"upstream_type"`
	UpstreamTarget string          `json:"upstream_target"`
	Websocket      *bool           `json:"websocket"`
	SSLForced      *bool           `json:"ssl_forced"`
	BlockExploits  *bool           `json:"block_exploits"`
	CachingEnabled *bool           `json:"caching_enabled"`
	HTTP2Support   *bool           `json:"http2_support"`
	CustomHeaders  models.JSONBAny `json:"custom_headers"`
	Notes          string          `json:"notes"`
	CertificateID  *string         `json:"certificate_id"`
	OrderNr        int             `json:"order_nr"`
	Locations      []locationBody  `json:"locations"`
}

func applyHostBody(row *models.ProxyHost, body *hostBody, creating bool) error {
	if creating {
		if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Domains) == "" {
			return errors.New("name and domains are required")
		}
	}
	if body.Name != "" {
		row.Name = body.Name
	}
	if body.Domains != "" {
		row.Domains = body.Domains
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	} else if creating {
		row.Enabled = true
	}
	if body.ListenScheme != "" {
		row.ListenScheme = body.ListenScheme
	}
	if body.ForwardScheme != "" {
		row.ForwardScheme = body.ForwardScheme
	}
	if body.ForwardHost != "" {
		row.ForwardHost = body.ForwardHost
	}
	if body.ForwardPort != nil {
		row.ForwardPort = *body.ForwardPort
	}
	if body.UpstreamType != "" {
		row.UpstreamType = body.UpstreamType
	}
	if body.UpstreamTarget != "" {
		row.UpstreamTarget = body.UpstreamTarget
	}
	if body.Websocket != nil {
		row.Websocket = *body.Websocket
	} else if creating {
		row.Websocket = true
	}
	if body.SSLForced != nil {
		row.SSLForced = *body.SSLForced
	}
	if body.BlockExploits != nil {
		row.BlockExploits = *body.BlockExploits
	} else if creating {
		row.BlockExploits = true
	}
	if body.CachingEnabled != nil {
		row.CachingEnabled = *body.CachingEnabled
	}
	if body.HTTP2Support != nil {
		row.HTTP2Support = *body.HTTP2Support
	} else if creating {
		row.HTTP2Support = true
	}
	if body.CustomHeaders != nil {
		row.CustomHeaders = body.CustomHeaders
	}
	row.Notes = body.Notes
	row.CertificateID = body.CertificateID
	row.OrderNr = body.OrderNr
	row.Normalize()
	if row.UpstreamType == models.ProxyUpstreamURL && strings.TrimSpace(row.UpstreamTarget) == "" && strings.TrimSpace(row.ForwardHost) == "" {
		return errors.New("forward host (or upstream URL) is required")
	}
	return nil
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body hostBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	row := models.ProxyHost{}
	if err := applyHostBody(&row, &body, true); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := cc.app.DB().WithContext(c.Context()).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := replaceLocations(cc.app.DB(), row.ID, body.Locations); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	_ = cc.app.DB().Preload("Locations").Where("id = ?", row.ID).First(&row)
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{"data": row, "message": "Host created"}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyHost
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body hostBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := applyHostBody(&row, &body, false); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := cc.app.DB().WithContext(c.Context()).Save(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if body.Locations != nil {
		if err := replaceLocations(cc.app.DB(), row.ID, body.Locations); err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	_ = cc.app.DB().Preload("Locations").Where("id = ?", row.ID).First(&row)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": row, "message": "Host updated"}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := c.Params("id")
	if err := cc.app.DB().WithContext(c.Context()).Where("host_id = ?", id).Delete(&models.ProxyLocation{}).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	res := cc.app.DB().WithContext(c.Context()).Where("id = ?", id).Delete(&models.ProxyHost{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Host deleted"}))
}

func replaceLocations(db *gorm.DB, hostID string, locs []locationBody) error {
	if err := db.Where("host_id = ?", hostID).Delete(&models.ProxyLocation{}).Error; err != nil {
		return err
	}
	for i, lb := range locs {
		enabled := true
		if lb.Enabled != nil {
			enabled = *lb.Enabled
		}
		loc := models.ProxyLocation{
			HostID:         hostID,
			PathPrefix:     lb.PathPrefix,
			UpstreamType:   lb.UpstreamType,
			UpstreamTarget: lb.UpstreamTarget,
			StripPrefix:    lb.StripPrefix,
			Websocket:      lb.Websocket,
			Extras:         lb.Extras,
			OrderNr:        lb.OrderNr,
			Enabled:        enabled,
		}
		if loc.OrderNr == 0 {
			loc.OrderNr = i
		}
		loc.Normalize()
		if err := db.Create(&loc).Error; err != nil {
			return err
		}
	}
	return nil
}
