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

	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/confirm"
	"github.com/sp3esu/mac-cleaner/internal/interactive"
	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/pkg/appleftovers"
	"github.com/sp3esu/mac-cleaner/pkg/browser"
	"github.com/sp3esu/mac-cleaner/pkg/creative"
	"github.com/sp3esu/mac-cleaner/pkg/developer"
	"github.com/sp3esu/mac-cleaner/pkg/system"
)

// version is set via ldflags at build time:
//
//	go build -ldflags "-X github.com/sp3esu/mac-cleaner/cmd.version=0.1.0"
var version = "dev"

var (
	flagDryRun       bool
	flagSystemCaches bool
	flagBrowserData  bool
	flagDevCaches    bool
	flagAppLeftovers bool
	flagCreativeCaches bool
	flagAll            bool
	flagJSON           bool
	flagVerbose      bool
	flagForce        bool
)

// Category-level skip flags prevent entire scanner groups from running.
var (
	flagSkipSystemCaches bool
	flagSkipBrowserData  bool
	flagSkipDevCaches    bool
	flagSkipAppLeftovers   bool
	flagSkipCreativeCaches bool
)

// Item-level skip flags filter specific categories from scan results.
var (
	flagSkipDerivedData   bool
	flagSkipNpm           bool
	flagSkipYarn          bool
	flagSkipHomebrew      bool
	flagSkipDocker        bool
	flagSkipSafari        bool
	flagSkipChrome        bool
	flagSkipFirefox       bool
	flagSkipQuicklook     bool
	flagSkipOrphanedPrefs bool
	flagSkipIosBackups    bool
	flagSkipOldDownloads      bool
	flagSkipSimulatorCaches   bool
	flagSkipSimulatorLogs     bool
	flagSkipXcodeDevSupport   bool
	flagSkipXcodeArchives     bool
	flagSkipAdobe             bool
	flagSkipAdobeMedia        bool
	flagSkipSketch            bool
	flagSkipFigma             bool
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
		if flagCreativeCaches {
			allResults = append(allResults, runCreativeCachesScan(cmd)...)
			ran = true
		}

		if flagJSON && !ran {
			fmt.Fprintln(os.Stderr, "Error: --json requires --all or a scan flag (--system-caches, --browser-data, --dev-caches, --app-leftovers, --creative-caches)")
			os.Exit(1)
		}

		if !ran {
			allResults = scanAll()
			// Apply item-level skip filtering in interactive mode.
			allResults = filterSkipped(allResults, buildSkipSet())
			printPermissionIssues(allResults)
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

			if !flagForce {
				if !confirm.PromptConfirmation(reader, os.Stdout, marked) {
					fmt.Println("Aborted.")
					return
				}
			}
			result := cleanup.Execute(marked)
			printCleanupSummary(result)
			return
		}

		// Apply item-level skip filtering.
		allResults = filterSkipped(allResults, buildSkipSet())

		if !flagJSON {
			printPermissionIssues(allResults)
		}

		if flagJSON {
			printJSON(allResults)
			if flagDryRun {
				return
			}
		}

		// Deletion flow: only when not in dry-run mode and there are results.
		if !flagDryRun && len(allResults) > 0 {
			if !flagForce {
				if !confirm.PromptConfirmation(os.Stdin, os.Stdout, allResults) {
					fmt.Println("Aborted.")
					return
				}
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
	rootCmd.Flags().BoolVar(&flagCreativeCaches, "creative-caches", false, "scan Adobe, Sketch, and Figma caches")
	rootCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "output results as JSON")
	rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "show detailed file listing")
	rootCmd.Flags().BoolVar(&flagForce, "force", false, "bypass confirmation prompt (for automation)")

	// Category-level skip flags.
	rootCmd.Flags().BoolVar(&flagSkipSystemCaches, "skip-system-caches", false, "skip system cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipBrowserData, "skip-browser-data", false, "skip browser data scanning")
	rootCmd.Flags().BoolVar(&flagSkipDevCaches, "skip-dev-caches", false, "skip developer cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipAppLeftovers, "skip-app-leftovers", false, "skip app leftover scanning")
	rootCmd.Flags().BoolVar(&flagSkipCreativeCaches, "skip-creative-caches", false, "skip creative app cache scanning")

	// Item-level skip flags.
	rootCmd.Flags().BoolVar(&flagSkipDerivedData, "skip-derived-data", false, "skip Xcode DerivedData")
	rootCmd.Flags().BoolVar(&flagSkipNpm, "skip-npm", false, "skip npm cache")
	rootCmd.Flags().BoolVar(&flagSkipYarn, "skip-yarn", false, "skip Yarn cache")
	rootCmd.Flags().BoolVar(&flagSkipHomebrew, "skip-homebrew", false, "skip Homebrew cache")
	rootCmd.Flags().BoolVar(&flagSkipDocker, "skip-docker", false, "skip Docker reclaimable space")
	rootCmd.Flags().BoolVar(&flagSkipSafari, "skip-safari", false, "skip Safari cache")
	rootCmd.Flags().BoolVar(&flagSkipChrome, "skip-chrome", false, "skip Chrome cache")
	rootCmd.Flags().BoolVar(&flagSkipFirefox, "skip-firefox", false, "skip Firefox cache")
	rootCmd.Flags().BoolVar(&flagSkipQuicklook, "skip-quicklook", false, "skip QuickLook thumbnails")
	rootCmd.Flags().BoolVar(&flagSkipOrphanedPrefs, "skip-orphaned-prefs", false, "skip orphaned preferences")
	rootCmd.Flags().BoolVar(&flagSkipIosBackups, "skip-ios-backups", false, "skip iOS device backups")
	rootCmd.Flags().BoolVar(&flagSkipOldDownloads, "skip-old-downloads", false, "skip old Downloads files")
	rootCmd.Flags().BoolVar(&flagSkipSimulatorCaches, "skip-simulator-caches", false, "skip iOS Simulator caches")
	rootCmd.Flags().BoolVar(&flagSkipSimulatorLogs, "skip-simulator-logs", false, "skip iOS Simulator logs")
	rootCmd.Flags().BoolVar(&flagSkipXcodeDevSupport, "skip-xcode-device-support", false, "skip Xcode Device Support files")
	rootCmd.Flags().BoolVar(&flagSkipXcodeArchives, "skip-xcode-archives", false, "skip Xcode Archives")
	rootCmd.Flags().BoolVar(&flagSkipAdobe, "skip-adobe", false, "skip Adobe caches")
	rootCmd.Flags().BoolVar(&flagSkipAdobeMedia, "skip-adobe-media", false, "skip Adobe media caches")
	rootCmd.Flags().BoolVar(&flagSkipSketch, "skip-sketch", false, "skip Sketch cache")
	rootCmd.Flags().BoolVar(&flagSkipFigma, "skip-figma", false, "skip Figma cache")

	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if flagAll {
			flagSystemCaches = true
			flagBrowserData = true
			flagDevCaches = true
			flagAppLeftovers = true
			flagCreativeCaches = true
		}
		// Apply category-level skip overrides (after --all expansion).
		if flagSkipSystemCaches {
			flagSystemCaches = false
		}
		if flagSkipBrowserData {
			flagBrowserData = false
		}
		if flagSkipDevCaches {
			flagDevCaches = false
		}
		if flagSkipAppLeftovers {
			flagAppLeftovers = false
		}
		if flagSkipCreativeCaches {
			flagCreativeCaches = false
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

// runCreativeCachesScan executes the creative app cache scan and prints results.
func runCreativeCachesScan(cmd *cobra.Command) []scan.CategoryResult {
	results, err := creative.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, "Creative App Caches")
	}
	return results
}

// buildSkipSet collects category IDs that should be excluded from results
// based on item-level skip flags.
func buildSkipSet() map[string]bool {
	type skipMapping struct {
		flag       *bool
		categoryID string
	}
	mappings := []skipMapping{
		{&flagSkipDerivedData, "dev-xcode"},
		{&flagSkipNpm, "dev-npm"},
		{&flagSkipYarn, "dev-yarn"},
		{&flagSkipHomebrew, "dev-homebrew"},
		{&flagSkipDocker, "dev-docker"},
		{&flagSkipSafari, "browser-safari"},
		{&flagSkipChrome, "browser-chrome"},
		{&flagSkipFirefox, "browser-firefox"},
		{&flagSkipQuicklook, "quicklook"},
		{&flagSkipOrphanedPrefs, "app-orphaned-prefs"},
		{&flagSkipIosBackups, "app-ios-backups"},
		{&flagSkipOldDownloads, "app-old-downloads"},
		{&flagSkipSimulatorCaches, "dev-simulator-caches"},
		{&flagSkipSimulatorLogs, "dev-simulator-logs"},
		{&flagSkipXcodeDevSupport, "dev-xcode-device-support"},
		{&flagSkipXcodeArchives, "dev-xcode-archives"},
		{&flagSkipAdobe, "creative-adobe"},
		{&flagSkipAdobeMedia, "creative-adobe-media"},
		{&flagSkipSketch, "creative-sketch"},
		{&flagSkipFigma, "creative-figma"},
	}
	skip := map[string]bool{}
	for _, m := range mappings {
		if *m.flag {
			skip[m.categoryID] = true
		}
	}
	return skip
}

// filterSkipped removes categories matching the skip set from results.
func filterSkipped(results []scan.CategoryResult, skip map[string]bool) []scan.CategoryResult {
	if len(skip) == 0 {
		return results
	}
	var filtered []scan.CategoryResult
	for _, cat := range results {
		if !skip[cat.Category] {
			filtered = append(filtered, cat)
		}
	}
	return filtered
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

	if results, err := creative.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else if len(results) > 0 {
		printResults(results, true, "Creative App Caches")
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
	var permIssues []scan.PermissionIssue
	for _, cat := range results {
		permIssues = append(permIssues, cat.PermissionIssues...)
	}
	summary := scan.ScanSummary{
		Categories:       results,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
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
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	// Header
	header := title
	if dryRun {
		header += " (dry run)"
	}
	fmt.Println()
	bold.Println(header)

	var grandTotal int64

	for _, cat := range results {
		if len(cat.Entries) == 0 {
			continue
		}

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
			riskTag := ""
			switch entry.RiskLevel {
			case safety.RiskRisky:
				riskTag = red.Sprint("  [risky]")
			case safety.RiskModerate:
				riskTag = yellow.Sprint("  [moderate]")
			}
			fmt.Fprintf(w, "    %s%s\t  %s\t\n", entry.Description, riskTag, cyan.Sprint(sizeStr))
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

// printPermissionIssues collects permission issues from all categories
// and prints them to stderr as a warning.
func printPermissionIssues(results []scan.CategoryResult) {
	var issues []scan.PermissionIssue
	for _, cat := range results {
		issues = append(issues, cat.PermissionIssues...)
	}
	if len(issues) == 0 {
		return
	}
	home, _ := os.UserHomeDir()
	yellow := color.New(color.FgYellow)
	fmt.Fprintln(os.Stderr)
	yellow.Fprintf(os.Stderr, "Note: %d path(s) could not be accessed (permission denied):\n", len(issues))
	for _, issue := range issues {
		path := shortenHome(issue.Path, home)
		fmt.Fprintf(os.Stderr, "  %s — %s\n", path, issue.Description)
	}
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
