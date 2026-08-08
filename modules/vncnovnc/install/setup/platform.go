package setup

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/izetmolla/containerws/packages/machine"
)

// PackageFamily is the OS package manager family used to pick install utilities.
type PackageFamily string

const (
	FamilyDebian  PackageFamily = "debian" // Ubuntu, Debian, Mint, Pop!_OS, …
	FamilyRHEL    PackageFamily = "rhel"   // Fedora, RHEL, Rocky, Alma, CentOS
	FamilyArch    PackageFamily = "arch"
	FamilyUnknown PackageFamily = "unknown"
)

// HostPlan describes what this machine is and how setup will install packages.
type HostPlan struct {
	Hostname         string        `json:"hostname"`
	OS               string        `json:"os"`
	Kernel           string        `json:"kernel"`
	Arch             string        `json:"arch"`
	Platform         string        `json:"platform"`
	Distro           string        `json:"distro"`
	DistroID         string        `json:"distro_id"`
	DistroVersion    string        `json:"distro_version"`
	DeviceType       string        `json:"device_type"`
	Virtualization   string        `json:"virtualization"`
	IsContainer      bool          `json:"is_container"`
	IsVM             bool          `json:"is_vm"`
	Family           PackageFamily `json:"package_family"`
	PackageManager   string        `json:"package_manager"`
	Supported        bool          `json:"supported"`
	Notes            []string      `json:"notes,omitempty"`
	Packages         []string      `json:"packages"`
	OptionalPackages []string      `json:"optional_packages"`
}

// DetectHost builds an install plan from machine.Detect() + os-release mapping.
func DetectHost() HostPlan {
	snap := machine.Detect()
	family, mgr := resolveFamily(snap.DistroID, snap.Distro)
	pkgs, optional, notes := packagesFor(family, snap.DistroID, snap.DistroVersion)

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
		plan.Notes = append(plan.Notes, "VNC/noVNC setup requires a Linux host")
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

func packagesFor(family PackageFamily, distroID, version string) (required, optional []string, notes []string) {
	switch family {
	case FamilyDebian:
		required = []string{
			"ca-certificates", "curl", "dbus-x11", "fonts-dejavu-core", "fonts-liberation",
			"iproute2", "novnc", "openssl", "python3", "python3-numpy",
			"tigervnc-standalone-server", "tigervnc-common", "websockify",
			"xfce4", "xfce4-terminal", "xfonts-base", "x11-xserver-utils",
		}
		optional = []string{"xfce4-goodies", "tigervnc-tools"}
		notes = debianVersionNotes(distroID, version)
	case FamilyRHEL:
		// Fedora/RHEL do not ship a single "xfce4" meta like Debian — pull the
		// components startxfce4 expects (wm, panel, desktop, settings, Thunar).
		required = []string{
			"tigervnc-server", "novnc", "python3-websockify", "dbus-x11",
			"xorg-x11-xauth", "xorg-x11-server-utils", "dejavu-sans-fonts",
			"xfce4-session", "xfce4-panel", "xfce4-settings", "xfce4-terminal",
			"xfce4-appfinder", "xfwm4", "xfdesktop", "Thunar",
			"curl", "openssl",
		}
		optional = []string{
			"xfce4-whiskermenu-plugin", "xfce4-screenshooter", "mousepad",
			"ristretto", "tigervnc",
		}
		notes = append(notes,
			"RHEL-family installs may need EPEL for novnc/websockify",
			"Fedora XFCE: installs xfwm4/xfdesktop/Thunar/xfce4-settings (not only xfce4-session)",
		)
	case FamilyArch:
		required = []string{
			"tigervnc", "novnc", "websockify", "xfce4", "xfce4-terminal",
			"dbus", "curl", "openssl", "ttf-dejavu",
		}
		notes = append(notes, "Arch packages come from community/extra; ensure mirrors are up to date")
	default:
		notes = append(notes, "Unsupported distro — extend packagesFor() for this OS")
	}
	return required, optional, notes
}

func debianVersionNotes(distroID, version string) []string {
	var notes []string
	id := strings.ToLower(distroID)
	major := versionMajor(version)

	if id == "ubuntu" {
		notes = append(notes, fmt.Sprintf("Detected Ubuntu %s — using apt packages for TigerVNC + noVNC + XFCE", version))
		if major >= 24 {
			notes = append(notes,
				"Ubuntu 24.04+ / 26.04: TigerVNC 1.15 uses ~/.config/tigervnc (legacy ~/.vnc is migrated)",
			)
		}
		if major >= 26 {
			notes = append(notes, "Ubuntu 26.04 aligned with containerws multi-user VNC layout")
		}
	} else if id == "debian" {
		notes = append(notes, fmt.Sprintf("Detected Debian %s — using apt packages", version))
	} else {
		notes = append(notes, fmt.Sprintf("Detected %s %s (Debian-family) — using apt packages", distroID, version))
	}
	return notes
}

func versionMajor(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	part := version
	if i := strings.IndexAny(version, ".-"); i >= 0 {
		part = version[:i]
	}
	n, _ := strconv.Atoi(part)
	return n
}

func lookPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// StatusReport checks whether key binaries from setup are present.
type StatusReport struct {
	Ready      bool            `json:"ready"`
	Binaries   map[string]bool `json:"binaries"`
	NovncRoots map[string]bool `json:"novnc_roots"`
	Missing    []string        `json:"missing,omitempty"`
	Plan       HostPlan        `json:"plan"`
}

func CheckStatus() StatusReport {
	plan := DetectHost()
	bins := map[string]bool{
		"vncserver":  lookPath("vncserver") || lookPath("tigervncserver"),
		"websockify": lookPath("websockify"),
		"startxfce4": lookPath("startxfce4"),
		"vncpasswd":  lookPath("vncpasswd"),
		"xfwm4":      lookPath("xfwm4"),
		"xfdesktop":  lookPath("xfdesktop"),
	}
	roots := map[string]bool{
		"/usr/local/share/containerws-novnc/vnc.html": fileExists("/usr/local/share/containerws-novnc/vnc.html"),
		"/usr/share/novnc/vnc.html":                   fileExists("/usr/share/novnc/vnc.html"),
		"/usr/share/novnc/vnc_lite.html":              fileExists("/usr/share/novnc/vnc_lite.html"),
	}

	var missing []string
	for name, ok := range bins {
		if !ok {
			missing = append(missing, name)
		}
	}
	if !roots["/usr/local/share/containerws-novnc/vnc.html"] &&
		!roots["/usr/share/novnc/vnc.html"] &&
		!roots["/usr/share/novnc/vnc_lite.html"] {
		missing = append(missing, "novnc-web-root")
	}

	return StatusReport{
		Ready:      len(missing) == 0,
		Binaries:   bins,
		NovncRoots: roots,
		Missing:    missing,
		Plan:       plan,
	}
}

func fileExists(path string) bool {
	return pathFileExists(path)
}
