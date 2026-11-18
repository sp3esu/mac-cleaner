package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/engine"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// overlapResults returns categories with every overlap kind: a file listed in
// two categories (same path), a cache nested inside another category's cache
// directory, a removed path nested inside a counted-but-not-removed component,
// and pseudo-path entries that must be ignored.
func overlapResults() []scan.CategoryResult {
	return []scan.CategoryResult{
		{
			Category:    "home-large-files",
			Description: "Large Files",
			Entries:     []scan.ScanEntry{{Path: "/h/Movies/a.mp4", Size: 1000}},
			TotalSize:   1000,
		},
		{
			Category:    "home-old-files",
			Description: "Old Files",
			Entries:     []scan.ScanEntry{{Path: "/h/Movies/a.mp4", Size: 1000}},
			TotalSize:   1000,
		},
		{
			Category:    "system-caches",
			Description: "User App Caches",
			Entries:     []scan.ScanEntry{{Path: "/h/Library/Caches/Google", Size: 500}},
			TotalSize:   500,
		},
		{
			Category:    "browser-chrome",
			Description: "Chrome Cache",
			Entries:     []scan.ScanEntry{{Path: "/h/Library/Caches/Google/Chrome/Default/", Size: 400}},
			TotalSize:   400,
		},
		{
			Category:    "unused-apps",
			Description: "Unused Applications",
			Entries: []scan.ScanEntry{{
				Path: "/Applications/Zoom.app",
				Size: 330,
				Components: []scan.SizeComponent{
					{Path: "/Applications/Zoom.app", Size: 30},
					{Path: "/h/Library/Application Support/zoom.us", Size: 300},
				},
			}},
			TotalSize: 330,
		},
		{
			Category:    "msg-zoom",
			Description: "Zoom Cache",
			Entries:     []scan.ScanEntry{{Path: "/h/Library/Application Support/zoom.us/data", Size: 200}},
			TotalSize:   200,
		},
		{
			Category:    "dev-docker",
			Description: "Docker",
			Entries: []scan.ScanEntry{{
				Path:       "docker:Images",
				Size:       20,
				Components: []scan.SizeComponent{{Path: "docker:Images", Size: 20}},
			}},
			TotalSize: 20,
		},
		{
			Category:    "git-repos",
			Description: "Git Repos",
			Entries: []scan.ScanEntry{{
				Path:       "git-gc:/h/Movies",
				Size:       50,
				Components: []scan.SizeComponent{{Path: "/h/Movies/.git/objects", Size: 50}},
			}},
			TotalSize: 50,
		},
	}
}

func TestFindOverlaps(t *testing.T) {
	overlaps, dupCounted, dupRemoved := findOverlaps(overlapResults())

	if len(overlaps) != 3 {
		t.Fatalf("expected 3 overlaps, got %d: %+v", len(overlaps), overlaps)
	}
	// Sorted by size, largest first.
	want := []struct{ inner, outer, innerCat, outerCat string }{
		{"/h/Movies/a.mp4", "/h/Movies/a.mp4", "Old Files", "Large Files"},
		{"/h/Library/Caches/Google/Chrome/Default", "/h/Library/Caches/Google", "Chrome Cache", "User App Caches"},
		{"/h/Library/Application Support/zoom.us/data", "/h/Library/Application Support/zoom.us", "Zoom Cache", "Unused Applications"},
	}
	for i, w := range want {
		o := overlaps[i]
		if o.inner.path != w.inner || o.outer.path != w.outer || o.inner.category != w.innerCat || o.outer.category != w.outerCat {
			t.Errorf("overlap %d = %+v, want %+v", i, o, w)
		}
	}
	if dupCounted != 1000+400+200 {
		t.Errorf("dupCounted = %d, want 1600", dupCounted)
	}
	// Zoom data is removed, but the zoom.us component covering it is not,
	// so its bytes are removed only once and are not a removal duplicate.
	if dupRemoved != 1000+400 {
		t.Errorf("dupRemoved = %d, want 1400", dupRemoved)
	}
}

func TestFindOverlaps_ChainCountsEachCoveredPath(t *testing.T) {
	results := []scan.CategoryResult{
		{Description: "A", Entries: []scan.ScanEntry{{Path: "/x", Size: 100}}},
		{Description: "B", Entries: []scan.ScanEntry{{Path: "/x/y", Size: 60}}},
		{Description: "C", Entries: []scan.ScanEntry{{Path: "/x/y/z", Size: 10}}},
	}
	overlaps, dupCounted, dupRemoved := findOverlaps(results)
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %+v", overlaps)
	}
	// /x/y/z reports its nearest covering ancestor.
	if overlaps[1].inner.path != "/x/y/z" || overlaps[1].outer.path != "/x/y" {
		t.Errorf("unexpected nearest ancestor: %+v", overlaps[1])
	}
	// Total 170, counted once 100.
	if dupCounted != 70 || dupRemoved != 70 {
		t.Errorf("dupCounted=%d dupRemoved=%d, want 70/70", dupCounted, dupRemoved)
	}
}

func TestFindOverlaps_PrefixIsNotAncestor(t *testing.T) {
	results := []scan.CategoryResult{
		{Description: "A", Entries: []scan.ScanEntry{{Path: "/h/Caches/pip", Size: 100}}},
		{Description: "B", Entries: []scan.ScanEntry{{Path: "/h/Caches/pipx", Size: 60}}},
	}
	if overlaps, d, _ := findOverlaps(results); len(overlaps) != 0 || d != 0 {
		t.Errorf("sibling with shared prefix reported as overlap: %+v", overlaps)
	}
}

func TestFindOverlaps_RemovedCoveredOnlyByRemoved(t *testing.T) {
	// A removed path inside a removed path is removed once; a removed path
	// inside only a non-removed component is not a removal duplicate.
	results := []scan.CategoryResult{
		{Description: "A", Entries: []scan.ScanEntry{{
			Path: "/a.app", Size: 100,
			Components: []scan.SizeComponent{{Path: "/lib", Size: 100}},
		}}},
		{Description: "B", Entries: []scan.ScanEntry{{Path: "/lib", Size: 100}}},
		{Description: "C", Entries: []scan.ScanEntry{{Path: "/lib/sub", Size: 40}}},
	}
	_, dupCounted, dupRemoved := findOverlaps(results)
	if dupCounted != 140 {
		t.Errorf("dupCounted = %d, want 140", dupCounted)
	}
	if dupRemoved != 40 {
		t.Errorf("dupRemoved = %d, want 40", dupRemoved)
	}
}

func TestFindOverlaps_NotRemovedAtOrInsideRemoved(t *testing.T) {
	// System caches runs before unused apps, so a removed cache entry can
	// cover an unused app's ~/Library component, which cleanup does not
	// remove. Those bytes are counted twice but removed only once.
	results := []scan.CategoryResult{
		{Description: "User App Caches", Entries: []scan.ScanEntry{{Path: "/h/Library/Caches/com.foo", Size: 100}}},
		{Description: "Unused Applications", Entries: []scan.ScanEntry{{
			Path: "/Applications/Foo.app", Size: 140,
			Components: []scan.SizeComponent{
				{Path: "/Applications/Foo.app", Size: 10},
				{Path: "/h/Library/Caches/com.foo", Size: 100},
				{Path: "/h/Library/Caches/com.foo/sub", Size: 30},
			},
		}}},
	}
	overlaps, dupCounted, dupRemoved := findOverlaps(results)
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %+v", overlaps)
	}
	if dupCounted != 130 {
		t.Errorf("dupCounted = %d, want 130", dupCounted)
	}
	if dupRemoved != 0 {
		t.Errorf("dupRemoved = %d, want 0: paths cleanup does not remove are not removal duplicates", dupRemoved)
	}
}

func TestFindOverlaps_PseudoPathsIgnored(t *testing.T) {
	results := []scan.CategoryResult{
		// git's .git/objects component is a count-objects estimate under a
		// pseudo-path entry: a real file inside it must not overlap it.
		{Description: "Git Repos", Entries: []scan.ScanEntry{{
			Path: "git-gc:/h/r", Size: 50,
			Components: []scan.SizeComponent{{Path: "/h/r/.git/objects", Size: 50}},
		}}},
		{Description: "Large Files", Entries: []scan.ScanEntry{{Path: "/h/r/.git/objects/pack/p.pack", Size: 30}}},
		// Non-filesystem components never overlap, even when two entries
		// with real paths both carry the same one.
		{Description: "A", Entries: []scan.ScanEntry{{
			Path: "/h/a", Size: 5,
			Components: []scan.SizeComponent{{Path: "docker:Images", Size: 5}},
		}}},
		{Description: "B", Entries: []scan.ScanEntry{{
			Path: "/h/b", Size: 5,
			Components: []scan.SizeComponent{{Path: "docker:Images", Size: 5}},
		}}},
	}
	if overlaps, dupCounted, dupRemoved := findOverlaps(results); len(overlaps) != 0 || dupCounted != 0 || dupRemoved != 0 {
		t.Errorf("spurious overlap %+v (dupCounted=%d dupRemoved=%d)", overlaps, dupCounted, dupRemoved)
	}
}

func TestPrintOverlaps_Verbose(t *testing.T) {
	withVerbose(t, true)

	var buf bytes.Buffer
	printOverlaps(&buf, overlapResults())
	out := buf.String()
	for _, want := range []string{
		"Counted More Than Once",
		"1.0 kB  /h/Movies/a.mp4  [Old Files]\n             also counted in [Large Files]\n",
		"400 B  /h/Library/Caches/Google/Chrome/Default  [Chrome Cache]\n             inside /h/Library/Caches/Google  [User App Caches]\n",
		"200 B  /h/Library/Application Support/zoom.us/data  [Zoom Cache]\n             inside /h/Library/Application Support/zoom.us  [Unused Applications]\n",
		// Total 3500, duplicates 1600.
		"Counted once: 1.9 kB of 3.5 kB total (1.6 kB counted more than once)",
		// Removed: 1000+1000+500+400+200 = 3100, minus 1400 duplicates.
		"cleanup removes 1.7 kB, each path counted once",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"docker:", ".git/objects"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("pseudo-path entry %q must be ignored:\n%s", unwanted, out)
		}
	}
}

func TestPrintOverlaps_NothingWithoutOverlap(t *testing.T) {
	withVerbose(t, true)

	// Only one of the two home-file categories scanned (e.g. --large-files):
	// nothing is counted twice, so nothing is printed.
	var buf bytes.Buffer
	printOverlaps(&buf, overlapResults()[:1])
	if buf.Len() != 0 {
		t.Errorf("expected no output, got:\n%s", buf.String())
	}
}

func TestPrintOverlaps_NonVerbose(t *testing.T) {
	withVerbose(t, false)

	var buf bytes.Buffer
	printOverlaps(&buf, overlapResults())
	if buf.Len() != 0 {
		t.Errorf("expected no output without --verbose, got:\n%s", buf.String())
	}
}

// runScanDryRunVerbose runs "scan" with a fake homedir scanner that lists
// the same file under Large Files and Old Files, and returns the output.
func runScanDryRunVerbose(t *testing.T, large, old bool) string {
	t.Helper()
	withVerbose(t, true)
	fakeHomedirEngine(t)
	origLarge, origOld := flagScanLargeFiles, flagScanOldFiles
	t.Cleanup(func() { flagScanLargeFiles, flagScanOldFiles = origLarge, origOld })
	flagScanLargeFiles, flagScanOldFiles = large, old

	return captureStdout(t, func() { scanCmd.Run(scanCmd, nil) })
}

// fakeHomedirEngine installs an engine whose only scanner lists the same
// file under Large Files and Old Files, restoring the real engine and the
// dry-run flag afterwards.
func fakeHomedirEngine(t *testing.T) {
	t.Helper()
	origEng, origDry := eng, flagDryRun
	t.Cleanup(func() { eng, flagDryRun = origEng, origDry })

	dup := scan.ScanEntry{Path: "/h/Movies/a.mp4", Description: "a.mp4", Size: 1000}
	eng = engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{ID: "homedir", Name: "Home Files"}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{
			{Category: "home-large-files", Description: "Large Files", Entries: []scan.ScanEntry{dup}, TotalSize: 1000},
			{Category: "home-old-files", Description: "Old Files", Entries: []scan.ScanEntry{dup}, TotalSize: 1000},
		}, nil
	}))
	flagDryRun = true
}

func TestScanCmd_VerboseOverlapsAfterFiltering(t *testing.T) {
	const heading = "\nCounted More Than Once\n"
	out := runScanDryRunVerbose(t, true, true)
	if !strings.Contains(out, heading) || !strings.Contains(out, "also counted in [Large Files]") {
		t.Errorf("expected overlap section when both categories are scanned:\n%s", out)
	}
	// The section follows the dry-run summary.
	if strings.Index(out, heading) < strings.Index(out, "Dry-Run Summary") {
		t.Errorf("overlap section should follow the dry-run summary:\n%s", out)
	}

	// Only --large-files: Old Files is filtered out, so nothing is counted twice.
	out = runScanDryRunVerbose(t, true, false)
	if strings.Contains(out, heading) || strings.Contains(out, "Old Files") {
		t.Errorf("no overlap expected when only one category is scanned:\n%s", out)
	}
}

func TestRootCmd_VerboseOverlapsDryRun(t *testing.T) {
	const heading = "\nCounted More Than Once\n"

	t.Run("group flag", func(t *testing.T) {
		withVerbose(t, true)
		fakeHomedirEngine(t)
		orig := flagHomeFiles
		flagHomeFiles = true
		t.Cleanup(func() { flagHomeFiles = orig })

		out := captureStdout(t, func() { rootCmd.Run(rootCmd, nil) })
		if !strings.Contains(out, heading) || !strings.Contains(out, "Counted once: 1.0 kB of 2.0 kB total") {
			t.Errorf("expected overlap section:\n%s", out)
		}
	})

	t.Run("interactive dry run", func(t *testing.T) {
		withVerbose(t, true)
		fakeHomedirEngine(t)

		out := captureStdout(t, func() { rootCmd.Run(rootCmd, nil) })
		if !strings.Contains(out, heading) {
			t.Errorf("expected overlap section:\n%s", out)
		}
		if strings.Index(out, heading) < strings.Index(out, "Dry-Run Summary") {
			t.Errorf("overlap section should follow the dry-run summary:\n%s", out)
		}
	})
}

func TestVerboseOverlapsBeforeCleanup(t *testing.T) {
	for _, jsonOut := range []bool{false, true} {
		name := "text"
		if jsonOut {
			name = "json"
		}
		t.Run("scan/"+name, func(t *testing.T) {
			assertOverlapsBeforeCleanup(t, jsonOut, func() { scanCmd.Run(scanCmd, nil) })
		})
		t.Run("root/"+name, func(t *testing.T) {
			orig := flagHomeFiles
			flagHomeFiles = true
			t.Cleanup(func() { flagHomeFiles = orig })
			assertOverlapsBeforeCleanup(t, jsonOut, func() { rootCmd.Run(rootCmd, nil) })
		})
	}
}

// assertOverlapsBeforeCleanup runs a forced (non-dry-run) cleanup of a temp
// file listed in two categories. In text mode it checks the overlap section
// is printed before the cleanup summary; with --json it checks the section
// is not appended to the JSON output.
func assertOverlapsBeforeCleanup(t *testing.T, jsonOut bool, run func()) {
	t.Helper()
	withVerbose(t, true)
	origEng, origDry, origForce, origLarge, origOld, origJSON := eng, flagDryRun, flagForce, flagScanLargeFiles, flagScanOldFiles, flagJSON
	t.Cleanup(func() {
		eng, flagDryRun, flagForce, flagScanLargeFiles, flagScanOldFiles, flagJSON = origEng, origDry, origForce, origLarge, origOld, origJSON
	})
	flagJSON = jsonOut

	file := filepath.Join(t.TempDir(), "a.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dup := scan.ScanEntry{Path: file, Description: "a.mp4", Size: 1}
	eng = engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{ID: "homedir", Name: "Home Files"}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{
			{Category: "home-large-files", Description: "Large Files", Entries: []scan.ScanEntry{dup}, TotalSize: 1},
			{Category: "home-old-files", Description: "Old Files", Entries: []scan.ScanEntry{dup}, TotalSize: 1},
		}, nil
	}))
	flagDryRun, flagForce, flagScanLargeFiles, flagScanOldFiles = false, true, true, true

	var out string
	captureStderr(t, func() {
		out = captureStdout(t, run)
	})
	section := strings.Index(out, "\nCounted More Than Once\n")
	if jsonOut {
		if section >= 0 {
			t.Errorf("overlap section must not be printed with --json:\n%s", out)
		}
		return
	}
	if section < 0 || section > strings.Index(out, "Cleanup complete") {
		t.Errorf("expected overlap section before cleanup summary:\n%s", out)
	}
}
