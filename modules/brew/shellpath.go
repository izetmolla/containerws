package brew

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	shellenvMarker = "# containerws: homebrew shellenv"
	profileDPath   = "/etc/profile.d/homebrew.sh"
)

// EnsureBrewShellPath makes `brew` available in interactive shells and the
// current process PATH after a panel/bootstrap install (Linuxbrew is not on
// PATH by default).
func EnsureBrewShellPath() {
	brewPath := ResolveBrewPath()
	if brewPath == "" {
		return
	}
	binDir := filepath.Dir(brewPath)
	sbinDir := filepath.Join(filepath.Dir(binDir), "sbin")

	prependProcessPATH(binDir, sbinDir)
	writeProfileD(binDir, sbinDir)
	appendUserShellRC(binDir)
}

func prependProcessPATH(dirs ...string) {
	cur := os.Getenv("PATH")
	parts := make([]string, 0, len(dirs)+8)
	seen := map[string]struct{}{}
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		parts = append(parts, d)
	}
	for _, p := range strings.Split(cur, string(os.PathListSeparator)) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		parts = append(parts, p)
	}
	_ = os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}

func writeProfileD(binDir, sbinDir string) {
	body := fmt.Sprintf(`%s
# Written by Container Workspace so brew works in login shells / Cloud Shell.
export HOMEBREW_PREFIX=%q
export HOMEBREW_CELLAR=%q
export HOMEBREW_REPOSITORY=%q
export PATH=%q:%q${PATH:+:$PATH}
export MANPATH=%q${MANPATH:+:$MANPATH}:
export INFOPATH=%q${INFOPATH:+:$INFOPATH}
`,
		shellenvMarker,
		filepath.Dir(binDir),
		filepath.Join(filepath.Dir(binDir), "Cellar"),
		filepath.Join(filepath.Dir(binDir), "Homebrew"),
		binDir,
		sbinDir,
		filepath.Join(filepath.Dir(binDir), "share", "man"),
		filepath.Join(filepath.Dir(binDir), "share", "info"),
	)

	dir := filepath.Dir(profileDPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		AppendBootstrapNote("shellenv: could not create " + dir + ": " + err.Error())
		return
	}
	if err := os.WriteFile(profileDPath, []byte(body), 0o644); err != nil {
		AppendBootstrapNote("shellenv: could not write " + profileDPath + ": " + err.Error())
		return
	}
}

func appendUserShellRC(binDir string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/root"
	}
	snippet := fmt.Sprintf(`
%s
if [ -x %q ]; then
  eval "$(%q shellenv)"
fi
`, shellenvMarker, filepath.Join(binDir, "brew"), filepath.Join(binDir, "brew"))

	for _, name := range []string{".bashrc", ".profile", ".zshrc"} {
		path := filepath.Join(home, name)
		appendOnce(path, shellenvMarker, snippet)
	}
}

func appendOnce(path, marker, snippet string) {
	data, err := os.ReadFile(path)
	if err == nil && strings.Contains(string(data), marker) {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(snippet)
}
