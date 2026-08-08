package codeserver

import "testing"

func TestNormalizeGitRepo(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"owner/repo", "https://github.com/owner/repo.git"},
		{"github.com/owner/repo", "https://github.com/owner/repo.git"},
		{"https://github.com/owner/repo", "https://github.com/owner/repo.git"},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo.git"},
	}
	for _, tc := range cases {
		got, err := NormalizeGitRepo(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestInjectGitToken(t *testing.T) {
	got := InjectGitToken("https://github.com/a/b.git", "sekret")
	if got != "https://x-access-token:sekret@github.com/a/b.git" {
		t.Fatalf("got %q", got)
	}
	if RedactGitURL(got) != "https://github.com/a/b.git" {
		t.Fatalf("redact failed: %q", RedactGitURL(got))
	}
}
