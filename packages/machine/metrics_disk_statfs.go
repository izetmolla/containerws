//go:build linux || darwin

package machine

import (
	"os"

	"golang.org/x/sys/unix"
)

func readDiskMetrics() []DiskMetrics {
	candidates := []string{"/", "/workspace", "/home", "/var", "/tmp"}
	seen := map[string]struct{}{}
	out := make([]DiskMetrics, 0, len(candidates))
	for _, mount := range candidates {
		st, err := os.Stat(mount)
		if err != nil || !st.IsDir() {
			continue
		}
		var fs unix.Statfs_t
		if err := unix.Statfs(mount, &fs); err != nil {
			continue
		}
		total := int64(fs.Blocks) * int64(fs.Bsize)
		free := int64(fs.Bavail) * int64(fs.Bsize)
		if total <= 0 {
			continue
		}
		used := total - free
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}
		pct := round2(float64(used) / float64(total) * 100)
		device, fstype := lookupMountInfo(mount)
		out = append(out, DiskMetrics{
			Mount:       mount,
			Device:      device,
			FSType:      fstype,
			TotalBytes:  total,
			UsedBytes:   used,
			FreeBytes:   free,
			UsedPercent: pct,
			TotalHuman:  humanBytes(total),
			UsedHuman:   humanBytes(used),
			FreeHuman:   humanBytes(free),
		})
	}
	return out
}
