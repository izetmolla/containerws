package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Docker connection modes for Engine API access.
const (
	DockerConnUnix = "unix" // local unix socket
	DockerConnSSH  = "ssh"  // Docker over SSH tunnel
	DockerConnTLS  = "tls"  // remote TCP + TLS (tcp://host:2376)
)

// DockerEnvironment is a named Docker Engine endpoint (local or remote).
type DockerEnvironment struct {
	ID          string `json:"id" gorm:"primaryKey;type:text"`
	Name        string `json:"name" gorm:"size:255;not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:text"`
	ConnType    string `json:"conn_type" gorm:"size:16;not null;index"` // unix|ssh|tls

	// HostURL is the canonical DOCKER_HOST-style URL (computed on save).
	HostURL string `json:"host_url" gorm:"size:512"`

	// unix
	SocketPath string `json:"socket_path" gorm:"size:512"`

	// ssh
	SSHHost         string `json:"ssh_host" gorm:"size:255"`
	SSHPort         int    `json:"ssh_port" gorm:"default:22"`
	SSHUser         string `json:"ssh_user" gorm:"size:128"`
	SSHPrivateKey   string `json:"ssh_private_key,omitempty" gorm:"type:text"`
	SSHPassphrase   string `json:"ssh_passphrase,omitempty" gorm:"type:text"`
	SSHRemoteSocket string `json:"ssh_remote_socket" gorm:"size:512"` // default /var/run/docker.sock

	// tls (tcp + mutual TLS)
	TCPHost       string `json:"tcp_host" gorm:"size:255"`
	TCPPort       int    `json:"tcp_port" gorm:"default:2376"`
	TLSCACert     string `json:"tls_ca_cert,omitempty" gorm:"type:text"`
	TLSCert       string `json:"tls_cert,omitempty" gorm:"type:text"`
	TLSKey        string `json:"tls_key,omitempty" gorm:"type:text"`
	TLSSkipVerify bool   `json:"tls_skip_verify" gorm:"default:false"`

	IsDefault  bool `json:"is_default" gorm:"not null;default:false;index"`
	IsDisabled bool `json:"is_disabled" gorm:"not null;default:false;index"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *DockerEnvironment) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.Normalize()
	return
}

func (b *DockerEnvironment) BeforeSave(tx *gorm.DB) (err error) {
	b.Normalize()
	return
}

func (DockerEnvironment) TableName() string {
	return "docker_environments"
}

// Normalize fills defaults and HostURL from connection fields.
func (b *DockerEnvironment) Normalize() {
	b.ConnType = strings.ToLower(strings.TrimSpace(b.ConnType))
	switch b.ConnType {
	case DockerConnUnix:
		if strings.TrimSpace(b.SocketPath) == "" {
			b.SocketPath = "/var/run/docker.sock"
		}
		b.HostURL = "unix://" + b.SocketPath
	case DockerConnSSH:
		if b.SSHPort <= 0 {
			b.SSHPort = 22
		}
		if strings.TrimSpace(b.SSHRemoteSocket) == "" {
			b.SSHRemoteSocket = "/var/run/docker.sock"
		}
		user := strings.TrimSpace(b.SSHUser)
		host := strings.TrimSpace(b.SSHHost)
		if user != "" {
			b.HostURL = "ssh://" + user + "@" + host
		} else {
			b.HostURL = "ssh://" + host
		}
		if b.SSHPort != 22 {
			b.HostURL += ":" + itoa(b.SSHPort)
		}
	case DockerConnTLS:
		if b.TCPPort <= 0 {
			b.TCPPort = 2376
		}
		b.HostURL = "tcp://" + strings.TrimSpace(b.TCPHost) + ":" + itoa(b.TCPPort)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
