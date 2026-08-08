package certificates

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

func publicCert(c *models.ProxyCertificate, includeSecrets bool) fiber.Map {
	m := fiber.Map{
		"id":                 c.ID,
		"name":               c.Name,
		"domains":            c.Domains,
		"source":             c.Source,
		"cert_path":          c.CertPath,
		"key_path":           c.KeyPath,
		"letsencrypt_email":  c.LetsEncryptEmail,
		"letsencrypt_status": c.LetsEncryptStatus,
		"expires_at":         c.ExpiresAt,
		"notes":              c.Notes,
		"created_at":         c.CreatedAt,
		"updated_at":         c.UpdatedAt,
		"has_cert_pem":       strings.TrimSpace(c.CertPEM) != "",
		"has_key_pem":        strings.TrimSpace(c.KeyPEM) != "",
	}
	if includeSecrets {
		m["cert_pem"] = c.CertPEM
		m["key_pem"] = c.KeyPEM
	}
	return m
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var rows []models.ProxyCertificate
	if err := cc.app.DB().WithContext(c.Context()).Order("name asc").Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := make([]fiber.Map, 0, len(rows))
	for i := range rows {
		out = append(out, publicCert(&rows[i], false))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyCertificate
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": publicCert(&row, true)}))
}

type certBody struct {
	Name              string `json:"name"`
	Domains           string `json:"domains"`
	Source            string `json:"source"`
	CertPEM           string `json:"cert_pem"`
	KeyPEM            string `json:"key_pem"`
	CertPath          string `json:"cert_path"`
	KeyPath           string `json:"key_path"`
	LetsEncryptEmail  string `json:"letsencrypt_email"`
	Notes             string `json:"notes"`
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body certBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(body.Name) == "" {
		return r.Api(c, r.WithError(errors.New("name is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	row := models.ProxyCertificate{
		Name:             body.Name,
		Domains:          body.Domains,
		Source:           body.Source,
		CertPEM:          body.CertPEM,
		KeyPEM:           body.KeyPEM,
		CertPath:         body.CertPath,
		KeyPath:          body.KeyPath,
		LetsEncryptEmail: body.LetsEncryptEmail,
		Notes:            body.Notes,
	}
	row.Normalize()
	if row.Source == models.ProxyCertLetsEncrypt {
		row.LetsEncryptStatus = "stub"
	}
	if err := cc.app.DB().WithContext(c.Context()).Create(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{"data": publicCert(&row, false), "message": "Certificate created"}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var row models.ProxyCertificate
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body certBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if body.Name != "" {
		row.Name = body.Name
	}
	row.Domains = body.Domains
	if body.Source != "" {
		row.Source = body.Source
	}
	if body.CertPEM != "" {
		row.CertPEM = body.CertPEM
	}
	if body.KeyPEM != "" {
		row.KeyPEM = body.KeyPEM
	}
	row.CertPath = body.CertPath
	row.KeyPath = body.KeyPath
	row.LetsEncryptEmail = body.LetsEncryptEmail
	row.Notes = body.Notes
	row.Normalize()
	if err := cc.app.DB().WithContext(c.Context()).Save(&row).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": publicCert(&row, false), "message": "Certificate updated"}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	res := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).Delete(&models.ProxyCertificate{})
	if res.Error != nil {
		return r.Api(c, r.WithError(res.Error), r.WithStatus(fiber.StatusInternalServerError))
	}
	if res.RowsAffected == 0 {
		return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
	}
	_ = proxymanager.MarkDirty(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Certificate deleted"}))
}
