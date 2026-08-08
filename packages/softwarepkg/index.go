package softwarepkg

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/models"
)

const catalogIndexPath = "softwares/index.json"

// CatalogIndex is softwares/index.json — lists packages available in the registry.
type CatalogIndex struct {
	Softwares []PackageMeta `json:"softwares"`
}

var (
	catalogCacheMu sync.Mutex
	catalogCache   = map[string]catalogCacheEntry{}
)

type catalogCacheEntry struct {
	items   []PackageMeta
	expires time.Time
}

const catalogCacheTTL = 0 // 0 = keep until InvalidateCatalogCache (no remote refetch)

// FetchCatalog loads softwares/index.json from the raw registry base.
// Successful responses are kept in memory until InvalidateCatalogCache so the
// softwares page does not re-hit GitHub on every search / source change.
func (c *Client) FetchCatalog(ctx context.Context, rawBase string, auth Auth) ([]PackageMeta, error) {
	key := strings.TrimRight(rawBase, "/")
	catalogCacheMu.Lock()
	if ent, ok := catalogCache[key]; ok && !catalogCacheExpired(ent) {
		items := append([]PackageMeta(nil), ent.items...)
		catalogCacheMu.Unlock()
		return items, nil
	}
	catalogCacheMu.Unlock()

	url := JoinRawURL(rawBase, catalogIndexPath)
	var idx CatalogIndex
	if err := c.FetchJSON(ctx, url, auth, &idx); err != nil {
		return nil, fmt.Errorf("index.json: %w", err)
	}
	items := make([]PackageMeta, 0, len(idx.Softwares))
	for _, m := range idx.Softwares {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		m.Name = name
		items = append(items, m)
	}

	catalogCacheMu.Lock()
	ent := catalogCacheEntry{items: items}
	if catalogCacheTTL > 0 {
		ent.expires = time.Now().Add(catalogCacheTTL)
	}
	catalogCache[key] = ent
	catalogCacheMu.Unlock()
	return items, nil
}

func catalogCacheExpired(ent catalogCacheEntry) bool {
	if catalogCacheTTL <= 0 {
		return false
	}
	return time.Now().After(ent.expires)
}

// InvalidateCatalogCache drops cached index entries (optional after registry change).
func InvalidateCatalogCache() {
	catalogCacheMu.Lock()
	catalogCache = map[string]catalogCacheEntry{}
	catalogCacheMu.Unlock()
	probeCacheMu.Lock()
	probeCache = map[string]probeCacheEntry{}
	probeCacheMu.Unlock()
}

// ListRemoteFromPackage loads the catalog for a SoftwarePackage registry row.
func ListRemoteFromPackage(ctx context.Context, pkg models.SoftwarePackage, ref string, client *Client) ([]PackageMeta, error) {
	if strings.TrimSpace(pkg.PackageURL) == "" {
		return nil, fmt.Errorf("package_url is empty")
	}
	if client == nil {
		client = &Client{}
	}
	rawBase, err := RawBaseURL(pkg.PackageURL, ref)
	if err != nil {
		return nil, err
	}
	items, err := client.FetchCatalog(ctx, rawBase, AuthFromPackage(pkg))
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Image = AbsoluteImageURL(rawBase, items[i].Image)
	}
	return items, nil
}

// MatchQuery reports whether meta matches a free-text catalog query.
func MatchQuery(m PackageMeta, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		m.Name, m.Details, m.Category, m.SubCategory, strings.Join(m.Tags, " "),
	}, " "))
	return strings.Contains(hay, q)
}
