//go:build linux || darwin

package fsutil

import (
	"os"
	"syscall"
)

func fillOwner(e *Entry, info os.FileInfo) {
	if e == nil || info == nil {
		return
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return
	}
	e.UID = uint32(st.Uid)
	e.GID = uint32(st.Gid)
}
