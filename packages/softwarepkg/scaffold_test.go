package softwarepkg_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func TestScaffoldWritesDistroTree(t *testing.T) {
	dir := t.TempDir()
	res, err := softwarepkg.Scaffold(softwarepkg.ScaffoldRequest{
		Name:      "nginx",
		Details:   "HTTP server",
		Category:  "Web",
		Version:   "1.26.2",
		Distros:   []string{"ubuntu", "fedora", "default"},
		OutputDir: dir,
		Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"softwares/nginx/package.json",
		"softwares/nginx/ubuntu/any/any/install.json",
		"softwares/nginx/fedora/any/any/install.json",
		"softwares/nginx/default/install.json",
		"softwares/index.json",
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	if res.Name != "nginx" {
		t.Fatalf("name=%s", res.Name)
	}
	script := softwarepkg.BuildInstallScript(softwarepkg.DistroTarget{
		PackageFamily: "apt",
		PkgName:       "nginx",
	})
	if !strings.Contains(script, "apt-get") || !strings.Contains(script, "nginx") {
		t.Fatalf("unexpected apt script: %s", script)
	}
	if !strings.Contains(script, "apt_update_safe") {
		t.Fatalf("expected apt_update_safe helper in script: %s", script)
	}
}
