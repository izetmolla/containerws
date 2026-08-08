//go:build windows

package cloudshell

import (
	"os"
	"os/exec"
)

func applyUnixCredentials(_ *exec.Cmd, _ string) {}

func signalHangup(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	return proc.Kill()
}
