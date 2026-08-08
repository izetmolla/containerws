package models

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

// SoftwareVersion is an installable/uninstallable release of a catalog software.
// Empty platform identity fields act as wildcards (match any host).
type SoftwareVersion struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	SoftwareID string    `json:"software_id" gorm:"size:255;index"`
	Software   *Software `json:"software,omitempty" gorm:"foreignKey:SoftwareID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Version    string    `json:"version" gorm:"size:255;"`

	IsLatest bool `json:"is_latest" gorm:"default:false;"`

	InstallScript   string `json:"install_script" gorm:"type:text;"`
	UninstallScript string `json:"uninstall_script" gorm:"type:text;"`
	UpgradeScript   string `json:"upgrade_script" gorm:"type:text;"`
	CustomScript    string `json:"custom_script" gorm:"type:text;"`

	// Host targeting — leave blank to match any. Used to pick the right
	// script set for the machine (e.g. ubuntu/24.04/amd64 vs debian/arm64).
	OS               string `json:"os" gorm:"size:64;index"`          // linux | darwin | windows
	OsVersion        string `json:"os_version" gorm:"size:128"`       // kernel release or generic OS version
	Distro           string `json:"distro" gorm:"size:128"`           // Ubuntu, Debian, Kali, …
	DistroID         string `json:"distro_id" gorm:"size:64;index"`   // ubuntu, debian, kali, rhel, …
	DistroVersion    string `json:"distro_version" gorm:"size:64"`    // 24.04, bookworm, rolling, …
	Arch             string `json:"arch" gorm:"size:32;index"`        // amd64, arm64, arm, …
	Platform         string `json:"platform" gorm:"size:64"`          // linux/amd64
	PackageFamily    string `json:"package_family" gorm:"size:32"`    // apt | dnf | pacman | apk | …
	Kernel           string `json:"kernel" gorm:"size:128"`           // optional kernel constraint
	Virtualization   string `json:"virtualization" gorm:"size:64"`    // baremetal | kvm | docker | …
	ContainerRuntime string `json:"container_runtime" gorm:"size:64"` // docker | podman | …
	CloudProvider    string `json:"cloud_provider" gorm:"size:64"`    // aws | gcp | azure | …

	// IsInstalled / HasUpdate are API-derived (from software_installed); not the source of truth.
	IsInstalled bool `json:"is_installed" gorm:"-"`
	HasUpdate   bool `json:"has_update" gorm:"-"`
	// CanUninstall is true when UninstallScript is present (API-derived).
	CanUninstall bool `json:"can_uninstall" gorm:"-"`

	// OsMissing is true when this version is marked installed in DB but absent on the host
	// (set by softwaresync on startup; not persisted).
	OsMissing bool `json:"os_missing" gorm:"-"`

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (b *SoftwareVersion) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return
}

func (b SoftwareVersion) TableName() string {
	return "softwares_versions"
}

// HostIdentity is the subset of host facts used to match a SoftwareVersion.
type HostIdentity struct {
	OS               string
	OsVersion        string
	Distro           string
	DistroID         string
	DistroVersion    string
	Arch             string
	Platform         string
	PackageFamily    string
	Kernel           string
	Virtualization   string
	ContainerRuntime string
	CloudProvider    string
}

func normID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func fieldMatches(want, have string) bool {
	want = strings.TrimSpace(want)
	if want == "" || want == "*" || strings.EqualFold(want, "any") {
		return true
	}
	have = strings.TrimSpace(have)
	if have == "" {
		return false
	}
	// Allow comma-separated alternatives on the version row.
	for part := range strings.SplitSeq(want, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(part, have) {
			return true
		}
		// Prefix match for distro versions (e.g. "24" matches "24.04").
		if strings.HasPrefix(strings.ToLower(have), strings.ToLower(part)) {
			return true
		}
	}
	return false
}

// MatchesHost reports whether this version targets the given host.
// All empty targeting fields are wildcards.
func (v SoftwareVersion) MatchesHost(h HostIdentity) bool {
	checks := [][2]string{
		{v.OS, h.OS},
		{v.OsVersion, h.OsVersion},
		{v.Distro, h.Distro},
		{v.DistroID, h.DistroID},
		{v.DistroVersion, h.DistroVersion},
		{v.Arch, h.Arch},
		{v.Platform, h.Platform},
		{v.PackageFamily, h.PackageFamily},
		{v.Kernel, h.Kernel},
		{v.Virtualization, h.Virtualization},
		{v.ContainerRuntime, h.ContainerRuntime},
		{v.CloudProvider, h.CloudProvider},
	}
	for _, c := range checks {
		if !fieldMatches(c[0], c[1]) {
			return false
		}
	}
	return true
}

// SpecificityScore ranks how tightly a version targets a host (higher = better).
func (v SoftwareVersion) SpecificityScore() int {
	score := 0
	for _, f := range []string{
		v.OS, v.OsVersion, v.Distro, v.DistroID, v.DistroVersion,
		v.Arch, v.Platform, v.PackageFamily, v.Kernel,
		v.Virtualization, v.ContainerRuntime, v.CloudProvider,
	} {
		if strings.TrimSpace(f) != "" {
			score++
		}
	}
	return score
}

// NormalizeArch maps runtime / uname arches to catalog form (amd64, arm64, …).
func NormalizeArch(arch string) string {
	switch normID(arch) {
	case "x86_64", "amd64", "x64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armhf", "armv7":
		return "arm"
	case "i386", "i686", "386":
		return "386"
	default:
		return normID(arch)
	}
}
