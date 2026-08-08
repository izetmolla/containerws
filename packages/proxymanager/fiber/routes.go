package fiberproxy

import (
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/izetmolla/containerws/models"
)

// Route is one resolved host+path → upstream mapping for Fiber.
type Route struct {
	Host          string            `json:"host"`
	PathPrefix    string            `json:"path_prefix"`
	UpstreamURL   string            `json:"upstream_url"`
	StripPrefix   bool              `json:"strip_prefix"`
	Websocket     bool              `json:"websocket"`
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
	HostID        string            `json:"host_id"`
	LocationID    string            `json:"location_id,omitempty"`
}

// RedirectRule is an in-process redirect.
type RedirectRule struct {
	FromHost     string `json:"from_host"`
	FromPath     string `json:"from_path"`
	ToURL        string `json:"to_url"`
	StatusCode   int    `json:"status_code"`
	PreservePath bool   `json:"preserve_path"`
}

// Table is the live Fiber proxy route table.
type Table struct {
	Routes    []Route        `json:"routes"`
	Redirects []RedirectRule `json:"redirects"`
	Engine    string         `json:"engine"`
	Active    bool           `json:"active"`
}

var (
	mu   sync.RWMutex
	live = &Table{Routes: nil, Redirects: nil, Engine: models.ProxyEngineFiber, Active: false}
)

// Get returns a copy of the live table.
func Get() Table {
	mu.RLock()
	defer mu.RUnlock()
	out := *live
	out.Routes = append([]Route(nil), live.Routes...)
	out.Redirects = append([]RedirectRule(nil), live.Redirects...)
	return out
}

// Set replaces the live table.
func Set(t *Table) {
	mu.Lock()
	defer mu.Unlock()
	if t == nil {
		live = &Table{Engine: models.ProxyEngineFiber, Active: false}
		return
	}
	cp := *t
	cp.Routes = append([]Route(nil), t.Routes...)
	cp.Redirects = append([]RedirectRule(nil), t.Redirects...)
	live = &cp
}

// Clear disables Fiber proxying.
func Clear() {
	Set(&Table{Engine: models.ProxyEngineFiber, Active: false})
}

// BuildInput is the data needed to build a route table (avoids importing parent package).
type BuildInput struct {
	ActiveEngine string
	Hosts        []models.ProxyHost
	Redirects    []models.ProxyRedirect
	AppBaseURL   string
}

// BuildTable constructs a Fiber route table.
func BuildTable(in BuildInput) *Table {
	t := &Table{
		Engine: models.ProxyEngineFiber,
		Active: in.ActiveEngine == models.ProxyEngineFiber,
	}
	appBaseURL := strings.TrimRight(strings.TrimSpace(in.AppBaseURL), "/")

	for i := range in.Redirects {
		r := in.Redirects[i]
		t.Redirects = append(t.Redirects, RedirectRule{
			FromHost:     strings.ToLower(r.FromHost),
			FromPath:     normalizePath(r.FromPath),
			ToURL:        r.ToURL,
			StatusCode:   r.StatusCode,
			PreservePath: r.PreservePath,
		})
	}

	for i := range in.Hosts {
		h := &in.Hosts[i]
		headers := map[string]string{}
		for k, v := range h.CustomHeaders {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
		baseUpstream := resolveUpstream(h.UpstreamType, h.UpstreamTarget, appBaseURL)
		domains := h.DomainList()
		if len(h.Locations) == 0 {
			for _, d := range domains {
				t.Routes = append(t.Routes, Route{
					Host:          d,
					PathPrefix:    "/",
					UpstreamURL:   baseUpstream,
					Websocket:     h.Websocket,
					CustomHeaders: headers,
					HostID:        h.ID,
				})
			}
			continue
		}
		for _, d := range domains {
			t.Routes = append(t.Routes, Route{
				Host:          d,
				PathPrefix:    "/",
				UpstreamURL:   baseUpstream,
				Websocket:     h.Websocket,
				CustomHeaders: headers,
				HostID:        h.ID,
			})
			for j := range h.Locations {
				loc := &h.Locations[j]
				up := baseUpstream
				if strings.TrimSpace(loc.UpstreamTarget) != "" {
					up = resolveUpstream(loc.UpstreamType, loc.UpstreamTarget, appBaseURL)
				}
				t.Routes = append(t.Routes, Route{
					Host:          d,
					PathPrefix:    normalizePath(loc.PathPrefix),
					UpstreamURL:   up,
					StripPrefix:   loc.StripPrefix,
					Websocket:     loc.Websocket || h.Websocket,
					CustomHeaders: headers,
					HostID:        h.ID,
					LocationID:    loc.ID,
				})
			}
		}
	}

	sort.SliceStable(t.Routes, func(i, j int) bool {
		if t.Routes[i].Host != t.Routes[j].Host {
			return t.Routes[i].Host < t.Routes[j].Host
		}
		return len(t.Routes[i].PathPrefix) > len(t.Routes[j].PathPrefix)
	})
	return t
}

func resolveUpstream(kind, target, appBaseURL string) string {
	target = strings.TrimSpace(target)
	if kind == models.ProxyUpstreamAppPath {
		if appBaseURL == "" {
			appBaseURL = "http://127.0.0.1"
		}
		if !strings.HasPrefix(target, "/") {
			target = "/" + target
		}
		return appBaseURL + target
	}
	return target
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" {
		p = strings.TrimRight(p, "/")
		if p == "" {
			p = "/"
		}
	}
	return p
}

// Match finds the best route for host+path.
func (t *Table) Match(host, reqPath string) *Route {
	if t == nil || !t.Active {
		return nil
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	reqPath = path.Clean("/" + strings.TrimSpace(reqPath))
	for i := range t.Routes {
		r := &t.Routes[i]
		if r.Host != host && r.Host != "*" {
			continue
		}
		if r.PathPrefix == "/" || reqPath == r.PathPrefix || strings.HasPrefix(reqPath, r.PathPrefix+"/") {
			cp := *r
			return &cp
		}
	}
	return nil
}

// MatchRedirect finds a redirect rule.
func (t *Table) MatchRedirect(host, reqPath string) *RedirectRule {
	if t == nil || !t.Active {
		return nil
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	reqPath = path.Clean("/" + strings.TrimSpace(reqPath))
	for i := range t.Redirects {
		r := &t.Redirects[i]
		if r.FromHost != host && r.FromHost != "*" {
			continue
		}
		prefix := strings.TrimRight(r.FromPath, "/")
		if r.FromPath == "/" || reqPath == r.FromPath || (prefix != "" && strings.HasPrefix(reqPath, prefix+"/")) {
			cp := *r
			return &cp
		}
	}
	return nil
}

// TargetURL builds the upstream URL for a matched route.
func (r *Route) TargetURL(reqPath, rawQuery string) (string, error) {
	if r == nil {
		return "", nil
	}
	base, err := url.Parse(r.UpstreamURL)
	if err != nil {
		return "", err
	}
	p := reqPath
	if r.StripPrefix && r.PathPrefix != "/" {
		p = strings.TrimPrefix(p, r.PathPrefix)
		if p == "" {
			p = "/"
		}
	}
	joined := path.Join(base.Path, p)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	base.Path = joined
	base.RawQuery = rawQuery
	return base.String(), nil
}

// SnapshotJSON marshals the table for audit files.
func SnapshotJSON(t *Table) ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}
