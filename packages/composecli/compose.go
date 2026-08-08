package composecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
)

// Result captures docker compose CLI output for API error messages.
type Result struct {
	Command string
	Stdout  string
	Stderr  string
	Err     error
}

func (r *Result) Error() string {
	if r == nil {
		return "compose: unknown error"
	}
	parts := make([]string, 0, 4)
	if r.Command != "" {
		parts = append(parts, "command: "+r.Command)
	}
	if r.Err != nil {
		parts = append(parts, r.Err.Error())
	}
	out := strings.TrimSpace(r.Stderr)
	if out == "" {
		out = strings.TrimSpace(r.Stdout)
	}
	if out != "" {
		parts = append(parts, out)
	}
	if len(parts) == 0 {
		return "compose command failed"
	}
	return strings.Join(parts, "\n")
}

// Up writes compose YAML and runs `docker compose up -d`.
// By default includes `--remove-orphans`. Pass Pull to re-pull images.
func Up(ctx context.Context, env *models.DockerEnvironment, project, yaml string, opt ...RunOptions) *Result {
	o := firstOpt(opt)
	args := []string{"up", "-d"}
	if o.Pull {
		args = append(args, "--pull", "always")
	}
	if !o.NoRemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	return run(ctx, env, project, yaml, o, args...)
}

// Down runs `docker compose down --remove-orphans` (volumes kept).
func Down(ctx context.Context, env *models.DockerEnvironment, project, yaml string, opt ...RunOptions) *Result {
	return run(ctx, env, project, yaml, firstOpt(opt), "down", "--remove-orphans")
}

// Config validates compose YAML with `docker compose config` (no containers created).
func Config(ctx context.Context, env *models.DockerEnvironment, project, yaml string, opt ...RunOptions) *Result {
	if strings.TrimSpace(project) == "" {
		project = "validate"
	}
	return run(ctx, env, project, yaml, firstOpt(opt), "config", "--quiet")
}

// RunOptions customizes compose CLI workdir contents and up flags.
type RunOptions struct {
	// EnvFile is written as `.env` (and `stack.env`) for Compose variable substitution.
	EnvFile string
	// Pull adds `--pull always` to `docker compose up`.
	Pull bool
	// NoRemoveOrphans skips `--remove-orphans` on up.
	NoRemoveOrphans bool
}

func firstOpt(opt []RunOptions) RunOptions {
	if len(opt) == 0 {
		return RunOptions{}
	}
	return opt[0]
}

func run(ctx context.Context, env *models.DockerEnvironment, project, yaml string, opt RunOptions, args ...string) *Result {
	project = sanitizeProject(project)
	if project == "" {
		return &Result{Err: fmt.Errorf("stack name is required")}
	}
	yaml = strings.TrimSpace(yaml)
	if yaml == "" {
		return &Result{Err: fmt.Errorf("compose YAML is required")}
	}

	dir, cleanup, envVars, err := prepareWorkdir(env, project, yaml, opt.EnvFile)
	if err != nil {
		return &Result{Err: err}
	}
	defer cleanup()

	composeFile := filepath.Join(dir, "docker-compose.yml")
	cmdArgs := append([]string{"compose", "-p", project, "-f", composeFile}, args...)
	cmdLine := "docker " + strings.Join(cmdArgs, " ")

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), envVars...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	res := &Result{
		Command: cmdLine,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Err:     runErr,
	}
	if runErr != nil {
		return res
	}
	return nil
}

func sanitizeProject(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func normalizeEnvFile(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out = append(out, line)
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func prepareWorkdir(env *models.DockerEnvironment, project, yaml, envFile string) (dir string, cleanup func(), envVars []string, err error) {
	dir, err = os.MkdirTemp("", "cws-compose-"+project+"-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yaml+"\n"), 0o600); err != nil {
		cleanup()
		return "", nil, nil, fmt.Errorf("write compose file: %w", err)
	}

	if body := normalizeEnvFile(envFile); body != "" {
		// Compose loads `.env` for ${VAR} substitution; also write stack.env for Portainer-style refs.
		for _, name := range []string{".env", "stack.env"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body+"\n"), 0o600); err != nil {
				cleanup()
				return "", nil, nil, fmt.Errorf("write %s: %w", name, err)
			}
		}
	}

	envVars = []string{}
	if env == nil || env.ConnType == "" || env.ConnType == models.DockerConnUnix {
		sock := "/var/run/docker.sock"
		if env != nil && strings.TrimSpace(env.SocketPath) != "" {
			sock = strings.TrimSpace(env.SocketPath)
		}
		envVars = append(envVars, "DOCKER_HOST=unix://"+sock)
		return dir, cleanup, envVars, nil
	}

	switch env.ConnType {
	case models.DockerConnTLS:
		certDir := filepath.Join(dir, "certs")
		if err := os.MkdirAll(certDir, 0o700); err != nil {
			cleanup()
			return "", nil, nil, err
		}
		writes := []struct {
			name string
			body string
		}{
			{"ca.pem", env.TLSCACert},
			{"cert.pem", env.TLSCert},
			{"key.pem", env.TLSKey},
		}
		for _, w := range writes {
			if strings.TrimSpace(w.body) == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(certDir, w.name), []byte(w.body), 0o600); err != nil {
				cleanup()
				return "", nil, nil, err
			}
		}
		host := strings.TrimSpace(env.HostURL)
		if host == "" {
			host = fmt.Sprintf("tcp://%s:%d", strings.TrimSpace(env.TCPHost), env.TCPPort)
		}
		envVars = append(envVars,
			"DOCKER_HOST="+host,
			"DOCKER_CERT_PATH="+certDir,
		)
		if env.TLSSkipVerify {
			envVars = append(envVars, "DOCKER_TLS_VERIFY=")
		} else {
			envVars = append(envVars, "DOCKER_TLS_VERIFY=1")
		}
		return dir, cleanup, envVars, nil

	case models.DockerConnSSH:
		sshDir := filepath.Join(dir, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			cleanup()
			return "", nil, nil, err
		}
		keyPath := filepath.Join(sshDir, "id_key")
		if strings.TrimSpace(env.SSHPrivateKey) != "" {
			if err := os.WriteFile(keyPath, []byte(env.SSHPrivateKey+"\n"), 0o600); err != nil {
				cleanup()
				return "", nil, nil, err
			}
		}
		hostAlias := "cws-docker"
		cfg := fmt.Sprintf("Host %s\n  HostName %s\n  User %s\n  Port %d\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n",
			hostAlias,
			strings.TrimSpace(env.SSHHost),
			strings.TrimSpace(env.SSHUser),
			env.SSHPort,
		)
		if strings.TrimSpace(env.SSHPrivateKey) != "" {
			cfg += "  IdentityFile " + keyPath + "\n  IdentitiesOnly yes\n"
		}
		if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(cfg), 0o600); err != nil {
			cleanup()
			return "", nil, nil, err
		}
		envVars = append(envVars,
			"HOME="+dir,
			"DOCKER_HOST=ssh://"+hostAlias,
		)
		return dir, cleanup, envVars, nil
	default:
		cleanup()
		return "", nil, nil, fmt.Errorf("unsupported docker connection type: %s", env.ConnType)
	}
}
