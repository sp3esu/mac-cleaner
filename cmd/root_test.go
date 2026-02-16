package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// --- printDryRunSummary tests ---

func TestPrintDryRunSummary_MultipleCategoriesSortedBySize(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		{Category: "a", Description: "Small Cat", TotalSize: 300_000_000},
		{Category: "b", Description: "Big Cat", TotalSize: 2_300_000_000},
		{Category: "c", Description: "Medium Cat", TotalSize: 1_100_000_000},
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	// Verify header present.
	if !strings.Contains(out, "Dry-Run Summary") {
		t.Errorf("expected header, got: %s", out)
	}
	// Verify descending order: Big Cat before Medium Cat before Small Cat.
	bigIdx := strings.Index(out, "Big Cat")
	medIdx := strings.Index(out, "Medium Cat")
	smallIdx := strings.Index(out, "Small Cat")
	if bigIdx < 0 || medIdx < 0 || smallIdx < 0 {
		t.Fatalf("expected all categories in output, got: %s", out)
	}
	if bigIdx > medIdx || medIdx > smallIdx {
		t.Errorf("categories not sorted descending by size, got: %s", out)
	}
	// Verify percentages exist.
	if !strings.Contains(out, "%") {
		t.Errorf("expected percentage in output, got: %s", out)
	}
	// Verify total line.
	if !strings.Contains(out, "Total:") || !strings.Contains(out, "reclaimable") {
		t.Errorf("expected total line, got: %s", out)
	}
}

func TestPrintDryRunSummary_SingleCategory_NoOutput(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		{Category: "a", Description: "Only One", TotalSize: 500_000_000},
	}
	printDryRunSummary(&buf, results)
	if buf.Len() != 0 {
		t.Errorf("expected no output for single category, got: %s", buf.String())
	}
}

func TestPrintDryRunSummary_EmptyResults_NoOutput(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	printDryRunSummary(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil results, got: %s", buf.String())
	}

	buf.Reset()
	printDryRunSummary(&buf, []scan.CategoryResult{})
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty results, got: %s", buf.String())
	}
}

func TestPrintDryRunSummary_ZeroSizeCategoriesFiltered(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		{Category: "a", Description: "Has Data", TotalSize: 1_000_000_000},
		{Category: "b", Description: "Empty One", TotalSize: 0},
		{Category: "c", Description: "Also Data", TotalSize: 500_000_000},
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	if strings.Contains(out, "Empty One") {
		t.Errorf("zero-size category should be filtered, got: %s", out)
	}
	if !strings.Contains(out, "Has Data") || !strings.Contains(out, "Also Data") {
		t.Errorf("non-zero categories should be present, got: %s", out)
	}
}

func TestPrintDryRunSummary_ExactlyTwoCategories(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		{Category: "a", Description: "First", TotalSize: 200_000_000},
		{Category: "b", Description: "Second", TotalSize: 800_000_000},
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	if !strings.Contains(out, "Dry-Run Summary") {
		t.Errorf("expected summary for exactly 2 categories, got: %s", out)
	}
	// Second should appear first (larger).
	secondIdx := strings.Index(out, "Second")
	firstIdx := strings.Index(out, "First")
	if secondIdx > firstIdx {
		t.Errorf("expected Second before First (sorted by size desc), got: %s", out)
	}
}

func TestPrintDryRunSummary_TwoCategoriesOneEmpty_NoOutput(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		{Category: "a", Description: "Has Data", TotalSize: 500_000_000},
		{Category: "b", Description: "No Data", TotalSize: 0},
	}
	printDryRunSummary(&buf, results)
	if buf.Len() != 0 {
		t.Errorf("expected no output when only 1 non-empty category, got: %s", buf.String())
	}
}

func TestPrintCleanupSummary_NoFailures(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	result := cleanup.CleanupResult{Removed: 5, BytesFreed: 1000000, Failed: 0}
	printCleanupSummary(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "5 items removed") {
		t.Errorf("expected removed count, got: %s", out)
	}
	if strings.Contains(out, "failed") {
		t.Errorf("should not mention failures when none exist, got: %s", out)
	}
}

func TestPrintCleanupSummary_WithFailures(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	result := cleanup.CleanupResult{
		Removed:    3,
		BytesFreed: 500000,
		Failed:     2,
		Errors: []error{
			errors.New("skip non-filesystem path: docker:BuildCache"),
			errors.New("remove /tmp/locked: permission denied"),
		},
	}
	printCleanupSummary(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "3 items removed") {
		t.Errorf("expected removed count, got: %s", out)
	}
	if !strings.Contains(out, "2 items failed:") {
		t.Errorf("expected failure header, got: %s", out)
	}
	if !strings.Contains(out, "  - skip non-filesystem path: docker:BuildCache") {
		t.Errorf("expected first error listed, got: %s", out)
	}
	if !strings.Contains(out, "  - remove /tmp/locked: permission denied") {
		t.Errorf("expected second error listed, got: %s", out)
	}
	if strings.Contains(out, "see warnings above") {
		t.Errorf("should not contain old message, got: %s", out)
	}
}

// captureStdout redirects os.Stdout and color.Output to a pipe and returns
// the captured output. Both must be redirected because the color package
// caches its own output writer at init time.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	oldColorOut := color.Output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	color.Output = w

	fn()

	w.Close()
	os.Stdout = oldStdout
	color.Output = oldColorOut

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr redirects os.Stderr to a pipe and returns the captured output.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- filterSkipped tests ---

func TestFilterSkipped_EmptySkipSet(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "a"},
		{Category: "b"},
	}
	got := filterSkipped(results, map[string]bool{})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_SingleSkip(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "a"},
		{Category: "b"},
		{Category: "c"},
	}
	got := filterSkipped(results, map[string]bool{"b": true})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
	for _, r := range got {
		if r.Category == "b" {
			t.Error("category 'b' should have been filtered out")
		}
	}
}

func TestFilterSkipped_MultipleSkips(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "a"},
		{Category: "b"},
		{Category: "c"},
	}
	got := filterSkipped(results, map[string]bool{"a": true, "c": true})
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Category != "b" {
		t.Errorf("expected category 'b', got %q", got[0].Category)
	}
}

func TestFilterSkipped_NonMatchingSkip(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "a"},
		{Category: "b"},
	}
	got := filterSkipped(results, map[string]bool{"z": true})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_EmptyResults(t *testing.T) {
	got := filterSkipped(nil, map[string]bool{"a": true})
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

// --- shortenHome tests ---

func TestShortenHome_ReplacesPrefix(t *testing.T) {
	got := shortenHome("/Users/test/Documents/file.txt", "/Users/test")
	if got != "~/Documents/file.txt" {
		t.Errorf("expected ~/Documents/file.txt, got %q", got)
	}
}

func TestShortenHome_EmptyHome(t *testing.T) {
	got := shortenHome("/some/path", "")
	if got != "/some/path" {
		t.Errorf("expected /some/path, got %q", got)
	}
}

func TestShortenHome_PathNotUnderHome(t *testing.T) {
	got := shortenHome("/var/log/something", "/Users/test")
	if got != "/var/log/something" {
		t.Errorf("expected /var/log/something, got %q", got)
	}
}

func TestShortenHome_HomeEqualsPath(t *testing.T) {
	got := shortenHome("/Users/test", "/Users/test")
	if got != "~" {
		t.Errorf("expected ~, got %q", got)
	}
}

// --- baseDirectory tests ---

func TestBaseDirectory(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/Users/test/Library/Caches/com.apple.cache", "/Users/test/Library/Caches"},
		{"/", "/"},
		{"/a/b/c/d/e", "/a/b/c/d"},
	}
	for _, tt := range tests {
		got := baseDirectory(tt.path)
		if got != tt.expected {
			t.Errorf("baseDirectory(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

// --- printJSON tests ---

func TestPrintJSON(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	results := []scan.CategoryResult{
		{
			Category:    "test-cat",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "a", Size: 100},
			},
			TotalSize: 100,
			PermissionIssues: []scan.PermissionIssue{
				{Path: "/tmp/b", Description: "perm denied"},
			},
		},
	}

	out := captureStdout(t, func() {
		printJSON(results)
	})

	var summary scan.ScanSummary
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(summary.Categories) != 1 {
		t.Errorf("expected 1 category, got %d", len(summary.Categories))
	}
	if summary.TotalSize != 100 {
		t.Errorf("expected total_size 100, got %d", summary.TotalSize)
	}
	if len(summary.PermissionIssues) != 1 {
		t.Errorf("expected 1 permission issue, got %d", len(summary.PermissionIssues))
	}
}

func TestPrintJSON_EmptyResults(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	out := captureStdout(t, func() {
		printJSON(nil)
	})

	var summary scan.ScanSummary
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if summary.TotalSize != 0 {
		t.Errorf("expected total_size 0, got %d", summary.TotalSize)
	}
}

// --- printResults tests ---

func TestPrintResults_Empty(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	out := captureStdout(t, func() {
		printResults(nil, false, "System Caches")
	})

	if !strings.Contains(out, "No system caches found.") {
		t.Errorf("expected 'No system caches found.', got: %s", out)
	}
}

func TestPrintResults_NonEmpty(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/test/item", Description: "item", Size: 1024},
			},
			TotalSize: 1024,
		},
	}

	out := captureStdout(t, func() {
		printResults(results, false, "Test Title")
	})

	if !strings.Contains(out, "item") {
		t.Errorf("expected entry description in output, got: %s", out)
	}
	if !strings.Contains(out, "Test Title") {
		t.Errorf("expected title in output, got: %s", out)
	}
}

func TestPrintResults_DryRun(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/test/item", Description: "item", Size: 100},
			},
			TotalSize: 100,
		},
	}

	out := captureStdout(t, func() {
		printResults(results, true, "My Title")
	})

	if !strings.Contains(out, "(dry run)") {
		t.Errorf("expected '(dry run)' in header, got: %s", out)
	}
}

// --- printPermissionIssues tests ---

func TestPrintPermissionIssues_NoIssues(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	results := []scan.CategoryResult{
		{Category: "test", Entries: []scan.ScanEntry{{Path: "/a", Size: 100}}},
	}

	out := captureStderr(t, func() {
		printPermissionIssues(results)
	})

	if out != "" {
		t.Errorf("expected no output for no permission issues, got: %s", out)
	}
}

func TestPrintPermissionIssues_WithIssues(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	results := []scan.CategoryResult{
		{
			Category: "test",
			PermissionIssues: []scan.PermissionIssue{
				{Path: "/var/private/cache", Description: "cache (permission denied)"},
			},
		},
	}

	out := captureStderr(t, func() {
		printPermissionIssues(results)
	})

	if !strings.Contains(out, "1 path(s) could not be accessed") {
		t.Errorf("expected permission issue header, got: %s", out)
	}
	if !strings.Contains(out, "/var/private/cache") {
		t.Errorf("expected path in output, got: %s", out)
	}
}
