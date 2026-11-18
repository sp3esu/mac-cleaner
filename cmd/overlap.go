package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// printOverlaps prints, in verbose mode, the paths counted by more than one
// entry (across all scanned categories, after filtering) and the totals with
// every byte counted once. It prints nothing when no path is counted twice.
func printOverlaps(w io.Writer, results []scan.CategoryResult) {
	if !flagVerbose {
		return
	}
	overlaps, dupCounted, _ := scan.FindOverlaps(results)
	if len(overlaps) == 0 {
		return
	}

	var total int64
	for _, cat := range results {
		total += cat.TotalSize
	}

	home, _ := os.UserHomeDir()
	bold := color.New(color.Bold)
	faint := color.New(color.Faint)
	yellow := color.New(color.FgYellow)

	fmt.Fprintln(w)
	_, _ = bold.Fprintln(w, "Counted More Than Once")
	_, _ = faint.Fprintln(w, "  These paths are included in more than one entry, so the totals above count their bytes more than once.")
	fmt.Fprintln(w)
	for _, o := range overlaps {
		fmt.Fprintf(w, "  %9s  %s  [%s]\n", scan.FormatSize(o.Inner.Size), shortenHome(o.Inner.Path, home), o.Inner.Category)
		if o.SamePath() {
			_, _ = faint.Fprintf(w, "             also counted in [%s]\n", o.Outer.Category)
		} else {
			_, _ = faint.Fprintf(w, "             inside %s  [%s]\n", shortenHome(o.Outer.Path, home), o.Outer.Category)
		}
	}
	fmt.Fprintln(w)
	_, _ = yellow.Fprintf(w, "  Counted once: %s of %s total (%s counted more than once)\n",
		scan.FormatSize(total-dupCounted), scan.FormatSize(total), scan.FormatSize(dupCounted))
	_, _ = yellow.Fprintf(w, "  cleanup removes %s, each path counted once\n", scan.FormatSize(scan.Reclaimable(results)))
	fmt.Fprintln(w)
}
