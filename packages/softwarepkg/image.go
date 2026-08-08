package softwarepkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// ImageAction for SetPackageImage.
const (
	ImageActionSet      = "set"
	ImageActionFind     = "find"
	ImageActionGenerate = "generate"
)

// SetImageRequest sets or discovers a package image URL.
type SetImageRequest struct {
	// Action: set | find | generate (default set when Image is non-empty, else find then generate).
	Action string
	// Name is the software / registry package name.
	Name string
	// Image is used when Action=set (https URL or data URI).
	Image string
	// Domain hint for find (e.g. nginx.org). Optional — derived from name when empty.
	Domain string
	// Query extra search slug (defaults to Name).
	Query string
	// Color used when generating an SVG logo (defaults to #0ea5e9).
	Color string
	// ApplyLocal upserts Softwares.Image when a local row exists (or creates metadata-only update).
	ApplyLocal bool
	// OutputDir when set writes package.json image (+ image.svg for generate) under the registry tree.
	OutputDir string
	// Overwrite allows replacing package.json / image.svg under OutputDir.
	Overwrite bool
	HTTP      *Client
}

// SetImageResult describes the chosen image.
type SetImageResult struct {
	Name       string   `json:"name"`
	Action     string   `json:"action"`
	Image      string   `json:"image"`
	Source     string   `json:"source"` // set | clearbit | google_favicon | simple_icons | duckduckgo | generated
	Candidates []string `json:"candidates,omitempty"`
	Applied    bool     `json:"applied"`
	SoftwareID string   `json:"software_id,omitempty"`
	Files      []string `json:"files,omitempty"`
	Message    string   `json:"message"`
}

// SetPackageImage finds/generates/sets an image URL and optionally applies it locally and/or to a registry tree.
func SetPackageImage(ctx context.Context, db *gorm.DB, req SetImageRequest) (*SetImageResult, error) {
	name := sanitizeSegment(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		if strings.TrimSpace(req.Image) != "" {
			action = ImageActionSet
		} else {
			action = ImageActionFind
		}
	}

	client := req.HTTP
	if client == nil {
		client = &Client{}
	}

	res := &SetImageResult{Name: name, Action: action, Candidates: []string{}}
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#0ea5e9"
	}

	switch action {
	case ImageActionSet:
		img := strings.TrimSpace(req.Image)
		if img == "" {
			return nil, errors.New("image URL is required for action=set")
		}
		if err := validateImageRef(img); err != nil {
			return nil, err
		}
		res.Image = img
		res.Source = "set"
	case ImageActionFind:
		found, source, tried, err := FindLogoURL(ctx, client, FindLogoOptions{
			Name:   name,
			Query:  firstNonEmpty(strings.TrimSpace(req.Query), name),
			Domain: strings.TrimSpace(req.Domain),
		})
		res.Candidates = tried
		if err != nil || found == "" {
			// Fall through to generate when find fails.
			svg := GenerateLogoSVG(name, color)
			res.Image = SVGDataURI(svg)
			res.Source = "generated"
			res.Action = ImageActionGenerate
			if err != nil {
				res.Message = "find failed (" + err.Error() + "); generated SVG logo"
			} else {
				res.Message = "no public logo found; generated SVG logo"
			}
		} else {
			res.Image = found
			res.Source = source
			res.Message = "found logo via " + source
		}
	case ImageActionGenerate:
		svg := GenerateLogoSVG(name, color)
		res.Image = SVGDataURI(svg)
		res.Source = "generated"
		res.Message = "generated SVG logo"
	default:
		return nil, fmt.Errorf("unknown action %q (use set, find, or generate)", action)
	}

	if req.ApplyLocal && db != nil {
		sw, err := applyImageLocal(ctx, db, name, res.Image)
		if err != nil {
			return nil, err
		}
		if sw != nil {
			res.Applied = true
			res.SoftwareID = sw.ID
		}
	}

	if dir := strings.TrimSpace(req.OutputDir); dir != "" {
		files, err := writeImageToRegistry(dir, name, res.Image, res.Source == "generated", color, req.Overwrite)
		if err != nil {
			return nil, err
		}
		res.Files = files
		// Prefer registry-relative file URL path for generated assets when written.
		if res.Source == "generated" {
			for _, f := range files {
				if strings.HasSuffix(f, "/image.svg") || strings.HasSuffix(f, "image.svg") {
					// Keep data URI for local apply; package.json gets relative path.
					break
				}
			}
		}
	}

	if res.Message == "" {
		res.Message = fmt.Sprintf("image set for %s (%s)", name, res.Source)
	}
	return res, nil
}

// FindLogoOptions controls logo URL discovery.
type FindLogoOptions struct {
	Name   string
	Query  string
	Domain string
}

// FindLogoURL probes common public logo CDNs. Returns URL, source label, and tried candidates.
func FindLogoURL(ctx context.Context, client *Client, opts FindLogoOptions) (string, string, []string, error) {
	if client == nil {
		client = &Client{}
	}
	slug := sanitizeSegment(opts.Query)
	if slug == "" {
		slug = sanitizeSegment(opts.Name)
	}
	domain := strings.TrimSpace(opts.Domain)
	if domain == "" {
		domain = guessDomain(slug)
	}
	domain = strings.TrimPrefix(strings.ToLower(domain), "www.")

	type cand struct {
		url    string
		source string
	}
	cands := make([]cand, 0, 6)
	if domain != "" {
		cands = append(cands,
			cand{fmt.Sprintf("https://logo.clearbit.com/%s", domain), "clearbit"},
			cand{fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=128", url.QueryEscape(domain)), "google_favicon"},
			cand{fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", domain), "duckduckgo"},
		)
	}
	if slug != "" {
		cands = append(cands,
			cand{fmt.Sprintf("https://cdn.simpleicons.org/%s", slug), "simple_icons"},
			cand{fmt.Sprintf("https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/%s.svg", slug), "simple_icons"},
		)
	}

	tried := make([]string, 0, len(cands))
	for _, c := range cands {
		tried = append(tried, c.url)
		ok, err := probeImageURL(ctx, client, c.url)
		if err != nil {
			continue
		}
		if ok {
			return c.url, c.source, tried, nil
		}
	}
	return "", "", tried, fmt.Errorf("no logo URL responded with an image")
}

// GenerateLogoSVG builds a simple branded square logo with initials.
func GenerateLogoSVG(name, color string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "pkg"
	}
	if color == "" {
		color = "#0ea5e9"
	}
	initials := packageInitials(name)
	escColor := xmlEscape(color)
	escText := xmlEscape(initials)
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" role="img">`+
			`<rect width="128" height="128" rx="28" fill="%s"/>`+
			`<text x="64" y="72" text-anchor="middle" font-family="ui-sans-serif,system-ui,sans-serif" `+
			`font-size="44" font-weight="700" fill="#ffffff">%s</text></svg>`,
		escColor, escText,
	)
}

// SVGDataURI encodes an SVG as a data URI suitable for <img src>.
func SVGDataURI(svg string) string {
	// Prefer URL-encoded UTF-8 so we avoid base64 bloat and keep it readable.
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(svg)
}

func validateImageRef(img string) error {
	img = strings.TrimSpace(img)
	if img == "" {
		return errors.New("image is empty")
	}
	if strings.HasPrefix(img, "data:image/") {
		return nil
	}
	u, err := url.Parse(img)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("image must be an http(s) URL or data:image URI")
	}
	return nil
}

func applyImageLocal(ctx context.Context, db *gorm.DB, name, image string) (*models.Software, error) {
	var sw models.Software
	err := db.WithContext(ctx).Where("LOWER(name) = ?", strings.ToLower(name)).First(&sw).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sw.Image = image
	if err := db.WithContext(ctx).Save(&sw).Error; err != nil {
		return nil, err
	}
	return &sw, nil
}

func writeImageToRegistry(root, name, image string, generated bool, color string, overwrite bool) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(abs, PackageMetaPath(name))
	var meta PackageMeta
	if raw, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(raw, &meta)
	}
	if meta.Name == "" {
		meta.Name = name
	}
	files := make([]string, 0, 2)

	imgRef := image
	if generated {
		svgPath := filepath.Join(abs, "softwares", sanitizeSegment(name), "image.svg")
		if err := os.MkdirAll(filepath.Dir(svgPath), 0o755); err != nil {
			return nil, err
		}
		if !overwrite {
			if _, err := os.Stat(svgPath); err == nil {
				return nil, fmt.Errorf("refusing to overwrite %s (set overwrite=true)", svgPath)
			}
		}
		svg := GenerateLogoSVG(name, color)
		if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join("softwares", sanitizeSegment(name), "image.svg"))
		files = append(files, rel)
		imgRef = rel
	}

	meta.Image = imgRef
	if err := writeJSONFile(metaPath, meta, true); err != nil {
		return nil, err
	}
	files = append(files, relFrom(abs, metaPath))

	// Keep index.json image in sync when present.
	_ = updateIndexImage(abs, name, imgRef)
	return files, nil
}

func updateIndexImage(root, name, image string) error {
	indexPath := filepath.Join(root, catalogIndexPath)
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var idx CatalogIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return err
	}
	changed := false
	for i := range idx.Softwares {
		if strings.EqualFold(idx.Softwares[i].Name, name) {
			idx.Softwares[i].Image = image
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	out, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(indexPath, out, 0o644)
}

func probeImageURL(ctx context.Context, client *Client, rawURL string) (bool, error) {
	httpClient := client.httpClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "containerws-softwarepkg/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 64)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, nil
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "image/") || strings.Contains(ct, "svg") || ct == "" {
		return true, nil
	}
	// Some CDNs return octet-stream / html wrappers — accept 200 with known hosts.
	u, _ := url.Parse(rawURL)
	if u != nil {
		host := strings.ToLower(u.Host)
		if strings.Contains(host, "clearbit") || strings.Contains(host, "simpleicons") ||
			strings.Contains(host, "gstatic") || strings.Contains(host, "google.com") ||
			strings.Contains(host, "duckduckgo") || strings.Contains(host, "jsdelivr") {
			return true, nil
		}
	}
	return false, nil
}

func guessDomain(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	known := map[string]string{
		"nginx": "nginx.org", "docker": "docker.com", "nodejs": "nodejs.org", "node": "nodejs.org",
		"go": "go.dev", "golang": "go.dev", "vscode": "code.visualstudio.com", "code": "code.visualstudio.com",
		"chrome": "google.com", "chromium": "chromium.org", "cursor": "cursor.com", "htop": "htop.dev",
		"postgresql": "postgresql.org", "postgres": "postgresql.org", "mysql": "mysql.com",
		"redis": "redis.io", "mongodb": "mongodb.com", "python": "python.org", "rust": "rust-lang.org",
		"kubernetes": "kubernetes.io", "k8s": "kubernetes.io", "terraform": "terraform.io",
	}
	if d, ok := known[slug]; ok {
		return d
	}
	if strings.Contains(slug, ".") {
		return slug
	}
	return slug + ".com"
}

func packageInitials(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return "P"
	}
	if len(parts) == 1 {
		s := strings.ToUpper(parts[0])
		if len(s) >= 2 {
			return s[:2]
		}
		return s
	}
	a := []rune(strings.ToUpper(parts[0]))
	b := []rune(strings.ToUpper(parts[1]))
	out := ""
	if len(a) > 0 {
		out += string(a[0])
	}
	if len(b) > 0 {
		out += string(b[0])
	}
	if out == "" {
		return "P"
	}
	return out
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// AbsoluteImageURL turns a relative registry image path into a raw URL, or returns
// absolute http(s)/data URIs unchanged.
func AbsoluteImageURL(rawBase, image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if strings.HasPrefix(image, "data:") {
		return image
	}
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		return image
	}
	return JoinRawURL(rawBase, strings.TrimPrefix(image, "/"))
}
