//go:build !windows

package codeserver

import "syscall"

func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// DetachSysProcAttr returns SysProcAttr that starts a new session (Unix).
func DetachSysProcAttr() *syscall.SysProcAttr {
	return detachSysProcAttr()
}

func killProcessGroup(pid int, force bool) error {
	if force {
		return syscall.Kill(-pid, syscall.SIGKILL)
	}
	return syscall.Kill(-pid, syscall.SIGTERM)
}
