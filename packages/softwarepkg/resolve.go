package softwarepkg

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/machine"
)

const defaultRef = "main"

// HostFacts is the subset of host identity used for registry path resolution.
type HostFacts struct {
	DistroID      string
	DistroVersion string
	Arch          string
}

// HostFromMachine builds HostFacts from machine.Detect().
func HostFromMachine() HostFacts {
	snap := machine.Detect()
	return HostFacts{
		DistroID:      strings.ToLower(strings.TrimSpace(snap.DistroID)),
		DistroVersion: strings.TrimSpace(snap.DistroVersion),
		Arch:          models.NormalizeArch(snap.Arch),
	}
}

// ResolveInstallPaths returns relative registry paths to try (most specific first).
// Layout: softwares/{name}/{distro_id}/{distro_version}/{arch}/install.json
func ResolveInstallPaths(softwareName string, host HostFacts) []string {
	name := sanitizeSegment(softwareName)
	if name == "" {
		return nil
	}
	distro := sanitizeSegment(host.DistroID)
	ver := sanitizeSegment(host.DistroVersion)
	arch := sanitizeSegment(models.NormalizeArch(host.Arch))
	if arch == "" {
		arch = "any"
	}

	out := make([]string, 0, 5)
	seen := map[string]struct{}{}
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	if distro != "" && ver != "" && arch != "" {
		add(fmt.Sprintf("softwares/%s/%s/%s/%s/install.json", name, distro, ver, arch))
	}
	if distro != "" && ver != "" {
		add(fmt.Sprintf("softwares/%s/%s/%s/any/install.json", name, distro, ver))
	}
	if distro != "" && arch != "" {
		add(fmt.Sprintf("softwares/%s/%s/any/%s/install.json", name, distro, arch))
	}
	if distro != "" {
		add(fmt.Sprintf("softwares/%s/%s/any/any/install.json", name, distro))
	}
	add(fmt.Sprintf("softwares/%s/default/install.json", name))
	return out
}

// PackageMetaPath is the shared metadata file for a software.
func PackageMetaPath(softwareName string) string {
	name := sanitizeSegment(softwareName)
	if name == "" {
		return ""
	}
	return fmt.Sprintf("softwares/%s/package.json", name)
}

// RawBaseURL converts a PackageURL into a raw content base ending without a trailing slash.
// Supports:
//   - https://github.com/owner/repo[/tree/ref[/…]]
//   - https://raw.githubusercontent.com/owner/repo/ref
//   - any other http(s) base used as-is (trimmed)
func RawBaseURL(packageURL, ref string) (string, error) {
	raw := strings.TrimSpace(packageURL)
	if raw == "" {
		return "", fmt.Errorf("package_url is empty")
	}
	if ref == "" {
		ref = defaultRef
	}
	ref = strings.Trim(ref, "/")

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse package_url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("package_url must be an absolute http(s) URL")
	}

	host := strings.ToLower(u.Host)
	switch {
	case host == "github.com" || host == "www.github.com":
		parts := splitPath(u.Path)
		if len(parts) < 2 {
			return "", fmt.Errorf("github package_url must be https://github.com/owner/repo")
		}
		owner, repo := parts[0], strings.TrimSuffix(parts[1], ".git")
		// Optional: /owner/repo/tree/<ref>/…
		if len(parts) >= 4 && parts[2] == "tree" {
			ref = parts[3]
		}
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", owner, repo, ref), nil

	case host == "raw.githubusercontent.com":
		parts := splitPath(u.Path)
		if len(parts) < 3 {
			return "", fmt.Errorf("raw.githubusercontent.com URL must include owner/repo/ref")
		}
		owner, repo, pathRef := parts[0], parts[1], parts[2]
		useRef := pathRef
		if ref != "" && ref != defaultRef {
			useRef = ref
		}
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", owner, repo, useRef), nil

	default:
		return strings.TrimRight(raw, "/"), nil
	}
}

// JoinRawURL joins a raw base with a relative registry path.
func JoinRawURL(rawBase, relPath string) string {
	base := strings.TrimRight(strings.TrimSpace(rawBase), "/")
	rel := strings.TrimLeft(path.Clean("/"+strings.TrimSpace(relPath)), "/")
	if base == "" || rel == "" || rel == "." {
		return base
	}
	return base + "/" + rel
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		out = append(out, part)
	}
	return out
}

func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "..", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	return s
}

// CandidateSlugs returns registry folder names to try for a display name.
// e.g. "Rust Language" → ["rust-language", "rust"] so renamed packages still resolve.
func CandidateSlugs(softwareName string) []string {
	primary := sanitizeSegment(softwareName)
	if primary == "" {
		return nil
	}
	out := []string{primary}
	seen := map[string]struct{}{primary: {}}
	add := func(s string) {
		s = sanitizeSegment(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	// First hyphen segment: rust-language → rust
	if i := strings.Index(primary, "-"); i > 0 {
		add(primary[:i])
	}
	// First whitespace token before sanitize (already collapsed to hyphens above).
	raw := strings.ToLower(strings.TrimSpace(softwareName))
	if f := strings.Fields(raw); len(f) > 0 {
		add(f[0])
	}
	return out
}
