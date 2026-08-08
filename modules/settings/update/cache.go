package update

import (
	"sync"
	"time"
)

// Release is a GitHub release suitable for the Update UI.
type Release struct {
	Tag         string    `json:"tag"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body,omitempty"`
	Newer       bool      `json:"newer"`
	AssetName   string    `json:"asset_name,omitempty"`
	AssetURL    string    `json:"asset_url,omitempty"`
	AssetSize   int64     `json:"asset_size,omitempty"`
	HasAsset    bool      `json:"has_asset"`
}

type cacheState struct {
	mu        sync.RWMutex
	releases  []Release
	lastCheck time.Time
	latestTag string
	err       string
}

var globalCache = &cacheState{}

func (c *cacheState) snapshot() (releases []Release, lastCheck time.Time, latest string, errMsg string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.releases != nil {
		releases = append([]Release(nil), c.releases...)
	}
	return releases, c.lastCheck, c.latestTag, c.err
}

func (c *cacheState) set(releases []Release, latest string, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases = append([]Release(nil), releases...)
	c.latestTag = latest
	c.err = errMsg
	c.lastCheck = time.Now().UTC()
}
