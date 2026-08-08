package softwarepkg_test

import (
	"context"
	"testing"
	"time"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func TestParseContainerWSTag(t *testing.T) {
	cases := []struct {
		tag, distro, ver string
		ok               bool
	}{
		{"ubuntu-26.04", "ubuntu", "26.04", true},
		{"debian-13", "debian", "13", true},
		{"fedora-44", "fedora", "44", true},
		{"kali-rolling", "kali", "rolling", true},
		{"latest", "", "", false},
		{"binoptimization", "", "", false},
		{"ubuntu-26.04-app", "", "", false},
	}
	for _, tc := range cases {
		d, v, ok := softwarepkg.ParseContainerWSTag(tc.tag)
		if ok != tc.ok || d != tc.distro || v != tc.ver {
			t.Fatalf("%s: got (%s,%s,%v) want (%s,%s,%v)", tc.tag, d, v, ok, tc.distro, tc.ver, tc.ok)
		}
	}
}

func TestListHubTagsLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tags, err := softwarepkg.ListHubTags(ctx, &softwarepkg.ListHubTagsOptions{
		Image: softwarepkg.DefaultHubImage,
	})
	if err != nil {
		t.Fatalf("ListHubTags: %v", err)
	}
	if len(tags) == 0 {
		t.Fatal("expected workspace tags from Docker Hub")
	}
	found := false
	for _, tag := range tags {
		if tag.Name == "ubuntu-26.04" {
			found = true
			if tag.DistroID != "ubuntu" || tag.DistroVersion != "26.04" {
				t.Fatalf("ubuntu-26.04 parse: %+v", tag)
			}
			if !tag.Workspace {
				t.Fatal("ubuntu-26.04 should be workspace")
			}
		}
		if tag.Name == "latest" {
			t.Fatal("latest should be filtered by default")
		}
	}
	if !found {
		t.Fatal("expected ubuntu-26.04 among hub tags")
	}
}

func TestScaffoldFromHub(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx

	res, err := softwarepkg.Scaffold(softwarepkg.ScaffoldRequest{
		Name:      "htop",
		Details:   "interactive process viewer",
		Category:  "Tools",
		Version:   "1.0.0",
		FromHub:   true,
		AlsoAny:   true,
		OutputDir: dir,
		Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) < 5 {
		t.Fatalf("expected many hub install files, got %d: %v", len(res.Files), res.Files)
	}
	hasExact := false
	hasAny := false
	hasDefault := false
	for _, f := range res.Files {
		if f == "softwares/htop/ubuntu/26.04/any/install.json" {
			hasExact = true
		}
		if f == "softwares/htop/ubuntu/any/any/install.json" {
			hasAny = true
		}
		if f == "softwares/htop/default/install.json" {
			hasDefault = true
		}
	}
	if !hasExact || !hasAny || !hasDefault {
		t.Fatalf("missing expected paths exact=%v any=%v default=%v files=%v", hasExact, hasAny, hasDefault, res.Files)
	}
}
