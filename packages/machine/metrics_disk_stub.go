//go:build !(linux || darwin)

package machine

func readDiskMetrics() []DiskMetrics {
	return nil
}
