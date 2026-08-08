package models

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Upstream / scheme constants for proxy hosts.
const (
	ProxyUpstreamURL     = "url"
	ProxyUpstreamAppPath = "app_path"

	ProxySchemeHTTP  = "http"
	ProxySchemeHTTPS = "https"
	ProxySchemeBoth  = "both"

	ProxyForwardHTTP  = "http"
	ProxyForwardHTTPS = "https"
)

// ProxyHost is a virtual host / domain entry managed by the proxy manager.
// Fields mirror Nginx Proxy Manager proxy_host concepts where practical.
type ProxyHost struct {
	ID           string `json:"id" gorm:"primaryKey;type:text"`
	Name         string `json:"name" gorm:"size:255;not null"`
	Domains      string `json:"domains" gorm:"type:text;not null"` // comma-separated hostnames
	Enabled      bool   `json:"enabled" gorm:"not null;default:true;index"`
	ListenScheme string `json:"listen_scheme" gorm:"size:16;not null;default:http"` // http|https|both

	// NPM-style forward target (preferred). UpstreamTarget is kept in sync.
	ForwardScheme string `json:"forward_scheme" gorm:"size:16;not null;default:http"` // http|https
	ForwardHost   string `json:"forward_host" gorm:"size:255"`
	ForwardPort   int    `json:"forward_port" gorm:"default:80"`

	UpstreamType   string `json:"upstream_type" gorm:"size:16;not null;default:url"` // url|app_path
	UpstreamTarget string `json:"upstream_target" gorm:"size:1024;not null"`         // computed URL or in-app path

	Websocket      bool `json:"websocket" gorm:"not null;default:true"` // allow websocket upgrade
	SSLForced      bool `json:"ssl_forced" gorm:"not null;default:false"`
	BlockExploits  bool `json:"block_exploits" gorm:"not null;default:true"`
	CachingEnabled bool `json:"caching_enabled" gorm:"not null;default:false"`
	HTTP2Support   bool `json:"http2_support" gorm:"not null;default:true"`

	CustomHeaders JSONBAny `json:"custom_headers" gorm:"type:text"`
	Notes         string   `json:"notes" gorm:"type:text"`

	CertificateID *string `json:"certificate_id" gorm:"size:36;index"`
	OrderNr       int     `json:"order_nr" gorm:"not null;default:0;index"`

	Locations []ProxyLocation `json:"locations,omitempty" gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *ProxyHost) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *ProxyHost) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (ProxyHost) TableName() string {
	return "proxy_hosts"
}

// DomainList returns trimmed non-empty domain names.
func (b *ProxyHost) DomainList() []string {
	parts := strings.Split(b.Domains, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Normalize cleans fields and keeps UpstreamTarget in sync with forward_* fields.
func (b *ProxyHost) Normalize() {
	b.Name = strings.TrimSpace(b.Name)
	b.Domains = strings.TrimSpace(b.Domains)
	b.UpstreamType = strings.ToLower(strings.TrimSpace(b.UpstreamType))
	if b.UpstreamType != ProxyUpstreamAppPath {
		b.UpstreamType = ProxyUpstreamURL
	}
	b.ListenScheme = strings.ToLower(strings.TrimSpace(b.ListenScheme))
	switch b.ListenScheme {
	case ProxySchemeHTTP, ProxySchemeHTTPS, ProxySchemeBoth:
	default:
		b.ListenScheme = ProxySchemeHTTP
	}

	b.ForwardScheme = strings.ToLower(strings.TrimSpace(b.ForwardScheme))
	if b.ForwardScheme != ProxyForwardHTTPS {
		b.ForwardScheme = ProxyForwardHTTP
	}
	b.ForwardHost = strings.TrimSpace(b.ForwardHost)
	if b.ForwardPort <= 0 {
		if b.ForwardScheme == ProxyForwardHTTPS {
			b.ForwardPort = 443
		} else {
			b.ForwardPort = 80
		}
	}

	// Prefer discrete forward_* fields; fall back to parsing UpstreamTarget.
	if b.UpstreamType == ProxyUpstreamURL {
		if b.ForwardHost != "" {
			b.UpstreamTarget = fmt.Sprintf("%s://%s:%d", b.ForwardScheme, b.ForwardHost, b.ForwardPort)
		} else if b.UpstreamTarget != "" {
			b.syncForwardFromUpstream()
		}
	}
	b.UpstreamTarget = strings.TrimSpace(b.UpstreamTarget)
	if b.CustomHeaders == nil {
		b.CustomHeaders = JSONBAny{}
	}
}

func (b *ProxyHost) syncForwardFromUpstream() {
	u, err := url.Parse(b.UpstreamTarget)
	if err != nil || u.Host == "" {
		return
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == ProxyForwardHTTPS {
		b.ForwardScheme = ProxyForwardHTTPS
	} else {
		b.ForwardScheme = ProxyForwardHTTP
	}
	host := u.Hostname()
	b.ForwardHost = host
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			b.ForwardPort = n
			return
		}
	}
	if b.ForwardScheme == ProxyForwardHTTPS {
		b.ForwardPort = 443
	} else {
		b.ForwardPort = 80
	}
}

// ProxyLocation is a path-specific upstream override under a host.
type ProxyLocation struct {
	ID             string   `json:"id" gorm:"primaryKey;type:text"`
	HostID         string   `json:"host_id" gorm:"size:36;not null;index"`
	PathPrefix     string   `json:"path_prefix" gorm:"size:512;not null"`
	UpstreamType   string   `json:"upstream_type" gorm:"size:16;not null;default:url"`
	UpstreamTarget string   `json:"upstream_target" gorm:"size:1024"`
	StripPrefix    bool     `json:"strip_prefix" gorm:"not null;default:false"`
	Websocket      bool     `json:"websocket" gorm:"not null;default:false"`
	Extras         JSONBAny `json:"extras" gorm:"type:text"`
	OrderNr        int      `json:"order_nr" gorm:"not null;default:0;index"`
	Enabled        bool     `json:"enabled" gorm:"not null;default:true;index"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *ProxyLocation) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *ProxyLocation) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (ProxyLocation) TableName() string {
	return "proxy_locations"
}

func (b *ProxyLocation) Normalize() {
	b.PathPrefix = strings.TrimSpace(b.PathPrefix)
	if b.PathPrefix == "" {
		b.PathPrefix = "/"
	}
	if !strings.HasPrefix(b.PathPrefix, "/") {
		b.PathPrefix = "/" + b.PathPrefix
	}
	b.UpstreamType = strings.ToLower(strings.TrimSpace(b.UpstreamType))
	if b.UpstreamType != ProxyUpstreamAppPath {
		b.UpstreamType = ProxyUpstreamURL
	}
	b.UpstreamTarget = strings.TrimSpace(b.UpstreamTarget)
	if b.Extras == nil {
		b.Extras = JSONBAny{}
	}
}
