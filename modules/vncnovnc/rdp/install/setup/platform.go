package setup

import (
	"os/exec"
	"strings"

	"github.com/izetmolla/containerws/packages/machine"
)

type PackageFamily string

const (
	FamilyDebian  PackageFamily = "debian"
	FamilyRHEL    PackageFamily = "rhel"
	FamilyArch    PackageFamily = "arch"
	FamilyUnknown PackageFamily = "unknown"
)

type HostPlan struct {
	Hostname       string        `json:"hostname"`
	OS             string        `json:"os"`
	Kernel         string        `json:"kernel"`
	Arch           string        `json:"arch"`
	Platform       string        `json:"platform"`
	Distro         string        `json:"distro"`
	DistroID       string        `json:"distro_id"`
	DistroVersion  string        `json:"distro_version"`
	DeviceType     string        `json:"device_type"`
	Virtualization string        `json:"virtualization"`
	IsContainer    bool          `json:"is_container"`
	IsVM           bool          `json:"is_vm"`
	Family         PackageFamily `json:"package_family"`
	PackageManager string        `json:"package_manager"`
	Supported      bool          `json:"supported"`
	Notes          []string      `json:"notes,omitempty"`
	Packages       []string      `json:"packages"`
	OptionalPackages []string    `json:"optional_packages"`
}

func DetectHost() HostPlan {
	snap := machine.Detect()
	family, mgr := resolveFamily(snap.DistroID, snap.Distro)
	pkgs, optional, notes := packagesFor(family)

	device := string(snap.Type)
	if device == "" {
		device = "host"
	}
	plan := HostPlan{
		Hostname:         snap.Hostname,
		OS:               snap.OS,
		Kernel:           snap.Kernel,
		Arch:             snap.Arch,
		Platform:         snap.Platform,
		Distro:           snap.Distro,
		DistroID:         snap.DistroID,
		DistroVersion:    snap.DistroVersion,
		DeviceType:       device,
		Virtualization:   snap.Virtualization,
		IsContainer:      snap.IsContainerized,
		IsVM:             snap.IsVirtualMachine,
		Family:           family,
		PackageManager:   mgr,
		Supported:        family != FamilyUnknown && snap.OS == "linux",
		Notes:            notes,
		Packages:         pkgs,
		OptionalPackages: optional,
	}
	if snap.OS != "linux" {
		plan.Supported = false
		plan.Notes = append(plan.Notes, "RDP (xrdp) setup requires a Linux host")
	}
	return plan
}

func resolveFamily(distroID, distroName string) (PackageFamily, string) {
	id := strings.ToLower(strings.TrimSpace(distroID))
	name := strings.ToLower(strings.TrimSpace(distroName))
	switch id {
	case "ubuntu", "debian", "linuxmint", "pop", "elementary", "zorin", "raspbian", "kali":
		return FamilyDebian, "apt-get"
	case "fedora", "rhel", "centos", "rocky", "almalinux", "ol", "amzn":
		if lookPath("dnf") {
			return FamilyRHEL, "dnf"
		}
		return FamilyRHEL, "yum"
	case "arch", "manjaro", "endeavouros", "garuda":
		return FamilyArch, "pacman"
	}
	switch {
	case strings.Contains(name, "ubuntu"), strings.Contains(name, "debian"):
		return FamilyDebian, "apt-get"
	case strings.Contains(name, "fedora"), strings.Contains(name, "red hat"), strings.Contains(name, "rocky"), strings.Contains(name, "alma"):
		if lookPath("dnf") {
			return FamilyRHEL, "dnf"
		}
		return FamilyRHEL, "yum"
	case strings.Contains(name, "arch"):
		return FamilyArch, "pacman"
	}
	return FamilyUnknown, ""
}

func packagesFor(family PackageFamily) (required, optional []string, notes []string) {
	switch family {
	case FamilyDebian:
		required = []string{"xrdp", "xorgxrdp", "dbus-x11"}
		optional = []string{"xorgxrdp"}
		notes = append(notes, "Debian-family: installs xrdp + xorgxrdp (optional desktop packages already covered by VNC/XFCE)")
	case FamilyRHEL:
		required = []string{"xrdp", "xorgxrdp", "dbus-x11"}
		notes = append(notes, "RHEL-family may need EPEL for xrdp")
	case FamilyArch:
		required = []string{"xrdp", "xorgxrdp"}
		notes = append(notes, "Arch: enable xrdp.service after install")
	default:
		notes = append(notes, "Unsupported distro — extend packagesFor() for this OS")
	}
	return required, optional, notes
}

func lookPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

type StatusReport struct {
	Ready    bool            `json:"ready"`
	Running  bool            `json:"running"`
	Binaries map[string]bool `json:"binaries"`
	Missing  []string        `json:"missing,omitempty"`
	Port     int             `json:"port"`
	Plan     HostPlan        `json:"plan"`
}

func CheckStatus() StatusReport {
	plan := DetectHost()
	bins := map[string]bool{
		"xrdp":     lookPath("xrdp") || fileExists("/usr/sbin/xrdp"),
		"xrdp-sesman": lookPath("xrdp-sesman") || fileExists("/usr/sbin/xrdp-sesman"),
	}
	var missing []string
	for name, ok := range bins {
		if !ok {
			missing = append(missing, name)
		}
	}
	running := isXrdpRunning()
	return StatusReport{
		Ready:    len(missing) == 0,
		Running:  running,
		Binaries: bins,
		Missing:  missing,
		Port:     3389,
		Plan:     plan,
	}
}

func fileExists(path string) bool {
	return pathFileExists(path)
}

func isXrdpRunning() bool {
	if out, err := exec.Command("bash", "-lc", "systemctl is-active --quiet xrdp && echo yes || true").CombinedOutput(); err == nil {
		if strings.Contains(string(out), "yes") {
			return true
		}
	}
	if out, err := exec.Command("bash", "-lc", "pgrep -x xrdp >/dev/null && echo yes || true").CombinedOutput(); err == nil {
		return strings.Contains(string(out), "yes")
	}
	return false
}
