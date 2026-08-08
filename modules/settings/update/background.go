package update

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/izetmolla/containerws/version"
)

var (
	bgOnce sync.Once
)

// StartBackgroundChecks refreshes the GitHub releases cache every 12 hours.
func StartBackgroundChecks() {
	bgOnce.Do(func() {
		go func() {
			// Initial delayed check so boot is not blocked by GitHub.
			time.Sleep(45 * time.Second)
			refreshCache(context.Background())
			t := time.NewTicker(12 * time.Hour)
			defer t.Stop()
			for range t.C {
				refreshCache(context.Background())
			}
		}()
	})
}

func refreshCache(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	releases, latest, err := fetchReleases(ctx, version.Version)
	if err != nil {
		log.Printf("settings/update: background check failed: %v", err)
		globalCache.set(nil, "", err.Error())
		return
	}
	globalCache.set(releases, latest, "")
	log.Printf("settings/update: checked %s — %d release(s), latest=%s", updateRepo(), len(releases), latest)
}
