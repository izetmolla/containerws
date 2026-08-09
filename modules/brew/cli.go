package brew

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	linuxbrewBin = "/home/linuxbrew/.linuxbrew/bin/brew"
	linuxbrewDir = "/home/linuxbrew/.linuxbrew"
)

// ResolveBrewPath returns the brew binary path if present.
func ResolveBrewPath() string {
	if p, err := exec.LookPath("brew"); err == nil && strings.TrimSpace(p) != "" {
		return p
	}
	if st, err := os.Stat(linuxbrewBin); err == nil && !st.IsDir() {
		return linuxbrewBin
	}
	return ""
}

// BrewPrefix returns Homebrew prefix from brew --prefix, or the Linux default.
func BrewPrefix(brewPath string) string {
	if brewPath == "" {
		brewPath = ResolveBrewPath()
	}
	if brewPath == "" {
		if st, err := os.Stat(linuxbrewDir); err == nil && st.IsDir() {
			return linuxbrewDir
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runBrew(ctx, brewPath, "--prefix")
	if err != nil {
		return filepath.Dir(filepath.Dir(brewPath))
	}
	return strings.TrimSpace(out)
}

func brewEnv(brewPath string) []string {
	env := os.Environ()
	binDir := filepath.Dir(brewPath)
	prefix := filepath.Dir(binDir)
	path := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env = append(env,
		"HOMEBREW_NO_AUTO_UPDATE=1",
		"HOMEBREW_NO_ENV_HINTS=1",
		"NONINTERACTIVE=1",
		"PATH="+path,
	)
	if prefix != "" {
		env = append(env, "HOMEBREW_PREFIX="+prefix)
	}
	return env
}

func runBrew(ctx context.Context, brewPath string, args ...string) (string, error) {
	if brewPath == "" {
		brewPath = ResolveBrewPath()
	}
	if brewPath == "" {
		return "", errors.New("brew is not installed")
	}
	cmd := exec.CommandContext(ctx, brewPath, args...)
	cmd.Env = brewEnv(brewPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("brew %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func runBrewCombined(ctx context.Context, brewPath string, args ...string) (string, error) {
	if brewPath == "" {
		brewPath = ResolveBrewPath()
	}
	if brewPath == "" {
		return "", errors.New("brew is not installed")
	}
	cmd := exec.CommandContext(ctx, brewPath, args...)
	cmd.Env = brewEnv(brewPath)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if err != nil {
		return out, fmt.Errorf("brew %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
