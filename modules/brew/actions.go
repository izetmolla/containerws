package brew

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type actionJob struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Names     []string  `json:"names"`
	Kind      string    `json:"kind"` // formula | cask
	Status    string    `json:"status"` // pending|running|success|error
	Log       string    `json:"log"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

var (
	jobsMu sync.RWMutex
	jobs   = map[string]*actionJob{}
)

func getJob(id string) *actionJob {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	j := jobs[id]
	if j == nil {
		return nil
	}
	cp := *j
	return &cp
}

func listRecentJobs(limit int) []actionJob {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	out := make([]actionJob, 0, len(jobs))
	for _, j := range jobs {
		if j != nil {
			out = append(out, *j)
		}
	}
	// newest first (simple)
	for i := 0; i < len(out); i++ {
		for k := i + 1; k < len(out); k++ {
			if out[k].StartedAt.After(out[i].StartedAt) {
				out[i], out[k] = out[k], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func startActionJob(action string, names []string, kind string) (*actionJob, error) {
	job, err := createActionJob(action, names, kind)
	if err != nil {
		return nil, err
	}
	go runActionJob(job.ID)
	return getJob(job.ID), nil
}

// RunActionSync runs a brew install/upgrade/uninstall on the calling goroutine
// (used by the Softwares install queue so jobs never overlap).
func RunActionSync(action string, names []string, kind string) (*actionJob, error) {
	job, err := createActionJob(action, names, kind)
	if err != nil {
		return nil, err
	}
	runActionJob(job.ID)
	final := getJob(job.ID)
	if final == nil {
		return nil, fmt.Errorf("brew job disappeared")
	}
	if final.Status == "error" {
		msg := strings.TrimSpace(final.Error)
		if msg == "" {
			msg = "brew action failed"
		}
		return final, fmt.Errorf("%s", msg)
	}
	return final, nil
}

func createActionJob(action string, names []string, kind string) (*actionJob, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "install", "upgrade", "uninstall":
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}
	clean := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		clean = append(clean, n)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("at least one package name is required")
	}
	if ResolveBrewPath() == "" {
		return nil, fmt.Errorf("brew is not installed")
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = resolvePackageKind(clean[0], "")
	}
	if kind != "cask" {
		kind = "formula"
	}
	// Prefer explicit cask when the token is cask-only.
	if kind == "formula" && !FormulaExists(clean[0]) && CaskExists(clean[0]) {
		kind = "cask"
	}

	id := fmt.Sprintf("brew-%d", time.Now().UnixNano())
	job := &actionJob{
		ID:        id,
		Action:    action,
		Names:     clean,
		Kind:      kind,
		Status:    "pending",
		StartedAt: time.Now(),
	}
	jobsMu.Lock()
	jobs[id] = job
	jobsMu.Unlock()
	return getJob(id), nil
}

func runActionJob(id string) {
	jobsMu.Lock()
	job := jobs[id]
	if job == nil {
		jobsMu.Unlock()
		return
	}
	job.Status = "running"
	names := append([]string(nil), job.Names...)
	action := job.Action
	kind := job.Kind
	jobsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	brewPath := ResolveBrewPath()
	flag := "--formula"
	if kind == "cask" {
		flag = "--cask"
	}

	var args []string
	switch action {
	case "install":
		args = append([]string{"install", flag}, names...)
	case "upgrade":
		args = append([]string{"upgrade", flag}, names...)
	case "uninstall":
		args = append([]string{"uninstall", flag, "--force"}, names...)
		out, err := runBrewCombined(ctx, brewPath, args...)
		if err != nil {
			args = append([]string{"uninstall", flag}, names...)
			out2, err2 := runBrewCombined(ctx, brewPath, args...)
			out = out + "\n" + out2
			err = err2
		}
		jobsMu.Lock()
		if j := jobs[id]; j != nil {
			j.Log = out
			j.EndedAt = time.Now()
			if err != nil {
				j.Status = "error"
				j.Error = err.Error()
			} else {
				j.Status = "success"
			}
		}
		jobsMu.Unlock()
		return
	}

	out, err := runBrewCombined(ctx, brewPath, args...)
	jobsMu.Lock()
	if j := jobs[id]; j != nil {
		j.Log = out
		j.EndedAt = time.Now()
		if err != nil {
			j.Status = "error"
			j.Error = err.Error()
		} else {
			j.Status = "success"
		}
	}
	jobsMu.Unlock()
}

type brewInfoFormula struct {
	Name              string `json:"name"`
	InstalledVersions []struct {
		Version string `json:"version"`
	} `json:"installed"`
	Outdated bool `json:"outdated"`
}

type brewInfoCask struct {
	Token             string `json:"token"`
	InstalledVersions any    `json:"installed"` // string or null
	Outdated          bool   `json:"outdated"`
}

type brewInfoJSON struct {
	Formulae []brewInfoFormula `json:"formulae"`
	Casks    []brewInfoCask    `json:"casks"`
}

func formulaInstallDetails(ctx context.Context, name string) (installed bool, versions []string, outdated bool) {
	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return false, nil, false
	}
	out, err := runBrew(ctx, brewPath, "info", "--json=v2", "--formula", name)
	if err != nil {
		return false, nil, false
	}
	var payload brewInfoJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil || len(payload.Formulae) == 0 {
		return false, nil, false
	}
	f := payload.Formulae[0]
	if len(f.InstalledVersions) == 0 {
		return false, nil, false
	}
	versions = make([]string, 0, len(f.InstalledVersions))
	for _, v := range f.InstalledVersions {
		if strings.TrimSpace(v.Version) != "" {
			versions = append(versions, v.Version)
		}
	}
	return true, versions, f.Outdated
}

func caskInstallDetails(ctx context.Context, name string) (installed bool, version string, outdated bool) {
	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return false, "", false
	}
	out, err := runBrew(ctx, brewPath, "info", "--json=v2", "--cask", name)
	if err != nil {
		return false, "", false
	}
	var payload brewInfoJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil || len(payload.Casks) == 0 {
		return false, "", false
	}
	c := payload.Casks[0]
	ver := caskInstalledVersion(c.InstalledVersions)
	if ver == "" {
		return false, "", false
	}
	return true, ver, c.Outdated
}

func caskInstalledVersion(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		if len(t) == 0 {
			return ""
		}
		if s, ok := t[0].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func formulaInstallState(ctx context.Context, name string) (installed bool, version string, outdated bool) {
	ok, versions, outdated := formulaInstallDetails(ctx, name)
	if !ok || len(versions) == 0 {
		return false, "", false
	}
	return true, versions[0], outdated
}

func listInstalledFormulae(ctx context.Context) ([]map[string]any, error) {
	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return nil, fmt.Errorf("brew is not installed")
	}
	out, err := runBrew(ctx, brewPath, "info", "--json=v2", "--installed")
	if err != nil {
		listOut, listErr := runBrew(ctx, brewPath, "list", "--formula")
		if listErr != nil {
			return nil, err
		}
		names := strings.Fields(listOut)
		items := make([]map[string]any, 0, len(names))
		for _, n := range names {
			items = append(items, map[string]any{
				"name":      n,
				"kind":      "formula",
				"version":   "",
				"outdated":  false,
				"installed": true,
			})
		}
		return items, nil
	}
	var payload brewInfoJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(payload.Formulae)+len(payload.Casks))
	for _, f := range payload.Formulae {
		ver := ""
		if len(f.InstalledVersions) > 0 {
			ver = f.InstalledVersions[0].Version
		}
		items = append(items, map[string]any{
			"name":      f.Name,
			"kind":      "formula",
			"version":   ver,
			"outdated":  f.Outdated,
			"installed": true,
		})
	}
	for _, c := range payload.Casks {
		items = append(items, map[string]any{
			"name":      c.Token,
			"kind":      "cask",
			"version":   caskInstalledVersion(c.InstalledVersions),
			"outdated":  c.Outdated,
			"installed": true,
		})
	}
	return items, nil
}
