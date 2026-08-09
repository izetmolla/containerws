package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/izetmolla/containerws/packages/hostsetup"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().Bool("no-start", false, "install dirs, cws link, and daemon but do not start yet")
	setupCmd.Flags().Bool("uninstall", false, "stop daemon and remove cws links / service units")
	setupCmd.Flags().String("binary", "", "path to containerws binary (default: this executable)")
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install cws symlink, host dirs, and background daemon (cws --start)",
	Long: `Prepare this host after installing Container Workspace (e.g. via Homebrew).

Mirrors the native installer (install/install.sh) post-binary steps:
  • create /config/containerws and /etc/containerws
  • link cws (and containerws) into /usr/local/bin (and Homebrew prefix when present)
  • install an OS daemon (systemd / launchd / OpenRC / direct) that runs: cws --start
  • start the daemon in the background (unless --no-start)

Requires root (re-executes with sudo when needed).

Examples:
  sudo containerws setup
  containerws setup            # will prompt via sudo
  containerws setup --no-start
  sudo containerws setup --uninstall
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		noStart, _ := cmd.Flags().GetBool("no-start")
		uninstall, _ := cmd.Flags().GetBool("uninstall")
		binary, _ := cmd.Flags().GetString("binary")

		if hostsetup.NeedRoot() {
			return reexecSetupWithSudo(cmd, noStart, uninstall, binary)
		}

		res, err := hostsetup.Run(hostsetup.Options{
			NoStart:   noStart,
			Uninstall: uninstall,
			Binary:    strings.TrimSpace(binary),
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), res.Message)
		if !uninstall {
			fmt.Fprintf(cmd.OutOrStdout(), `
  Binary:   %s
  CLI:      %s
  Config:   %s
  Env file: %s
  Daemon:   %s → cws --start
  Web UI:   http://127.0.0.1:9000

`, res.Binary, hostsetup.CLIPath, hostsetup.ConfigRoot, hostsetup.EnvFile, res.InitSystem)
			if !res.Started && !noStart {
				fmt.Fprintln(cmd.OutOrStdout(), "warning: daemon did not stay up; check logs / systemctl status containerws")
			}
		}
		return nil
	},
}

func reexecSetupWithSudo(cmd *cobra.Command, noStart, uninstall bool, binary string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("root required: re-run as root or install sudo, then: sudo %s setup", exe)
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Root required — re-executing with sudo…")
	args := []string{"-E", exe, "setup"}
	if noStart {
		args = append(args, "--no-start")
	}
	if uninstall {
		args = append(args, "--uninstall")
	}
	if b := strings.TrimSpace(binary); b != "" {
		args = append(args, "--binary", b)
	}
	c := exec.Command("sudo", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	return nil
}
