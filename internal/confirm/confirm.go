// Package confirm provides an interactive confirmation prompt for cleanup
// operations. The prompt displays items to be deleted and requires an
// explicit "yes" response before proceeding.
package confirm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// PromptConfirmation displays a summary of items to be deleted and asks
// the user to type "yes" to proceed. Returns true only on exact "yes"
// input (case-sensitive, whitespace-trimmed). Returns false on any other
// input or read error.
//
// The listing separates what cleanup deletes from what it only lists:
// filesystem entries cleanup deletes (with the bytes it removes), git
// repositories it repacks with git gc (with the scan's estimate), and
// entries or parts of entries it leaves in place (non-filesystem entries
// such as docker: and tmutil:, entries whose path cleanup does not delete,
// such as /Applications bundles blocked by the safety check, and counted
// components that are not removed, such as an unused app's ~/Library data).
// The total is scan.Reclaimable: the bytes deleted, each path counted once.
func PromptConfirmation(in io.Reader, out io.Writer, results []scan.CategoryResult) bool {
	home, _ := os.UserHomeDir()

	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	riskTag := func(level string) string {
		switch level {
		case safety.RiskRisky:
			return red.Sprint(" [risky]")
		case safety.RiskModerate:
			return yellow.Sprint(" [moderate]")
		}
		return ""
	}

	var deleted, repacked, listed section
	var gitEstimate int64
	for _, cat := range results {
		for _, entry := range cat.Entries {
			path := shortenHome(entry.Path, home)
			if repo, ok := strings.CutPrefix(entry.Path, "git-gc:"); ok {
				repacked.add(cat.Description, fmt.Sprintf("    %s%s  (~%s)",
					shortenHome(repo, home), riskTag(entry.RiskLevel), scan.FormatSize(entry.Size)))
				gitEstimate += entry.Size
				continue
			}
			if !deletesPath(entry) {
				listed.add(cat.Description, fmt.Sprintf("    %s%s  (%s counted)%s",
					path, riskTag(entry.RiskLevel), scan.FormatSize(entry.Size), noteSuffix(entryNote(entry))))
				continue
			}
			deleted.add(cat.Description, fmt.Sprintf("    %s%s  (%s)",
				path, riskTag(entry.RiskLevel), scan.FormatSize(entry.RemovedSize())))
			for _, c := range entry.Components {
				if !c.Removed {
					listed.add(cat.Description, fmt.Sprintf("    %s  left in place (%s)%s",
						shortenHome(c.Path, home), scan.FormatSize(c.Size), noteSuffix(c.Note)))
				}
			}
		}
	}

	fmt.Fprintln(out, "\nThe following items will be permanently deleted:")
	if len(deleted.order) == 0 {
		fmt.Fprintln(out, "\n    (nothing)")
	}
	deleted.print(out)
	if len(repacked.order) > 0 {
		fmt.Fprintf(out, "\ngit gc will repack (estimate ~%s):\n", scan.FormatSize(gitEstimate))
		repacked.print(out)
	}
	if len(listed.order) > 0 {
		fmt.Fprintln(out, "\nListed but not deleted:")
		listed.print(out)
	}

	fmt.Fprintf(out, "\nTotal: %s will be permanently deleted.\n", scan.FormatSize(scan.Reclaimable(results)))
	if hasRiskyItems(results) {
		redBold := color.New(color.FgRed, color.Bold)
		_, _ = redBold.Fprintln(out, "\nWARNING: Selection includes risky items that may be difficult or impossible to recover.")
	}
	fmt.Fprint(out, "Type 'yes' to proceed: ")

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(response) == "yes"
}

// section collects prompt lines grouped by category, in first-seen order.
type section struct {
	order []string
	lines map[string][]string
}

func (s *section) add(category, line string) {
	if s.lines == nil {
		s.lines = map[string][]string{}
	}
	if _, ok := s.lines[category]; !ok {
		s.order = append(s.order, category)
	}
	s.lines[category] = append(s.lines[category], line)
}

func (s *section) print(out io.Writer) {
	bold := color.New(color.Bold)
	for _, cat := range s.order {
		fmt.Fprintln(out)
		_, _ = bold.Fprintln(out, "  "+cat)
		for _, line := range s.lines[cat] {
			fmt.Fprintln(out, line)
		}
	}
}

// deletesPath reports whether cleanup deletes the entry's path: it must be a
// real filesystem path and, when the entry has a size breakdown, at least
// one component must be marked removed. An entry such as an /Applications
// bundle blocked by the safety check has a breakdown with nothing removed.
func deletesPath(e scan.ScanEntry) bool {
	if !filepath.IsAbs(e.Path) {
		return false
	}
	if len(e.Components) == 0 {
		return true
	}
	for _, c := range e.Components {
		if c.Removed {
			return true
		}
	}
	return false
}

// entryNote returns the note explaining why an entry is not deleted: the
// note of the component for the entry's own path, else the first note.
func entryNote(e scan.ScanEntry) string {
	for _, c := range e.Components {
		if c.Path == e.Path && c.Note != "" {
			return c.Note
		}
	}
	for _, c := range e.Components {
		if c.Note != "" {
			return c.Note
		}
	}
	if !filepath.IsAbs(e.Path) {
		return "not a filesystem path"
	}
	return ""
}

func noteSuffix(note string) string {
	if note == "" {
		return ""
	}
	return " - " + note
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
