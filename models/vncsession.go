package models

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	VncSessionStatusActive   = "active"
	VncSessionStatusInactive = "inactive"

	VncDefaultGeometry       = "1920x1080"
	VncDefaultDepth          = 24
	VncDefaultDPI            = 96
	VncDefaultFramerate      = 60
	VncDefaultQuality        = 9
	VncDefaultCompression    = 0
	VncDefaultReconnectDelay = 2000
	VncDefaultResize         = "remote"
	VncDefaultDesktop        = "xfce"
	VncDefaultSecurityTypes  = "VncAuth"
)

type VncSession struct {
	ID     string `json:"id" gorm:"primaryKey;type:text"`
	UserID string `json:"user_id" gorm:"size:50;uniqueIndex;not null"`
	User   User   `json:"user" gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Status      string `json:"status" gorm:"type:text;default:active"`
	VncPassword string `json:"vnc_password" gorm:"type:text;default:''"`

	Address   string `json:"address" gorm:"type:text;default:127.0.0.1"`
	NoVncPort int    `json:"no_vnc_port" gorm:"default:0"`
	VncPort   int    `json:"vnc_port" gorm:"default:0"`

	// Display / TigerVNC server
	Geometry             string `json:"geometry" gorm:"type:text;default:1920x1080"`
	Depth                int    `json:"depth" gorm:"default:24"`
	Dpi                  int    `json:"dpi" gorm:"default:96"`
	Framerate            int    `json:"framerate" gorm:"default:60"`
	LocalhostOnly        bool   `json:"localhost_only" gorm:"default:true"`
	AlwaysShared         bool   `json:"always_shared" gorm:"default:true"`
	AcceptSetDesktopSize bool   `json:"accept_set_desktop_size" gorm:"default:true"`
	SecurityTypes        string `json:"security_types" gorm:"type:text;default:VncAuth"`
	CompareFB            int    `json:"compare_fb" gorm:"default:0"`
	ImprovedHextile      bool   `json:"improved_hextile" gorm:"default:true"`
	DesktopSession       string `json:"desktop_session" gorm:"type:text;default:xfce"`

	// Performance / noVNC client (Tight encoding)
	Quality     int `json:"quality" gorm:"default:9"`
	Compression int `json:"compression" gorm:"default:0"`

	// Client UX / noVNC UI
	Autoconnect    bool   `json:"autoconnect" gorm:"default:true"`
	Reconnect      bool   `json:"reconnect" gorm:"default:true"`
	ReconnectDelay int    `json:"reconnect_delay" gorm:"default:2000"`
	Resize         string `json:"resize" gorm:"type:text;default:remote"` // off | scale | remote
	ViewOnly       bool   `json:"view_only" gorm:"default:false"`
	ShowDot        bool   `json:"show_dot" gorm:"default:false"`
	ViewClip       bool   `json:"view_clip" gorm:"default:false"`
	Shared         bool   `json:"shared" gorm:"default:true"`
	Bell           string `json:"bell" gorm:"type:text;default:on"` // on | off
	Logging        string `json:"logging" gorm:"type:text;default:warn"`

	// Desktop wallpaper (absolute path on host; empty = system default)
	WallpaperPath string `json:"wallpaper_path" gorm:"type:text;default:''"`

	// Optional RDP access (requires separate xrdp host install)
	RdpEnabled bool   `json:"rdp_enabled" gorm:"default:false"`
	RDPAddress string `json:"rdp_address" gorm:"type:text;default:''"`
	RDPPort    int    `json:"rdp_port" gorm:"default:3389"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (VncSession) TableName() string {
	return "vnc_sessions"
}

func (s *VncSession) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(s.ID) == "" {
		s.ID = uuid.New().String()
	}
	s.ApplyDefaults()
	return nil
}

// ApplyDefaults fills zero-value option fields with sensible defaults.
func (s *VncSession) ApplyDefaults() {
	if strings.TrimSpace(s.Status) == "" {
		s.Status = VncSessionStatusActive
	}
	if strings.TrimSpace(s.Address) == "" {
		s.Address = "127.0.0.1"
	}
	if strings.TrimSpace(s.RDPAddress) == "" {
		s.RDPAddress = "127.0.0.1"
	}
	if strings.TrimSpace(s.Geometry) == "" {
		s.Geometry = VncDefaultGeometry
	}
	if s.Depth <= 0 {
		s.Depth = VncDefaultDepth
	}
	if s.Dpi <= 0 {
		s.Dpi = VncDefaultDPI
	}
	if s.Framerate <= 0 {
		s.Framerate = VncDefaultFramerate
	}
	if strings.TrimSpace(s.SecurityTypes) == "" {
		s.SecurityTypes = VncDefaultSecurityTypes
	}
	if strings.TrimSpace(s.DesktopSession) == "" {
		s.DesktopSession = VncDefaultDesktop
	}
	if s.Quality < 0 || s.Quality > 9 {
		s.Quality = VncDefaultQuality
	}
	if s.Compression < 0 || s.Compression > 9 {
		s.Compression = VncDefaultCompression
	}
	if s.ReconnectDelay <= 0 {
		s.ReconnectDelay = VncDefaultReconnectDelay
	}
	resize := strings.ToLower(strings.TrimSpace(s.Resize))
	switch resize {
	case "off", "scale", "remote":
		s.Resize = resize
	default:
		s.Resize = VncDefaultResize
	}
	bell := strings.ToLower(strings.TrimSpace(s.Bell))
	if bell != "on" && bell != "off" {
		s.Bell = "on"
	} else {
		s.Bell = bell
	}
	logging := strings.ToLower(strings.TrimSpace(s.Logging))
	if logging == "" {
		s.Logging = "warn"
	} else {
		s.Logging = logging
	}
}

// UpstreamHostPort returns host:port for the noVNC/websockify HTTP+WS endpoint.
func (s VncSession) UpstreamHostPort() string {
	addr := strings.TrimSpace(s.Address)
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := s.NoVncPort
	if port <= 0 {
		port = 6080
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

// UpstreamBaseURL is http://address:no_vnc_port (no trailing slash).
func (s VncSession) UpstreamBaseURL() string {
	return "http://" + s.UpstreamHostPort()
}

func boolQuery(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// NovncQuery builds vnc.html query params from stored client options.
func (s VncSession) NovncQuery() url.Values {
	s.ApplyDefaults()
	q := url.Values{}
	id := strings.TrimSpace(s.ID)
	if id != "" {
		q.Set("session_id", id)
		q.Set("path", "websockify?session_id="+id)
	} else {
		q.Set("path", "websockify")
	}
	q.Set("autoconnect", boolQuery(s.Autoconnect))
	q.Set("reconnect", boolQuery(s.Reconnect))
	q.Set("reconnect_delay", strconv.Itoa(s.ReconnectDelay))
	q.Set("resize", s.Resize)
	q.Set("quality", strconv.Itoa(s.Quality))
	q.Set("compression", strconv.Itoa(s.Compression))
	q.Set("show_dot", boolQuery(s.ShowDot))
	q.Set("view_only", boolQuery(s.ViewOnly))
	q.Set("view_clip", boolQuery(s.ViewClip))
	q.Set("shared", boolQuery(s.Shared))
	q.Set("bell", s.Bell)
	q.Set("logging", s.Logging)
	return q
}

// ClientURL is /novnc/vnc.html?... using this session's stored options.
func (s VncSession) ClientURL() string {
	return "/novnc/vnc.html?" + s.NovncQuery().Encode()
}

// GeometryOrDefault returns a TigerVNC -geometry value.
func (s VncSession) GeometryOrDefault() string {
	g := strings.TrimSpace(s.Geometry)
	if g == "" {
		return VncDefaultGeometry
	}
	return g
}
