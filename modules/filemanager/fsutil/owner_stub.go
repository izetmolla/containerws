//go:build !linux && !darwin

package fsutil

import "os"

func fillOwner(e *Entry, info os.FileInfo) {
	_ = e
	_ = info
}
