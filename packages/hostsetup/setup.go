package hostsetup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	BinName      = "containerws"
	CLIName      = "cws"
	InstallDir   = "/usr/local/lib/containerws"
	BinDir       = InstallDir + "/bin"
	ConfigRoot   = "/config/containerws"
	EtcDir       = "/etc/containerws"
	EnvFile      = EtcDir + "/environment"
	ServiceName  = "containerws"
	UnitPath     = "/etc/systemd/system/" + ServiceName + ".service"
	LaunchdLabel = "com.izetmolla.containerws"
	LaunchdPath  = "/Library/LaunchDaemons/" + LaunchdLabel + ".plist"
	OpenRCPath   = "/etc/init.d/" + ServiceName
	FreeBSDRC    = "/usr/local/etc/rc.d/" + ServiceName
	PIDFile      = "/var/run/containerws.pid"
	DaemonWrap   = BinDir + "/cws-daemon.sh"
	CronFile     = "/etc/cron.d/containerws"
	LogOut       = "/var/log/containerws/cws.out.log"
	LogErr       = "/var/log/containerws/cws.err.log"
	CLIPath      = "/usr/local/bin/" + CLIName
	AliasPath    = "/usr/local/bin/" + BinName
	RepoDefault  = "izetmolla/containerws"
)

// Options controls host setup / uninstall.
type Options struct {
	NoStart   bool
	Uninstall bool
	Binary    string // optional override; default: resolved os.Executable()
	Repo      string
}

// Result summarizes what setup did.
type Result struct {
	Binary     string
	InitSystem string
	CLILinks   []string
	Started    bool
	Message    string
}

// Run installs dirs, cws symlink, env file, and OS daemon (or uninstalls).
func Run(opts Options) (*Result, error) {
	if opts.Repo == "" {
		opts.Repo = RepoDefault
	}
	bin, err := resolveBinary(opts.Binary)
	if err != nil {
		return nil, err
	}
	initSys := DetectInit()

	if opts.Uninstall {
		if err := uninstall(initSys, bin); err != nil {
			return nil, err
		}
		return &Result{
			Binary:     bin,
			InitSystem: initSys,
			Message:    "Binaries links and daemon removed. Data kept in " + ConfigRoot + " and " + EtcDir,
		}, nil
	}

	if err := prepareDirs(); err != nil {
		return nil, err
	}
	if err := writeEnvFile(); err != nil {
		return nil, err
	}
	links, err := linkCLI(bin)
	if err != nil {
		return nil, err
	}
	if err := installDaemon(initSys, bin, opts); err != nil {
		return nil, err
	}
	started := false
	if !opts.NoStart {
		started = verifyDaemonStarted(initSys)
	}
	msg := fmt.Sprintf("CLI linked (%s), daemon=%s", strings.Join(links, ", "), initSys)
	if started {
		msg += "; cws --start is running"
	} else if opts.NoStart {
		msg += "; daemon installed but not started (--no-start)"
	}
	return &Result{
		Binary:     bin,
		InitSystem: initSys,
		CLILinks:   links,
		Started:    started,
		Message:    msg,
	}, nil
}

func resolveBinary(override string) (string, error) {
	if s := strings.TrimSpace(override); s != "" {
		abs, err := filepath.Abs(s)
		if err != nil {
			return "", err
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			return "", fmt.Errorf("binary not found: %s", abs)
		}
		return abs, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil && resolved != "" {
		exe = resolved
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func prepareDirs() error {
	dirs := []string{
		BinDir,
		"/usr/local/bin",
		ConfigRoot + "/database",
		ConfigRoot + "/ssl",
		ConfigRoot + "/vnc-sessions",
		EtcDir,
		"/var/lib/containerws",
		"/var/log/containerws",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func writeEnvFile() error {
	if _, err := os.Stat(EnvFile); err == nil {
		return nil
	}
	body := `# Container Workspace daemon environment
ENV=production
# DATABASE_URL=` + ConfigRoot + `/database/database.sqlite
# Optional: MCP_PORT=9100
# Optional: ENABLE_HTTPS=true
`
	if err := os.WriteFile(EnvFile, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", EnvFile, err)
	}
	return nil
}

func linkCLI(bin string) ([]string, error) {
	targets := []string{CLIPath}
	// Also expose "containerws" under /usr/local/bin when the live binary lives elsewhere (e.g. Homebrew).
	if filepath.Clean(bin) != filepath.Clean(AliasPath) {
		targets = append(targets, AliasPath)
	}
	if brewBin := brewPrefixBin(); brewBin != "" {
		cwsInBrew := filepath.Join(brewBin, CLIName)
		if filepath.Clean(cwsInBrew) != filepath.Clean(CLIPath) {
			targets = append(targets, cwsInBrew)
		}
	}

	linked := make([]string, 0, len(targets))
	seen := map[string]bool{}
	for _, t := range targets {
		t = filepath.Clean(t)
		if seen[t] {
			continue
		}
		seen[t] = true
		if err := os.MkdirAll(filepath.Dir(t), 0o755); err != nil {
			return linked, err
		}
		_ = os.Remove(t)
		if err := os.Symlink(bin, t); err != nil {
			return linked, fmt.Errorf("symlink %s → %s: %w", t, bin, err)
		}
		linked = append(linked, t)
	}
	return linked, nil
}

func brewPrefixBin() string {
	out, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		for _, p := range []string{
			"/home/linuxbrew/.linuxbrew/bin",
			"/opt/homebrew/bin",
			"/usr/local/Homebrew/bin",
		} {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
		return ""
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return ""
	}
	return filepath.Join(prefix, "bin")
}

// NeedRoot reports whether the process is not root (setup needs privileges on Unix).
func NeedRoot() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return os.Geteuid() != 0
}
