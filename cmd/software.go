package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(softwareCmd)
	softwareCmd.AddCommand(softwareListCmd)
	softwareCmd.AddCommand(softwareInstallCmd)

	softwareListCmd.Flags().Bool("all", false, "include inactive softwares")
	softwareListCmd.Flags().Bool("json", false, "print JSON instead of a table")

	softwareInstallCmd.Flags().String("version", "", "install a specific version (default: latest)")
	softwareInstallCmd.Flags().Bool("dry-run", false, "print the install script without running it")
}

var softwareCmd = &cobra.Command{
	Use:     "software",
	Aliases: []string{"softwares", "sw"},
	Short:   "Software catalog utility",
	Long: `List and install softwares from the Container Workspace catalog.

Examples:
  cws software list
  cws software install Go
  cws software install docker --version 29.6.2
  cws software status "Docker Engine"
  cws software restart novnc
  cws software stop docker
  cws software start "XFCE + noVNC"
`,
	Args: cobra.NoArgs,
}
