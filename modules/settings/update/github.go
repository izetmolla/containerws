package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const defaultUpdateRepo = "izetmolla/containerws"

// updateRepo returns owner/name for GitHub API/download URLs.
// Accepts CONTAINERWS_UPDATE_REPO as owner/name or a github.com URL.
func updateRepo() string {
	v := strings.TrimSpace(os.Getenv("CONTAINERWS_UPDATE_REPO"))
	if v == "" {
		return defaultUpdateRepo
	}
	v = strings.TrimSuffix(v, "/")
	v = strings.TrimSuffix(v, ".git")
	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
		"github.com/",
	} {
		if strings.HasPrefix(strings.ToLower(v), prefix) {
			v = v[len(prefix):]
			break
		}
	}
	v = strings.Trim(v, "/")
	if v == "" {
		return defaultUpdateRepo
	}
	return v
}

func githubReleasesURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", updateRepo())
}

// releaseDownloadURL is the public GitHub Releases CDN URL for an asset.
func releaseDownloadURL(tag, assetName string) string {
	tag = strings.TrimSpace(tag)
	assetName = strings.TrimSpace(assetName)
	if tag == "" || assetName == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", updateRepo(), tag, assetName)
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func githubToken() string {
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func fetchReleases(ctx context.Context, currentVersion string) ([]Release, string, error) {
	url := githubReleasesURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "containerws-update")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("github releases (%s): HTTP %d: %s", updateRepo(), resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw []ghRelease
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", err
	}

	out := make([]Release, 0, len(raw))
	latestStable := ""
	for _, r := range raw {
		if r.Draft {
			continue
		}
		rel := Release{
			Tag:         r.TagName,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			Prerelease:  r.Prerelease,
			Draft:       r.Draft,
			HTMLURL:     r.HTMLURL,
			Body:        r.Body,
			Newer:       isNewer(r.TagName, currentVersion),
		}
		if asset, ok := pickAsset(r.Assets, r.TagName); ok {
			rel.AssetName = asset.Name
			// Prefer the stable public release URL so downloads work without API auth
			// and survive CDN redirects cleanly.
			rel.AssetURL = releaseDownloadURL(r.TagName, asset.Name)
			if rel.AssetURL == "" {
				rel.AssetURL = asset.BrowserDownloadURL
			}
			rel.AssetSize = asset.Size
			rel.HasAsset = true
		}
		out = append(out, rel)
		if !r.Prerelease && latestStable == "" {
			latestStable = r.TagName
		}
	}
	if latestStable == "" && len(out) > 0 {
		latestStable = out[0].Tag
	}
	return out, latestStable, nil
}

func pickAsset(assets []ghAsset, tag string) (ghAsset, bool) {
	expected := expectedAssetName(tag)
	var loose ghAsset
	looseOK := false
	for _, a := range assets {
		if matchAssetExact(a.Name, expected, tag) {
			return a, true
		}
		if !looseOK && matchAssetLoose(a.Name) {
			loose = a
			looseOK = true
		}
	}
	if looseOK {
		return loose, true
	}
	return ghAsset{}, false
}

// expectedAssetName builds the GoReleaser archive basename (without extension).
// name_template: containerws_{Version}_{Os}_{Arch}{Arm}
func expectedAssetName(tag string) string {
	ver := normalizeVersion(tag)
	osName := runtime.GOOS
	arch := runtime.GOARCH
	armSuffix := ""
	if arch == "arm" {
		arm := os.Getenv("GOARM")
		if arm == "" {
			arm = "7"
		}
		armSuffix = "v" + arm
	}
	return fmt.Sprintf("containerws_%s_%s_%s%s", ver, osName, arch, armSuffix)
}

func platformSuffix() string {
	arch := runtime.GOARCH
	if arch == "arm" {
		arm := os.Getenv("GOARM")
		if arm == "" {
			arm = "7"
		}
		return fmt.Sprintf("_%s_armv%s", runtime.GOOS, arm)
	}
	return fmt.Sprintf("_%s_%s", runtime.GOOS, arch)
}

func archiveBase(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".tgz")
	name = strings.TrimSuffix(name, ".zip")
	return name
}

func matchAssetExact(name, expected, tag string) bool {
	base := archiveBase(name)
	if base == "" {
		return false
	}
	if base == expected {
		return true
	}
	// Tolerate tag-with-v in filename (containerws_v0.0.3_linux_amd64).
	alt := fmt.Sprintf("containerws_%s_%s_%s", tag, runtime.GOOS, runtime.GOARCH)
	if runtime.GOARCH == "arm" {
		arm := os.Getenv("GOARM")
		if arm == "" {
			arm = "7"
		}
		alt += "v" + arm
	}
	return base == alt
}

func matchAssetLoose(name string) bool {
	base := archiveBase(name)
	if !strings.HasPrefix(base, "containerws_") {
		return false
	}
	suffix := platformSuffix()
	// Require exact platform suffix at end so linux_arm does not match linux_arm64.
	return strings.HasSuffix(base, suffix)
}

// matchAsset kept for tests / callers; prefers exact then loose.
func matchAsset(name, expected, tag string) bool {
	return matchAssetExact(name, expected, tag) || matchAssetLoose(name)
}
