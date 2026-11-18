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
