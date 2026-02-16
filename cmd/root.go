package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set via ldflags at build time:
//
//	go build -ldflags "-X github.com/gregor/mac-cleaner/cmd.version=0.1.0"
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mac-cleaner",
	Short: "scan and remove macOS junk files",
	Long:  "scan and remove system caches, browser data, developer caches, and app leftovers",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// Execute runs the root command. Errors are printed to stderr.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
