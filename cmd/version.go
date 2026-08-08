package cmd

import (
	"fmt"

	"github.com/izetmolla/containerws/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Container Workspace v" + version.Version + "/" + version.CommitSHA)
	},
}
