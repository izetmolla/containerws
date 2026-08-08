package list

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitBrowsePath(t *testing.T) {
	dir, prefix := splitBrowsePath("/workspace/app", false)
	if dir != "/workspace" || prefix != "app" {
		t.Fatalf("got dir=%q prefix=%q", dir, prefix)
	}
	dir, prefix = splitBrowsePath("/workspace/app", true)
	if dir != "/workspace/app" || prefix != "" {
		t.Fatalf("trailing slash: dir=%q prefix=%q", dir, prefix)
	}
	dir, prefix = splitBrowsePath("/", false)
	if dir != "/" || prefix != "" {
		t.Fatalf("root: dir=%q prefix=%q", dir, prefix)
	}
}

func TestListChildDirsFiltersPrefix(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "alpha"))
	mustMkdir(t, filepath.Join(root, "beta"))
	mustMkdir(t, filepath.Join(root, "alpine"))
	_ = os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("x"), 0o644)

	roots := []pathRoot{{Path: root, Label: "tmp"}}
	got := listChildDirs(root, "al", roots, true)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %#v", got)
	}
	if got[0].Name != "alpha" || got[1].Name != "alpine" {
		t.Fatalf("unexpected order/names: %#v", got)
	}
}

func TestIsPathAllowedNonAdmin(t *testing.T) {
	roots := []pathRoot{
		{Path: "/workspace", Label: "Workspace"},
		{Path: "/home/bob", Label: "Home"},
	}
	if !isPathAllowed("/workspace/app", roots, false) {
		t.Fatal("expected /workspace/app allowed")
	}
	if !isPathAllowed("/home/bob", roots, false) {
		t.Fatal("expected home allowed")
	}
	if isPathAllowed("/etc", roots, false) {
		t.Fatal("expected /etc denied")
	}
	if !isPathAllowed("/etc", roots, true) {
		t.Fatal("admin should allow /etc")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
