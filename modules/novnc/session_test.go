package novnc

import (
	"net/url"
	"strings"
	"testing"
)

func TestClientURLForSessionEmbedsSessionInPath(t *testing.T) {
	const id = "e61d9b22-a781-4343-9d0b-fc32d7b4c18e"
	raw := ClientURLForSession(id)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if got := q.Get("session_id"); got != id {
		t.Fatalf("session_id=%q, want %q", got, id)
	}
	path := q.Get("path")
	if path != "websockify?session_id="+id {
		t.Fatalf("path=%q, want websockify?session_id=…", path)
	}
	// Mimic noVNC: resolve path against the vnc.html URL.
	ws, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse(raw)
	resolved := base.ResolveReference(ws)
	if !strings.HasPrefix(resolved.Path, "/novnc/websockify") {
		t.Fatalf("resolved path=%q", resolved.Path)
	}
	if got := resolved.Query().Get("session_id"); got != id {
		t.Fatalf("resolved session_id=%q, want %q", got, id)
	}
}

func TestSessionIDFromReferer(t *testing.T) {
	const id = "e61d9b22-a781-4343-9d0b-fc32d7b4c18e"
	got := sessionIDFromReferer(
		"http://localhost:5173/novnc/vnc.html?session_id=" + id + "&path=websockify",
	)
	if got != id {
		t.Fatalf("got %q, want %q", got, id)
	}
	if sessionIDFromReferer("") != "" {
		t.Fatal("empty referer should yield empty id")
	}
}

func TestRfbPasswordTruncatesTo8(t *testing.T) {
	if got := rfbPassword("  secret12extra "); got != "secret12" {
		t.Fatalf("got %q", got)
	}
	if got := rfbPassword("short"); got != "short" {
		t.Fatalf("got %q", got)
	}
	if rfbPassword("   ") != "" {
		t.Fatal("blank should be empty")
	}
}

func TestIsMandatoryJSON(t *testing.T) {
	if !isMandatoryJSON("/mandatory.json") {
		t.Fatal("expected match")
	}
	if isMandatoryJSON("/vnc.html") {
		t.Fatal("vnc.html is not mandatory.json")
	}
}

func TestStripAccessTokenQuery(t *testing.T) {
	got := stripAccessTokenQuery("/websockify?session_id=abc&access_token=secret&shared=1")
	if got != "/websockify?session_id=abc&shared=1" {
		t.Fatalf("got %q", got)
	}
	if stripAccessTokenQuery("/vnc.html") != "/vnc.html" {
		t.Fatal("path-only URI should be unchanged")
	}
}

func TestWithAccessToken(t *testing.T) {
	const id = "e61d9b22-a781-4343-9d0b-fc32d7b4c18e"
	raw := withAccessToken(ClientURLForSession(id), "jwt.token.here")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("access_token") != "jwt.token.here" {
		t.Fatalf("missing page access_token: %q", raw)
	}
	path := u.Query().Get("path")
	ws, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Query().Get("access_token") != "jwt.token.here" {
		t.Fatalf("missing path access_token: %q", path)
	}
	if ws.Query().Get("session_id") != id {
		t.Fatalf("session_id lost: %q", path)
	}
}

func TestAccessTokenFromReferer(t *testing.T) {
	got := accessTokenFromReferer(
		"http://localhost:5173/novnc/vnc.html?session_id=x&access_token=abc.def",
	)
	if got != "abc.def" {
		t.Fatalf("got %q", got)
	}
	if accessTokenFromReferer("") != "" {
		t.Fatal("empty referer")
	}
}

