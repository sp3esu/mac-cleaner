package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// countedPath is one filesystem path whose size a scan entry counts: the
// entry's own path, or one of its size components.
type countedPath struct {
	path     string
	size     int64
	removed  bool
	category string // category description, for display
}

// overlap is a counted path that is also covered by another counted path
// (the same path, or an ancestor directory), so its bytes are counted more
// than once in the totals.
type overlap struct {
	inner countedPath
	outer countedPath
}

// collectCountedPaths returns every absolute filesystem path counted in the
// results. Entries without a breakdown count their own path with their full
// size; entries with a breakdown count each component. Pseudo-paths such as
// "docker:Images", "git-gc:<repo>" and "tmutil:snapshot:<name>" are skipped
// (along with their components): their sizes are estimates, not the size of
// a filesystem path, so they cannot be compared with other entries.
func collectCountedPaths(results []scan.CategoryResult) []countedPath {
	var out []countedPath
	for _, cat := range results {
		for _, e := range cat.Entries {
			if !filepath.IsAbs(e.Path) {
				continue
			}
			if len(e.Components) == 0 {
				out = append(out, countedPath{filepath.Clean(e.Path), e.Size, true, cat.Description})
				continue
			}
			for _, c := range e.Components {
				if filepath.IsAbs(c.Path) {
					out = append(out, countedPath{filepath.Clean(c.Path), c.Size, c.Removed, cat.Description})
				}
			}
		}
	}
	return out
}

// findCover returns the index of a counted path that covers paths[i] among
// those accepted by ok: an earlier occurrence of the same path, otherwise
// the nearest ancestor directory. It returns -1 when nothing covers it.
func findCover(paths []countedPath, byPath map[string][]int, i int, ok func(int) bool) int {
	for _, j := range byPath[paths[i].path] {
		if j >= i {
			break
		}
		if ok(j) {
			return j
		}
	}
	for p := paths[i].path; ; {
		parent := filepath.Dir(p)
		if parent == p {
			return -1
		}
		p = parent
		for _, j := range byPath[p] {
			if ok(j) {
				return j
			}
		}
	}
}

// findOverlaps reports every counted path covered by another counted path,
// plus the bytes counted more than once overall and among removed paths.
// Each covered path's bytes are already included in the path covering it,
// so subtracting them yields the total with every byte counted once.
func findOverlaps(results []scan.CategoryResult) (overlaps []overlap, dupCounted, dupRemoved int64) {
	paths := collectCountedPaths(results)
	byPath := make(map[string][]int, len(paths))
	for i, p := range paths {
		byPath[p.path] = append(byPath[p.path], i)
	}
	all := func(int) bool { return true }
	removed := func(j int) bool { return paths[j].removed }

	for i, p := range paths {
		if j := findCover(paths, byPath, i, all); j >= 0 {
			overlaps = append(overlaps, overlap{inner: p, outer: paths[j]})
			dupCounted += p.size
		}
		if p.removed && findCover(paths, byPath, i, removed) >= 0 {
			dupRemoved += p.size
		}
	}
	sort.SliceStable(overlaps, func(a, b int) bool { return overlaps[a].inner.size > overlaps[b].inner.size })
	return overlaps, dupCounted, dupRemoved
}

// printOverlaps prints, in verbose mode, the paths counted by more than one
// entry (across all scanned categories, after filtering) and the totals with
// every byte counted once. It prints nothing when no path is counted twice.
func printOverlaps(w io.Writer, results []scan.CategoryResult) {
	if !flagVerbose {
		return
	}
	overlaps, dupCounted, dupRemoved := findOverlaps(results)
	if len(overlaps) == 0 {
		return
	}

	var total, removed int64
	for _, cat := range results {
		total += cat.TotalSize
		for _, e := range cat.Entries {
			removed += e.RemovedSize()
		}
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
		fmt.Fprintf(w, "  %9s  %s  [%s]\n", scan.FormatSize(o.inner.size), shortenHome(o.inner.path, home), o.inner.category)
		if o.inner.path == o.outer.path {
			_, _ = faint.Fprintf(w, "             also counted in [%s]\n", o.outer.category)
		} else {
			_, _ = faint.Fprintf(w, "             inside %s  [%s]\n", shortenHome(o.outer.path, home), o.outer.category)
		}
	}
	fmt.Fprintln(w)
	_, _ = yellow.Fprintf(w, "  Counted once: %s of %s total (%s counted more than once)\n",
		scan.FormatSize(total-dupCounted), scan.FormatSize(total), scan.FormatSize(dupCounted))
	_, _ = yellow.Fprintf(w, "  cleanup removes %s, each path counted once\n", scan.FormatSize(removed-dupRemoved))
	fmt.Fprintln(w)
}
