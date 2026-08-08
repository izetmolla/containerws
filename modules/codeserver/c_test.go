package codeserver

import "testing"

func TestStripSessionURI(t *testing.T) {
	id := "abc-123"
	tests := []struct {
		in   string
		want string
	}{
		{"/codeserver/abc-123", "/"},
		{"/codeserver/abc-123/", "/"},
		{"/codeserver/abc-123/?", "/?"},
		{"/codeserver/abc-123/?tkn=1", "/?tkn=1"},
		{"/codeserver/abc-123/stable-x/static/out/vs.js", "/stable-x/static/out/vs.js"},
		{"/codeserver/abc-123/stable-x?reconnectionToken=z", "/stable-x?reconnectionToken=z"},
	}
	for _, tc := range tests {
		if got := stripSessionURI(tc.in, id); got != tc.want {
			t.Fatalf("stripSessionURI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestServerBasePath(t *testing.T) {
	if got := ServerBasePath("abc"); got != "/codeserver/abc" {
		t.Fatalf("got %q", got)
	}
	if got := ClientURL("abc"); got != "/codeserver/abc/" {
		t.Fatalf("got %q", got)
	}
	if got := ClientURLForFolder("abc", "/home/moe/proj"); got != "/codeserver/abc/?folder=/home/moe/proj" {
		t.Fatalf("got %q", got)
	}
	if got := FolderQueryValue("/workspace/testapp"); got != "/workspace/testapp" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeProto(t *testing.T) {
	if got := normalizeProto("HTTPS"); got != "https" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeProto("bogus"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstCSV(t *testing.T) {
	if got := firstCSV(" https, http "); got != "https" {
		t.Fatalf("got %q", got)
	}
}

func TestIsHTTP101(t *testing.T) {
	if !isHTTP101([]byte("HTTP/1.1 101 Switching Protocols\r\n")) {
		t.Fatal("expected 101")
	}
	if isHTTP101([]byte("HTTP/1.1 400 Bad Request\r\n")) {
		t.Fatal("expected false")
	}
}

