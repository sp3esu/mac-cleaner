package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/confirm"
	"github.com/sp3esu/mac-cleaner/internal/engine"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/spinner"
)

var scanCmd = &cobra.Command{
	Use:   "scan [flags]",
	Short: "scan specific categories or items",
	Long: `Scan specific scanner groups or individual items.

Group flags scan an entire category (e.g. --dev-caches scans all developer caches).
Targeted flags scan a single item (e.g. --npm scans only the npm cache).
Combine them freely: --dev-caches --safari scans all dev plus Safari only.

Skip flags exclude items: --dev-caches --skip-docker scans all dev except Docker.
Use --all to scan everything, then skip what you don't want.

At least one scan flag is required. Without flags, this help is shown.

Examples:
  mac-cleaner scan --dev-caches                        all developer caches
  mac-cleaner scan --npm --yarn                        only npm and yarn
  mac-cleaner scan --dev-caches --safari               all dev + Safari
  mac-cleaner scan --dev-caches --skip-docker          all dev except Docker
  mac-cleaner scan --all --skip-docker --skip-safari   everything except Docker and Safari
  mac-cleaner scan --npm --json --dry-run              npm cache as JSON (no deletion)`,
	PreRun: func(cmd *cobra.Command, args []string) {
		eng = engine.New()
		engine.RegisterDefaults(eng)

		if flagAll {
			for _, g := range scanGroups {
				*g.ScanFlag = true
			}
		}
		for _, g := range scanGroups {
			if g.SkipFlag != nil && *g.SkipFlag {
				*g.ScanFlag = false
			}
		}
		if flagJSON {
			color.NoColor = true
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Collect what to scan.
		groupSet := map[string]bool{}    // scanner IDs from group flags
		itemSet := map[string]string{}   // categoryID -> scannerID from targeted item flags
		for _, g := range scanGroups {
			if *g.ScanFlag {
				groupSet[g.ScannerID] = true
			}
			for _, item := range g.Items {
				if item.ScanFlag != nil && *item.ScanFlag {
					itemSet[item.CategoryID] = g.ScannerID
				}
			}
		}

		if len(groupSet) == 0 && len(itemSet) == 0 {
			_ = cmd.Help()
			return
		}

		// Determine which scanners need to run.
		scannersToRun := map[string]bool{}
		for id := range groupSet {
			scannersToRun[id] = true
		}
		for _, sid := range itemSet {
			scannersToRun[sid] = true
		}

		sp := spinner.New("Scanning...", !flagJSON)
		skipSet := buildSkipSet()
		var allResults []scan.CategoryResult

		for _, g := range scanGroups {
			if !scannersToRun[g.ScannerID] {
				continue
			}

			isGroup := groupSet[g.ScannerID]

			// For item-targeted (not full group), find which items are requested.
			var targetedItems map[string]bool
			if !isGroup {
				targetedItems = map[string]bool{}
				for _, item := range g.Items {
					if _, ok := itemSet[item.CategoryID]; ok {
						targetedItems[item.CategoryID] = true
					}
				}
				if len(targetedItems) == 0 {
					continue
				}
			}

			// Run the scanner.
			info := findScannerInfo(g.ScannerID)
			sp.UpdateMessage("Scanning " + strings.ToLower(info.Name) + "...")
			sp.Start()
			results, err := eng.Run(context.Background(), g.ScannerID)
			sp.Stop()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			// Filter to targeted items only (if not full group).
			if !isGroup {
				var filtered []scan.CategoryResult
				for _, r := range results {
					if targetedItems[r.Category] {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}

			// Apply skip filtering.
			results = engine.FilterSkipped(results, skipSet)

			if !flagJSON && len(results) > 0 {
				printResults(results, flagDryRun, info.Name)
			}

			allResults = append(allResults, results...)
		}

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
			return
		}

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
	// Group scan flags.
	for _, g := range scanGroups {
		scanCmd.Flags().BoolVar(g.ScanFlag, g.FlagName, false, "scan "+g.Description)
	}
	scanCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")

	// Targeted item scan flags.
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName != "" && item.ScanFlag != nil {
				scanCmd.Flags().BoolVar(item.ScanFlag, item.FlagName, false, "scan "+item.Description)
			}
		}
	}

	// Category-level skip flags.
	for _, g := range scanGroups {
		scanCmd.Flags().BoolVar(g.SkipFlag, "skip-"+g.FlagName, false, "skip "+g.Description+" scanning")
	}

	// Item-level skip flags.
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName != "" && item.SkipFlag != nil {
				scanCmd.Flags().BoolVar(item.SkipFlag, "skip-"+item.FlagName, false, "skip "+item.Description)
			}
		}
	}

	// Output flags.
	scanCmd.Flags().BoolVar(&flagJSON, "json", false, "output results as JSON")
	scanCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "show detailed file listing")
	scanCmd.Flags().BoolVar(&flagForce, "force", false, "bypass confirmation prompt (for automation)")

	scanCmd.SetUsageFunc(scanUsageFunc)
	rootCmd.AddCommand(scanCmd)
}

// scanUsageFunc renders grouped help for the scan command.
// Long description is printed by cobra's help template; this only adds
// the usage line and grouped flag sections.
func scanUsageFunc(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Usage:\n  %s\n", cmd.UseLine())

	// Scanner Groups section.
	fmt.Fprintf(w, "\nScanner Groups:\n")
	for _, g := range scanGroups {
		fmt.Fprintf(w, "  --%-24s %s\n", g.FlagName, "scan "+g.Description)
	}
	fmt.Fprintf(w, "  --%-24s %s\n", "all", "scan all categories")

	// Targeted Scans sections (one per group with items).
	for _, g := range scanGroups {
		hasItems := false
		for _, item := range g.Items {
			if item.FlagName != "" {
				hasItems = true
				break
			}
		}
		if !hasItems {
			continue
		}
		fmt.Fprintf(w, "\nTargeted Scans — %s:\n", g.GroupName)
		for _, item := range g.Items {
			if item.FlagName != "" {
				fmt.Fprintf(w, "  --%-24s %s\n", item.FlagName, "scan "+item.Description)
			}
		}
	}

	// Skip Flags section.
	fmt.Fprintf(w, "\nSkip Flags:\n")
	for _, g := range scanGroups {
		fmt.Fprintf(w, "  --%-24s %s\n", "skip-"+g.FlagName, "skip "+g.Description+" scanning")
		for _, item := range g.Items {
			if item.FlagName != "" && item.SkipFlag != nil {
				fmt.Fprintf(w, "  --%-24s %s\n", "skip-"+item.FlagName, "skip "+item.Description)
			}
		}
	}

	// Output Options section.
	fmt.Fprintf(w, "\nOutput Options:\n")
	fmt.Fprintf(w, "  --%-24s %s\n", "json", "output results as JSON")
	fmt.Fprintf(w, "  --%-24s %s\n", "verbose", "show detailed file listing")
	fmt.Fprintf(w, "  --%-24s %s\n", "force", "bypass confirmation prompt (for automation)")
	fmt.Fprintf(w, "  --%-24s %s\n", "dry-run", "preview what would be removed without deleting")

	fmt.Fprintln(w)
	return nil
}
