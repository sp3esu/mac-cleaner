// Package confirm provides an interactive confirmation prompt for cleanup
// operations. The prompt displays items to be deleted and requires an
// explicit "yes" response before proceeding.
package confirm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// PromptConfirmation displays a summary of items to be deleted and asks
// the user to type "yes" to proceed. Returns true only on exact "yes"
// input (case-sensitive, whitespace-trimmed). Returns false on any other
// input or read error.
func PromptConfirmation(in io.Reader, out io.Writer, results []scan.CategoryResult) bool {
	home, _ := os.UserHomeDir()

	bold := color.New(color.Bold)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	fmt.Fprintln(out, "\nThe following items will be permanently deleted:")

	var totalSize int64
	for _, cat := range results {
		fmt.Fprintln(out)
		bold.Fprintln(out, "  "+cat.Description)
		for _, entry := range cat.Entries {
			path := shortenHome(entry.Path, home)
			riskTag := ""
			switch entry.RiskLevel {
			case safety.RiskRisky:
				riskTag = red.Sprint(" [risky]")
			case safety.RiskModerate:
				riskTag = yellow.Sprint(" [moderate]")
			}
			fmt.Fprintf(out, "    %s%s  (%s)\n", path, riskTag, scan.FormatSize(entry.Size))
		}
		totalSize += cat.TotalSize
	}

	fmt.Fprintf(out, "\nTotal: %s will be permanently deleted.\n", scan.FormatSize(totalSize))
	if hasRiskyItems(results) {
		redBold := color.New(color.FgRed, color.Bold)
		redBold.Fprintln(out, "\nWARNING: Selection includes risky items that may be difficult or impossible to recover.")
	}
	fmt.Fprint(out, "Type 'yes' to proceed: ")

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(response) == "yes"
}

// hasRiskyItems returns true if any entry in the results has a risky risk level.
func hasRiskyItems(results []scan.CategoryResult) bool {
	for _, cat := range results {
		for _, entry := range cat.Entries {
			if entry.RiskLevel == safety.RiskRisky {
				return true
			}
		}
	}
	return false
}

// shortenHome replaces the home directory prefix with ~ for display.
func shortenHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
