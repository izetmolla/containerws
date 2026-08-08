package cmd

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

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/codeserver"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func init() {
	rootCmd.AddCommand(csCmd)
	csCmd.AddCommand(csStartCmd)
	csCmd.AddCommand(csStopCmd)
	csCmd.AddCommand(csStatusCmd)

	csStartCmd.Flags().String("path", "", "folder to open (default: cwd or positional arg)")
	csStartCmd.Flags().String("host", adduser.BindHost, "listen address (default 127.0.0.1)")
	csStartCmd.Flags().String("token", "none", "connection token: none | auto | <secret>")
	csStartCmd.Flags().Bool("restart", true, "restart if an active session already exists for this user")
}

var csCmd = &cobra.Command{
	Use:     "cs",
	Aliases: []string{"codeserver", "code-server"},
	Short:   "VS Code Server (serve-web) for a folder",
	Long: `Start or stop a Microsoft VS Code Server (code serve-web) instance.

Picks an unused localhost port, runs in the background, and stores the
session (path, user, address, port) in the database.

Examples:
  cws cs start /workspace
  cws cs start .
  cws cs stop
  cws cs status
`,
	Args: cobra.NoArgs,
}

var csStartCmd = &cobra.Command{
	Use:   "start [path]",
	Short: "Start VS Code Server in the background on a free port",
	Args:  cobra.MaximumNArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		codeBin, err := requireCodeCLIInstalled()
		if err != nil {
			return err
		}
		db := appClients.DB()

		pathFlag, _ := cmd.Flags().GetString("path")
		host, _ := cmd.Flags().GetString("host")
		token, _ := cmd.Flags().GetString("token")
		restart, _ := cmd.Flags().GetBool("restart")

		folder := strings.TrimSpace(pathFlag)
		if folder == "" && len(args) > 0 {
			folder = strings.TrimSpace(args[0])
		}
		if folder == "" {
			folder, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		folder, err = filepath.Abs(folder)
		if err != nil {
			return err
		}
		if st, err := os.Stat(folder); err != nil || !st.IsDir() {
			return fmt.Errorf("folder not found: %s", folder)
		}

		host = strings.TrimSpace(host)
		if host == "" {
			host = adduser.BindHost
		}

		dbUser, linuxName, err := resolveCodeserverUser(db)
		if err != nil {
			return err
		}

		existing, err := loadOrCreateCodeserverSession(db, dbUser.ID, folder, host)
		if err != nil {
			return err
		}
		if strings.EqualFold(existing.Status, models.CodeserverSessionStatusActive) {
			if isCodeserverLive(*existing) && !restart {
				printCodeserverSession(existing)
				fmt.Println("already running (pass --restart to replace)")
				return nil
			}
			_ = stopCodeserverProcess(existing)
		}
		sessionID := existing.ID

		used := codeserverUsedPorts(db)
		port, err := adduser.PickUnusedLocalPort(used)
		if err != nil {
			return fmt.Errorf("allocate port: %w", err)
		}

		home := os.Getenv("HOME")
		if home == "" {
			if u, err := user.Current(); err == nil && u.HomeDir != "" {
				home = u.HomeDir
			} else {
				home = "/root"
			}
		}
		dataDir := filepath.Join(home, ".vscode-server-web")
		logDir := filepath.Join(dataDir, "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return err
		}
		logPath := filepath.Join(logDir, fmt.Sprintf("serve-web-%d.log", port))
		pidPath := filepath.Join(dataDir, fmt.Sprintf("serve-web-%d.pid", port))

		// Upstream serves at /. The Fiber proxy strips /codeserver/<uuid> and
		// sets X-Forwarded-Prefix so the workbench keeps public URLs under that path.
		// --default-folder is required: proc.Dir alone does not open the workspace.
		argsCLI := []string{
			"serve-web",
			"--accept-server-license-terms",
			"--host", host,
			"--port", strconv.Itoa(port),
			"--default-folder", folder,
			"--server-data-dir", dataDir,
			"--cli-data-dir", dataDir,
		}
		switch strings.TrimSpace(token) {
		case "", "none":
			argsCLI = append(argsCLI, "--without-connection-token")
		case "auto":
			// CLI generates a token and prints it in the URL / log.
		default:
			argsCLI = append(argsCLI, "--connection-token", token)
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		defer logFile.Close()

		proc := exec.Command(codeBin, argsCLI...)
		proc.Dir = folder
		proc.Stdout = logFile
		proc.Stderr = logFile
		proc.Env = append(os.Environ(),
			"HOME="+home,
			"USER="+linuxName,
		)
		proc.SysProcAttr = codeserver.DetachSysProcAttr()

		if err := proc.Start(); err != nil {
			return fmt.Errorf("start code serve-web: %w", err)
		}
		pid := proc.Process.Pid
		_ = os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644)
		// Detach: release child so parent exit does not wait.
		_ = proc.Process.Release()

		if err := waitForLocalPort(host, port, 15*time.Second); err != nil {
			_ = codeserver.KillPID(pid)
			return fmt.Errorf("code server did not become ready on %s:%d: %w (see %s)", host, port, err, logPath)
		}

		if err := db.Unscoped().Model(existing).Updates(map[string]any{
			"deleted_at": nil,
			"status":     models.CodeserverSessionStatusActive,
			"path":       folder,
			"address":    host,
			"port":       port,
			"pid":        pid,
		}).Error; err != nil {
			_ = codeserver.KillPID(pid)
			return fmt.Errorf("update session: %w", err)
		}

		session := models.CodeserverSession{
			ID:      sessionID,
			UserID:  dbUser.ID,
			Status:  models.CodeserverSessionStatusActive,
			Path:    folder,
			Address: host,
			Port:    port,
			Pid:     pid,
		}

		printCodeserverSession(&session)
		return nil
	}),
}

var csStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current user's VS Code Server instance",
	Args:  cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, appClients *config.AppClients) error {
		db := appClients.DB()

		dbUser, linuxName, err := resolveCodeserverUser(db)
		if err != nil {
			return err
		}

		var session models.CodeserverSession
		if err := db.Where("user_id = ?", dbUser.ID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("no codeserver session for linux user %q", linuxName)
			}
			return err
		}

		if err := stopCodeserverProcess(&session); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}

		if err := db.Model(&session).Updates(map[string]any{
			"status": models.CodeserverSessionStatusInactive,
			"pid":    0,
		}).Error; err != nil {
			return err
		}

		fmt.Printf("Stopped VS Code Server for %s (was %s)\n", linuxName, session.UpstreamBaseURL())
		return nil
	}),
}

var csStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"show", "info"},
	Short:   "Show the current user's VS Code Server session",
	Args:    cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, appClients *config.AppClients) error {
		db := appClients.DB()

		dbUser, linuxName, err := resolveCodeserverUser(db)
		if err != nil {
			return err
		}

		var session models.CodeserverSession
		if err := db.Where("user_id = ?", dbUser.ID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				fmt.Printf("No codeserver session for linux user %q\n", linuxName)
				return nil
			}
			return err
		}

		live := isCodeserverLive(session)
		printCodeserverSession(&session)
		if live {
			fmt.Println("State: live")
		} else {
			fmt.Println("State: not listening")
		}
		return nil
	}),
}

func resolveCodeserverUser(db *gorm.DB) (*models.User, string, error) {
	cu, err := user.Current()
	if err != nil {
		return nil, "", fmt.Errorf("current linux user: %w", err)
	}
	linuxName := strings.TrimSpace(cu.Username)
	if linuxName == "" {
		return nil, "", errors.New("current linux username is empty")
	}

	var u models.User
	err = db.Where("username = ? OR ldap_username = ?", linuxName, linuxName).First(&u).Error
	if err == nil {
		return &u, linuxName, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, linuxName, err
	}

	return nil, linuxName, fmt.Errorf(
		"no panel user with username %q — create one with: cws users add --username %s ...",
		linuxName, linuxName,
	)
}

// loadOrCreateCodeserverSession returns the per-user session row, restoring a
// soft-deleted row when present so the unique user_id index does not block
// create and the admin /vscode list can show the session again.
func loadOrCreateCodeserverSession(db *gorm.DB, userID, folder, host string) (*models.CodeserverSession, error) {
	var existing models.CodeserverSession
	err := db.Unscoped().Where("user_id = ?", userID).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			if err := db.Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
				return nil, fmt.Errorf("restore session: %w", err)
			}
			existing.DeletedAt = gorm.DeletedAt{}
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	row := models.CodeserverSession{
		UserID:  userID,
		Status:  models.CodeserverSessionStatusInactive,
		Path:    folder,
		Address: host,
	}
	if err := db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &row, nil
}

func lookupCodeCLI() (string, error) {
	candidates := []string{
		"/usr/local/lib/vscode-cli/code",
		"/usr/local/bin/code-cli",
	}
	for _, c := range candidates {
		if isExecutableFile(c) {
			return c, nil
		}
	}
	if p, err := exec.LookPath("code-cli"); err == nil && isExecutableFile(p) {
		return p, nil
	}
	// Prefer the Microsoft CLI over a desktop "code" binary that may lack serve-web.
	if p, err := exec.LookPath("code"); err == nil && isExecutableFile(p) {
		return p, nil
	}
	return "", errCodeCLINotInstalled
}

// requireCodeCLIInstalled ensures the Microsoft VS Code CLI (serve-web) is present
// before allocating ports or writing session rows.
func requireCodeCLIInstalled() (string, error) {
	bin, err := lookupCodeCLI()
	if err != nil {
		return "", err
	}
	if !codeCLISupportsServeWeb(bin) {
		return "", fmt.Errorf(
			"%w\nfound %q but it does not support `serve-web` — install the panel software \"VS Code Server\":\n  cws software install \"VS Code Server\"",
			errCodeCLINotInstalled, bin,
		)
	}
	return bin, nil
}

var errCodeCLINotInstalled = errors.New(
	`VS Code Server is not installed

Install it from the panel Softwares page, or:
  cws software install "VS Code Server"

Expected binary: /usr/local/lib/vscode-cli/code (or code-cli on PATH)`,
)

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
	// Some builds exit non-zero on --help but still print usage.
	lower := strings.ToLower(string(out))
	return strings.Contains(lower, "serve-web") || strings.Contains(lower, "without-connection-token")
}

func codeserverUsedPorts(db *gorm.DB) map[int]struct{} {
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

func isCodeserverLive(s models.CodeserverSession) bool {
	if s.Port > 0 && adduser.IsLocalPortListening(s.Port) {
		return true
	}
	return s.Pid > 0 && processAlive(s.Pid)
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

func stopCodeserverProcess(s *models.CodeserverSession) error {
	var errs []string
	if s.Pid > 0 {
		if err := codeserver.KillPID(s.Pid); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if s.Port > 0 && adduser.IsLocalPortListening(s.Port) {
		// Best-effort: kill whatever still holds the port.
		_ = exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", s.Port)).Run()
		time.Sleep(200 * time.Millisecond)
	}
	if len(errs) > 0 && adduser.IsLocalPortListening(s.Port) {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func waitForLocalPort(host string, port int, timeout time.Duration) error {
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

func printCodeserverSession(s *models.CodeserverSession) {
	fmt.Println(codeserver.PublicClientURLForFolder(s.ID, s.Path))
}
