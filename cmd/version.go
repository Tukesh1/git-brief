package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is injected at build time via:
//
//	go build -ldflags="-X github.com/tukesh1/git-brief/cmd.Version=v1.2.3"
//
// It defaults to "dev" for local builds.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of git-brief",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("git-brief %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	// Also support the conventional --version flag on the root command.
	rootCmd.Version = Version
}
