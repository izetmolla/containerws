package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Proxy engine and runtime identifiers.
const (
	ProxyEngineFiber   = "fiber"
	ProxyEngineNginx   = "nginx"
	ProxyEngineTraefik = "traefik"

	ProxyRuntimeHost   = "host"
	ProxyRuntimeDocker = "docker"

	// Docker network modes for nginx/traefik containers.
	ProxyDockerNetPublished = "published" // bridge + port publish (default)
	ProxyDockerNetHost      = "host"      // network_mode=host
	ProxyDockerNetMacvlan   = "macvlan"   // attach to named network (macvlan/ipvlan/custom) + optional static IP

	ProxySettingsSingletonID = "default"
)

// ProxySettings is the singleton proxy-manager configuration row.
type ProxySettings struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	ActiveEngine string `json:"active_engine" gorm:"size:16;not null;default:fiber;index"` // fiber|nginx|traefik

	NginxRuntime   string `json:"nginx_runtime" gorm:"size:16;not null;default:docker"`   // host|docker
	TraefikRuntime string `json:"traefik_runtime" gorm:"size:16;not null;default:docker"` // host|docker

	HTTPPort  int `json:"http_port" gorm:"not null;default:80"`
	HTTPSPort int `json:"https_port" gorm:"not null;default:443"`

	// Docker runtime options (nginx / traefik).
	DockerEnvironmentID  string `json:"docker_environment_id" gorm:"size:36;index"`
	NginxImage           string `json:"nginx_image" gorm:"size:255;default:nginx:alpine"`
	TraefikImage         string `json:"traefik_image" gorm:"size:255;default:traefik:v3.3"`
	NginxContainerName   string `json:"nginx_container_name" gorm:"size:128;default:cws-proxy-nginx"`
	TraefikContainerName string `json:"traefik_container_name" gorm:"size:128;default:cws-proxy-traefik"`

	// Docker networking (when runtime=docker). Avoids relying only on host port publishing.
	DockerNetworkMode  string `json:"docker_network_mode" gorm:"size:16;not null;default:published"` // published|host|macvlan
	DockerPublishIP    string `json:"docker_publish_ip" gorm:"size:64"`                              // host IP for published ports (empty = 0.0.0.0)
	DockerNetworkName  string `json:"docker_network_name" gorm:"size:128"`                           // macvlan/custom network name
	DockerIPv4Address  string `json:"docker_ipv4_address" gorm:"size:64"`                            // optional static IP on that network

	// Host runtime paths / units.
	NginxBinaryPath   string `json:"nginx_binary_path" gorm:"size:512"`
	NginxConfigPath   string `json:"nginx_config_path" gorm:"size:512"`
	NginxSystemdUnit  string `json:"nginx_systemd_unit" gorm:"size:128;default:nginx"`
	TraefikBinaryPath string `json:"traefik_binary_path" gorm:"size:512"`
	TraefikConfigPath string `json:"traefik_config_path" gorm:"size:512"`
	TraefikSystemdUnit string `json:"traefik_systemd_unit" gorm:"size:128;default:traefik"`

	ConfigDir string `json:"config_dir" gorm:"size:512;not null"`

	Dirty            bool       `json:"dirty" gorm:"not null;default:true"`
	LastAppliedAt    *time.Time `json:"last_applied_at"`
	LastApplyError   string     `json:"last_apply_error" gorm:"type:text"`
	LastApplyEngine  string     `json:"last_apply_engine" gorm:"size:16"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *ProxySettings) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = ProxySettingsSingletonID
	}
	b.Normalize()
	return
}

func (b *ProxySettings) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (ProxySettings) TableName() string {
	return "proxy_settings"
}

// Normalize fills defaults and lowercases engine/runtime fields.
func (b *ProxySettings) Normalize() {
	b.ActiveEngine = strings.ToLower(strings.TrimSpace(b.ActiveEngine))
	switch b.ActiveEngine {
	case ProxyEngineFiber, ProxyEngineNginx, ProxyEngineTraefik:
	default:
		b.ActiveEngine = ProxyEngineFiber
	}

	b.NginxRuntime = strings.ToLower(strings.TrimSpace(b.NginxRuntime))
	if b.NginxRuntime != ProxyRuntimeHost && b.NginxRuntime != ProxyRuntimeDocker {
		b.NginxRuntime = ProxyRuntimeDocker
	}
	b.TraefikRuntime = strings.ToLower(strings.TrimSpace(b.TraefikRuntime))
	if b.TraefikRuntime != ProxyRuntimeHost && b.TraefikRuntime != ProxyRuntimeDocker {
		b.TraefikRuntime = ProxyRuntimeDocker
	}

	if b.HTTPPort <= 0 {
		b.HTTPPort = 80
	}
	if b.HTTPSPort <= 0 {
		b.HTTPSPort = 443
	}
	if strings.TrimSpace(b.NginxImage) == "" {
		b.NginxImage = "nginx:alpine"
	}
	if strings.TrimSpace(b.TraefikImage) == "" {
		b.TraefikImage = "traefik:v3.3"
	}
	if strings.TrimSpace(b.NginxContainerName) == "" {
		b.NginxContainerName = "cws-proxy-nginx"
	}
	if strings.TrimSpace(b.TraefikContainerName) == "" {
		b.TraefikContainerName = "cws-proxy-traefik"
	}
	if strings.TrimSpace(b.NginxSystemdUnit) == "" {
		b.NginxSystemdUnit = "nginx"
	}
	if strings.TrimSpace(b.TraefikSystemdUnit) == "" {
		b.TraefikSystemdUnit = "traefik"
	}
	if strings.TrimSpace(b.ConfigDir) == "" {
		// Durable volume path; packages/proxymanager.ResolveConfigDir may override for
		// development (./tmp/proxymanager) or PROXYMANAGER_CONFIG_DIR.
		b.ConfigDir = "/config/containerws/proxymanager"
	}

	b.DockerNetworkMode = strings.ToLower(strings.TrimSpace(b.DockerNetworkMode))
	switch b.DockerNetworkMode {
	case ProxyDockerNetPublished, ProxyDockerNetHost, ProxyDockerNetMacvlan:
	case "bridge", "": // legacy / empty → published ports
		b.DockerNetworkMode = ProxyDockerNetPublished
	default:
		b.DockerNetworkMode = ProxyDockerNetPublished
	}
	b.DockerPublishIP = strings.TrimSpace(b.DockerPublishIP)
	b.DockerNetworkName = strings.TrimSpace(b.DockerNetworkName)
	b.DockerIPv4Address = strings.TrimSpace(b.DockerIPv4Address)
}

// DockerUsesPublishedPorts is true when the container gets host port mappings.
func (b *ProxySettings) DockerUsesPublishedPorts() bool {
	if b == nil {
		return true
	}
	mode := b.DockerNetworkMode
	if mode == "" {
		mode = ProxyDockerNetPublished
	}
	return mode == ProxyDockerNetPublished || mode == "bridge"
}

// NginxContainerListenPorts returns the ports nginx should listen on inside the container.
// Published-port mode keeps classic 80/443 inside and maps host ports onto them.
func (b *ProxySettings) NginxContainerListenPorts() (httpPort, httpsPort int) {
	if b == nil {
		return 80, 443
	}
	if b.NginxRuntime == ProxyRuntimeDocker && b.DockerUsesPublishedPorts() {
		return 80, 443
	}
	httpPort, httpsPort = b.HTTPPort, b.HTTPSPort
	if httpPort <= 0 {
		httpPort = 80
	}
	if httpsPort <= 0 {
		httpsPort = 443
	}
	return httpPort, httpsPort
}

// NewDefaultProxySettings returns a ready-to-create singleton.
func NewDefaultProxySettings() *ProxySettings {
	s := &ProxySettings{
		ID:           ProxySettingsSingletonID,
		ActiveEngine: ProxyEngineFiber,
		Dirty:        true,
	}
	s.Normalize()
	return s
}
