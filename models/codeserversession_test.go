package models

import "testing"

func TestCodeserverWorkspaceName(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"frontend", "/workspace/app", "frontend"},
		{"", "/workspace/my-app", "my-app"},
		{"", "/workspace/my-app/", "my-app"},
		{"", "/", "workspace"},
		{"", "", "workspace"},
		{"  api  ", "/x", "api"},
	}
	for _, tc := range cases {
		got := CodeserverWorkspaceName(tc.name, tc.path)
		if got != tc.want {
			t.Fatalf("CodeserverWorkspaceName(%q,%q)=%q want %q", tc.name, tc.path, got, tc.want)
		}
	}
}
