package brew

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// OwnershipForToken reports Softwares package_manager ownership for a brew token
// (local | brew | empty when unmatched).
func OwnershipForToken(db *gorm.DB, token string) string {
	return ownershipForToken(db, token)
}

// ListCatalog returns cached Homebrew catalogue entries (formula and/or cask).
// kindFilter: all | formula | cask.
func ListCatalog(kindFilter string) ([]CatalogEntry, error) {
	return listCatalogCached(kindFilter)
}

// ListInstalled returns brew CLI installed formulae/casks (name, kind, version, outdated).
func ListInstalled(ctx context.Context) ([]map[string]any, error) {
	return listInstalledFormulae(ctx)
}

// EnqueueSoftwaresActions queues brew install/upgrade/uninstall onto the Softwares
// install queue when wired; otherwise returns an error.
func EnqueueSoftwaresActions(db *gorm.DB, action string, names []string, kind string) (queued int, snapshot any, err error) {
	if softwaresEnqueue == nil {
		return 0, nil, ErrQueueNotWired
	}
	return softwaresEnqueue(db, action, names, kind)
}

// UpdateIndex runs `brew update` (refresh formulae.homebrew.org / taps).
func UpdateIndex(ctx context.Context) error {
	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return errors.New("brew is not installed")
	}
	_, err := runBrewCombined(ctx, brewPath, "update")
	return err
}

// ErrQueueNotWired means Softwares queue bridge was not registered at boot.
var ErrQueueNotWired = errQueueNotWired{}

type errQueueNotWired struct{}

func (errQueueNotWired) Error() string {
	return "softwares install queue is not wired for brew"
}
