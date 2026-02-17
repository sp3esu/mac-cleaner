package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sp3esu/mac-cleaner/internal/safety"
)

// helpJSON is the top-level structure for --help-json output.
type helpJSON struct {
	Version       string                  `json:"version"`
	Commands      map[string]helpCommand  `json:"commands"`
	ScannerGroups []helpScannerGroup      `json:"scanner_groups"`
	GlobalFlags   []helpFlag              `json:"global_flags"`
	OutputFlags   []helpFlag              `json:"output_flags"`
	Examples      []helpExample           `json:"examples"`
}

type helpCommand struct {
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Notes       string `json:"notes,omitempty"`
}

type helpScannerGroup struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	GroupFlag  string         `json:"group_flag"`
	SkipFlag   string         `json:"skip_flag"`
	Categories []helpCategory `json:"categories"`
}

type helpCategory struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ScanFlag    string `json:"scan_flag,omitempty"`
	SkipFlag    string `json:"skip_flag,omitempty"`
	RiskLevel   string `json:"risk_level"`
}

type helpFlag struct {
	Flag        string `json:"flag"`
	Description string `json:"description"`
}

type helpExample struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// buildHelpJSON constructs the structured help output from scanGroups.
func buildHelpJSON() helpJSON {
	h := helpJSON{
		Version: version,
		Commands: map[string]helpCommand{
			"root": {
				Usage:       "mac-cleaner [flags]",
				Description: "Interactive scan and cleanup (no subcommand needed)",
				Notes:       "Without scan flags, enters interactive walkthrough mode",
			},
			"scan": {
				Usage:       "mac-cleaner scan [flags]",
				Description: "Scan specific categories or items",
				Notes:       "Requires at least one scan flag",
			},
			"serve": {
				Usage:       "mac-cleaner serve --socket <path>",
				Description: "Start IPC server for Swift app integration",
			},
		},
		GlobalFlags: []helpFlag{
			{Flag: "--dry-run", Description: "preview what would be removed without deleting"},
		},
		OutputFlags: []helpFlag{
			{Flag: "--json", Description: "output results as JSON"},
			{Flag: "--verbose", Description: "show detailed file listing"},
			{Flag: "--force", Description: "bypass confirmation prompt (for automation)"},
		},
		Examples: []helpExample{
			{Command: "mac-cleaner scan --npm --yarn --json", Description: "Scan only npm and yarn caches, output as JSON"},
			{Command: "mac-cleaner scan --all --skip-docker --dry-run", Description: "Dry-run scan everything except Docker"},
			{Command: "mac-cleaner scan --dev-caches --safari", Description: "Scan all developer caches plus Safari"},
			{Command: "mac-cleaner --all --dry-run", Description: "Preview all reclaimable space"},
			{Command: "mac-cleaner", Description: "Interactive walkthrough mode"},
		},
	}

	for _, g := range scanGroups {
		group := helpScannerGroup{
			ID:        g.ScannerID,
			Name:      g.GroupName,
			GroupFlag: "--" + g.FlagName,
			SkipFlag:  "--skip-" + g.FlagName,
		}
		for _, item := range g.Items {
			cat := helpCategory{
				ID:          item.CategoryID,
				Description: item.Description,
				RiskLevel:   safety.RiskForCategory(item.CategoryID),
			}
			if item.FlagName != "" {
				cat.ScanFlag = "--" + item.FlagName
				cat.SkipFlag = "--skip-" + item.FlagName
			}
			group.Categories = append(group.Categories, cat)
		}
		h.ScannerGroups = append(h.ScannerGroups, group)
	}

	return h
}

// printHelpJSON writes the structured help JSON to w.
func printHelpJSON(w io.Writer) {
	h := buildHelpJSON()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(h); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding help JSON: %v\n", err)
		os.Exit(1)
	}
}
