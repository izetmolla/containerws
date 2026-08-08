package install

import (
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/machine"
)

// CurrentHostIdentity builds matching facts for this machine.
func CurrentHostIdentity() models.HostIdentity {
	snap := machine.Detect()
	arch := models.NormalizeArch(snap.Arch)
	platform := strings.TrimSpace(snap.Platform)
	if platform == "" {
		platform = snap.OS + "/" + arch
	}
	return models.HostIdentity{
		OS:               snap.OS,
		OsVersion:        snap.OSVersion,
		Distro:           snap.Distro,
		DistroID:         snap.DistroID,
		DistroVersion:    snap.DistroVersion,
		Arch:             arch,
		Platform:         platform,
		PackageFamily:    detectPackageFamily(snap.DistroID, snap.Distro),
		Kernel:           snap.Kernel,
		Virtualization:   snap.Virtualization,
		ContainerRuntime: snap.ContainerRuntime,
		CloudProvider:    snap.CloudProvider,
	}
}

func currentHostIdentity() models.HostIdentity {
	return CurrentHostIdentity()
}

// MatchingVersion returns the best host-matching version, or nil if none match.
func MatchingVersion(versions []models.SoftwareVersion, host models.HostIdentity, preferLatest bool) *models.SoftwareVersion {
	if len(versions) == 0 {
		return nil
	}
	var best *models.SoftwareVersion
	bestScore := -1
	for i := range versions {
		v := &versions[i]
		if !v.MatchesHost(host) {
			continue
		}
		score := v.SpecificityScore()
		if preferLatest && v.IsLatest {
			score += 100
		}
		if score > bestScore {
			bestScore = score
			best = v
		}
	}
	return best
}

func detectPackageFamily(distroID, distroName string) string {
	id := strings.ToLower(strings.TrimSpace(distroID))
	name := strings.ToLower(strings.TrimSpace(distroName))
	switch {
	case strings.Contains(id, "ubuntu"), strings.Contains(id, "debian"),
		strings.Contains(id, "kali"), strings.Contains(id, "mint"),
		strings.Contains(id, "pop"), strings.Contains(name, "ubuntu"),
		strings.Contains(name, "debian"), strings.Contains(name, "kali"):
		return "apt"
	case strings.Contains(id, "rhel"), strings.Contains(id, "fedora"),
		strings.Contains(id, "centos"), strings.Contains(id, "rocky"),
		strings.Contains(id, "alma"), strings.Contains(name, "fedora"):
		return "dnf"
	case strings.Contains(id, "arch"), strings.Contains(id, "manjaro"),
		strings.Contains(name, "arch"):
		return "pacman"
	case strings.Contains(id, "alpine"):
		return "apk"
	default:
		return ""
	}
}

// pickBestVersion returns the most specific host-matching version from the list.
// Prefer isLatest among ties when preferLatest is true.
// Falls back to is_latest / first row when nothing matches the host (install path).
func pickBestVersion(versions []models.SoftwareVersion, preferLatest bool) *models.SoftwareVersion {
	if len(versions) == 0 {
		return nil
	}
	if best := MatchingVersion(versions, currentHostIdentity(), preferLatest); best != nil {
		return best
	}
	for i := range versions {
		if versions[i].IsLatest {
			return &versions[i]
		}
	}
	return &versions[0]
}
