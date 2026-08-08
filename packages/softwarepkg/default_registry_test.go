package softwarepkg_test

import (
	"testing"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func TestSameGitHubRepo(t *testing.T) {
	base := softwarepkg.DefaultRegistryURL
	cases := []struct {
		a, b string
		want bool
	}{
		{base, base, true},
		{base, base + ".git", true},
		{base, "https://github.com/izetmolla/containerwspkg/tree/main", true},
		{base, "https://raw.githubusercontent.com/izetmolla/containerwspkg/main", true},
		{base, "https://github.com/other/repo", false},
	}
	for _, tc := range cases {
		if got := softwarepkg.SameGitHubRepo(tc.a, tc.b); got != tc.want {
			t.Fatalf("SameGitHubRepo(%q,%q)=%v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
