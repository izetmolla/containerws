package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContainerType classifies how this workspace is hosted.
type ContainerType string

const (
	ContainerTypePhysical  ContainerType = "physical"
	ContainerTypeVM        ContainerType = "vm"
	ContainerTypeContainer ContainerType = "container"
	ContainerTypeWSL       ContainerType = "wsl"
	ContainerTypeUnknown   ContainerType = "unknown"
)

// Container identifies the machine / virtualization running this app instance.
// One row is upserted on every process start (typically the master workspace).
type Container struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	// Display / routing
	Name        string `json:"name" gorm:"size:255;uniqueIndex"`
	Title       string `json:"title" gorm:"size:255"`
	Description string `json:"description" gorm:"type:text"`
	Icon        string `json:"icon" gorm:"size:64;default:'Container'"`

	IsMaster bool `json:"is_master" gorm:"default:false;index"`
	IsActive bool `json:"is_active" gorm:"default:true;index"`

	// Stable host identity (machine-id / product uuid / env override).
	MachineID string `json:"machine_id" gorm:"size:128;uniqueIndex"`

	// Host naming
	Hostname string `json:"hostname" gorm:"size:255;index"`

	// OS / distro
	OS            string `json:"os" gorm:"size:64"`             // linux, windows, darwin
	OSVersion     string `json:"os_version" gorm:"size:128"`    // kernel release
	Kernel        string `json:"kernel" gorm:"size:128"`
	Platform      string `json:"platform" gorm:"size:64"`       // GOOS/GOARCH summary
	Distro        string `json:"distro" gorm:"size:128"`        // Ubuntu, Debian, Fedora
	DistroID      string `json:"distro_id" gorm:"size:64"`      // ubuntu, debian, fedora
	DistroVersion string `json:"distro_version" gorm:"size:64"` // 26.04, 13, 44

	// CPU / arch
	Arch         string `json:"arch" gorm:"size:32"` // amd64, arm64
	Processor    string `json:"processor" gorm:"size:255"`
	CPUModel     string `json:"cpu_model" gorm:"size:512"`
	CPUCores     int    `json:"cpu_cores" gorm:"default:0"`
	MemoryTotal  int64  `json:"memory_total" gorm:"default:0"` // bytes
	MemoryHuman  string `json:"memory_human" gorm:"size:32"`

	// Network
	IPs           JSONBStringArray `json:"ips" gorm:"type:text;default:'[]'"`
	MACAddresses  JSONBStringArray `json:"mac_addresses" gorm:"type:text;default:'[]'"`
	PrimaryIP     string           `json:"primary_ip" gorm:"size:64"`
	DefaultIface  string           `json:"default_iface" gorm:"size:64"`

	// Virtualization / runtime
	Type              ContainerType `json:"type" gorm:"size:32;default:'unknown';index"`
	Virtualization    string        `json:"virtualization" gorm:"size:64"`     // docker, wsl, kvm, qemu, vmware, none
	Hypervisor        string        `json:"hypervisor" gorm:"size:128"`
	ContainerRuntime  string        `json:"container_runtime" gorm:"size:64"`  // docker, podman, containerd, ""
	CloudProvider     string        `json:"cloud_provider" gorm:"size:64"`     // aws, gcp, azure, ""
	IsContainerized   bool          `json:"is_containerized" gorm:"default:false"`
	IsVirtualMachine  bool          `json:"is_virtual_machine" gorm:"default:false"`

	// Product / board (DMI)
	ProductName   string `json:"product_name" gorm:"size:255"`
	ProductUUID   string `json:"product_uuid" gorm:"size:128"`
	SysVendor     string `json:"sys_vendor" gorm:"size:255"`
	BoardName     string `json:"board_name" gorm:"size:255"`

	// App build running on this host
	AppVersion string `json:"app_version" gorm:"size:64"`
	CommitSHA  string `json:"commit_sha" gorm:"size:64"`

	// Lifecycle
	BootedAt   *time.Time `json:"booted_at"`
	LastSeenAt *time.Time `json:"last_seen_at" gorm:"index"`

	Metadata JSONBAny `json:"metadata" gorm:"type:text;default:'{}'"`

	Navigations []Navigation `json:"navigations" gorm:"foreignKey:ContainerID;references:ID"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *Container) BeforeCreate(_ *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if b.Type == "" {
		b.Type = ContainerTypeUnknown
	}
	if b.Icon == "" {
		b.Icon = "Container"
	}
	return
}

func (b Container) TableName() string {
	return "containers"
}
