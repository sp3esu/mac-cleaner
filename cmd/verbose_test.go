package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// withVerbose enables verbose, plain-text output for the duration of a test
// and resets the once-per-run legend flag.
func withVerbose(t *testing.T, verbose bool) {
	t.Helper()
	oldVerbose, oldLegend, oldNoColor := flagVerbose, verboseLegendShown, color.NoColor
	flagVerbose, verboseLegendShown, color.NoColor = verbose, false, true
	t.Cleanup(func() {
		flagVerbose, verboseLegendShown, color.NoColor = oldVerbose, oldLegend, oldNoColor
	})
}

// compositeResults returns one category with a composite entry (bundle plus
// a library path that cleanup does not remove) and a plain entry.
func compositeResults() []scan.CategoryResult {
	return []scan.CategoryResult{{
		Category:    "unused-apps",
		Description: "Unused Applications",
		Entries: []scan.ScanEntry{
			{
				Path:        "/tmp/vt/Big.app",
				Description: "Big",
				Size:        3000,
				Components: []scan.SizeComponent{
					{Path: "/tmp/vt/Big.app", Size: 1000, Removed: true, Note: "app bundle"},
					{Path: "/tmp/vt/Library/Containers/big", Size: 2000, Note: "app data in ~/Library"},
				},
			},
			{Path: "/tmp/vt/plain", Description: "plain", Size: 500},
		},
		TotalSize: 3500,
	}}
}

func TestPrintResults_VerboseBreakdown(t *testing.T) {
	withVerbose(t, true)

	out := captureStdout(t, func() {
		printResults(compositeResults(), true, "Title")
	})

	for _, want := range []string{
		"Sizes are apparent file sizes",
		"        /tmp/vt/Big.app\n",
		"2.0 kB  /tmp/vt/Library/Containers/big  (counted, not removed by cleanup)  - app data in ~/Library",
		"1.0 kB  /tmp/vt/Big.app  - app bundle\n",
		"cleanup removes 1.0 kB of 3.0 kB counted",
		"        /tmp/vt/plain\n",
		"Total: 3.5 kB reclaimable",
		"cleanup removes 1.5 kB of 3.5 kB counted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}

	// Largest component first.
	if strings.Index(out, "Library/Containers/big") > strings.Index(out, "1.0 kB  /tmp/vt/Big.app") {
		t.Errorf("components not sorted by size desc:\n%s", out)
	}
	// The removed bundle must not carry the not-removed marker.
	if strings.Contains(out, "/tmp/vt/Big.app  (counted, not removed") {
		t.Errorf("removed component wrongly marked:\n%s", out)
	}
	// Plain entry has no breakdown, so only one "cleanup removes" line per composite entry plus the total.
	if n := strings.Count(out, "cleanup removes"); n != 2 {
		t.Errorf("expected 2 'cleanup removes' lines, got %d:\n%s", n, out)
	}
}

func TestPrintResults_VerboseLegendOnce(t *testing.T) {
	withVerbose(t, true)

	out := captureStdout(t, func() {
		printResults(compositeResults(), true, "First")
		printResults(compositeResults(), true, "Second")
	})
	if n := strings.Count(out, "Sizes are apparent file sizes"); n != 1 {
		t.Errorf("legend printed %d times, want 1:\n%s", n, out)
	}
}

func TestPrintResults_VerboseNoComponents(t *testing.T) {
	withVerbose(t, true)

	results := []scan.CategoryResult{{
		Category:    "system-caches",
		Description: "Caches",
		Entries:     []scan.ScanEntry{{Path: "/tmp/vt/cache", Description: "cache", Size: 100}},
		TotalSize:   100,
	}}
	out := captureStdout(t, func() {
		printResults(results, true, "Title")
	})
	if !strings.Contains(out, "        /tmp/vt/cache\n") {
		t.Errorf("expected path line, got:\n%s", out)
	}
	if strings.Contains(out, "cleanup removes") || strings.Contains(out, "not removed by cleanup") {
		t.Errorf("plain entry must not print breakdown lines:\n%s", out)
	}
}

func TestPrintResults_NonVerboseUnchanged(t *testing.T) {
	withVerbose(t, false)

	out := captureStdout(t, func() {
		printResults(compositeResults(), true, "Title")
	})
	for _, unwanted := range []string{"Sizes are apparent", "/tmp/vt/Library", "cleanup removes", "not removed"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("non-verbose output contains %q:\n%s", unwanted, out)
		}
	}
	if !strings.Contains(out, "Total: 3.5 kB reclaimable") {
		t.Errorf("expected total line, got:\n%s", out)
	}
}

func TestPrintBreakdown_ZeroSizeNotRemoved(t *testing.T) {
	withVerbose(t, true)

	var buf bytes.Buffer
	printBreakdown(&buf, scan.ScanEntry{
		Path: "tmutil:snapshot:x",
		Components: []scan.SizeComponent{
			{Path: "tmutil:snapshot:x", Note: "size unavailable"},
		},
	}, "")
	out := buf.String()
	if !strings.Contains(out, "0 B  tmutil:snapshot:x  (not removed by cleanup)  - size unavailable") {
		t.Errorf("unexpected breakdown:\n%s", out)
	}
	// Size equals removed size (both 0), so no summary line.
	if strings.Contains(out, "cleanup removes") {
		t.Errorf("unexpected summary line:\n%s", out)
	}
}

func TestPrintBreakdown_ShortensHome(t *testing.T) {
	withVerbose(t, true)

	var buf bytes.Buffer
	printBreakdown(&buf, scan.ScanEntry{
		Path: "/h/App.app",
		Size: 10,
		Components: []scan.SizeComponent{
			{Path: "/h/Library/Caches/x", Size: 10},
		},
	}, "/h")
	if !strings.Contains(buf.String(), "~/Library/Caches/x  (counted, not removed by cleanup)\n") {
		t.Errorf("expected shortened path without note, got:\n%s", buf.String())
	}
}

func TestCleanupProgress_VerboseReportsNotRemoved(t *testing.T) {
	withVerbose(t, true)
	oldJSON := flagJSON
	flagJSON = false
	defer func() { flagJSON = oldJSON }()

	var buf bytes.Buffer
	cb := cleanupProgress(nil, &buf, compositeResults())
	cb("Unused Applications", "", 1, 2)
	cb("Unused Applications", "/tmp/vt/Big.app", 1, 2)
	cb("Unused Applications", "/tmp/vt/plain", 2, 2)

	out := buf.String()
	if !strings.Contains(out, "  removing /tmp/vt/Big.app\n    not removed: /tmp/vt/Library/Containers/big (2.0 kB) - app data in ~/Library\n") {
		t.Errorf("expected not-removed component after entry, got:\n%s", out)
	}
	if strings.Contains(out, "not removed: /tmp/vt/Big.app") {
		t.Errorf("removed bundle must not be reported as not removed:\n%s", out)
	}
	if strings.Count(out, "not removed:") != 1 {
		t.Errorf("expected exactly one not-removed line, got:\n%s", out)
	}
}

func TestCleanupProgress_VerboseNoteOmittedWhenEmpty(t *testing.T) {
	withVerbose(t, true)
	oldJSON := flagJSON
	flagJSON = false
	defer func() { flagJSON = oldJSON }()

	results := []scan.CategoryResult{{Entries: []scan.ScanEntry{{
		Path:       "docker:Images",
		Size:       5,
		Components: []scan.SizeComponent{{Path: "docker:Images", Size: 5}},
	}}}}
	var buf bytes.Buffer
	cleanupProgress(nil, &buf, results)("Docker", "docker:Images", 1, 1)
	if !strings.HasSuffix(buf.String(), "    not removed: docker:Images (5 B)\n") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}
