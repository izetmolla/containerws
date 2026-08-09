package brew

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	formulaeAPIURL   = "https://formulae.brew.sh/api/formula.json"
	formulaeCacheTTL = 6 * time.Hour
)

type Formula struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Tap      string `json:"tap"`
	Desc     string `json:"desc"`
	Homepage string `json:"homepage"`
	License  string `json:"license"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Deprecated bool     `json:"deprecated"`
	Disabled   bool     `json:"disabled"`
	Aliases    []string `json:"aliases"`
	Category   string   `json:"category,omitempty"` // derived
}

// FormulaIconURL returns a favicon URL for the formula homepage (empty if none).
func FormulaIconURL(homepage string) string {
	host := homepageHost(homepage)
	if host == "" {
		return ""
	}
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(host) + "&sz=128"
}

// FormulaLogoURL returns a larger site icon suitable for detail heroes.
func FormulaLogoURL(homepage string) string {
	host := homepageHost(homepage)
	if host == "" {
		return ""
	}
	return "https://icons.duckduckgo.com/ip3/" + host + ".ico"
}

func homepageHost(homepage string) string {
	homepage = strings.TrimSpace(homepage)
	if homepage == "" {
		return ""
	}
	u, err := url.Parse(homepage)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}

// FormulaDetail is the rich single-formula payload from formulae.brew.sh.
type FormulaDetail struct {
	Name                string   `json:"name"`
	FullName            string   `json:"full_name"`
	Tap                 string   `json:"tap"`
	Desc                string   `json:"desc"`
	Homepage            string   `json:"homepage"`
	License             string   `json:"license"`
	Aliases             []string `json:"aliases"`
	Oldnames            []string `json:"oldnames"`
	VersionedFormulae   []string `json:"versioned_formulae"`
	Dependencies        []string `json:"dependencies"`
	BuildDependencies   []string `json:"build_dependencies"`
	Executables         []string `json:"executables"`
	Revision            int      `json:"revision"`
	Deprecated          bool     `json:"deprecated"`
	Disabled            bool     `json:"disabled"`
	Versions            struct {
		Stable string  `json:"stable"`
		Head   *string `json:"head"`
		Bottle bool    `json:"bottle"`
	} `json:"versions"`
	URLs struct {
		Stable struct {
			URL string `json:"url"`
		} `json:"stable"`
	} `json:"urls"`
	Analytics struct {
		Install struct {
			D30  map[string]int `json:"30d"`
			D90  map[string]int `json:"90d"`
			D365 map[string]int `json:"365d"`
		} `json:"install"`
	} `json:"analytics"`
}

func fetchFormulaDetail(name string) (*FormulaDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://formulae.brew.sh/api/formula/" + url.PathEscape(name) + ".json")
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
	var detail FormulaDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func analyticsCount(m map[string]int, name string) int {
	if len(m) == 0 {
		return 0
	}
	if v, ok := m[name]; ok {
		return v
	}
	for _, v := range m {
		return v
	}
	return 0
}

type formulaeCache struct {
	mu        sync.RWMutex
	fetchedAt time.Time
	items     []Formula
	byName    map[string]*Formula
}

var formulaeStore = &formulaeCache{}

func categoryFor(f Formula) string {
	name := strings.ToLower(f.Name + " " + f.Desc)
	switch {
	case strings.Contains(name, "database") || strings.Contains(name, "sql") || strings.Contains(name, "redis") || strings.Contains(name, "mongo"):
		return "Databases"
	case strings.Contains(name, "language") || strings.Contains(name, "compiler") || strings.Contains(name, "python") || strings.Contains(name, "node") || strings.Contains(name, "golang") || strings.Contains(name, "rust"):
		return "Languages"
	case strings.Contains(name, "editor") || strings.Contains(name, "vim") || strings.Contains(name, "emacs") || strings.Contains(name, "git"):
		return "Developer Tools"
	case strings.Contains(name, "http") || strings.Contains(name, "proxy") || strings.Contains(name, "dns") || strings.Contains(name, "network") || strings.Contains(name, "ssh"):
		return "Networking"
	case strings.Contains(name, "image") || strings.Contains(name, "video") || strings.Contains(name, "audio") || strings.Contains(name, "ffmpeg") || strings.Contains(name, "media"):
		return "Media"
	case strings.Contains(name, "security") || strings.Contains(name, "crypt") || strings.Contains(name, "ssl") || strings.Contains(name, "gpg"):
		return "Security"
	case strings.Contains(name, "monitor") || strings.Contains(name, "metric") || strings.Contains(name, "log") || strings.Contains(name, "htop") || strings.Contains(name, "top"):
		return "Monitoring"
	default:
		return "Utilities"
	}
}

func refreshFormulae() error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(formulaeAPIURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return errFormulaeHTTP(resp.StatusCode, string(body))
	}
	var raw []Formula
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}
	byName := make(map[string]*Formula, len(raw))
	items := make([]Formula, 0, len(raw))
	for i := range raw {
		if raw[i].Deprecated || raw[i].Disabled {
			continue
		}
		raw[i].Category = categoryFor(raw[i])
		items = append(items, raw[i])
		f := items[len(items)-1]
		byName[strings.ToLower(f.Name)] = &items[len(items)-1]
	}
	formulaeStore.mu.Lock()
	formulaeStore.items = items
	formulaeStore.byName = byName
	formulaeStore.fetchedAt = time.Now()
	formulaeStore.mu.Unlock()
	return nil
}

type formulaeHTTPError struct {
	code int
	body string
}

func (e formulaeHTTPError) Error() string {
	return "formulae api: " + http.StatusText(e.code)
}

func errFormulaeHTTP(code int, body string) error {
	return formulaeHTTPError{code: code, body: body}
}

func ensureFormulae() error {
	formulaeStore.mu.RLock()
	fresh := time.Since(formulaeStore.fetchedAt) < formulaeCacheTTL && len(formulaeStore.items) > 0
	formulaeStore.mu.RUnlock()
	if fresh {
		return nil
	}
	return refreshFormulae()
}

func listFormulaeCached() ([]Formula, error) {
	if err := ensureFormulae(); err != nil {
		formulaeStore.mu.RLock()
		defer formulaeStore.mu.RUnlock()
		if len(formulaeStore.items) > 0 {
			return append([]Formula(nil), formulaeStore.items...), nil
		}
		return nil, err
	}
	formulaeStore.mu.RLock()
	defer formulaeStore.mu.RUnlock()
	return append([]Formula(nil), formulaeStore.items...), nil
}

func getFormulaCached(name string) (*Formula, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil, nil
	}
	if err := ensureFormulae(); err != nil {
		formulaeStore.mu.RLock()
		f := formulaeStore.byName[name]
		formulaeStore.mu.RUnlock()
		if f != nil {
			cp := *f
			return &cp, nil
		}
		return nil, err
	}
	formulaeStore.mu.RLock()
	defer formulaeStore.mu.RUnlock()
	f := formulaeStore.byName[name]
	if f == nil {
		return nil, nil
	}
	cp := *f
	return &cp, nil
}

func FormulaExists(name string) bool {
	f, _ := getFormulaCached(name)
	return f != nil
}

type analyticsPayload struct {
	Items []struct {
		Number  int    `json:"number"`
		Formula string `json:"formula"`
		Count   string `json:"count"`
	} `json:"items"`
}

func fetchAnalytics(days int) ([]string, error) {
	if days != 30 && days != 90 && days != 365 {
		days = 30
	}
	url := "https://formulae.brew.sh/api/analytics/install/" + strconv.Itoa(days) + "d.json"
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errFormulaeHTTP(resp.StatusCode, "")
	}
	var payload analyticsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, 100)
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Formula)
		if name == "" {
			continue
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		out = append(out, name)
		if len(out) >= 100 {
			break
		}
	}
	return out, nil
}
