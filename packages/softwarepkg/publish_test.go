package softwarepkg_test

import (
	"strings"
	"testing"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func TestParseGitHubRepo(t *testing.T) {
	cases := []struct {
		in, owner, repo string
		ok              bool
	}{
		{"https://github.com/acme/cws-packages", "acme", "cws-packages", true},
		{"https://github.com/acme/cws-packages.git", "acme", "cws-packages", true},
		{"https://github.com/acme/cws-packages/tree/main", "acme", "cws-packages", true},
		{"https://raw.githubusercontent.com/acme/cws-packages/main", "acme", "cws-packages", true},
		{"https://example.com/foo", "", "", false},
	}
	for _, tc := range cases {
		owner, repo, err := softwarepkg.ParseGitHubRepo(tc.in)
		if tc.ok {
			if err != nil || owner != tc.owner || repo != tc.repo {
				t.Fatalf("%s: got (%s,%s,%v) want (%s,%s,nil)", tc.in, owner, repo, err, tc.owner, tc.repo)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: expected error", tc.in)
		}
	}
}

func TestGitCloneURLAuth(t *testing.T) {
	clone, public, err := softwarepkg.GitCloneURL(
		"https://github.com/acme/cws-packages",
		softwarepkg.Auth{Token: "secret-token"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if public != "https://github.com/acme/cws-packages" {
		t.Fatalf("public=%s", public)
	}
	if !strings.Contains(clone, "x-access-token:secret-token@github.com") {
		t.Fatalf("clone missing token auth: %s", clone)
	}
	if !strings.HasSuffix(clone, "acme/cws-packages.git") {
		t.Fatalf("clone=%s", clone)
	}
}

func TestGitCloneURLSSHWhenNoAuth(t *testing.T) {
	clone, public, err := softwarepkg.GitCloneURL(
		"https://github.com/izetmolla/containerwspkg",
		softwarepkg.Auth{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if public != "https://github.com/izetmolla/containerwspkg" {
		t.Fatalf("public=%s", public)
	}
	if clone != "git@github.com:izetmolla/containerwspkg.git" {
		t.Fatalf("clone=%s", clone)
	}
}
