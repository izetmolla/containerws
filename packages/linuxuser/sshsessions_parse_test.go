package linuxuser

import "testing"

func TestParseSSHDSessionArgs(t *testing.T) {
	cases := []struct {
		args string
		user string
		tty  string
		ok   bool
	}{
		{"sshd-session: root@pts/2", "root", "pts/2", true},
		{"sshd-session: root@notty", "root", "", true},
		{"sshd: alice@pts/0", "alice", "pts/0", true},
		{"sshd-session: root [priv]", "", "", false},
		{"sshd: /usr/sbin/sshd [listener] 0 of 10-100 startups", "", "", false},
		{"bash", "", "", false},
	}
	for _, tc := range cases {
		user, tty, ok := parseSSHDSessionArgs(tc.args)
		if ok != tc.ok || user != tc.user || tty != tc.tty {
			t.Fatalf("%q => (%q,%q,%v), want (%q,%q,%v)",
				tc.args, user, tty, ok, tc.user, tc.tty, tc.ok)
		}
	}
}
