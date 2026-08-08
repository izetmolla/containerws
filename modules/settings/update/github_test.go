package update

import "testing"

func TestArchiveBase(t *testing.T) {
	cases := map[string]string{
		"containerws_0.0.4_linux_amd64.tar.gz": "containerws_0.0.4_linux_amd64",
		"containerws_0.0.4_linux_arm64.tgz":    "containerws_0.0.4_linux_arm64",
		"containerws_0.0.4_windows_amd64.zip":  "containerws_0.0.4_windows_amd64",
	}
	for in, want := range cases {
		if got := archiveBase(in); got != want {
			t.Fatalf("archiveBase(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMatchAssetExact(t *testing.T) {
	expected := "containerws_0.0.4_linux_amd64"
	if !matchAssetExact("containerws_0.0.4_linux_amd64.tar.gz", expected, "v0.0.4") {
		t.Fatal("exact match failed")
	}
	if matchAssetExact("containerws_0.0.4_linux_arm64.tar.gz", expected, "v0.0.4") {
		t.Fatal("arm64 must not exact-match amd64 expected")
	}
}

func TestMatchAssetLooseSuffix(t *testing.T) {
	// platformSuffix is derived from this machine; just assert the helper is consistent.
	suffix := platformSuffix()
	if suffix == "" || suffix[0] != '_' {
		t.Fatalf("bad platform suffix %q", suffix)
	}
	name := "containerws_0.0.4" + suffix + ".tar.gz"
	if !matchAssetLoose(name) {
		t.Fatalf("expected loose match for %q (suffix %q)", name, suffix)
	}
	// Adjacent arch must not match when suffix is amd64/arm64/etc.
	if suffix == "_linux_amd64" {
		if matchAssetLoose("containerws_0.0.4_linux_arm64.tar.gz") {
			t.Fatal("amd64 host must not loose-match arm64 asset")
		}
	}
	if suffix == "_linux_arm64" {
		if matchAssetLoose("containerws_0.0.4_linux_amd64.tar.gz") {
			t.Fatal("arm64 host must not loose-match amd64 asset")
		}
		if matchAssetLoose("containerws_0.0.4_linux_armv7.tar.gz") {
			t.Fatal("arm64 host must not loose-match armv7 asset")
		}
	}
}

func TestUpdateRepoNormalization(t *testing.T) {
	t.Setenv("CONTAINERWS_UPDATE_REPO", "")
	if got := updateRepo(); got != defaultUpdateRepo {
		t.Fatalf("default repo=%q", got)
	}
	t.Setenv("CONTAINERWS_UPDATE_REPO", "https://github.com/izetmolla/containerws/")
	if got := updateRepo(); got != "izetmolla/containerws" {
		t.Fatalf("normalized=%q", got)
	}
	t.Setenv("CONTAINERWS_UPDATE_REPO", "https://github.com/izetmolla/containerws.git")
	if got := updateRepo(); got != "izetmolla/containerws" {
		t.Fatalf("normalized git=%q", got)
	}
}

func TestReleaseDownloadURL(t *testing.T) {
	t.Setenv("CONTAINERWS_UPDATE_REPO", "izetmolla/containerws")
	got := releaseDownloadURL("v0.0.4", "containerws_0.0.4_linux_amd64.tar.gz")
	want := "https://github.com/izetmolla/containerws/releases/download/v0.0.4/containerws_0.0.4_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRestartArgvEnsuresStart(t *testing.T) {
	argv := restartArgv("/usr/local/lib/containerws/bin/containerws")
	if len(argv) < 2 || argv[0] != "/usr/local/lib/containerws/bin/containerws" {
		t.Fatalf("argv=%v", argv)
	}
	found := false
	for _, a := range argv[1:] {
		if a == "--start" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing --start in %v", argv)
	}
}
