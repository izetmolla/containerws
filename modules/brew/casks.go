package brew

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	casksAPIURL   = "https://formulae.brew.sh/api/cask.json"
	casksCacheTTL = 6 * time.Hour
)

// Cask is a Homebrew cask (desktop / GUI app) from formulae.brew.sh.
type Cask struct {
	Token     string   `json:"token"`
	FullToken string   `json:"full_token"`
	Tap       string   `json:"tap"`
	Name      []string `json:"name"`
	Desc      string   `json:"desc"`
	Homepage  string   `json:"homepage"`
	Version   string   `json:"version"`
	Deprecated bool    `json:"deprecated"`
	Disabled   bool    `json:"disabled"`
	Category   string  `json:"category,omitempty"`
}

// CaskDetail is the rich single-cask payload.
type CaskDetail struct {
	Cask
	URL string `json:"url"`
	Analytics struct {
		Install struct {
			D30  map[string]int `json:"30d"`
			D90  map[string]int `json:"90d"`
			D365 map[string]int `json:"365d"`
		} `json:"install"`
	} `json:"analytics"`
}

type casksCache struct {
	mu        sync.RWMutex
	fetchedAt time.Time
	items     []Cask
	byToken   map[string]*Cask
}

var casksStore = &casksCache{}

func caskDisplayName(c Cask) string {
	if len(c.Name) > 0 && strings.TrimSpace(c.Name[0]) != "" {
		return strings.TrimSpace(c.Name[0])
	}
	return c.Token
}

func caskCategory(_ Cask) string {
	return "Desktop Apps"
}

func refreshCasks() error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(casksAPIURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return errFormulaeHTTP(resp.StatusCode, string(body))
	}
	var raw []Cask
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}
	byToken := make(map[string]*Cask, len(raw))
	items := make([]Cask, 0, len(raw))
	for i := range raw {
		if raw[i].Deprecated || raw[i].Disabled {
			continue
		}
		tok := strings.TrimSpace(raw[i].Token)
		if tok == "" {
			continue
		}
		raw[i].Category = caskCategory(raw[i])
		items = append(items, raw[i])
		byToken[strings.ToLower(tok)] = &items[len(items)-1]
	}
	casksStore.mu.Lock()
	casksStore.items = items
	casksStore.byToken = byToken
	casksStore.fetchedAt = time.Now()
	casksStore.mu.Unlock()
	return nil
}

func ensureCasks() error {
	casksStore.mu.RLock()
	fresh := time.Since(casksStore.fetchedAt) < casksCacheTTL && len(casksStore.items) > 0
	casksStore.mu.RUnlock()
	if fresh {
		return nil
	}
	return refreshCasks()
}

func listCasksCached() ([]Cask, error) {
	if err := ensureCasks(); err != nil {
		casksStore.mu.RLock()
		defer casksStore.mu.RUnlock()
		if len(casksStore.items) > 0 {
			return append([]Cask(nil), casksStore.items...), nil
		}
		return nil, err
	}
	casksStore.mu.RLock()
	defer casksStore.mu.RUnlock()
	return append([]Cask(nil), casksStore.items...), nil
}

func getCaskCached(token string) (*Cask, error) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return nil, nil
	}
	if err := ensureCasks(); err != nil {
		casksStore.mu.RLock()
		c := casksStore.byToken[token]
		casksStore.mu.RUnlock()
		if c != nil {
			cp := *c
			return &cp, nil
		}
		return nil, err
	}
	casksStore.mu.RLock()
	defer casksStore.mu.RUnlock()
	c := casksStore.byToken[token]
	if c == nil {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

func fetchCaskDetail(token string) (*CaskDetail, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://formulae.brew.sh/api/cask/" + url.PathEscape(token) + ".json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, errFormulaeHTTP(resp.StatusCode, string(body))
	}
	var detail CaskDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func CaskExists(token string) bool {
	c, _ := getCaskCached(token)
	return c != nil
}

// CatalogEntry is a unified formula/cask row for browse + search.
type CatalogEntry struct {
	Kind        string   // formula | cask
	Name        string
	DisplayName string
	FullName    string
	Tap         string
	Desc        string
	Homepage    string
	License     string
	Version     string
	Category    string
	Aliases     []string
}

func listCatalogCached(kindFilter string) ([]CatalogEntry, error) {
	kindFilter = strings.ToLower(strings.TrimSpace(kindFilter))
	out := make([]CatalogEntry, 0, 16384)
	var firstErr error

	if kindFilter == "" || kindFilter == "all" || kindFilter == "formula" {
		formulae, err := listFormulaeCached()
		if err != nil {
			firstErr = err
		} else {
			for _, f := range formulae {
				out = append(out, CatalogEntry{
					Kind:        "formula",
					Name:        f.Name,
					DisplayName: f.Name,
					FullName:    f.FullName,
					Tap:         f.Tap,
					Desc:        f.Desc,
					Homepage:    f.Homepage,
					License:     f.License,
					Version:     f.Versions.Stable,
					Category:    f.Category,
					Aliases:     f.Aliases,
				})
			}
		}
	}

	if kindFilter == "" || kindFilter == "all" || kindFilter == "cask" {
		casks, err := listCasksCached()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			for _, c := range casks {
				out = append(out, CatalogEntry{
					Kind:        "cask",
					Name:        c.Token,
					DisplayName: caskDisplayName(c),
					FullName:    firstNonEmpty(c.FullToken, c.Token),
					Tap:         firstNonEmpty(c.Tap, "homebrew/cask"),
					Desc:        c.Desc,
					Homepage:    c.Homepage,
					Version:     c.Version,
					Category:    c.Category,
				})
			}
		}
	}

	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func resolvePackageKind(name, preferred string) string {
	preferred = strings.ToLower(strings.TrimSpace(preferred))
	name = strings.ToLower(strings.TrimSpace(name))
	switch preferred {
	case "cask":
		if CaskExists(name) {
			return "cask"
		}
		return ""
	case "formula":
		if FormulaExists(name) {
			return "formula"
		}
		return ""
	}
	if FormulaExists(name) {
		return "formula"
	}
	if CaskExists(name) {
		return "cask"
	}
	return ""
}
