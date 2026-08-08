package softwarepkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/models"
)

// Auth holds credentials from models.SoftwarePackage.
type Auth struct {
	Token    string
	Username string
	Password string
}

// AuthFromPackage maps a SoftwarePackage row to Auth.
func AuthFromPackage(pkg models.SoftwarePackage) Auth {
	return Auth{
		Token:    strings.TrimSpace(pkg.Token),
		Username: strings.TrimSpace(pkg.Username),
		Password: pkg.Password,
	}
}

// PackageMeta is softwares/{name}/package.json
type PackageMeta struct {
	Name           string   `json:"name"`
	Details        string   `json:"details"`
	Category       string   `json:"category"`
	SubCategory    string   `json:"sub_category"`
	Tags           []string `json:"tags"`
	Icon           string   `json:"icon"`
	Image          string   `json:"image,omitempty"` // logo URL (https) or data:image URI / relative registry path
	Color          string   `json:"color"`
	Order          int      `json:"order"`
	ServiceUnits   []string `json:"service_units"`
	CanControl     *bool    `json:"can_control,omitempty"`     // Start/Stop/Restart typology; nil → infer from service_units
	ControlBackend string   `json:"control_backend,omitempty"` // systemd | docker
	StartCommand   string   `json:"start_command,omitempty"`
	RestartCommand string   `json:"restart_command,omitempty"`
	StopCommand    string   `json:"stop_command,omitempty"`
	IsActive       *bool    `json:"is_active"`
}

// ResolveControlFields returns can_control + control_backend for catalog upserts.
// When CanControl is omitted, non-empty service_units imply controllable.
func (m PackageMeta) ResolveControlFields() (canControl bool, backend string) {
	units := cleanStrings(m.ServiceUnits)
	if m.CanControl != nil {
		canControl = *m.CanControl
	} else {
		canControl = len(units) > 0 ||
			strings.TrimSpace(m.StartCommand) != "" ||
			strings.TrimSpace(m.RestartCommand) != "" ||
			strings.TrimSpace(m.StopCommand) != ""
	}
	backend = strings.ToLower(strings.TrimSpace(m.ControlBackend))
	if canControl && backend == "" {
		backend = models.InferSoftwareControlBackend(models.JSONBStringArray(units))
	}
	return canControl, backend
}

// ResolveControlCommands returns start/restart/stop commands, filling systemd
// defaults from service_units when controllable and a field is empty.
func (m PackageMeta) ResolveControlCommands() (start, restart, stop string) {
	start = strings.TrimSpace(m.StartCommand)
	restart = strings.TrimSpace(m.RestartCommand)
	stop = strings.TrimSpace(m.StopCommand)
	canControl, _ := m.ResolveControlFields()
	if !canControl {
		return start, restart, stop
	}
	dStart, dRestart, dStop := models.DefaultSystemdCommands(models.JSONBStringArray(cleanStrings(m.ServiceUnits)))
	if start == "" {
		start = dStart
	}
	if restart == "" {
		restart = dRestart
	}
	if stop == "" {
		stop = dStop
	}
	return start, restart, stop
}

// InstallSpec is softwares/.../install.json
type InstallSpec struct {
	Version          string `json:"version"`
	IsLatest         *bool  `json:"is_latest"`
	OS               string `json:"os"`
	DistroID         string `json:"distro_id"`
	DistroVersion    string `json:"distro_version"`
	Distro           string `json:"distro"`
	Arch             string `json:"arch"`
	Platform         string `json:"platform"`
	PackageFamily    string `json:"package_family"`
	Kernel           string `json:"kernel"`
	Virtualization   string `json:"virtualization"`
	ContainerRuntime string `json:"container_runtime"`
	CloudProvider    string `json:"cloud_provider"`
	InstallScript    string `json:"install_script"`
	UninstallScript  string `json:"uninstall_script"`
	UpgradeScript    string `json:"upgrade_script"`
	// CustomScript runs after a successful install when non-empty (post-setup / config).
	CustomScript string `json:"custom_script,omitempty"`
}

// FetchResult is the successful install.json resolution.
type FetchResult struct {
	Meta        PackageMeta
	Install     InstallSpec
	InstallURL  string
	MetaURL     string
	TriedPaths  []string
	Slug        string // registry folder softwares/{slug}/ that resolved
}

// Client performs registry HTTP fetches.
type Client struct {
	HTTP *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// FetchJSON GETs url and unmarshals JSON into dest. Returns ErrNotFound on 404.
func (c *Client) FetchJSON(ctx context.Context, url string, auth Auth, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	applyAuth(req, auth)

	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d: %s", url, res.StatusCode, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// ErrNotFound means the remote path does not exist (HTTP 404).
var ErrNotFound = fmt.Errorf("not found")

func applyAuth(req *http.Request, auth Auth) {
	if auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Token)
		return
	}
	if auth.Username != "" || auth.Password != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// FetchForHost loads package.json and the first matching install.json for host.
// Tries CandidateSlugs so display names like "Rust Language" still resolve to
// softwares/rust/ when that is the registry folder.
func (c *Client) FetchForHost(
	ctx context.Context,
	rawBase string,
	softwareName string,
	host HostFacts,
	auth Auth,
) (*FetchResult, error) {
	slugs := CandidateSlugs(softwareName)
	if len(slugs) == 0 {
		return nil, fmt.Errorf("software name is required")
	}

	var lastMetaErr error
	triedMeta := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		metaPath := PackageMetaPath(slug)
		triedMeta = append(triedMeta, metaPath)
		metaURL := JoinRawURL(rawBase, metaPath)
		var meta PackageMeta
		err := c.FetchJSON(ctx, metaURL, auth, &meta)
		if err == ErrNotFound {
			lastMetaErr = err
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("package.json: %w", err)
		}
		if strings.TrimSpace(meta.Name) == "" {
			meta.Name = softwareName
		}
		meta.Image = AbsoluteImageURL(rawBase, meta.Image)

		candidates := ResolveInstallPaths(slug, host)
		tried := append([]string{}, triedMeta...)
		for _, rel := range candidates {
			tried = append(tried, rel)
			u := JoinRawURL(rawBase, rel)
			var spec InstallSpec
			err := c.FetchJSON(ctx, u, auth, &spec)
			if err == ErrNotFound {
				continue
			}
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(spec.Version) == "" {
				return nil, fmt.Errorf("%s: version is required", rel)
			}
			return &FetchResult{
				Meta:       meta,
				Install:    spec,
				InstallURL: u,
				MetaURL:    metaURL,
				TriedPaths: tried,
				Slug:       slug,
			}, nil
		}
		return nil, fmt.Errorf("no install.json found for %q (slug %q) on %s/%s/%s (tried %d paths)",
			softwareName, slug, host.DistroID, host.DistroVersion, host.Arch, len(tried))
	}
	if lastMetaErr != nil {
		return nil, fmt.Errorf("package.json: %w (tried %v)", lastMetaErr, triedMeta)
	}
	return nil, fmt.Errorf("package.json not found for %q (tried %v)", softwareName, triedMeta)
}

var (
	probeCacheMu sync.Mutex
	probeCache   = map[string]probeCacheEntry{}
)

type probeCacheEntry struct {
	ok      bool
	expires time.Time
}

const probeCacheTTL = 0 // 0 = keep until InvalidateCatalogCache

// HasInstallForHost reports whether any install.json resolves for host (cached).
// Uses HEAD when possible, falls back to GET. Network errors return false, err.
func (c *Client) HasInstallForHost(
	ctx context.Context,
	rawBase string,
	softwareName string,
	host HostFacts,
	auth Auth,
) (bool, error) {
	slugs := CandidateSlugs(softwareName)
	if len(slugs) == 0 {
		return false, nil
	}
	name := slugs[0]
	cacheKey := strings.TrimRight(rawBase, "/") + "|" + name + "|" +
		sanitizeSegment(host.DistroID) + "|" + sanitizeSegment(host.DistroVersion) + "|" +
		sanitizeSegment(models.NormalizeArch(host.Arch))

	probeCacheMu.Lock()
	if ent, ok := probeCache[cacheKey]; ok && !probeCacheExpired(ent) {
		probeCacheMu.Unlock()
		return ent.ok, nil
	}
	probeCacheMu.Unlock()

	ok := false
	var lastErr error
	for _, slug := range slugs {
		for _, rel := range ResolveInstallPaths(slug, host) {
			u := JoinRawURL(rawBase, rel)
			found, err := c.probeURL(ctx, u, auth)
			if err != nil {
				lastErr = err
				continue
			}
			if found {
				ok = true
				lastErr = nil
				break
			}
		}
		if ok {
			break
		}
	}

	probeCacheMu.Lock()
	ent := probeCacheEntry{ok: ok}
	if probeCacheTTL > 0 {
		ent.expires = time.Now().Add(probeCacheTTL)
	}
	probeCache[cacheKey] = ent
	probeCacheMu.Unlock()

	if !ok && lastErr != nil {
		return false, lastErr
	}
	return ok, nil
}

func probeCacheExpired(ent probeCacheEntry) bool {
	if probeCacheTTL <= 0 {
		return false
	}
	return time.Now().After(ent.expires)
}

func (c *Client) probeURL(ctx context.Context, url string, auth Auth) (bool, error) {
	// Prefer HEAD (cheap). Some hosts reject HEAD — fall back to GET.
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return false, err
		}
		applyAuth(req, auth)
		res, err := c.httpClient().Do(req)
		if err != nil {
			return false, err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64))
		res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return false, nil
		}
		if res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusForbidden {
			if method == http.MethodHead {
				continue
			}
			return false, nil
		}
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return true, nil
		}
		if method == http.MethodHead {
			continue
		}
		return false, fmt.Errorf("probe %s: HTTP %d", url, res.StatusCode)
	}
	return false, nil
}
