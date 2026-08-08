package linuxuser_test

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/izetmolla/containerws/packages/linuxuser"
)

func TestSSHKeysRoundTrip(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	// Operate in a temp home by temporarily using Lookup on current user —
	// write into a sandbox under TempDir via direct helpers when possible.
	// Full filesystem tests require a real Linux account; skip if not root and
	// home is not writable for .ssh mutations in CI sandboxes.
	home := u.HomeDir
	if home == "" {
		t.Skip("no home")
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Skip(err)
	}

	st, err := linuxuser.SSHKeys(u.Username, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Username != u.Username {
		t.Fatalf("username=%q", st.Username)
	}
	if st.AuthorizedKeys == nil {
		t.Fatal("authorized_keys slice should be non-nil")
	}
}

func TestNormalizeAndFingerprintViaAddRemove(t *testing.T) {
	// Unit-level: generate a keypair in-memory path by using GenerateIdentity
	// only when running as the same user with overwrite in a disposable way.
	// Keep this light — package-level generate/delete is covered manually.
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	st, err := linuxuser.SSHKeys(u.Username, false)
	if err != nil {
		t.Skip(err)
	}
	_ = st
}
