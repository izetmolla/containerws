package softwarepkg

import "testing"

func TestResolveInstallPaths(t *testing.T) {
	paths := ResolveInstallPaths("Nginx", HostFacts{
		DistroID:      "Ubuntu",
		DistroVersion: "26.10",
		Arch:          "aarch64",
	})
	wantExact := "softwares/nginx/ubuntu/26.10/arm64/install.json"
	if len(paths) != 5 {
		t.Fatalf("got %d paths: %#v", len(paths), paths)
	}
	if paths[0] != wantExact {
		t.Fatalf("exact path = %q want %q", paths[0], wantExact)
	}
	if paths[len(paths)-1] != "softwares/nginx/default/install.json" {
		t.Fatalf("default path = %q", paths[len(paths)-1])
	}
}

func TestResolveInstallPathsNormalizedArch(t *testing.T) {
	paths := ResolveInstallPaths("nginx", HostFacts{
		DistroID:      "ubuntu",
		DistroVersion: "26.10",
		Arch:          "arm64",
	})
	if paths[0] != "softwares/nginx/ubuntu/26.10/arm64/install.json" {
		t.Fatalf("got %q", paths[0])
	}
}

func TestRawBaseURLGitHub(t *testing.T) {
	base, err := RawBaseURL("https://github.com/acme/cws-packages", "main")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/acme/cws-packages/main"
	if base != want {
		t.Fatalf("got %q want %q", base, want)
	}
}

func TestRawBaseURLGitHubTree(t *testing.T) {
	base, err := RawBaseURL("https://github.com/acme/cws-packages/tree/develop", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/acme/cws-packages/develop"
	if base != want {
		t.Fatalf("got %q want %q", base, want)
	}
}

func TestRawBaseURLAlreadyRaw(t *testing.T) {
	base, err := RawBaseURL("https://raw.githubusercontent.com/acme/cws-packages/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/acme/cws-packages/main"
	if base != want {
		t.Fatalf("got %q want %q", base, want)
	}
}

func TestJoinRawURL(t *testing.T) {
	got := JoinRawURL("https://raw.githubusercontent.com/acme/cws-packages/main/", "softwares/nginx/package.json")
	want := "https://raw.githubusercontent.com/acme/cws-packages/main/softwares/nginx/package.json"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPackageMetaPath(t *testing.T) {
	if got := PackageMetaPath("Nginx"); got != "softwares/nginx/package.json" {
		t.Fatalf("got %q", got)
	}
}
