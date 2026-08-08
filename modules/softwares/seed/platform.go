package seed

// linuxAptTargets is the default host identity for apt-based install/uninstall scripts.
// Empty Arch / DistroID = match any arch / any debian-family distro that uses apt.
func linuxAptTargets() (os, distroID, pkgFamily, arch string) {
	return "linux", "", "apt", ""
}

func applyLinuxAptTargets(v *VersionMeta) {
	if v == nil {
		return
	}
	osName, distroID, pkg, arch := linuxAptTargets()
	if v.OS == "" {
		v.OS = osName
	}
	if v.DistroID == "" {
		v.DistroID = distroID
	}
	if v.PackageFamily == "" {
		v.PackageFamily = pkg
	}
	if v.Arch == "" {
		v.Arch = arch
	}
	if v.Platform == "" && v.OS != "" {
		if v.Arch != "" {
			v.Platform = v.OS + "/" + v.Arch
		} else {
			v.Platform = v.OS
		}
	}
}
