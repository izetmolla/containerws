package codeserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"gorm.io/gorm"
)

var ErrCodeCLINotInstalled = errors.New(
	`VS Code Server is not installed

Install it from the panel Softwares page, or:
  cws software install "VS Code Server"

Expected binary: /usr/local/lib/vscode-cli/code (or code-cli on PATH)`,
)

// EnsureFolder creates path (and parents) when missing; errors if path exists and is not a directory.
func EnsureFolder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/workspace"
	}
	st, err := os.Stat(path)
	if err == nil {
		if !st.IsDir() {
			return fmt.Errorf("path exists and is not a directory: %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

// LookupCodeCLI finds a Microsoft VS Code CLI that supports serve-web.
func LookupCodeCLI() (string, error) {
	candidates := []string{
		"/usr/local/lib/vscode-cli/code",
		"/usr/local/bin/code-cli",
	}
	for _, c := range candidates {
		if isExecutableFile(c) {
			if codeCLISupportsServeWeb(c) {
				return c, nil
			}
		}
	}
	if p, err := exec.LookPath("code-cli"); err == nil && isExecutableFile(p) && codeCLISupportsServeWeb(p) {
		return p, nil
	}
	if p, err := exec.LookPath("code"); err == nil && isExecutableFile(p) && codeCLISupportsServeWeb(p) {
		return p, nil
	}
	return "", ErrCodeCLINotInstalled
}

// UsedPorts returns ports claimed by active codeserver sessions.
func UsedPorts(db *gorm.DB) map[int]struct{} {
	used := map[int]struct{}{}
	var rows []models.CodeserverSession
	if err := db.Where("status = ? AND port > 0", models.CodeserverSessionStatusActive).Find(&rows).Error; err != nil {
		return used
	}
	for _, row := range rows {
		used[row.Port] = struct{}{}
	}
	return used
}

// IsLive reports whether the session process/port is accepting connections.
func IsLive(s models.CodeserverSession) bool {
	if s.Port > 0 && adduser.IsLocalPortListening(s.Port) {
		return true
	}
	return s.Pid > 0 && processAlive(s.Pid)
}

// StopProcess stops a session process (and anything still bound to its port).
func StopProcess(s *models.CodeserverSession) error {
	if s == nil {
		return nil
	}
	var errs []string
	if s.Pid > 0 {
		if err := KillPID(s.Pid); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if s.Port > 0 && adduser.IsLocalPortListening(s.Port) {
		_ = exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", s.Port)).Run()
		time.Sleep(200 * time.Millisecond)
	}
	if len(errs) > 0 && adduser.IsLocalPortListening(s.Port) {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// StartOptions configures a serve-web launch.
type StartOptions struct {
	Folder  string
	Host    string
	Port    int
	LinuxUser string
	Token   string // "", "none", "auto", or secret
}

// StartResult is the outcome of a successful serve-web start.
type StartResult struct {
	Port    int
	Pid     int
	LogPath string
}

// StartServeWeb launches code serve-web in the background and waits until the port is ready.
func StartServeWeb(opts StartOptions) (*StartResult, error) {
	bin, err := LookupCodeCLI()
	if err != nil {
		return nil, err
	}

	folder := strings.TrimSpace(opts.Folder)
	if folder == "" {
		folder = "/workspace"
	}
	folder, err = filepath.Abs(folder)
	if err != nil {
		return nil, err
	}
	if err := EnsureFolder(folder); err != nil {
		return nil, err
	}

	host := strings.TrimSpace(opts.Host)
	if host == "" {
		host = adduser.BindHost
	}
	port := opts.Port
	if port <= 0 {
		port, err = adduser.PickUnusedLocalPort(nil)
		if err != nil {
			return nil, fmt.Errorf("allocate port: %w", err)
		}
	}

	linuxName := strings.TrimSpace(opts.LinuxUser)
	home := os.Getenv("HOME")
	if home == "" {
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			home = u.HomeDir
			if linuxName == "" {
				linuxName = u.Username
			}
		} else {
			home = "/root"
		}
	}
	if linuxName == "" {
		linuxName = "root"
	}

	dataDir := filepath.Join(home, ".vscode-server-web")
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("serve-web-%d.log", port))
	pidPath := filepath.Join(dataDir, fmt.Sprintf("serve-web-%d.pid", port))

	argsCLI := []string{
		"serve-web",
		"--accept-server-license-terms",
		"--host", host,
		"--port", strconv.Itoa(port),
		"--default-folder", folder,
		"--server-data-dir", dataDir,
		"--cli-data-dir", dataDir,
	}
	switch strings.TrimSpace(opts.Token) {
	case "", "none":
		argsCLI = append(argsCLI, "--without-connection-token")
	case "auto":
		// CLI generates a token.
	default:
		argsCLI = append(argsCLI, "--connection-token", opts.Token)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()

	proc := exec.Command(bin, argsCLI...)
	proc.Dir = folder
	proc.Stdout = logFile
	proc.Stderr = logFile
	proc.Env = append(os.Environ(),
		"HOME="+home,
		"USER="+linuxName,
	)
	proc.SysProcAttr = detachSysProcAttr()

	if err := proc.Start(); err != nil {
		return nil, fmt.Errorf("start code serve-web: %w", err)
	}
	pid := proc.Process.Pid
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644)
	_ = proc.Process.Release()

	if err := WaitForLocalPort(host, port, 15*time.Second); err != nil {
		_ = KillPID(pid)
		return nil, fmt.Errorf("code server did not become ready on %s:%d: %w (see %s)", host, port, err, logPath)
	}

	return &StartResult{Port: port, Pid: pid, LogPath: logPath}, nil
}

// KillPID terminates a process group started with Setsid.
func KillPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := killProcessGroup(pid, false); err != nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = killProcessGroup(pid, true)
	_ = proc.Kill()
	return nil
}

// WaitForLocalPort polls until host:port accepts TCP connections.
func WaitForLocalPort(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if host == "0.0.0.0" || host == "::" {
		addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		last = err
		time.Sleep(150 * time.Millisecond)
	}
	if last == nil {
		last = errors.New("timeout")
	}
	return last
}

func isExecutableFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func codeCLISupportsServeWeb(bin string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "serve-web", "--help")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true
	}
	lower := strings.ToLower(string(out))
	return strings.Contains(lower, "serve-web") || strings.Contains(lower, "without-connection-token")
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
