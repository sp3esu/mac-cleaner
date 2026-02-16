package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gregor/mac-cleaner/internal/scan"
	"github.com/gregor/mac-cleaner/pkg/appleftovers"
	"github.com/gregor/mac-cleaner/pkg/browser"
	"github.com/gregor/mac-cleaner/pkg/developer"
	"github.com/gregor/mac-cleaner/pkg/system"
)

// version is set via ldflags at build time:
//
//	go build -ldflags "-X github.com/gregor/mac-cleaner/cmd.version=0.1.0"
var version = "dev"

var (
	flagDryRun       bool
	flagSystemCaches bool
	flagBrowserData  bool
	flagDevCaches    bool
	flagAppLeftovers bool
)

var rootCmd = &cobra.Command{
	Use:   "mac-cleaner",
	Short: "scan and remove macOS junk files",
	Long:  "scan and remove system caches, browser data, developer caches, and app leftovers",
	Run: func(cmd *cobra.Command, args []string) {
		ran := false
		if flagSystemCaches {
			runSystemCachesScan(cmd)
			ran = true
		}
		if flagBrowserData {
			runBrowserDataScan(cmd)
			ran = true
		}
		if flagDevCaches {
			runDevCachesScan(cmd)
			ran = true
		}
		if flagAppLeftovers {
			runAppLeftoversScan(cmd)
			ran = true
		}
		if !ran {
			cmd.Help()
		}
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "preview what would be removed without deleting")
	rootCmd.Flags().BoolVar(&flagSystemCaches, "system-caches", false, "scan user app caches, logs, and QuickLook thumbnails")
	rootCmd.Flags().BoolVar(&flagBrowserData, "browser-data", false, "scan Safari, Chrome, and Firefox caches")
	rootCmd.Flags().BoolVar(&flagDevCaches, "dev-caches", false, "scan Xcode, npm/yarn, Homebrew, and Docker caches")
	rootCmd.Flags().BoolVar(&flagAppLeftovers, "app-leftovers", false, "scan orphaned preferences, iOS backups, and old Downloads")
}

// Execute runs the root command. Errors are printed to stderr.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runSystemCachesScan executes the system cache scan and prints results.
func runSystemCachesScan(cmd *cobra.Command) {
	results, err := system.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	printResults(results, flagDryRun, "System Caches")
}

// runBrowserDataScan executes the browser data scan and prints results.
func runBrowserDataScan(cmd *cobra.Command) {
	results, err := browser.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	printResults(results, flagDryRun, "Browser Data")
}

// runDevCachesScan executes the developer cache scan and prints results.
func runDevCachesScan(cmd *cobra.Command) {
	results, err := developer.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	printResults(results, flagDryRun, "Developer Caches")
}

// runAppLeftoversScan executes the app leftovers scan and prints results.
func runAppLeftoversScan(cmd *cobra.Command) {
	results, err := appleftovers.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	printResults(results, flagDryRun, "App Leftovers")
}

// printResults displays scan results as a formatted table with color.
func printResults(results []scan.CategoryResult, dryRun bool, title string) {
	if len(results) == 0 {
		fmt.Printf("No %s found.\n", strings.ToLower(title))
		return
	}

	home, _ := os.UserHomeDir()

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	greenBold := color.New(color.FgGreen, color.Bold)

	// Header
	header := title
	if dryRun {
		header += " (dry run)"
	}
	fmt.Println()
	bold.Println(header)

	var grandTotal int64

	for _, cat := range results {
		fmt.Println()

		// Category header with base directory path.
		catHeader := "  " + cat.Description
		if len(cat.Entries) > 0 {
			baseDir := shortenHome(baseDirectory(cat.Entries[0].Path), home)
			catHeader += "    " + baseDir
		}
		bold.Println(catHeader)

		// Entries in a tabwriter for alignment.
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		for _, entry := range cat.Entries {
			sizeStr := scan.FormatSize(entry.Size)
			fmt.Fprintf(w, "    %s\t  %s\t\n", entry.Description, cyan.Sprint(sizeStr))
		}
		w.Flush()

		grandTotal += cat.TotalSize
	}

	// Summary line.
	fmt.Println()
	greenBold.Printf("  Total: %s reclaimable\n", scan.FormatSize(grandTotal))
	fmt.Println()
}

// shortenHome replaces the home directory prefix with ~ for display.
func shortenHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// baseDirectory returns the parent directory of a path.
func baseDirectory(path string) string {
	return filepath.Dir(path)
}
