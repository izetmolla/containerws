package buildin

// SoftwareMeta describes a catalog software entry owned by a build-in tool.
type SoftwareMeta struct {
	Name           string
	Details        string
	Category       string
	SubCategory    string
	Tags           []string
	ServiceUnits   []string
	CanControl     bool
	ControlBackend string
	StartCommand   string
	RestartCommand string
	StopCommand    string
	Icon           string
	Color          string
	Order          int
	IsActive       bool
}

// VersionMeta is an installable version of a build-in tool.
type VersionMeta struct {
	Version         string
	IsLatest        bool
	InstallScript   string
	UninstallScript string
	OS              string
	OsVersion       string
	Distro          string
	DistroID        string
	DistroVersion   string
	Arch            string
	Platform        string
	PackageFamily   string
}

// CatalogItem is one build-in tool ready to seed into the softwares catalog.
type CatalogItem struct {
	Software SoftwareMeta
	Versions []VersionMeta
	// Key is the stable endpoint segment (htop, sysstat, iftop).
	Key string
	// Binary is the primary CLI used for presence probes.
	Binary string
}
