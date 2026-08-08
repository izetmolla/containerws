package softwarepkg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	DefaultHubImage      = "izetmolla/containerws"
	defaultHubTagsURL    = "https://hub.docker.com/v2/repositories/%s/%s/tags"
	defaultHubHTTPTimeout = 30 * time.Second
)

// HubTag is one Docker Hub tag for a containerws (or similar) image.
type HubTag struct {
	Name         string   `json:"name"`
	DistroID     string   `json:"distro_id,omitempty"`
	DistroVersion string  `json:"distro_version,omitempty"`
	Image        string   `json:"image"` // full image:tag
	PackageFamily string  `json:"package_family,omitempty"`
	Architectures []string `json:"architectures,omitempty"`
	Workspace    bool     `json:"workspace"` // true when tag maps to a runnable workspace distro
	LastUpdated  string   `json:"last_updated,omitempty"`
}

// ListHubTagsOptions controls Docker Hub tag listing.
type ListHubTagsOptions struct {
	// Image is "namespace/name" or "namespace/name:ignored" (default izetmolla/containerws).
	Image string
	// HTTP client override (tests).
	HTTP *http.Client
	// IncludeNonWorkspace keeps latest/binoptimization/unparsed tags.
	IncludeNonWorkspace bool
}

// ListHubTags fetches tags from Docker Hub for the containerws image.
// See https://hub.docker.com/r/izetmolla/containerws
func ListHubTags(ctx context.Context, opts *ListHubTagsOptions) ([]HubTag, error) {
	image := DefaultHubImage
	client := &http.Client{Timeout: defaultHubHTTPTimeout}
	includeAll := false
	if opts != nil {
		if strings.TrimSpace(opts.Image) != "" {
			image = strings.TrimSpace(opts.Image)
		}
		if opts.HTTP != nil {
			client = opts.HTTP
		}
		includeAll = opts.IncludeNonWorkspace
	}
	ns, repo, err := splitHubImage(image)
	if err != nil {
		return nil, err
	}

	out := make([]HubTag, 0)
	next := fmt.Sprintf(defaultHubTagsURL+"?page_size=100", url.PathEscape(ns), url.PathEscape(repo))
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("docker hub tags: %w", err)
		}
		var body hubTagsPage
		decErr := json.NewDecoder(res.Body).Decode(&body)
		_ = res.Body.Close()
		if decErr != nil {
			return nil, fmt.Errorf("docker hub tags decode: %w", decErr)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("docker hub tags: HTTP %d", res.StatusCode)
		}
		for _, row := range body.Results {
			tag := hubTagFromAPI(ns+"/"+repo, row)
			if !includeAll && !tag.Workspace {
				continue
			}
			out = append(out, tag)
		}
		next = strings.TrimSpace(body.Next)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DistroID != out[j].DistroID {
			return out[i].DistroID < out[j].DistroID
		}
		if out[i].DistroVersion != out[j].DistroVersion {
			return out[i].DistroVersion > out[j].DistroVersion
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ParseContainerWSTag maps a Hub tag name to distro_id + distro_version.
// Examples: ubuntu-26.04 → (ubuntu, 26.04), debian-13 → (debian, 13), kali-rolling → (kali, rolling).
func ParseContainerWSTag(tag string) (distroID, distroVersion string, ok bool) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" || tag == "latest" || tag == "binoptimization" {
		return "", "", false
	}
	known := []string{
		"almalinux", "ubuntu", "debian", "fedora", "centos", "rocky", "rhel",
		"alpine", "arch", "kali", "opensuse", "suse",
	}
	for _, d := range known {
		prefix := d + "-"
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		rem := strings.TrimPrefix(tag, prefix)
		if rem == "" {
			return "", "", false
		}
		// Workspace tags are <distro>-<version> with a single version token (26.04, 13, rolling).
		if !strings.Contains(rem, "-") {
			return d, rem, true
		}
		// Reject Dockerfile.<extra> style: ubuntu-26.04-app
		parts := strings.SplitN(rem, "-", 2)
		if isVersionToken(parts[0]) {
			return "", "", false
		}
		return d, rem, true
	}
	return "", "", false
}

// HubTargets converts workspace Hub tags into DistroTargets for scaffolding.
func HubTargets(tags []HubTag, softName string, req ScaffoldRequest) []DistroTarget {
	out := make([]DistroTarget, 0, len(tags))
	seen := map[string]struct{}{}
	for _, t := range tags {
		if !t.Workspace || t.DistroID == "" {
			continue
		}
		key := t.DistroID + "/" + t.DistroVersion + "/any"
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		family := packageFamilyForDistro(t.DistroID)
		pkg := pkgNameForFamily(family, softName, req)
		out = append(out, DistroTarget{
			DistroID:      t.DistroID,
			DistroVersion: t.DistroVersion,
			Arch:          "any",
			PackageFamily: family,
			PkgName:       pkg,
			Image:         t.Image,
		})
	}
	return out
}

type hubTagsPage struct {
	Count   int           `json:"count"`
	Next    string        `json:"next"`
	Results []hubTagRow   `json:"results"`
}

type hubTagRow struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
	Images      []struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
	} `json:"images"`
}

func hubTagFromAPI(repo string, row hubTagRow) HubTag {
	tag := HubTag{
		Name:        row.Name,
		Image:       repo + ":" + row.Name,
		LastUpdated: row.LastUpdated,
	}
	for _, img := range row.Images {
		arch := strings.TrimSpace(img.Architecture)
		if arch == "" || arch == "unknown" {
			continue
		}
		tag.Architectures = append(tag.Architectures, arch)
	}
	if d, v, ok := ParseContainerWSTag(row.Name); ok {
		tag.DistroID = d
		tag.DistroVersion = v
		tag.PackageFamily = packageFamilyForDistro(d)
		tag.Workspace = true
	}
	return tag
}

func splitHubImage(image string) (ns, repo string, err error) {
	image = strings.TrimSpace(image)
	image = strings.SplitN(image, ":", 2)[0]
	parts := strings.Split(image, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("hub image must be namespace/name (got %q)", image)
	}
	return parts[0], parts[1], nil
}

func isVersionToken(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if s == "rolling" || s == "stable" || s == "sid" || s == "testing" || s == "rawhide" {
		return true
	}
	hasDigit := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		if r == '.' {
			continue
		}
		return false
	}
	return hasDigit
}

func packageFamilyForDistro(distroID string) string {
	switch strings.ToLower(strings.TrimSpace(distroID)) {
	case "ubuntu", "debian", "kali":
		return "apt"
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return "dnf"
	case "alpine":
		return "apk"
	case "arch", "manjaro":
		return "pacman"
	default:
		return "mixed"
	}
}

func pkgNameForFamily(family, softName string, req ScaffoldRequest) string {
	switch family {
	case "apt":
		return firstNonEmpty(req.AptPackage, softName)
	case "dnf":
		return firstNonEmpty(req.DnfPackage, softName)
	case "apk":
		return firstNonEmpty(req.ApkPackage, softName)
	case "pacman":
		return firstNonEmpty(req.PacmanPackage, softName)
	default:
		return firstNonEmpty(req.AptPackage, req.DnfPackage, softName)
	}
}
