package proxymanager

import (
	"path/filepath"

	"github.com/izetmolla/containerws/models"
	fiberproxy "github.com/izetmolla/containerws/packages/proxymanager/fiber"
)

// GenerateFiber writes routes.json and returns the live table.
func GenerateFiber(snap *Snapshot, appBaseURL string) (files []string, table *fiberproxy.Table, err error) {
	if snap == nil || snap.Settings == nil {
		return nil, nil, nil
	}
	dir, err := ConfigDirFor(snap.Settings, models.ProxyEngineFiber)
	if err != nil {
		return nil, nil, err
	}
	table = fiberproxy.BuildTable(fiberproxy.BuildInput{
		ActiveEngine: snap.Settings.ActiveEngine,
		Hosts:        snap.Hosts,
		Redirects:    snap.Redirects,
		AppBaseURL:   appBaseURL,
	})
	data, err := fiberproxy.SnapshotJSON(table)
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "routes.json")
	if err := WriteFileAtomic(path, data, 0o644); err != nil {
		return nil, nil, err
	}
	return []string{path}, table, nil
}
