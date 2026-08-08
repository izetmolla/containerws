package softwarepkg_test

import (
	"strings"
	"testing"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func TestGenerateLogoSVG(t *testing.T) {
	svg := softwarepkg.GenerateLogoSVG("nginx", "#009639")
	if !strings.Contains(svg, "NG") && !strings.Contains(svg, ">N") {
		// initials from "nginx" → NG (single word takes 2 chars)
		if !strings.Contains(svg, "NG") {
			t.Fatalf("expected initials in svg, got %s", svg)
		}
	}
	if !strings.Contains(svg, "#009639") {
		t.Fatalf("expected color in svg")
	}
	uri := softwarepkg.SVGDataURI(svg)
	if !strings.HasPrefix(uri, "data:image/svg+xml") {
		t.Fatalf("bad data uri: %s", uri[:min(40, len(uri))])
	}
}

func TestAbsoluteImageURL(t *testing.T) {
	base := "https://raw.githubusercontent.com/o/r/main"
	if got := softwarepkg.AbsoluteImageURL(base, "softwares/nginx/image.svg"); got != base+"/softwares/nginx/image.svg" {
		t.Fatalf("got %q", got)
	}
	if got := softwarepkg.AbsoluteImageURL(base, "https://cdn.example/logo.png"); got != "https://cdn.example/logo.png" {
		t.Fatalf("got %q", got)
	}
	if got := softwarepkg.AbsoluteImageURL(base, "data:image/svg+xml,abc"); got != "data:image/svg+xml,abc" {
		t.Fatalf("got %q", got)
	}
}

func TestPackageInitialsViaGenerate(t *testing.T) {
	svg := softwarepkg.GenerateLogoSVG("Google Chrome", "#4285F4")
	if !strings.Contains(svg, "GC") {
		t.Fatalf("expected GC initials, got %s", svg)
	}
}
