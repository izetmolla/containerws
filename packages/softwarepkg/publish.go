package softwarepkg

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
)

// PublishRequest clones the package registry GitHub repo into a temp dir,
// scaffolds a new/updated software package, commits, and pushes.
type PublishRequest struct {
	// Registry row (PackageURL + credentials). Required.
	Registry models.SoftwarePackage
	// Git ref / branch to clone and push (default main).
	Ref string
	// Package metadata + scripts (same fields as ScaffoldRequest).
	Name          string
	Details       string
	Category      string
	SubCategory   string
	Tags          []string
	Icon          string
	Image         string
	Color         string
	Order         int
	ServiceUnits  []string
	CanControl    *bool
	ControlBackend string
	StartCommand   string
	RestartCommand string
	StopCommand    string
	Version       string
	Distros       []string
	AptPackage    string
	DnfPackage    string
	ApkPackage    string
	PacmanPackage string
	// CustomScript is copied into every install.json (post-install setup; optional).
	CustomScript string
	// FromHub scaffolds install.json for every izetmolla/containerws workspace tag.
	FromHub  bool
	HubImage string
	AlsoAny  bool
	// CommitMessage overrides the default commit message.
	CommitMessage string
	// AuthorName / AuthorEmail for the commit (optional; uses git defaults / env).
	AuthorName  string
	AuthorEmail string
	// DryRun scaffolds + commits locally but does not push.
	DryRun bool
	// KeepWorkDir leaves the temp clone on disk (returned in result).
	KeepWorkDir bool
	// WorkDirParent overrides where the temp dir is created (default os.TempDir).
	WorkDirParent string
	// GitBin overrides the git executable (default git).
	GitBin string
}

// PublishResult summarizes a publish run.
type PublishResult struct {
	Name       string   `json:"name"`
	Repo       string   `json:"repo"`
	Ref        string   `json:"ref"`
	WorkDir    string   `json:"work_dir,omitempty"`
	Files      []string `json:"files"`
	Distros    []string `json:"distros"`
	Commit     string   `json:"commit,omitempty"`
	Pushed     bool     `json:"pushed"`
	DryRun     bool     `json:"dry_run,omitempty"`
	RemoteURL  string   `json:"remote_url"` // sanitized (no credentials)
	Message    string   `json:"message"`
	CloneLog   string   `json:"clone_log,omitempty"`
	CommitLog  string   `json:"commit_log,omitempty"`
	PushLog    string   `json:"push_log,omitempty"`
}

// Publish clones the registry repo, scaffolds the package, commits, and pushes.
func Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	name := sanitizeSegment(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.Registry.PackageURL) == "" {
		return nil, fmt.Errorf("registry package_url is empty")
	}
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		ref = defaultRef
	}
	gitBin := firstNonEmpty(req.GitBin, "git")
	if _, err := exec.LookPath(gitBin); err != nil {
		return nil, fmt.Errorf("git not found on PATH")
	}

	cloneURL, publicURL, err := GitCloneURL(req.Registry.PackageURL, AuthFromPackage(req.Registry))
	if err != nil {
		return nil, err
	}

	parent := req.WorkDirParent
	if parent == "" {
		parent = os.TempDir()
	}
	workDir, err := os.MkdirTemp(parent, "cws-packages-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	cleanup := !req.KeepWorkDir
	defer func() {
		if cleanup {
			_ = os.RemoveAll(workDir)
		}
	}()

	out := &PublishResult{
		Name:      name,
		Repo:      publicURL,
		Ref:       ref,
		RemoteURL: publicURL,
		DryRun:    req.DryRun,
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cloneArgs := []string{"clone", "--branch", ref, "--single-branch", "--depth", "1", cloneURL, workDir}
	cloneOut, err := runGit(cloneCtx, gitBin, "", cloneArgs...)
	out.CloneLog = redactSecrets(cloneOut, req.Registry)
	if err != nil {
		// Branch might not exist yet — try default clone then checkout/create.
		_ = os.RemoveAll(workDir)
		_ = os.MkdirAll(workDir, 0o755)
		cloneOut2, err2 := runGit(ctx, gitBin, "", "clone", "--depth", "1", cloneURL, workDir)
		out.CloneLog = redactSecrets(cloneOut+"\n"+cloneOut2, req.Registry)
		if err2 != nil {
			return out, fmt.Errorf("git clone: %w", err2)
		}
		// Ensure we're on the desired ref.
		if co, cerr := runGit(ctx, gitBin, workDir, "checkout", "-B", ref); cerr != nil {
			out.CloneLog += "\n" + redactSecrets(co, req.Registry)
			return out, fmt.Errorf("git checkout %s: %w", ref, cerr)
		}
	}

	scaffoldReq := ScaffoldRequest{
		Name:          name,
		Details:       req.Details,
		Category:      req.Category,
		SubCategory:   req.SubCategory,
		Tags:          req.Tags,
		Icon:          req.Icon,
		Image:         req.Image,
		Color:         req.Color,
		Order:         req.Order,
		ServiceUnits:   req.ServiceUnits,
		CanControl:     req.CanControl,
		ControlBackend: req.ControlBackend,
		StartCommand:   req.StartCommand,
		RestartCommand: req.RestartCommand,
		StopCommand:    req.StopCommand,
		Version:        req.Version,
		Distros:       req.Distros,
		AptPackage:    req.AptPackage,
		DnfPackage:    req.DnfPackage,
		ApkPackage:    req.ApkPackage,
		PacmanPackage: req.PacmanPackage,
		CustomScript:  req.CustomScript,
		OutputDir:     workDir,
		Overwrite:     true,
		FromHub:       req.FromHub,
		HubImage:      req.HubImage,
		AlsoAny:       req.AlsoAny || req.FromHub,
	}
	scaf, err := Scaffold(scaffoldReq)
	if err != nil {
		return out, fmt.Errorf("scaffold: %w", err)
	}
	out.Files = scaf.Files
	out.Distros = scaf.Distros

	// Configure author for this repo only (does not touch global git config).
	if name := strings.TrimSpace(req.AuthorName); name != "" {
		if _, err := runGit(ctx, gitBin, workDir, "config", "user.name", name); err != nil {
			return out, fmt.Errorf("git config user.name: %w", err)
		}
	}
	if email := strings.TrimSpace(req.AuthorEmail); email != "" {
		if _, err := runGit(ctx, gitBin, workDir, "config", "user.email", email); err != nil {
			return out, fmt.Errorf("git config user.email: %w", err)
		}
	}
	// Ensure identity exists for commit (CI/temp envs often lack one).
	if _, err := runGit(ctx, gitBin, workDir, "config", "--get", "user.email"); err != nil {
		_ = runGitQuiet(ctx, gitBin, workDir, "config", "user.email", "containerws@localhost")
		_ = runGitQuiet(ctx, gitBin, workDir, "config", "user.name", "Container Workspace")
	}

	if _, err := runGit(ctx, gitBin, workDir, "add", "-A", "softwares"); err != nil {
		return out, fmt.Errorf("git add: %w", err)
	}
	status, _ := runGit(ctx, gitBin, workDir, "status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		out.Message = fmt.Sprintf("No changes for %s — already up to date on %s", name, publicURL)
		if req.KeepWorkDir {
			cleanup = false
			out.WorkDir = workDir
		}
		return out, nil
	}

	msg := strings.TrimSpace(req.CommitMessage)
	if msg == "" {
		msg = fmt.Sprintf("Add/update software package %s", name)
	}
	commitOut, err := runGit(ctx, gitBin, workDir, "commit", "-m", msg)
	out.CommitLog = redactSecrets(commitOut, req.Registry)
	if err != nil {
		return out, fmt.Errorf("git commit: %w", err)
	}
	if hash, herr := runGit(ctx, gitBin, workDir, "rev-parse", "HEAD"); herr == nil {
		out.Commit = strings.TrimSpace(hash)
	}

	if req.DryRun {
		out.Message = fmt.Sprintf("Dry run: committed %s (%s) locally — not pushed", name, shortSHA(out.Commit))
		cleanup = false
		out.WorkDir = workDir
		return out, nil
	}

	pushOut, err := runGit(ctx, gitBin, workDir, "push", "origin", "HEAD:"+ref)
	out.PushLog = redactSecrets(pushOut, req.Registry)
	if err != nil {
		cleanup = false
		out.WorkDir = workDir
		return out, fmt.Errorf("git push: %w", err)
	}
	out.Pushed = true
	out.Message = fmt.Sprintf("Published %s to %s@%s (%s)", name, publicURL, ref, shortSHA(out.Commit))
	InvalidateCatalogCache()

	if req.KeepWorkDir {
		cleanup = false
		out.WorkDir = workDir
	}
	return out, nil
}

// GitCloneURL builds an authenticated git clone URL and a public display URL.
// When no token/password is set, uses SSH (git@github.com:owner/repo.git) so
// agent hosts with deployed SSH keys can push without a PAT.
func GitCloneURL(packageURL string, auth Auth) (cloneURL, publicURL string, err error) {
	owner, repo, err := ParseGitHubRepo(packageURL)
	if err != nil {
		return "", "", err
	}
	publicURL = fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	token := strings.TrimSpace(auth.Token)
	user := strings.TrimSpace(auth.Username)
	pass := auth.Password
	switch {
	case token != "" && user != "":
		u, err := url.Parse(publicURL + ".git")
		if err != nil {
			return "", "", err
		}
		u.User = url.UserPassword(user, token)
		return u.String(), publicURL, nil
	case token != "":
		u, err := url.Parse(publicURL + ".git")
		if err != nil {
			return "", "", err
		}
		u.User = url.UserPassword("x-access-token", token)
		return u.String(), publicURL, nil
	case user != "" && pass != "":
		u, err := url.Parse(publicURL + ".git")
		if err != nil {
			return "", "", err
		}
		u.User = url.UserPassword(user, pass)
		return u.String(), publicURL, nil
	default:
		// Prefer SSH for public-key auth (no PAT required).
		return fmt.Sprintf("git@github.com:%s/%s.git", owner, repo), publicURL, nil
	}
}

// ParseGitHubRepo extracts owner/repo from a github or raw.githubusercontent URL.
func ParseGitHubRepo(packageURL string) (owner, repo string, err error) {
	raw := strings.TrimSpace(packageURL)
	if raw == "" {
		return "", "", fmt.Errorf("package_url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse package_url: %w", err)
	}
	host := strings.ToLower(u.Host)
	parts := splitPath(u.Path)
	switch {
	case host == "github.com" || host == "www.github.com":
		if len(parts) < 2 {
			return "", "", fmt.Errorf("github URL must include owner/repo")
		}
		return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
	case host == "raw.githubusercontent.com":
		if len(parts) < 2 {
			return "", "", fmt.Errorf("raw.githubusercontent.com URL must include owner/repo")
		}
		return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
	default:
		return "", "", fmt.Errorf("publish requires a GitHub package_url (got host %s)", host)
	}
}

func runGit(ctx context.Context, gitBin, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gitBin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Avoid interactive prompts hanging the tool.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=echo",
	)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func runGitQuiet(ctx context.Context, gitBin, dir string, args ...string) error {
	_, err := runGit(ctx, gitBin, dir, args...)
	return err
}

func shortSHA(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func redactSecrets(s string, reg models.SoftwarePackage) string {
	out := s
	for _, secret := range []string{reg.Token, reg.Password} {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, "***")
		out = strings.ReplaceAll(out, url.QueryEscape(secret), "***")
	}
	// Also redact x-access-token:…@ patterns loosely.
	if idx := strings.Index(out, "x-access-token:"); idx >= 0 {
		rest := out[idx:]
		if at := strings.Index(rest, "@"); at > 0 {
			out = out[:idx] + "x-access-token:***" + rest[at:]
		}
	}
	return out
}
