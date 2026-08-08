package linuxuser

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateED25519RoundTrip(t *testing.T) {
	privPEM, pubLine, err := generateED25519("unit@test", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(privPEM), "OPENSSH PRIVATE KEY") {
		t.Fatalf("unexpected private pem: %s", string(privPEM[:min(40, len(privPEM))]))
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(pubLine)
	if err != nil {
		t.Fatal(err)
	}
	if pub.Type() != "ssh-ed25519" {
		t.Fatalf("type=%s", pub.Type())
	}
	if comment != "unit@test" {
		t.Fatalf("comment=%q", comment)
	}
	fp := ssh.FingerprintSHA256(pub)
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("fingerprint=%q", fp)
	}

	normalized, err := normalizeAuthorizedKeyLine(string(pubLine), "override")
	if err != nil {
		t.Fatal(err)
	}
	_, c2, _, _, err := ssh.ParseAuthorizedKey([]byte(normalized))
	if err != nil {
		t.Fatal(err)
	}
	if c2 != "override" {
		t.Fatalf("override comment=%q", c2)
	}
}

func TestNormalizeRejectsGarbage(t *testing.T) {
	if _, err := normalizeAuthorizedKeyLine("not-a-key", ""); err == nil {
		t.Fatal("expected error")
	}
}
