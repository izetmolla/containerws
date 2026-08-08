package linuxuser_test

import (
	"testing"

	"github.com/izetmolla/containerws/packages/linuxuser"
)

func TestListSSHConnectionsEmptyUser(t *testing.T) {
	if _, err := linuxuser.ListSSHConnections(""); err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestKillSSHConnectionValidation(t *testing.T) {
	if err := linuxuser.KillSSHConnection("root", ""); err == nil {
		t.Fatal("expected error for empty id")
	}
}
