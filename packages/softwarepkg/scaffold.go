package softwarepkg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DistroTarget describes one install.json tree to write under softwares/{name}/.
type DistroTarget struct {
	DistroID      string // ubuntu, debian, fedora, alpine, arch, default
	DistroVersion string // concrete VERSION_ID or "any"
	Arch          string // amd64, arm64, any
	PackageFamily string // apt, dnf, apk, pacman, mixed
	PkgName       string // OS package name (defaults to software name)
	Image         string // optional Hub image:tag used for install tests
}

// ScaffoldRequest builds a registry package tree on disk.
type ScaffoldRequest struct {
	Name         string
	Details      string
	Category     string
	SubCategory  string
	Tags         []string
	Icon         string
	Image        string // optional logo URL (https or data URI)
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
	// Distros to scaffold (e.g. ubuntu,debian,fedora,alpine,arch,default). Empty → common set.
	Distros []string
	// Targets, when non-empty, are used instead of Distros (exact distro/version/arch paths).
	Targets []DistroTarget
	// FromHub loads izetmolla/containerws (or HubImage) tags and scaffolds one install.json per tag.
	FromHub bool
	// HubImage is namespace/name (default izetmolla/containerws).
	HubImage string
	// AlsoAny also writes {distro}/any/any/install.json for each unique distro_id from Hub.
	AlsoAny bool
	// AlsoDefault writes softwares/{name}/default/install.json (default true when FromHub).
	AlsoDefault *bool
	// Optional OS package name overrides (default: Name).
	AptPackage    string
	DnfPackage    string
	ApkPackage    string
	PacmanPackage string
	// CustomScript is copied into every install.json (post-install setup; optional).
	CustomScript string
	// OutputDir is the registry repo root (contains softwares/).
	OutputDir string
	Overwrite bool
	// UpdateIndex merges this software into softwares/index.json when true (default true if OutputDir set).
	UpdateIndex *bool
}

// ScaffoldResult lists files written.
type ScaffoldResult struct {
	Name      string   `json:"name"`
	Root      string   `json:"root"`
	Files     []string `json:"files"`
	Distros   []string `json:"distros"`
	IndexPath string   `json:"index_path,omitempty"`
	Meta      PackageMeta
}

// DefaultDistros is the common set for multi-distro packages.
func DefaultDistros() []string {
	return []string{"ubuntu", "debian", "fedora", "alpine", "arch", "default"}
}

// Scaffold writes package.json, distro install.json files, and optionally index.json.
func Scaffold(req ScaffoldRequest) (*ScaffoldResult, error) {
	name := sanitizeSegment(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	outDir := strings.TrimSpace(req.OutputDir)
	if outDir == "" {
		return nil, fmt.Errorf("output_dir is required")
	}
	root, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "softwares", name), 0o755); err != nil {
		return nil, err
	}

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
	}
	active := true
	meta.IsActive = &active
	// Fill default systemctl commands when controllable + units present.
	if start, restart, stop := meta.ResolveControlCommands(); true {
		meta.StartCommand, meta.RestartCommand, meta.StopCommand = start, restart, stop
	}

	targets, err := resolveScaffoldTargets(req, name)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no valid distros to scaffold")
	}

	files := make([]string, 0)
	metaPath := filepath.Join(root, PackageMetaPath(name))
	if err := writeJSONFile(metaPath, meta, req.Overwrite); err != nil {
		return nil, err
	}
	files = append(files, relFrom(root, metaPath))

	latest := true
	for _, t := range targets {
		spec := InstallSpec{
			Version:         version,
			IsLatest:        &latest,
			OS:              "linux",
			DistroID:        t.DistroID,
			DistroVersion:   t.DistroVersion,
			Arch:            t.Arch,
			PackageFamily:   t.PackageFamily,
			InstallScript:   BuildInstallScript(t),
			UninstallScript: BuildUninstallScript(t),
			UpgradeScript:   BuildUpgradeScript(t),
			CustomScript:    strings.TrimSpace(req.CustomScript),
		}
		rel := installRelPath(name, t)
		abs := filepath.Join(root, rel)
		if err := writeJSONFile(abs, spec, req.Overwrite); err != nil {
			return nil, err
		}
		files = append(files, rel)
	}

	updateIndex := true
	if req.UpdateIndex != nil {
		updateIndex = *req.UpdateIndex
	}
	indexPath := ""
	if updateIndex {
		ip, err := mergeCatalogIndex(root, meta, req.Overwrite)
		if err != nil {
			return nil, err
		}
		indexPath = ip
		files = append(files, relFrom(root, ip))
	}

	distroIDs := make([]string, 0, len(targets))
	seenDistro := map[string]struct{}{}
	for _, t := range targets {
		key := t.DistroID
		if t.DistroVersion != "" && t.DistroVersion != "any" {
			key = t.DistroID + "-" + t.DistroVersion
		}
		if _, ok := seenDistro[key]; ok {
			continue
		}
		seenDistro[key] = struct{}{}
		distroIDs = append(distroIDs, key)
	}
	return &ScaffoldResult{
		Name:      name,
		Root:      root,
		Files:     files,
		Distros:   distroIDs,
		IndexPath: indexPath,
		Meta:      meta,
	}, nil
}

func resolveScaffoldTargets(req ScaffoldRequest, softName string) ([]DistroTarget, error) {
	if len(req.Targets) > 0 {
		return req.Targets, nil
	}
	if req.FromHub {
		tags, err := ListHubTags(context.Background(), &ListHubTagsOptions{Image: req.HubImage})
		if err != nil {
			return nil, fmt.Errorf("hub tags: %w", err)
		}
		targets := HubTargets(tags, softName, req)
		alsoDefault := true
		if req.AlsoDefault != nil {
			alsoDefault = *req.AlsoDefault
		}
		if req.AlsoAny {
			seen := map[string]struct{}{}
			extra := make([]DistroTarget, 0)
			for _, t := range targets {
				if _, ok := seen[t.DistroID]; ok {
					continue
				}
				seen[t.DistroID] = struct{}{}
				extra = append(extra, DistroTarget{
					DistroID:      t.DistroID,
					DistroVersion: "any",
					Arch:          "any",
					PackageFamily: t.PackageFamily,
					PkgName:       t.PkgName,
				})
			}
			targets = append(targets, extra...)
		}
		if alsoDefault {
			pkg := firstNonEmpty(req.AptPackage, req.DnfPackage, softName)
			targets = append(targets, DistroTarget{
				DistroID:      "default",
				PackageFamily: "mixed",
				PkgName:       pkg,
			})
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("no workspace tags found on Docker Hub for %s", firstNonEmpty(req.HubImage, DefaultHubImage))
		}
		return targets, nil
	}

	distros := req.Distros
	if len(distros) == 0 {
		distros = DefaultDistros()
	}
	targets := make([]DistroTarget, 0, len(distros))
	for _, d := range distros {
		t, ok := resolveDistroTarget(d, softName, req)
		if !ok {
			continue
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func resolveDistroTarget(raw, softName string, req ScaffoldRequest) (DistroTarget, bool) {
	id := strings.ToLower(strings.TrimSpace(raw))
	id = strings.ReplaceAll(id, " ", "")
	switch id {
	case "ubuntu", "debian", "kali":
		pkg := firstNonEmpty(req.AptPackage, softName)
		ver := "any"
		if id == "kali" {
			// Match Hub tag kali-rolling path when scaffolding by distro id alone.
			ver = "any"
		}
		return DistroTarget{DistroID: id, DistroVersion: ver, Arch: "any", PackageFamily: "apt", PkgName: pkg}, true
	case "fedora", "rhel", "centos", "rocky", "almalinux", "alma":
		if id == "alma" {
			id = "almalinux"
		}
		pkg := firstNonEmpty(req.DnfPackage, softName)
		return DistroTarget{DistroID: id, DistroVersion: "any", Arch: "any", PackageFamily: "dnf", PkgName: pkg}, true
	case "alpine":
		pkg := firstNonEmpty(req.ApkPackage, softName)
		return DistroTarget{DistroID: id, DistroVersion: "any", Arch: "any", PackageFamily: "apk", PkgName: pkg}, true
	case "arch", "manjaro":
		pkg := firstNonEmpty(req.PacmanPackage, softName)
		return DistroTarget{DistroID: id, DistroVersion: "any", Arch: "any", PackageFamily: "pacman", PkgName: pkg}, true
	case "default", "any", "linux":
		pkg := firstNonEmpty(req.AptPackage, req.DnfPackage, softName)
		return DistroTarget{DistroID: "default", DistroVersion: "", Arch: "", PackageFamily: "mixed", PkgName: pkg}, true
	default:
		return DistroTarget{}, false
	}
}

func installRelPath(name string, t DistroTarget) string {
	if t.DistroID == "default" || t.DistroID == "" {
		return fmt.Sprintf("softwares/%s/default/install.json", name)
	}
	ver := t.DistroVersion
	if ver == "" {
		ver = "any"
	}
	arch := t.Arch
	if arch == "" {
		arch = "any"
	}
	return fmt.Sprintf("softwares/%s/%s/%s/%s/install.json", name, t.DistroID, ver, arch)
}

// aptUpdateSafeBash is embedded into apt install/upgrade scripts so a broken
// third-party repo (e.g. stale Microsoft VS Code OpenPGP key) does not make
// apt-get update exit 100 and block installing distro packages like nginx.
const aptUpdateSafeBash = `# Prefer distro archives only — ignore broken third-party repos (exit 100).
apt_update_safe() {
  local opts=(-o APT::Get::List-Cleanup=0)
  if [[ -f /etc/apt/sources.list.d/ubuntu.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/ubuntu.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list.d/debian.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/debian.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list.d/kali.sources ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list.d/kali.sources \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  if [[ -f /etc/apt/sources.list ]]; then
    apt-get update "${opts[@]}" \
      -o Dir::Etc::sourcelist=sources.list \
      -o Dir::Etc::sourceparts=- && return 0
  fi
  apt-get update \
    -o Acquire::AllowInsecureRepositories=true \
    -o Acquire::AllowDowngradeToInsecureRepositories=true
}
apt_update_safe
`

// BuildInstallScript returns a bash install script for the distro family.
func BuildInstallScript(t DistroTarget) string {
	pkg := shellQuote(t.PkgName)
	switch t.PackageFamily {
	case "apt":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
%sapt-get install -y --no-install-recommends %s
command -v %s >/dev/null
`, aptUpdateSafeBash, pkg, t.PkgName)
	case "dnf":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
dnf install -y %s
`, pkg)
	case "apk":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
apk add --no-cache %s
`, pkg)
	case "pacman":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
pacman -Sy --noconfirm %s
`, pkg)
	default:
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
PKG=%s
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
%s  apt-get install -y --no-install-recommends "$PKG"
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y "$PKG"
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache "$PKG"
elif command -v pacman >/dev/null 2>&1; then
  pacman -Sy --noconfirm "$PKG"
else
  echo "unsupported package manager" >&2
  exit 1
fi
`, pkg, indentBlock(aptUpdateSafeBash, "  "))
	}
}

// BuildUninstallScript returns a bash uninstall script for the distro family.
func BuildUninstallScript(t DistroTarget) string {
	pkg := shellQuote(t.PkgName)
	switch t.PackageFamily {
	case "apt":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get remove -y %s || true
apt-get autoremove -y || true
`, pkg)
	case "dnf":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
dnf remove -y %s || true
`, pkg)
	case "apk":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
apk del %s || true
`, pkg)
	case "pacman":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
pacman -R --noconfirm %s || true
`, pkg)
	default:
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
PKG=%s
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get remove -y "$PKG" || true
elif command -v dnf >/dev/null 2>&1; then
  dnf remove -y "$PKG" || true
elif command -v apk >/dev/null 2>&1; then
  apk del "$PKG" || true
elif command -v pacman >/dev/null 2>&1; then
  pacman -R --noconfirm "$PKG" || true
fi
`, pkg)
	}
}

// BuildUpgradeScript returns a bash upgrade script for the distro family.
func BuildUpgradeScript(t DistroTarget) string {
	pkg := shellQuote(t.PkgName)
	switch t.PackageFamily {
	case "apt":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
%sapt-get install -y --only-upgrade %s
`, aptUpdateSafeBash, pkg)
	case "dnf":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
dnf upgrade -y %s
`, pkg)
	case "apk":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
apk upgrade %s
`, pkg)
	case "pacman":
		return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
pacman -Syu --noconfirm %s
`, pkg)
	default:
		return BuildInstallScript(t)
	}
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func mergeCatalogIndex(root string, meta PackageMeta, overwrite bool) (string, error) {
	indexPath := filepath.Join(root, catalogIndexPath)
	var idx CatalogIndex
	if raw, err := os.ReadFile(indexPath); err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &idx)
	}
	found := -1
	for i, m := range idx.Softwares {
		if strings.EqualFold(strings.TrimSpace(m.Name), meta.Name) {
			found = i
			break
		}
	}
	if found >= 0 {
		if !overwrite {
			return indexPath, nil
		}
		idx.Softwares[found] = meta
	} else {
		idx.Softwares = append(idx.Softwares, meta)
	}
	if err := writeJSONFile(indexPath, idx, true); err != nil {
		return "", err
	}
	return indexPath, nil
}

func writeJSONFile(path string, v any, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s (set overwrite=true)", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func relFrom(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func shellQuote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '+' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
