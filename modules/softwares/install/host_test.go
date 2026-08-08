package install

import (
	"testing"

	"github.com/izetmolla/containerws/models"
)

func TestMatchingVersionFiltersByHost(t *testing.T) {
	host := models.HostIdentity{
		OS:            "linux",
		DistroID:      "ubuntu",
		DistroVersion: "26.04",
		Arch:          "amd64",
		PackageFamily: "apt",
	}
	fedora := models.SoftwareVersion{
		ID: "fedora", Version: "1", DistroID: "fedora", DistroVersion: "44",
		Arch: "amd64", PackageFamily: "dnf", OS: "linux",
	}
	ubuntu := models.SoftwareVersion{
		ID: "ubuntu", Version: "2", DistroID: "ubuntu", DistroVersion: "26.04",
		Arch: "any", PackageFamily: "apt", OS: "linux", IsLatest: true,
	}
	wildcard := models.SoftwareVersion{
		ID: "any", Version: "3", OS: "linux", PackageFamily: "apt",
	}

	if got := MatchingVersion([]models.SoftwareVersion{fedora}, host, true); got != nil {
		t.Fatalf("expected no match for fedora-only version, got %s", got.ID)
	}
	if got := MatchingVersion([]models.SoftwareVersion{fedora, ubuntu}, host, true); got == nil || got.ID != "ubuntu" {
		t.Fatalf("expected ubuntu match, got %#v", got)
	}
	if got := MatchingVersion([]models.SoftwareVersion{wildcard}, host, true); got == nil || got.ID != "any" {
		t.Fatalf("expected wildcard match, got %#v", got)
	}
}
