package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gregor/mac-cleaner/internal/cleanup"
	"github.com/gregor/mac-cleaner/internal/confirm"
	"github.com/gregor/mac-cleaner/internal/interactive"
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
	flagAll          bool
	flagJSON         bool
	flagVerbose      bool
)

var rootCmd = &cobra.Command{
	Use:   "mac-cleaner",
	Short: "scan and remove macOS junk files",
	Long:  "scan and remove system caches, browser data, developer caches, and app leftovers",
	Run: func(cmd *cobra.Command, args []string) {
		ran := false
		var allResults []scan.CategoryResult

		if flagSystemCaches {
			allResults = append(allResults, runSystemCachesScan(cmd)...)
			ran = true
		}
		if flagBrowserData {
			allResults = append(allResults, runBrowserDataScan(cmd)...)
			ran = true
		}
		if flagDevCaches {
			allResults = append(allResults, runDevCachesScan(cmd)...)
			ran = true
		}
		if flagAppLeftovers {
			allResults = append(allResults, runAppLeftoversScan(cmd)...)
			ran = true
		}

		if flagJSON && !ran {
			fmt.Fprintln(os.Stderr, "Error: --json requires --all or a scan flag (--system-caches, --browser-data, --dev-caches, --app-leftovers)")
			os.Exit(1)
		}

		if !ran {
			allResults = scanAll()
			if len(allResults) == 0 {
				fmt.Println("Nothing to clean.")
				return
			}

			reader := bufio.NewReader(os.Stdin)
			marked := interactive.RunWalkthrough(reader, os.Stdout, allResults)
			if marked == nil {
				return
			}

			if flagDryRun {
				return
			}

			if !confirm.PromptConfirmation(reader, os.Stdout, marked) {
				fmt.Println("Aborted.")
				return
			}
			result := cleanup.Execute(marked)
			printCleanupSummary(result)
			return
		}

		if flagJSON {
			printJSON(allResults)
			if flagDryRun {
				return
			}
		}

		// Deletion flow: only when not in dry-run mode and there are results.
		if !flagDryRun && len(allResults) > 0 {
			if !confirm.PromptConfirmation(os.Stdin, os.Stdout, allResults) {
				fmt.Println("Aborted.")
				return
			}
			result := cleanup.Execute(allResults)
			printCleanupSummary(result)
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
	rootCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "output results as JSON")
	rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "show detailed file listing")

	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if flagAll {
			flagSystemCaches = true
			flagBrowserData = true
			flagDevCaches = true
			flagAppLeftovers = true
		}
		if flagJSON {
			color.NoColor = true
		}
	}
}

// Execute runs the root command. Errors are printed to stderr.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runSystemCachesScan executes the system cache scan and prints results.
func runSystemCachesScan(cmd *cobra.Command) []scan.CategoryResult {
	results, err := system.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, "System Caches")
	}
	return results
}

// runBrowserDataScan executes the browser data scan and prints results.
func runBrowserDataScan(cmd *cobra.Command) []scan.CategoryResult {
	results, err := browser.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, "Browser Data")
	}
	return results
}

// runDevCachesScan executes the developer cache scan and prints results.
func runDevCachesScan(cmd *cobra.Command) []scan.CategoryResult {
	results, err := developer.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, "Developer Caches")
	}
	return results
}

// runAppLeftoversScan executes the app leftovers scan and prints results.
func runAppLeftoversScan(cmd *cobra.Command) []scan.CategoryResult {
	results, err := appleftovers.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, "App Leftovers")
	}
	return results
}

// scanAll runs all four scanners and returns aggregated results.
// Scanner errors are logged to stderr; partial results are still returned.
// Results are printed with dryRun=true since interactive mode handles
// deletion decisions separately.
func scanAll() []scan.CategoryResult {
	var allResults []scan.CategoryResult

	if results, err := system.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if len(results) > 0 {
		printResults(results, true, "System Caches")
		allResults = append(allResults, results...)
	}

	if results, err := browser.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if len(results) > 0 {
		printResults(results, true, "Browser Data")
		allResults = append(allResults, results...)
	}

	if results, err := developer.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if len(results) > 0 {
		printResults(results, true, "Developer Caches")
		allResults = append(allResults, results...)
	}

	if results, err := appleftovers.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if len(results) > 0 {
		printResults(results, true, "App Leftovers")
		allResults = append(allResults, results...)
	}

	return allResults
}

// printCleanupSummary displays the results of a cleanup operation.
func printCleanupSummary(result cleanup.CleanupResult) {
	greenBold := color.New(color.FgGreen, color.Bold)
	fmt.Println()
	greenBold.Printf("Cleanup complete: %d items removed, %s freed\n",
		result.Removed, scan.FormatSize(result.BytesFreed))
	if result.Failed > 0 {
		yellow := color.New(color.FgYellow)
		yellow.Printf("%d items failed (see warnings above)\n", result.Failed)
	}
	fmt.Println()
}

// printJSON outputs scan results as formatted JSON to stdout.
func printJSON(results []scan.CategoryResult) {
	var totalSize int64
	for _, cat := range results {
		totalSize += cat.TotalSize
	}
	summary := scan.ScanSummary{
		Categories: results,
		TotalSize:  totalSize,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
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
			if flagVerbose {
				path := shortenHome(entry.Path, home)
				fmt.Fprintf(w, "      %s\t\t\n", path)
			}
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
