//go:build windows

package codeserver

import (
	"os"
	"syscall"
)

func detachSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// DetachSysProcAttr returns SysProcAttr that starts a new session (no-op on Windows).
func DetachSysProcAttr() *syscall.SysProcAttr {
	return detachSysProcAttr()
}

func killProcessGroup(pid int, force bool) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if force {
		return proc.Kill()
	}
	return proc.Signal(syscall.SIGTERM)
}
