package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(vncCmd)
	vncCmd.AddCommand(vncPasswordCmd)
	vncCmd.AddCommand(vncShowCmd)

	vncPasswordCmd.Flags().StringP("password", "p", "", "new VNC password (prompted if omitted)")
	vncPasswordCmd.Flags().Bool("restart", true, "restart containerws-vnc / containerws-novnc after updating")
	vncPasswordCmd.Flags().Bool("generate", false, "generate a random password instead of prompting")
}

var vncCmd = &cobra.Command{
	Use:   "vnc",
	Short: "VNC / noVNC utilities",
	Long: `Manage the TigerVNC / noVNC desktop password and related helpers.

Examples:
  cws vnc password
  cws vnc password 'MyNewPass'
  cws vnc password --password 'MyNewPass'
  cws vnc password --generate
  cws vnc show
`,
	Args: cobra.NoArgs,
}

var vncPasswordCmd = &cobra.Command{
	Use:     "password [new-password]",
	Aliases: []string{"passwd", "reset-password", "set-password"},
	Short:   "Set or reset the VNC / noVNC password",
	Long: `Update the VNC password used by noVNC (browser desktop).

Writes:
  /config/containerws/vnc.pass
  /etc/containerws/novnc_password
  ~/.config/tigervnc/passwd  (TigerVNC auth file)

Then restarts containerws-vnc and containerws-novnc unless --restart=false.
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		passFlag, _ := cmd.Flags().GetString("password")
		restart, _ := cmd.Flags().GetBool("restart")
		generate, _ := cmd.Flags().GetBool("generate")

		var password string
		switch {
		case generate:
			out, err := exec.Command("openssl", "rand", "-hex", "8").Output()
			if err != nil {
				return fmt.Errorf("generate password: %w", err)
			}
			password = strings.TrimSpace(string(out))
		case strings.TrimSpace(passFlag) != "":
			password = passFlag
		case len(args) > 0 && strings.TrimSpace(args[0]) != "":
			password = args[0]
		default:
			var err error
			password, err = promptVNCPassword()
			if err != nil {
				return err
			}
		}

		password = strings.TrimSpace(password)
		if password == "" {
			return errors.New("password cannot be empty")
		}
		if len(password) > 8 {
			// TigerVNC historically truncates to 8 chars for VncAuth.
			fmt.Fprintln(os.Stderr, "note: TigerVNC VncAuth uses at most 8 characters; extra characters are ignored by many clients")
		}

		home := os.Getenv("HOME")
		if home == "" {
			home = "/root"
		}

		if err := writeVNCPasswordFiles(home, password); err != nil {
			return err
		}

		if restart {
			if err := restartVNCServices(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: password updated but service restart failed: %v\n", err)
				fmt.Fprintln(os.Stderr, "run: systemctl restart containerws-vnc containerws-novnc")
			} else {
				fmt.Println("Restarted containerws-vnc and containerws-novnc")
			}
		}

		fmt.Println("VNC password updated")
		fmt.Printf("Password: %s\n", password)
		fmt.Println("Connect via noVNC (default http://<host>:6080/) and use this password")
		return nil
	},
}

var vncShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the stored VNC password",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		pass, src, err := readStoredVNCPassword()
		if err != nil {
			return err
		}
		fmt.Printf("Password: %s\n", pass)
		fmt.Printf("Source:   %s\n", src)
		return nil
	},
}

func promptVNCPassword() (string, error) {
	fmt.Fprint(os.Stderr, "New VNC password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	pass := strings.TrimSpace(line)
	if pass == "" {
		return "", errors.New("password cannot be empty")
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	line2, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(line2) != pass {
		return "", errors.New("passwords do not match")
	}
	return pass, nil
}

func writeVNCPasswordFiles(home, password string) error {
	paths := []string{
		"/config/containerws/vnc.pass",
		"/etc/containerws/novnc_password",
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(password), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
	}

	tigervncDir := filepath.Join(home, ".config", "tigervnc")
	if err := os.MkdirAll(tigervncDir, 0o700); err != nil {
		return err
	}
	vncLink := filepath.Join(home, ".vnc")
	if fi, err := os.Lstat(vncLink); err == nil && fi.Mode()&os.ModeSymlink == 0 && fi.IsDir() {
		// Leave existing dir; also write legacy path below.
	} else {
		_ = os.RemoveAll(vncLink)
		_ = os.Symlink(tigervncDir, vncLink)
	}

	passwdPath := filepath.Join(tigervncDir, "passwd")
	if err := writeTigerVNCPasswd(passwdPath, password); err != nil {
		return err
	}
	// Legacy path if ~/.vnc is a real directory (older installs).
	legacy := filepath.Join(home, ".vnc", "passwd")
	if fi, err := os.Lstat(filepath.Join(home, ".vnc")); err == nil && fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
		_ = writeTigerVNCPasswd(legacy, password)
	}
	return nil
}

func writeTigerVNCPasswd(path, password string) error {
	vncpasswd, err := exec.LookPath("vncpasswd")
	if err != nil {
		return errors.New("vncpasswd not found; install TigerVNC / run: cws software install \"XFCE + noVNC\"")
	}
	cmd := exec.Command(vncpasswd, "-f")
	cmd.Stdin = strings.NewReader(password + "\n")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("vncpasswd -f: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func restartVNCServices() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemctl not available")
	}
	cmd := exec.Command("systemctl", "restart", "containerws-vnc", "containerws-novnc")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readStoredVNCPassword() (password, source string, err error) {
	candidates := []string{
		"/config/containerws/vnc.pass",
		"/etc/containerws/novnc_password",
	}
	for _, p := range candidates {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			continue
		}
		pass := strings.TrimSpace(string(b))
		if pass != "" {
			return pass, p, nil
		}
	}
	return "", "", errors.New("no stored VNC password found (expected /config/containerws/vnc.pass)")
}
