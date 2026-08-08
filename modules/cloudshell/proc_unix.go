//go:build !windows

package cloudshell

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func applyUnixCredentials(cmd *exec.Cmd, username string) {
	u, err := user.Lookup(username)
	if err != nil {
		return
	}
	uid, err1 := strconv.ParseUint(u.Uid, 10, 32)
	gid, err2 := strconv.ParseUint(u.Gid, 10, 32)
	if err1 != nil || err2 != nil {
		return
	}
	if os.Geteuid() != 0 {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
	}
}

func signalHangup(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	return proc.Signal(syscall.SIGHUP)
}
