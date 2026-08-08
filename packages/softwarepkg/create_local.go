package softwarepkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// CreateLocalRequest upserts a generated package into the local Softwares catalog
// for the current (or provided) host distro — without requiring a GitHub registry.
type CreateLocalRequest struct {
	Name         string
	Details      string
	Category     string
	SubCategory  string
	Tags         []string
	Icon         string
	Image        string
	Color        string
	Order        int
	ServiceUnits []string
	// CanControl marks Start/Stop/Restart; nil → infer from ServiceUnits.
	CanControl     *bool
	ControlBackend string
	StartCommand   string
	RestartCommand string
	StopCommand    string
	Version        string
	AptPackage     string
	DnfPackage     string
	ApkPackage     string
	PacmanPackage  string
	Host           *HostFacts
	// CustomScript is stored on the version and run after a successful install when set.
	CustomScript string
}

// CreateLocalResult summarizes a local catalog upsert.
type CreateLocalResult struct {
	Software   models.Software
	Version    models.SoftwareVersion
	CreatedSW  bool
	CreatedVer bool
	DistroID   string
	Family     string
	PkgName    string
}

// CreateLocal builds distro-appropriate scripts and upserts Software + SoftwareVersion.
func CreateLocal(ctx context.Context, db *gorm.DB, req CreateLocalRequest) (*CreateLocalResult, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	name := sanitizeSegment(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}

	host := HostFromMachine()
	if req.Host != nil {
		host = *req.Host
	}
	if host.Arch != "" {
		host.Arch = models.NormalizeArch(host.Arch)
	}

	target := targetForHost(host, name, req)
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = "1.0.0"
	}
	details := strings.TrimSpace(req.Details)
	if details == "" {
		details = fmt.Sprintf("%s package", name)
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "Tools"
	}
	icon := strings.TrimSpace(req.Icon)
	if icon == "" {
		icon = "Package"
	}
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#0ea5e9"
	}
	active := true
	meta := PackageMeta{
		Name:           name,
		Details:        details,
		Category:       category,
		SubCategory:    strings.TrimSpace(req.SubCategory),
		Tags:           req.Tags,
		Icon:           icon,
		Image:          strings.TrimSpace(req.Image),
		Color:          color,
		Order:          req.Order,
		ServiceUnits:   req.ServiceUnits,
		CanControl:     req.CanControl,
		ControlBackend: req.ControlBackend,
		StartCommand:   strings.TrimSpace(req.StartCommand),
		RestartCommand: strings.TrimSpace(req.RestartCommand),
		StopCommand:    strings.TrimSpace(req.StopCommand),
		IsActive:       &active,
	}
	meta.StartCommand, meta.RestartCommand, meta.StopCommand = meta.ResolveControlCommands()
	latest := true
	spec := InstallSpec{
		Version:          version,
		IsLatest:         &latest,
		OS:               "linux",
		DistroID:         firstNonEmpty(host.DistroID, target.DistroID),
		DistroVersion:    host.DistroVersion,
		Arch:             host.Arch,
		PackageFamily:    target.PackageFamily,
		InstallScript:    BuildInstallScript(target),
		UninstallScript:  BuildUninstallScript(target),
		UpgradeScript:    BuildUpgradeScript(target),
		CustomScript:     strings.TrimSpace(req.CustomScript),
	}

	res := &CreateLocalResult{
		DistroID: target.DistroID,
		Family:   target.PackageFamily,
		PkgName:  target.PkgName,
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sw, createdSW, err := upsertSoftware(tx, meta, "", "")
		if err != nil {
			return err
		}
		ver, createdVer, err := upsertVersion(tx, sw.ID, spec, host)
		if err != nil {
			return err
		}
		res.Software = sw
		res.Version = ver
		res.CreatedSW = createdSW
		res.CreatedVer = createdVer
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func targetForHost(host HostFacts, softName string, req CreateLocalRequest) DistroTarget {
	id := strings.ToLower(strings.TrimSpace(host.DistroID))
	scaffoldReq := ScaffoldRequest{
		AptPackage:    req.AptPackage,
		DnfPackage:    req.DnfPackage,
		ApkPackage:    req.ApkPackage,
		PacmanPackage: req.PacmanPackage,
	}
	if t, ok := resolveDistroTarget(id, softName, scaffoldReq); ok && t.DistroID != "default" {
		return t
	}
	// ID_LIKE-style fallbacks
	switch {
	case strings.Contains(id, "ubuntu") || strings.Contains(id, "debian"):
		t, _ := resolveDistroTarget("debian", softName, scaffoldReq)
		return t
	case strings.Contains(id, "fedora") || strings.Contains(id, "rhel") || strings.Contains(id, "centos"):
		t, _ := resolveDistroTarget("fedora", softName, scaffoldReq)
		return t
	case strings.Contains(id, "alpine"):
		t, _ := resolveDistroTarget("alpine", softName, scaffoldReq)
		return t
	case strings.Contains(id, "arch"):
		t, _ := resolveDistroTarget("arch", softName, scaffoldReq)
		return t
	default:
		t, _ := resolveDistroTarget("default", softName, scaffoldReq)
		return t
	}
}
