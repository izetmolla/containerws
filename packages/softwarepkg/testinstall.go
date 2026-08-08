package softwarepkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestInstallRequest runs install scripts inside Hub workspace images.
type TestInstallRequest struct {
	// Name is the software package name under softwares/{name}/.
	Name string
	// PackageRoot is the registry repo root (contains softwares/).
	PackageRoot string
	// HubImage namespace/name (default izetmolla/containerws).
	HubImage string
	// Tags limits which Hub tags to test (e.g. ubuntu-26.04). Empty = all workspace tags.
	Tags []string
	// VerifyCommand runs after install (default: command -v <pkg-or-name>).
	VerifyCommand string
	// Pull runs docker pull before each test.
	Pull bool
	// TimeoutSeconds per tag (default 600).
	TimeoutSeconds int
	// DryRun resolves scripts/images but does not run docker.
	DryRun bool
	// DockerBin overrides the docker executable (default docker).
	DockerBin string
}

// TagTestResult is the outcome for one image tag.
type TagTestResult struct {
	Tag           string `json:"tag"`
	Image         string `json:"image"`
	DistroID      string `json:"distro_id"`
	DistroVersion string `json:"distro_version"`
	InstallPath   string `json:"install_path,omitempty"`
	VerifyCommand string `json:"verify_command,omitempty"`
	Passed        bool   `json:"passed"`
	Skipped       bool   `json:"skipped,omitempty"`
	ExitCode      int    `json:"exit_code,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	Error         string `json:"error,omitempty"`
	Message       string `json:"message"`
}

// TestInstallResult aggregates container install tests.
type TestInstallResult struct {
	Name    string          `json:"name"`
	Passed  int             `json:"passed"`
	Failed  int             `json:"failed"`
	Skipped int             `json:"skipped"`
	Results []TagTestResult `json:"results"`
	Message string          `json:"message"`
}

// TestInstall pulls/runs izetmolla/containerws tags and executes the matching install.json script.
func TestInstall(ctx context.Context, req TestInstallRequest) (*TestInstallResult, error) {
	name := sanitizeSegment(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	root := strings.TrimSpace(req.PackageRoot)
	if root == "" {
		return nil, fmt.Errorf("package_root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	hubImage := firstNonEmpty(strings.TrimSpace(req.HubImage), DefaultHubImage)
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 600
	}
	dockerBin := firstNonEmpty(req.DockerBin, "docker")

	hubTags, err := ListHubTags(ctx, &ListHubTagsOptions{Image: hubImage})
	if err != nil {
		return nil, err
	}
	filter := map[string]struct{}{}
	for _, t := range req.Tags {
		t = strings.TrimSpace(t)
		if t != "" {
			filter[t] = struct{}{}
		}
	}

	out := &TestInstallResult{Name: name, Results: make([]TagTestResult, 0)}
	for _, tag := range hubTags {
		if !tag.Workspace {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[tag.Name]; !ok {
				continue
			}
		}
		res := testOneTag(ctx, testOneOpts{
			root:       root,
			name:       name,
			tag:        tag,
			verify:     strings.TrimSpace(req.VerifyCommand),
			pull:       req.Pull,
			dryRun:     req.DryRun,
			dockerBin:  dockerBin,
			timeoutSec: timeout,
		})
		out.Results = append(out.Results, res)
		switch {
		case res.Skipped:
			out.Skipped++
		case res.Passed:
			out.Passed++
		default:
			out.Failed++
		}
	}
	if len(out.Results) == 0 {
		return nil, fmt.Errorf("no matching workspace tags to test")
	}
	out.Message = fmt.Sprintf("%s: passed=%d failed=%d skipped=%d", name, out.Passed, out.Failed, out.Skipped)
	return out, nil
}

type testOneOpts struct {
	root, name, verify, dockerBin string
	tag                           HubTag
	pull, dryRun                  bool
	timeoutSec                    int
}

func testOneTag(ctx context.Context, opts testOneOpts) TagTestResult {
	res := TagTestResult{
		Tag:           opts.tag.Name,
		Image:         opts.tag.Image,
		DistroID:      opts.tag.DistroID,
		DistroVersion: opts.tag.DistroVersion,
	}
	host := HostFacts{
		DistroID:      opts.tag.DistroID,
		DistroVersion: opts.tag.DistroVersion,
		Arch:          "any",
	}
	rel, script, err := loadInstallScript(opts.root, opts.name, host)
	if err != nil {
		res.Skipped = true
		res.Error = err.Error()
		res.Message = "skipped — no matching install.json"
		return res
	}
	res.InstallPath = rel

	verify := opts.verify
	if verify == "" {
		pkg := guessPkgFromScript(script, opts.name)
		verify = "command -v " + shellQuote(pkg)
	}
	res.VerifyCommand = verify

	if opts.dryRun {
		res.Skipped = true
		res.Message = "dry_run — would pull/run " + opts.tag.Image + " with " + rel
		return res
	}

	if _, err := exec.LookPath(opts.dockerBin); err != nil {
		res.Error = "docker not found on PATH"
		res.Message = "failed — docker unavailable"
		return res
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutSec)*time.Second)
	defer cancel()

	if opts.pull {
		pull := exec.CommandContext(runCtx, opts.dockerBin, "pull", opts.tag.Image)
		var pstdout, pstderr bytes.Buffer
		pull.Stdout = &pstdout
		pull.Stderr = &pstderr
		if err := pull.Run(); err != nil {
			res.Error = fmt.Sprintf("docker pull: %v", err)
			res.Stderr = truncateOut(pstderr.String(), 4000)
			res.Stdout = truncateOut(pstdout.String(), 2000)
			res.Message = "failed — pull"
			return res
		}
	}

	body := strings.TrimSpace(script) + "\n\necho '==> verify'\n" + verify + "\n"
	start := time.Now()
	cmd := exec.CommandContext(runCtx, opts.dockerBin, "run", "--rm",
		"--entrypoint", "bash",
		opts.tag.Image,
		"-lc", body,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	res.DurationMS = time.Since(start).Milliseconds()
	res.Stdout = truncateOut(stdout.String(), 8000)
	res.Stderr = truncateOut(stderr.String(), 8000)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
		res.Error = err.Error()
		if runCtx.Err() != nil {
			res.Message = "failed — timeout"
		} else {
			res.Message = "failed — install or verify"
		}
		return res
	}
	res.Passed = true
	res.ExitCode = 0
	res.Message = "ok"
	return res
}

func loadInstallScript(root, name string, host HostFacts) (relPath, script string, err error) {
	paths := ResolveInstallPaths(name, host)
	for _, rel := range paths {
		abs := filepath.Join(root, rel)
		raw, rerr := os.ReadFile(abs)
		if rerr != nil {
			continue
		}
		var spec InstallSpec
		if jerr := json.Unmarshal(raw, &spec); jerr != nil {
			return rel, "", fmt.Errorf("%s: %w", rel, jerr)
		}
		script = strings.TrimSpace(spec.InstallScript)
		if script == "" {
			return rel, "", fmt.Errorf("%s: empty install_script", rel)
		}
		return rel, script, nil
	}
	return "", "", fmt.Errorf("no install.json for %s on %s/%s", name, host.DistroID, host.DistroVersion)
}

func guessPkgFromScript(script, fallback string) string {
	// Prefer last apt-get/dnf/apk/pacman package token as a weak heuristic.
	lines := strings.Split(script, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch {
		case strings.Contains(line, "apt-get install"),
			strings.Contains(line, "dnf install"),
			strings.Contains(line, "dnf upgrade"),
			strings.Contains(line, "apk add"),
			strings.Contains(line, "pacman -"):
			pkg := fields[len(fields)-1]
			pkg = strings.Trim(pkg, `"'`)
			if pkg != "" && pkg != `\n` && !strings.HasPrefix(pkg, "-") {
				return pkg
			}
		}
	}
	return sanitizeSegment(fallback)
}

func truncateOut(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n…(truncated)"
}
