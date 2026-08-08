package novnc

import (
	"errors"
	"testing"

	"github.com/izetmolla/containerws/models"
)

func TestGateProfileRequiresActiveSession(t *testing.T) {
	_, err := gateProfile(nil)
	if !errors.Is(err, errNoProfile) {
		t.Fatalf("nil session: got %v", err)
	}

	_, err = gateProfile(&models.VncSession{Status: models.VncSessionStatusInactive})
	if !errors.Is(err, errNoProfile) {
		t.Fatalf("inactive: got %v", err)
	}

	session := &models.VncSession{
		Status: models.VncSessionStatusActive,
		User:   models.User{},
	}
	_, err = gateProfile(session)
	if !errors.Is(err, errNoProfile) {
		t.Fatalf("missing username: got %v", err)
	}

	var gate *sessionGateError
	if !errors.As(err, &gate) || gate.title == "" {
		t.Fatal("expected sessionGateError with title")
	}
}

func TestGateProfileRequiresPassword(t *testing.T) {
	// root almost always exists on the build host; use it to pass Lookup.
	session := &models.VncSession{
		Status:      models.VncSessionStatusActive,
		VncPassword: "",
		User:        models.User{Username: "root"},
	}
	_, err := gateProfile(session)
	if !errors.Is(err, errNoProfile) {
		t.Fatalf("missing password: got %v", err)
	}
}

func TestGateProfileOK(t *testing.T) {
	session := &models.VncSession{
		Status:      models.VncSessionStatusActive,
		VncPassword: "secret",
		User:        models.User{Username: "root"},
	}
	username, err := gateProfile(session)
	if err != nil {
		t.Fatal(err)
	}
	if username != "root" {
		t.Fatalf("username=%q", username)
	}
}
