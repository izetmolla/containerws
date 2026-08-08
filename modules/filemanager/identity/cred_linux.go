//go:build linux

package identity

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// withCredentials temporarily switches the calling thread's filesystem uid/gid
// so os.* permission checks follow the Linux user's DAC rights.
// setfsuid/setfsgid are thread-local; LockOSThread keeps the switch on this
// goroutine for the duration of fn.
func withCredentials(uid, gid uint32, fn func() error) error {
	if fn == nil {
		return nil
	}
	// Non-root process cannot switch; rely on OS permissions of the process.
	if os.Geteuid() != 0 {
		return fn()
	}
	// Already acting as this user (or root Linux identity).
	if uid == 0 {
		return fn()
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := unix.Setfsgid(int(gid)); err != nil {
		return fmt.Errorf("setfsgid: %w", err)
	}
	defer func() { _ = unix.Setfsgid(0) }()

	if err := unix.Setfsuid(int(uid)); err != nil {
		return fmt.Errorf("setfsuid: %w", err)
	}
	defer func() { _ = unix.Setfsuid(0) }()

	return fn()
}
