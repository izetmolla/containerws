package adduser

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()
	if got := shellQuote("plain"); got != "'plain'" {
		t.Fatalf("got %q", got)
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("got %q", got)
	}
}

func TestOrDefault(t *testing.T) {
	t.Parallel()
	if got := orDefault("  ", DefaultGeometry); got != DefaultGeometry {
		t.Fatalf("got %q", got)
	}
	if got := orDefault("1280x800", DefaultGeometry); got != "1280x800" {
		t.Fatalf("got %q", got)
	}
}

func TestStartOptionsValidation(t *testing.T) {
	t.Parallel()
	_, err := StartUserSession(StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("expected username error, got %v", err)
	}
	_, err = StartUserSession(StartOptions{Username: "nobody-cws-test-user"})
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected password error, got %v", err)
	}
}

func TestStopUserSessionRequiresUsername(t *testing.T) {
	t.Parallel()
	if err := StopUserSession("", 0, 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteUserVncPasswordValidation(t *testing.T) {
	t.Parallel()
	if err := WriteUserVncPassword("", "secret"); err == nil {
		t.Fatal("expected username error")
	}
	if err := WriteUserVncPassword("someone", ""); err == nil {
		t.Fatal("expected password error")
	}
	if err := WriteUserVncPassword("definitely-missing-cws-user-xyz", "secret"); err == nil {
		t.Fatal("expected missing user error")
	}
}
