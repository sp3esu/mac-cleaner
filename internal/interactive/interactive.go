// Package interactive provides the guided walkthrough mode for mac-cleaner.
// When the user runs mac-cleaner with no flags, each scan result is presented
// one-by-one and the user chooses to keep or remove it.
package interactive

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"

	"github.com/gregor/mac-cleaner/internal/scan"
)

// RunWalkthrough presents each scan entry one-by-one and asks the user
// whether to keep or remove it. It returns a filtered slice containing
// only categories/entries that the user marked for removal. If no items
// exist or none are marked for removal, it returns nil.
func RunWalkthrough(in io.Reader, out io.Writer, results []scan.CategoryResult) []scan.CategoryResult {
	// Count total items across all categories.
	totalItems := 0
	for _, cat := range results {
		totalItems += len(cat.Entries)
	}

	if totalItems == 0 {
		fmt.Fprintln(out, "Nothing to clean.")
		return nil
	}

	fmt.Fprintf(out, "\nFound %d items. Review each to keep or remove:\n", totalItems)

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	reader := bufio.NewReader(in)
	itemNum := 0
	var filtered []scan.CategoryResult

	for _, cat := range results {
		// Print category header.
		fmt.Fprintln(out)
		bold.Fprintln(out, cat.Description)

		var removedEntries []scan.ScanEntry
		var removedSize int64

		for _, entry := range cat.Entries {
			itemNum++
			sizeStr := scan.FormatSize(entry.Size)

			fmt.Fprintf(out, "  [%d/%d] %s  %s\n", itemNum, totalItems,
				entry.Description, cyan.Sprint(sizeStr))
			fmt.Fprint(out, "  keep or remove? [k/r]: ")

			choice := readChoice(reader, out)
			if choice == "remove" {
				removedEntries = append(removedEntries, entry)
				removedSize += entry.Size
			}
		}

		if len(removedEntries) > 0 {
			filtered = append(filtered, scan.CategoryResult{
				Category:    cat.Category,
				Description: cat.Description,
				Entries:     removedEntries,
				TotalSize:   removedSize,
			})
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintln(out, "Nothing marked for removal.")
		return nil
	}

	return filtered
}

// readChoice reads user input and returns either "keep" or "remove".
// On EOF or read error, it defaults to "keep" (safe default).
// On invalid input, it re-prompts until a valid response is given.
func readChoice(reader *bufio.Reader, out io.Writer) string {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF or other error: safe default is keep.
			return "keep"
		}

		normalized := strings.ToLower(strings.TrimSpace(line))
		switch normalized {
		case "r", "remove":
			return "remove"
		case "k", "keep":
			return "keep"
		default:
			fmt.Fprint(out, "  Please enter 'k' to keep or 'r' to remove: ")
		}
	}
}
