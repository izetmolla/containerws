package codeserver

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneOptions configures an optional git clone into the session folder.
type CloneOptions struct {
	Repo   string
	Branch string
	Token  string
}

// NormalizeGitRepo turns owner/repo or a GitHub URL into an https clone URL.
func NormalizeGitRepo(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("repository is required")
	}
	raw = strings.TrimSuffix(raw, ".git")

	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "git@"):
		if !strings.HasSuffix(strings.ToLower(raw), ".git") && (strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) {
			raw += ".git"
		}
		return raw, nil
	case strings.HasPrefix(lower, "github.com/"):
		return "https://" + strings.TrimSuffix(raw, "/") + ".git", nil
	case strings.Count(raw, "/") == 1 && !strings.Contains(raw, ":"):
		parts := strings.Split(raw, "/")
		if parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("invalid repository %q", raw)
		}
		return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", nil
	default:
		return "", fmt.Errorf("unsupported repository format %q (use owner/repo or a git URL)", raw)
	}
}

// InjectGitToken embeds a token into https clone URLs for private repos.
// The returned URL should only be used for the git process, never logged.
func InjectGitToken(repoURL, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return repoURL
	}
	u, err := url.Parse(repoURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return repoURL
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String()
}

// RedactGitURL strips userinfo from a URL for safe logging.
func RedactGitURL(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil || u.User == nil {
		return repoURL
	}
	u.User = nil
	return u.String()
}

func dirIsEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func isGitRepo(path string) bool {
	st, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && st.IsDir()
}

// CloneRepoInto clones repo into dest when empty, or checks out branch when already a git repo.
// onLine receives stdout/stderr lines for streaming UIs.
func CloneRepoInto(dest string, opts CloneOptions, onLine func(line, stream string)) error {
	repo, err := NormalizeGitRepo(opts.Repo)
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(opts.Branch)
	authURL := InjectGitToken(repo, opts.Token)
	safeURL := RedactGitURL(repo)

	if err := EnsureFolder(dest); err != nil {
		return err
	}

	empty, err := dirIsEmpty(dest)
	if err != nil {
		return err
	}

	run := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = dest
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GIT_ASKPASS=echo",
		)
		var stderr bytes.Buffer
		cmd.Stdout = &lineWriter{stream: "stdout", onLine: onLine}
		cmd.Stderr = &lineWriter{stream: "stderr", onLine: onLine, also: &stderr}
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("git %s: %s", args[0], msg)
		}
		return nil
	}

	if isGitRepo(dest) {
		if onLine != nil {
			onLine("existing git repository detected", "system")
		}
		if branch != "" {
			if onLine != nil {
				onLine("checking out branch "+branch+"…", "system")
			}
			_ = run("fetch", "--depth", "1", "origin", branch)
			if err := run("checkout", branch); err != nil {
				return err
			}
		}
		return nil
	}

	if !empty {
		return fmt.Errorf("folder is not empty and is not a git repository: %s", dest)
	}

	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, authURL, ".")
	if onLine != nil {
		msg := "cloning " + safeURL
		if branch != "" {
			msg += " (" + branch + ")"
		}
		onLine(msg+"…", "system")
	}
	return run(args...)
}

type lineWriter struct {
	stream string
	onLine func(line, stream string)
	also   *bytes.Buffer
	buf    []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	if w.also != nil {
		_, _ = w.also.Write(p)
	}
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:i]), "\r")
		w.buf = w.buf[i+1:]
		if w.onLine != nil && strings.TrimSpace(line) != "" {
			w.onLine(line, w.stream)
		}
	}
	return len(p), nil
}
