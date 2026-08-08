package settings

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
	fiberproxy "github.com/izetmolla/containerws/packages/proxymanager/fiber"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/", cc.GetAPI)
	api.Put("/", cc.UpdateAPI)
	api.Get("/runtime", cc.RuntimeAPI)
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	s, err := proxymanager.EnsureSettings(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": s}))
}

func (cc *controller) RuntimeAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	st, err := proxymanager.DetectRuntime(c.Context(), cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": st}))
}

type updateBody struct {
	ActiveEngine         string `json:"active_engine"`
	NginxRuntime         string `json:"nginx_runtime"`
	TraefikRuntime       string `json:"traefik_runtime"`
	HTTPPort             *int   `json:"http_port"`
	HTTPSPort            *int   `json:"https_port"`
	DockerEnvironmentID  string `json:"docker_environment_id"`
	NginxImage           string `json:"nginx_image"`
	TraefikImage         string `json:"traefik_image"`
	NginxContainerName   string `json:"nginx_container_name"`
	TraefikContainerName string `json:"traefik_container_name"`
	DockerNetworkMode    string `json:"docker_network_mode"`
	DockerPublishIP      string `json:"docker_publish_ip"`
	DockerNetworkName    string `json:"docker_network_name"`
	DockerIPv4Address    string `json:"docker_ipv4_address"`
	NginxBinaryPath      string `json:"nginx_binary_path"`
	NginxConfigPath      string `json:"nginx_config_path"`
	NginxSystemdUnit     string `json:"nginx_systemd_unit"`
	TraefikBinaryPath    string `json:"traefik_binary_path"`
	TraefikConfigPath    string `json:"traefik_config_path"`
	TraefikSystemdUnit   string `json:"traefik_systemd_unit"`
	ConfigDir            string `json:"config_dir"`
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	s, err := proxymanager.EnsureSettings(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	prevEngine := s.ActiveEngine

	var body updateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if body.ActiveEngine != "" {
		s.ActiveEngine = body.ActiveEngine
	}
	if body.NginxRuntime != "" {
		s.NginxRuntime = body.NginxRuntime
	}
	if body.TraefikRuntime != "" {
		s.TraefikRuntime = body.TraefikRuntime
	}
	if body.HTTPPort != nil {
		s.HTTPPort = *body.HTTPPort
	}
	if body.HTTPSPort != nil {
		s.HTTPSPort = *body.HTTPSPort
	}
	s.DockerEnvironmentID = body.DockerEnvironmentID
	if body.NginxImage != "" {
		s.NginxImage = body.NginxImage
	}
	if body.TraefikImage != "" {
		s.TraefikImage = body.TraefikImage
	}
	if body.NginxContainerName != "" {
		s.NginxContainerName = body.NginxContainerName
	}
	if body.TraefikContainerName != "" {
		s.TraefikContainerName = body.TraefikContainerName
	}
	if body.DockerNetworkMode != "" {
		s.DockerNetworkMode = body.DockerNetworkMode
	}
	s.DockerPublishIP = body.DockerPublishIP
	s.DockerNetworkName = body.DockerNetworkName
	s.DockerIPv4Address = body.DockerIPv4Address
	s.NginxBinaryPath = body.NginxBinaryPath
	s.NginxConfigPath = body.NginxConfigPath
	if body.NginxSystemdUnit != "" {
		s.NginxSystemdUnit = body.NginxSystemdUnit
	}
	s.TraefikBinaryPath = body.TraefikBinaryPath
	s.TraefikConfigPath = body.TraefikConfigPath
	if body.TraefikSystemdUnit != "" {
		s.TraefikSystemdUnit = body.TraefikSystemdUnit
	}
	if body.ConfigDir != "" {
		s.ConfigDir = body.ConfigDir
	}
	s.Normalize()
	switch s.ActiveEngine {
	case models.ProxyEngineFiber, models.ProxyEngineNginx, models.ProxyEngineTraefik:
	default:
		return r.Api(c, r.WithError(errors.New("invalid active_engine")), r.WithStatus(fiber.StatusBadRequest))
	}

	if err := proxymanager.SaveSettings(cc.app.DB(), s); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if prevEngine == models.ProxyEngineFiber && s.ActiveEngine != models.ProxyEngineFiber {
		fiberproxy.Clear()
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": s, "message": "Settings saved"}))
}
