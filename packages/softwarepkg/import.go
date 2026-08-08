package softwarepkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// ImportResult summarizes an Import.
type ImportResult struct {
	Software   models.Software
	Version    models.SoftwareVersion
	CreatedSW  bool
	CreatedVer bool
	InstallURL string
	MetaURL    string
	TriedPaths []string
}

// ImportOptions controls Import behavior.
type ImportOptions struct {
	Ref  string // git ref for GitHub raw URLs; default main
	Host *HostFacts
	HTTP *Client
}

// Import fetches package.json + matching install.json from pkg.PackageURL and
// upserts Software + SoftwareVersion for the current (or provided) host.
func Import(
	ctx context.Context,
	db *gorm.DB,
	pkg models.SoftwarePackage,
	softwareName string,
	opts *ImportOptions,
) (*ImportResult, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	softwareName = strings.TrimSpace(softwareName)
	if softwareName == "" {
		return nil, errors.New("software name is required")
	}
	if strings.TrimSpace(pkg.PackageURL) == "" {
		return nil, errors.New("package_url is empty")
	}

	ref := defaultRef
	var host HostFacts
	client := &Client{}
	if opts != nil {
		if opts.Ref != "" {
			ref = opts.Ref
		}
		if opts.Host != nil {
			host = *opts.Host
		}
		if opts.HTTP != nil {
			client = opts.HTTP
		}
	}
	if host.DistroID == "" && host.Arch == "" {
		host = HostFromMachine()
	}
	if host.Arch != "" {
		host.Arch = models.NormalizeArch(host.Arch)
	}

	rawBase, err := RawBaseURL(pkg.PackageURL, ref)
	if err != nil {
		return nil, err
	}

	fetched, err := client.FetchForHost(ctx, rawBase, softwareName, host, AuthFromPackage(pkg))
	if err != nil {
		return nil, err
	}

	res := &ImportResult{
		InstallURL: fetched.InstallURL,
		MetaURL:    fetched.MetaURL,
		TriedPaths: fetched.TriedPaths,
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sw, createdSW, err := upsertSoftware(tx, fetched.Meta, pkg.ID, fetched.Slug)
		if err != nil {
			return err
		}
		ver, createdVer, err := upsertVersion(tx, sw.ID, fetched.Install, host)
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

func upsertSoftware(
	tx *gorm.DB,
	meta PackageMeta,
	registryPackageID string,
	registrySlug string,
) (models.Software, bool, error) {
	name := strings.TrimSpace(meta.Name)
	var sw models.Software
	err := tx.Where("LOWER(name) = ?", strings.ToLower(name)).First(&sw).Error
	created := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Also match by registry slug when display name changed (rust → Rust Language).
		slug := sanitizeSegment(registrySlug)
		if slug != "" {
			err = tx.Where("LOWER(registry_slug) = ?", slug).First(&sw).Error
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sw = models.Software{Name: name}
			created = true
			err = nil
		}
	}
	if err != nil {
		return sw, false, err
	}

	sw.Name = name
	sw.Details = strings.TrimSpace(meta.Details)
	sw.Category = strings.TrimSpace(meta.Category)
	sw.SubCategory = strings.TrimSpace(meta.SubCategory)
	sw.Tags = models.JSONBStringArray(cleanStrings(meta.Tags))
	sw.ServiceUnits = models.JSONBStringArray(cleanStrings(meta.ServiceUnits))
	sw.CanControl, sw.ControlBackend = meta.ResolveControlFields()
	sw.StartCommand, sw.RestartCommand, sw.StopCommand = meta.ResolveControlCommands()
	sw.Icon = strings.TrimSpace(meta.Icon)
	sw.Image = strings.TrimSpace(meta.Image)
	sw.Color = strings.TrimSpace(meta.Color)
	if registryPackageID != "" {
		sw.RegistryPackageID = registryPackageID
	}
	if slug := sanitizeSegment(registrySlug); slug != "" {
		sw.RegistrySlug = slug
	}
	if meta.Order != 0 || created {
		sw.Order = meta.Order
	}
	if meta.IsActive != nil {
		sw.IsActive = *meta.IsActive
	} else if created {
		sw.IsActive = true
	}

	if created {
		if err := tx.Create(&sw).Error; err != nil {
			return sw, false, err
		}
	} else if err := tx.Save(&sw).Error; err != nil {
		return sw, false, err
	}
	return sw, created, nil
}

func upsertVersion(
	tx *gorm.DB,
	softwareID string,
	spec InstallSpec,
	host HostFacts,
) (models.SoftwareVersion, bool, error) {
	version := strings.TrimSpace(spec.Version)
	var ver models.SoftwareVersion
	err := tx.Where("software_id = ? AND version = ?", softwareID, version).First(&ver).Error
	created := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ver = models.SoftwareVersion{
			SoftwareID: softwareID,
			Version:    version,
		}
		created = true
	} else if err != nil {
		return ver, false, err
	}

	ver.InstallScript = spec.InstallScript
	ver.UninstallScript = spec.UninstallScript
	ver.UpgradeScript = spec.UpgradeScript
	ver.CustomScript = spec.CustomScript

	ver.OS = firstNonEmpty(spec.OS, "linux")
	ver.DistroID = firstNonEmpty(spec.DistroID, host.DistroID)
	ver.DistroVersion = firstNonEmpty(spec.DistroVersion, host.DistroVersion)
	ver.Distro = strings.TrimSpace(spec.Distro)
	ver.Arch = models.NormalizeArch(firstNonEmpty(spec.Arch, host.Arch))
	ver.Platform = strings.TrimSpace(spec.Platform)
	if ver.Platform == "" && ver.OS != "" {
		if ver.Arch != "" {
			ver.Platform = ver.OS + "/" + ver.Arch
		} else {
			ver.Platform = ver.OS
		}
	}
	ver.PackageFamily = strings.TrimSpace(spec.PackageFamily)
	ver.Kernel = strings.TrimSpace(spec.Kernel)
	ver.Virtualization = strings.TrimSpace(spec.Virtualization)
	ver.ContainerRuntime = strings.TrimSpace(spec.ContainerRuntime)
	ver.CloudProvider = strings.TrimSpace(spec.CloudProvider)

	makeLatest := true
	if spec.IsLatest != nil {
		makeLatest = *spec.IsLatest
	}
	ver.IsLatest = makeLatest

	if makeLatest {
		if err := tx.Model(&models.SoftwareVersion{}).
			Where("software_id = ? AND id <> ?", softwareID, ver.ID).
			Update("is_latest", false).Error; err != nil {
			return ver, false, err
		}
	}

	if created {
		if err := tx.Create(&ver).Error; err != nil {
			return ver, false, fmt.Errorf("create version: %w", err)
		}
	} else if err := tx.Save(&ver).Error; err != nil {
		return ver, false, fmt.Errorf("update version: %w", err)
	}
	return ver, created, nil
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
