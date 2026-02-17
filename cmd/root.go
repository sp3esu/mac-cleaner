package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/confirm"
	"github.com/sp3esu/mac-cleaner/internal/engine"
	"github.com/sp3esu/mac-cleaner/internal/interactive"
	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/spinner"
)

// version is set via ldflags at build time:
//
//	go build -ldflags "-X github.com/sp3esu/mac-cleaner/cmd.version=0.1.0"
var version = "dev"

// eng is the scan/cleanup engine, initialized in PreRun.
var eng *engine.Engine

var (
	flagDryRun       bool
	flagSystemCaches bool
	flagBrowserData  bool
	flagDevCaches    bool
	flagAppLeftovers bool
	flagCreativeCaches  bool
	flagMessagingCaches bool
	flagUnusedApps      bool
	flagPhotos          bool
	flagSystemData      bool
	flagAll             bool
	flagJSON           bool
	flagVerbose      bool
	flagForce        bool
	flagHelpJSON     bool
)

// Category-level skip flags prevent entire scanner groups from running.
var (
	flagSkipSystemCaches bool
	flagSkipBrowserData  bool
	flagSkipDevCaches    bool
	flagSkipAppLeftovers   bool
	flagSkipCreativeCaches  bool
	flagSkipMessagingCaches bool
	flagSkipUnusedApps      bool
	flagSkipPhotos          bool
	flagSkipSystemData      bool
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
	flagSkipPnpm              bool
	flagSkipCocoapods         bool
	flagSkipGradle            bool
	flagSkipPip               bool
	flagSkipAdobe             bool
	flagSkipAdobeMedia        bool
	flagSkipSketch            bool
	flagSkipFigma             bool
	flagSkipSlack             bool
	flagSkipDiscord           bool
	flagSkipTeams             bool
	flagSkipZoom              bool
	flagSkipPhotosCaches      bool
	flagSkipPhotosAnalysis    bool
	flagSkipPhotosIcloudCache bool
	flagSkipPhotosSyndication bool
	flagSkipSpotlight        bool
	flagSkipMail             bool
	flagSkipMailDownloads    bool
	flagSkipMessages         bool
	flagSkipIOSUpdates       bool
	flagSkipTimemachine      bool
	flagSkipVMParallels      bool
	flagSkipVMUTM            bool
	flagSkipVMVMware         bool
)

// scannerMapping maps a CLI flag to a scanner ID in the engine.
type scannerMapping struct {
	flag      *bool
	scannerID string
}

var rootCmd = &cobra.Command{
	Use:   "mac-cleaner",
	Short: "scan and remove macOS junk files",
	Long: `Scan and remove system caches, browser data, developer caches, app leftovers,
photos caches, system data, and unused applications.

Without flags, enters interactive walkthrough mode. Use scan flags (--system-caches,
--dev-caches, etc.) with --all for a full non-interactive scan. Use the "scan"
subcommand for targeted item-level scanning (e.g. mac-cleaner scan --npm --safari).

Examples:
  mac-cleaner                                  interactive walkthrough
  mac-cleaner --all --dry-run                  preview everything
  mac-cleaner --dev-caches --browser-data      scan specific groups
  mac-cleaner scan --npm --safari --dry-run    scan specific items
  mac-cleaner --help-json                      structured help for AI agents`,
	Run: func(cmd *cobra.Command, args []string) {
		if flagHelpJSON {
			printHelpJSON(os.Stdout)
			return
		}

		sp := spinner.New("Scanning...", !flagJSON)
		ran := false
		var allResults []scan.CategoryResult

		flagScanners := []scannerMapping{
			{&flagSystemCaches, "system"},
			{&flagBrowserData, "browser"},
			{&flagDevCaches, "developer"},
			{&flagAppLeftovers, "appleftovers"},
			{&flagCreativeCaches, "creative"},
			{&flagMessagingCaches, "messaging"},
			{&flagUnusedApps, "unused"},
			{&flagPhotos, "photos"},
			{&flagSystemData, "systemdata"},
		}
		for _, m := range flagScanners {
			if *m.flag {
				allResults = append(allResults, runScannerByID(m.scannerID, sp)...)
				ran = true
			}
		}

		if flagJSON && !ran {
			fmt.Fprintln(os.Stderr, "Error: --json requires --all or a scan flag (--system-caches, --browser-data, --dev-caches, --app-leftovers, --creative-caches, --messaging-caches, --unused-apps, --photos, --system-data)")
			os.Exit(1)
		}

		if !ran {
			allResults = scanAll(sp)
			// Apply item-level skip filtering in interactive mode.
			allResults = engine.FilterSkipped(allResults, buildSkipSet())
			printPermissionIssues(allResults)
			printDryRunSummary(os.Stdout, allResults)
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
			sp.UpdateMessage("Cleaning up...")
			sp.Start()
			result := cleanup.Execute(marked, cleanupProgress(sp, os.Stderr))
			sp.Stop()
			printCleanupSummary(os.Stdout, result)
			return
		}

		// Apply item-level skip filtering.
		allResults = engine.FilterSkipped(allResults, buildSkipSet())

		if !flagJSON {
			printPermissionIssues(allResults)
		}

		if flagJSON {
			printJSON(allResults)
			if flagDryRun {
				return
			}
		}

		if flagDryRun && !flagJSON {
			printDryRunSummary(os.Stdout, allResults)
		}

		// Deletion flow: only when not in dry-run mode and there are results.
		if !flagDryRun && len(allResults) > 0 {
			if !flagForce {
				if !confirm.PromptConfirmation(os.Stdin, os.Stdout, allResults) {
					fmt.Println("Aborted.")
					return
				}
			}
			sp.UpdateMessage("Cleaning up...")
			sp.Start()
			result := cleanup.Execute(allResults, cleanupProgress(sp, os.Stderr))
			sp.Stop()
			printCleanupSummary(os.Stdout, result)
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
	rootCmd.Flags().BoolVar(&flagMessagingCaches, "messaging-caches", false, "scan Slack, Discord, Teams, and Zoom caches")
	rootCmd.Flags().BoolVar(&flagUnusedApps, "unused-apps", false, "scan applications not opened in 180+ days")
	rootCmd.Flags().BoolVar(&flagPhotos, "photos", false, "scan Photos app caches and media analysis data")
	rootCmd.Flags().BoolVar(&flagSystemData, "system-data", false, "scan Spotlight, Mail, Messages, iOS updates, Time Machine, and VMs")
	rootCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "output results as JSON")
	rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "show detailed file listing")
	rootCmd.Flags().BoolVar(&flagForce, "force", false, "bypass confirmation prompt (for automation)")
	rootCmd.Flags().BoolVar(&flagHelpJSON, "help-json", false, "output structured help as JSON for AI agents")

	// Category-level skip flags.
	rootCmd.Flags().BoolVar(&flagSkipSystemCaches, "skip-system-caches", false, "skip system cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipBrowserData, "skip-browser-data", false, "skip browser data scanning")
	rootCmd.Flags().BoolVar(&flagSkipDevCaches, "skip-dev-caches", false, "skip developer cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipAppLeftovers, "skip-app-leftovers", false, "skip app leftover scanning")
	rootCmd.Flags().BoolVar(&flagSkipCreativeCaches, "skip-creative-caches", false, "skip creative app cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipMessagingCaches, "skip-messaging-caches", false, "skip messaging app cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipUnusedApps, "skip-unused-apps", false, "skip unused applications scanning")
	rootCmd.Flags().BoolVar(&flagSkipPhotos, "skip-photos", false, "skip Photos cache scanning")
	rootCmd.Flags().BoolVar(&flagSkipSystemData, "skip-system-data", false, "skip system data scanning")

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
	rootCmd.Flags().BoolVar(&flagSkipPnpm, "skip-pnpm", false, "skip pnpm store")
	rootCmd.Flags().BoolVar(&flagSkipCocoapods, "skip-cocoapods", false, "skip CocoaPods cache")
	rootCmd.Flags().BoolVar(&flagSkipGradle, "skip-gradle", false, "skip Gradle cache")
	rootCmd.Flags().BoolVar(&flagSkipPip, "skip-pip", false, "skip pip cache")
	rootCmd.Flags().BoolVar(&flagSkipAdobe, "skip-adobe", false, "skip Adobe caches")
	rootCmd.Flags().BoolVar(&flagSkipAdobeMedia, "skip-adobe-media", false, "skip Adobe media caches")
	rootCmd.Flags().BoolVar(&flagSkipSketch, "skip-sketch", false, "skip Sketch cache")
	rootCmd.Flags().BoolVar(&flagSkipFigma, "skip-figma", false, "skip Figma cache")
	rootCmd.Flags().BoolVar(&flagSkipSlack, "skip-slack", false, "skip Slack cache")
	rootCmd.Flags().BoolVar(&flagSkipDiscord, "skip-discord", false, "skip Discord cache")
	rootCmd.Flags().BoolVar(&flagSkipTeams, "skip-teams", false, "skip Microsoft Teams cache")
	rootCmd.Flags().BoolVar(&flagSkipZoom, "skip-zoom", false, "skip Zoom cache")
	rootCmd.Flags().BoolVar(&flagSkipPhotosCaches, "skip-photos-caches", false, "skip Photos app caches")
	rootCmd.Flags().BoolVar(&flagSkipPhotosAnalysis, "skip-photos-analysis", false, "skip Photos analysis caches")
	rootCmd.Flags().BoolVar(&flagSkipPhotosIcloudCache, "skip-photos-icloud-cache", false, "skip iCloud Photos sync cache")
	rootCmd.Flags().BoolVar(&flagSkipPhotosSyndication, "skip-photos-syndication", false, "skip Messages shared photos")
	rootCmd.Flags().BoolVar(&flagSkipSpotlight, "skip-spotlight", false, "skip CoreSpotlight metadata")
	rootCmd.Flags().BoolVar(&flagSkipMail, "skip-mail", false, "skip Mail database")
	rootCmd.Flags().BoolVar(&flagSkipMailDownloads, "skip-mail-downloads", false, "skip Mail attachment cache")
	rootCmd.Flags().BoolVar(&flagSkipMessages, "skip-messages", false, "skip Messages attachments")
	rootCmd.Flags().BoolVar(&flagSkipIOSUpdates, "skip-ios-updates", false, "skip iOS software updates")
	rootCmd.Flags().BoolVar(&flagSkipTimemachine, "skip-timemachine", false, "skip Time Machine local snapshots")
	rootCmd.Flags().BoolVar(&flagSkipVMParallels, "skip-vm-parallels", false, "skip Parallels VMs")
	rootCmd.Flags().BoolVar(&flagSkipVMUTM, "skip-vm-utm", false, "skip UTM VMs")
	rootCmd.Flags().BoolVar(&flagSkipVMVMware, "skip-vm-vmware", false, "skip VMware Fusion VMs")

	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		// Initialize the engine.
		eng = engine.New()
		engine.RegisterDefaults(eng)

		if flagAll {
			flagSystemCaches = true
			flagBrowserData = true
			flagDevCaches = true
			flagAppLeftovers = true
			flagCreativeCaches = true
			flagMessagingCaches = true
			flagUnusedApps = true
			flagPhotos = true
			flagSystemData = true
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
		if flagSkipMessagingCaches {
			flagMessagingCaches = false
		}
		if flagSkipUnusedApps {
			flagUnusedApps = false
		}
		if flagSkipPhotos {
			flagPhotos = false
		}
		if flagSkipSystemData {
			flagSystemData = false
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

// findScannerInfo looks up scanner metadata from the engine's registry.
func findScannerInfo(scannerID string) engine.ScannerInfo {
	for _, info := range eng.Categories() {
		if info.ID == scannerID {
			return info
		}
	}
	return engine.ScannerInfo{ID: scannerID, Name: scannerID}
}

// runScannerByID runs a single scanner by ID using the engine and prints results.
func runScannerByID(scannerID string, sp *spinner.Spinner) []scan.CategoryResult {
	info := findScannerInfo(scannerID)
	sp.UpdateMessage("Scanning " + strings.ToLower(info.Name) + "...")
	sp.Start()
	results, err := eng.Run(context.Background(), scannerID)
	sp.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
	}
	if !flagJSON {
		printResults(results, flagDryRun, info.Name)
	}
	return results
}

// buildSkipSet collects category IDs that should be excluded from results
// based on item-level skip flags. Uses scanGroups as the source of truth.
func buildSkipSet() map[string]bool {
	skip := map[string]bool{}
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.SkipFlag != nil && *item.SkipFlag {
				skip[item.CategoryID] = true
			}
		}
	}
	return skip
}

// scanAll runs all registered scanners via the engine's channel-based API
// and returns aggregated results. Scanner errors are logged to stderr; partial
// results are still returned. Results are printed with dryRun=true since
// interactive mode handles deletion decisions separately.
func scanAll(sp *spinner.Spinner) []scan.CategoryResult {
	events, done := eng.ScanAll(context.Background(), nil)
	for event := range events {
		switch event.Type {
		case engine.EventScannerStart:
			sp.UpdateMessage("Scanning " + strings.ToLower(event.Label) + "...")
			sp.Start()
		case engine.EventScannerDone:
			sp.Stop()
			if len(event.Results) > 0 {
				printResults(event.Results, true, event.Label)
			}
		case engine.EventScannerError:
			sp.Stop()
			fmt.Fprintf(os.Stderr, "Warning: %v\n", event.Err)
		}
	}
	result := <-done
	return result.Results
}

// printCleanupSummary displays the results of a cleanup operation.
func printCleanupSummary(w io.Writer, result cleanup.CleanupResult) {
	greenBold := color.New(color.FgGreen, color.Bold)
	fmt.Fprintln(w)
	_, _ = greenBold.Fprintf(w, "Cleanup complete: %d items removed, %s freed\n",
		result.Removed, scan.FormatSize(result.BytesFreed))
	if result.Failed > 0 {
		yellow := color.New(color.FgYellow)
		fmt.Fprintln(w)
		_, _ = yellow.Fprintf(w, "%d items failed:\n", result.Failed)
		for _, err := range result.Errors {
			fmt.Fprintf(w, "  - %s\n", err)
		}
	}
	fmt.Fprintln(w)
}

// cleanupProgress returns a ProgressFunc that drives the spinner (normal mode)
// or prints per-entry detail (verbose mode). It returns nil for JSON mode.
func cleanupProgress(sp *spinner.Spinner, w io.Writer) cleanup.ProgressFunc {
	if flagJSON {
		return nil
	}
	if flagVerbose {
		return func(categoryDesc, entryPath string, current, total int) {
			if entryPath == "" {
				fmt.Fprintf(w, "Cleaning %s (%d/%d)\n", categoryDesc, current, total)
			} else {
				home, _ := os.UserHomeDir()
				fmt.Fprintf(w, "  removing %s\n", shortenHome(entryPath, home))
			}
		}
	}
	return func(categoryDesc, entryPath string, current, total int) {
		if entryPath == "" {
			sp.UpdateMessage(fmt.Sprintf("Cleaning %s... (%d/%d)", categoryDesc, current, total))
		}
	}
}

// flagForCategory returns the CLI scan flag (e.g. "--dev-caches") that covers
// the given category ID. It returns "" for unrecognised IDs.
// Uses scanGroups as the source of truth.
func flagForCategory(categoryID string) string {
	if g := groupForCategory(categoryID); g != nil {
		return "--" + g.FlagName
	}
	return ""
}

// printDryRunSummary prints a compact size-sorted summary table when at least
// two categories have data. It is intended for dry-run output so the user can
// quickly see where disk space is reclaimable.
func printDryRunSummary(w io.Writer, results []scan.CategoryResult) {
	var nonEmpty []scan.CategoryResult
	for _, cat := range results {
		if cat.TotalSize > 0 {
			nonEmpty = append(nonEmpty, cat)
		}
	}
	if len(nonEmpty) < 2 {
		return
	}

	sort.Slice(nonEmpty, func(i, j int) bool {
		return nonEmpty[i].TotalSize > nonEmpty[j].TotalSize
	})

	var total int64
	for _, cat := range nonEmpty {
		total += cat.TotalSize
	}

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	faint := color.New(color.Faint)
	greenBold := color.New(color.FgGreen, color.Bold)

	fmt.Fprintln(w)
	_, _ = bold.Fprintln(w, "Dry-Run Summary")
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	for _, cat := range nonEmpty {
		pct := float64(cat.TotalSize) / float64(total) * 100
		hint := ""
		if flag := flagForCategory(cat.Category); flag != "" {
			hint = faint.Sprintf("(%s)", flag)
		}
		fmt.Fprintf(tw, "  %s\t  %s\t  (%4.1f%%)\t  %s\t\n",
			cat.Description,
			cyan.Sprint(scan.FormatSize(cat.TotalSize)),
			pct,
			hint)
	}
	_ = tw.Flush()

	fmt.Fprintln(w)
	_, _ = greenBold.Fprintf(w, "  Total: %s reclaimable\n", scan.FormatSize(total))
	fmt.Fprintln(w)
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
	_, _ = bold.Println(header)

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
		_, _ = bold.Println(catHeader)

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
		_ = w.Flush()

		grandTotal += cat.TotalSize
	}

	// Summary line.
	fmt.Println()
	_, _ = greenBold.Printf("  Total: %s reclaimable\n", scan.FormatSize(grandTotal))
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
	_, _ = yellow.Fprintf(os.Stderr, "Note: %d path(s) could not be accessed (permission denied):\n", len(issues))
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
