package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/engine"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/spinner"
)

// --- flagForCategory tests ---

func TestFlagForCategory(t *testing.T) {
	tests := []struct {
		categoryID string
		want       string
	}{
		// system
		{"system-caches", "--system-caches"},
		{"system-logs", "--system-caches"},
		// quicklook special case
		{"quicklook", "--system-caches"},
		// browser
		{"browser-safari", "--browser-data"},
		{"browser-chrome", "--browser-data"},
		{"browser-firefox", "--browser-data"},
		// developer
		{"dev-xcode", "--dev-caches"},
		{"dev-npm", "--dev-caches"},
		{"dev-yarn", "--dev-caches"},
		{"dev-homebrew", "--dev-caches"},
		{"dev-docker", "--dev-caches"},
		{"dev-simulator-caches", "--dev-caches"},
		{"dev-simulator-logs", "--dev-caches"},
		{"dev-xcode-device-support", "--dev-caches"},
		{"dev-xcode-archives", "--dev-caches"},
		{"dev-pnpm", "--dev-caches"},
		{"dev-cocoapods", "--dev-caches"},
		{"dev-gradle", "--dev-caches"},
		{"dev-pip", "--dev-caches"},
		// app leftovers
		{"app-orphaned-prefs", "--app-leftovers"},
		{"app-ios-backups", "--app-leftovers"},
		{"app-old-downloads", "--app-leftovers"},
		// creative
		{"creative-adobe", "--creative-caches"},
		{"creative-adobe-media", "--creative-caches"},
		{"creative-sketch", "--creative-caches"},
		{"creative-figma", "--creative-caches"},
		// messaging
		{"msg-slack", "--messaging-caches"},
		{"msg-discord", "--messaging-caches"},
		{"msg-teams", "--messaging-caches"},
		{"msg-zoom", "--messaging-caches"},
		// unknown / empty
		{"unknown-thing", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.categoryID, func(t *testing.T) {
			got := flagForCategory(tt.categoryID)
			if got != tt.want {
				t.Errorf("flagForCategory(%q) = %q, want %q", tt.categoryID, got, tt.want)
			}
		})
	}
}

// --- printDryRunSummary tests ---

func TestPrintDryRunSummary_MultipleCategoriesSortedBySize(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		sized("a", "Small Cat", 300_000_000),
		sized("b", "Big Cat", 2_300_000_000),
		sized("c", "Medium Cat", 1_100_000_000),
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
		sized("a", "Only One", 500_000_000),
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
		sized("a", "Has Data", 1_000_000_000),
		sized("b", "Empty One", 0),
		sized("c", "Also Data", 500_000_000),
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
		sized("a", "First", 200_000_000),
		sized("b", "Second", 800_000_000),
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
		sized("a", "Has Data", 500_000_000),
		sized("b", "No Data", 0),
	}
	printDryRunSummary(&buf, results)
	if buf.Len() != 0 {
		t.Errorf("expected no output when only 1 non-empty category, got: %s", buf.String())
	}
}

func TestPrintDryRunSummary_ShowsFlagHints(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		sized("dev-npm", "npm Cache", 5_500_000_000),
		sized("system-caches", "User App Caches", 1_400_000_000),
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	if !strings.Contains(out, "(--dev-caches)") {
		t.Errorf("expected (--dev-caches) hint, got: %s", out)
	}
	if !strings.Contains(out, "(--system-caches)") {
		t.Errorf("expected (--system-caches) hint, got: %s", out)
	}
}

func TestPrintDryRunSummary_FlagHintForQuicklook(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		sized("quicklook", "QuickLook Thumbnails", 200_000_000),
		sized("system-logs", "User Logs", 300_000_000),
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	if !strings.Contains(out, "(--system-caches)") {
		t.Errorf("expected (--system-caches) hint for quicklook, got: %s", out)
	}
}

func TestPrintDryRunSummary_UnknownCategoryNoFlagHint(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		sized("unknown-a", "Unknown A", 100_000_000),
		sized("unknown-b", "Unknown B", 200_000_000),
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	if strings.Contains(out, "--") {
		t.Errorf("expected no flag hints for unknown categories, got: %s", out)
	}
}

func TestPrintDryRunSummary_AllScannerGroups(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	results := []scan.CategoryResult{
		sized("system-caches", "User App Caches", 100_000_000),
		sized("browser-safari", "Safari Cache", 100_000_000),
		sized("dev-npm", "npm Cache", 100_000_000),
		sized("app-orphaned-prefs", "Orphaned Preferences", 100_000_000),
		sized("creative-adobe", "Adobe Caches", 100_000_000),
		sized("msg-slack", "Slack Cache", 100_000_000),
	}
	printDryRunSummary(&buf, results)
	out := buf.String()

	for _, flag := range []string{
		"--system-caches",
		"--browser-data",
		"--dev-caches",
		"--app-leftovers",
		"--creative-caches",
		"--messaging-caches",
	} {
		if !strings.Contains(out, "("+flag+")") {
			t.Errorf("expected (%s) hint in output, got: %s", flag, out)
		}
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

// --- engine.FilterSkipped tests (called through engine package) ---

func TestFilterSkipped_EmptySkipSet(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "a"},
		{Category: "b"},
	}
	got := engine.FilterSkipped(results, map[string]bool{})
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
	got := engine.FilterSkipped(results, map[string]bool{"b": true})
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
	got := engine.FilterSkipped(results, map[string]bool{"a": true, "c": true})
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
	got := engine.FilterSkipped(results, map[string]bool{"z": true})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_EmptyResults(t *testing.T) {
	got := engine.FilterSkipped(nil, map[string]bool{"a": true})
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

// --- cleanupProgress tests ---

func TestCleanupProgress_JSON(t *testing.T) {
	oldJSON := flagJSON
	flagJSON = true
	defer func() { flagJSON = oldJSON }()

	var buf bytes.Buffer
	cb := cleanupProgress(nil, &buf, nil)
	if cb != nil {
		t.Error("expected nil callback in JSON mode")
	}
}

func TestCleanupProgress_Verbose(t *testing.T) {
	oldVerbose := flagVerbose
	oldJSON := flagJSON
	flagVerbose = true
	flagJSON = false
	defer func() { flagVerbose = oldVerbose; flagJSON = oldJSON }()

	var buf bytes.Buffer
	cb := cleanupProgress(nil, &buf, nil)
	if cb == nil {
		t.Fatal("expected non-nil callback in verbose mode")
	}

	// Category-start event.
	cb("User App Caches", "", 1, 5)
	// Entry-level event, for a path that passes the safety check.
	file := t.TempDir() + "/file.txt"
	cb("User App Caches", file, 1, 5)

	out := buf.String()
	if !strings.Contains(out, "Cleaning User App Caches (1/5)") {
		t.Errorf("expected category header, got: %s", out)
	}
	if !strings.Contains(out, "  removing "+file) {
		t.Errorf("expected entry path, got: %s", out)
	}
}

func TestCleanupProgress_Normal(t *testing.T) {
	oldVerbose := flagVerbose
	oldJSON := flagJSON
	flagVerbose = false
	flagJSON = false
	defer func() { flagVerbose = oldVerbose; flagJSON = oldJSON }()

	// Use a disabled spinner (all methods are safe no-ops).
	sp := spinner.New("", false)

	var buf bytes.Buffer
	cb := cleanupProgress(sp, &buf, nil)
	if cb == nil {
		t.Fatal("expected non-nil callback in normal mode")
	}

	// In normal mode, callback updates the spinner (no text written to buf).
	cb("User App Caches", "", 1, 5)
	cb("User App Caches", "/tmp/test/file.txt", 1, 5)

	if buf.Len() != 0 {
		t.Errorf("expected no output in normal mode (spinner-only), got: %s", buf.String())
	}
}

// --- Engine integration tests (regression: verify engine wiring) ---

// TestFlagScannerMappings verifies all 6 scanner IDs exist in the engine.
func TestFlagScannerMappings(t *testing.T) {
	eng := engine.New()
	engine.RegisterDefaults(eng)
	cats := eng.Categories()
	ids := map[string]bool{}
	for _, c := range cats {
		ids[c.ID] = true
	}
	expected := []string{"system", "browser", "developer", "appleftovers", "creative", "messaging"}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("expected scanner ID %q not found in engine", id)
		}
	}
}

// TestFindScannerInfo verifies scanner metadata lookup.
func TestFindScannerInfo(t *testing.T) {
	// Initialize the engine (normally done in PreRun).
	eng = engine.New()
	engine.RegisterDefaults(eng)
	defer func() { eng = nil }()

	info := findScannerInfo("system")
	if info.ID != "system" {
		t.Errorf("expected ID 'system', got %q", info.ID)
	}
	if info.Name != "System Caches" {
		t.Errorf("expected Name 'System Caches', got %q", info.Name)
	}

	// Unknown scanner returns fallback.
	unknown := findScannerInfo("nonexistent")
	if unknown.ID != "nonexistent" {
		t.Errorf("expected fallback ID 'nonexistent', got %q", unknown.ID)
	}
}

// TestEngineCategories verifies RegisterDefaults produces exactly 7 scanners.
func TestEngineCategories(t *testing.T) {
	eng := engine.New()
	engine.RegisterDefaults(eng)
	cats := eng.Categories()
	if len(cats) != 12 {
		t.Fatalf("expected 12 scanner categories, got %d", len(cats))
	}
	// Verify all have non-empty names.
	for _, c := range cats {
		if c.Name == "" {
			t.Errorf("scanner %q has empty Name", c.ID)
		}
	}
}

// sized returns a category with one plain entry of the given size, so its
// counted and removable sizes are both size.
func sized(id, desc string, size int64) scan.CategoryResult {
	return scan.CategoryResult{
		Category:    id,
		Description: desc,
		Entries:     []scan.ScanEntry{{Path: "/h/" + id, Size: size}},
		TotalSize:   size,
	}
}

func TestPrintDryRunSummary_ShowsRemovableSize(t *testing.T) {
	withVerbose(t, false)

	results := []scan.CategoryResult{
		// Counted large, deletes nothing: an /Applications bundle.
		{
			Category: "unused-apps", Description: "Unused Applications", TotalSize: 71_000_000_000,
			Entries: []scan.ScanEntry{{
				Path: "/Applications/Docker.app", Size: 71_000_000_000,
				Components: []scan.SizeComponent{{Path: "/Applications/Docker.app", Size: 71_000_000_000}},
			}},
		},
		sized("system-caches", "User App Caches", 5_000_000_000),
		// Nested in system-caches: counted twice, deleted once.
		{
			Category: "browser-chrome", Description: "Chrome Cache", TotalSize: 1_000_000_000,
			Entries: []scan.ScanEntry{{Path: "/h/system-caches/Google/Chrome", Size: 1_000_000_000}},
		},
	}
	var buf bytes.Buffer
	printDryRunSummary(&buf, results)
	out := buf.String()

	// Sorted by removable size: caches, chrome, then the 0 B app.
	caches, chrome, unused := strings.Index(out, "User App Caches"), strings.Index(out, "Chrome Cache"), strings.Index(out, "Unused Applications")
	if !(caches < chrome && chrome < unused) {
		t.Errorf("expected sort by removable size:\n%s", out)
	}
	if !regexp.MustCompile(`Unused Applications +0 B`).MatchString(out) {
		t.Errorf("unused apps should show 0 B removable:\n%s", out)
	}
	if !strings.Contains(out, "Total: 5.0 GB reclaimable (77.0 GB counted, see --verbose)") {
		t.Errorf("unexpected total line:\n%s", out)
	}
}

func TestTotalLine(t *testing.T) {
	withVerbose(t, false)
	plain := []scan.CategoryResult{sized("a", "A", 100)}
	if got := totalLine(plain); got != "Total: 100 B reclaimable" {
		t.Errorf("totalLine = %q", got)
	}
	dup := []scan.CategoryResult{sized("a", "A", 100), sized("a", "A", 100)}
	if got := totalLine(dup); got != "Total: 100 B reclaimable (200 B counted, see --verbose)" {
		t.Errorf("totalLine = %q", got)
	}
	withVerbose(t, true)
	if got := totalLine(dup); got != "Total: 100 B reclaimable (200 B counted)" {
		t.Errorf("verbose totalLine = %q", got)
	}
}

func TestPrintJSON_AddsHonestFields(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	dup := scan.ScanEntry{Path: "/h/a.mp4", Size: 100}
	results := []scan.CategoryResult{
		{Category: "home-large-files", Entries: []scan.ScanEntry{dup}, TotalSize: 100},
		{Category: "home-old-files", Entries: []scan.ScanEntry{dup}, TotalSize: 100},
		{Category: "dev-docker", TotalSize: 50, Entries: []scan.ScanEntry{{
			Path: "docker:Images", Size: 50, Components: []scan.SizeComponent{{Path: "docker:Images", Size: 50}},
		}}},
	}
	out := captureStdout(t, func() { printJSON(results) })
	var summary scan.ScanSummary
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if summary.TotalSize != 250 || summary.UniqueSize != 150 || summary.RemovableSize != 100 {
		t.Errorf("total=%d unique=%d removable=%d, want 250/150/100", summary.TotalSize, summary.UniqueSize, summary.RemovableSize)
	}
	if summary.Categories[2].RemovableSize != 0 || summary.Categories[0].RemovableSize != 100 {
		t.Errorf("unexpected category removable sizes: %+v", summary.Categories)
	}
}

func TestPrintCleanupSummary_SkippedSeparateFromFailures(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	printCleanupSummary(&buf, cleanup.CleanupResult{Removed: 1, Skipped: 3, BytesFreed: 10})
	out := buf.String()
	if !strings.Contains(out, "3 items skipped") {
		t.Errorf("expected skipped count, got: %s", out)
	}
	if strings.Contains(out, "failed") {
		t.Errorf("skipped items are not failures, got: %s", out)
	}
}

func TestRootCmd_SkipFlagsAppliedBeforePrinting(t *testing.T) {
	withVerbose(t, false)
	origEng, origDry, origDev, origSkip := eng, flagDryRun, flagDevCaches, flagSkipNpm
	t.Cleanup(func() { eng, flagDryRun, flagDevCaches, flagSkipNpm = origEng, origDry, origDev, origSkip })

	eng = engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{ID: "developer", Name: "Developer Caches"}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{
			{Category: "dev-npm", Description: "npm Cache", Entries: []scan.ScanEntry{{Path: "/h/.npm", Description: "npm-entry", Size: 1_200_000}}, TotalSize: 1_200_000},
			{Category: "dev-yarn", Description: "Yarn Cache", Entries: []scan.ScanEntry{{Path: "/h/yarn", Description: "yarn-entry", Size: 300_000}}, TotalSize: 300_000},
		}, nil
	}))

	t.Run("group flag", func(t *testing.T) {
		flagDryRun, flagDevCaches, flagSkipNpm = true, true, true
		out := captureStdout(t, func() { rootCmd.Run(rootCmd, nil) })
		assertNpmSkipped(t, out)
	})
	t.Run("interactive dry run", func(t *testing.T) {
		flagDryRun, flagDevCaches, flagSkipNpm = true, false, true
		out := captureStdout(t, func() { rootCmd.Run(rootCmd, nil) })
		assertNpmSkipped(t, out)
	})
}

func assertNpmSkipped(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "npm") {
		t.Errorf("skipped npm category printed:\n%s", out)
	}
	if !strings.Contains(out, "yarn-entry") || !strings.Contains(out, "Total: 300.0 kB reclaimable\n") {
		t.Errorf("expected only yarn in table and total:\n%s", out)
	}
	if strings.Contains(out, "1.5 MB") {
		t.Errorf("total includes the skipped category:\n%s", out)
	}
}

func TestJSONWithoutDryRun_StdoutIsOneJSONDocument(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "prompt"
		if force {
			name = "force"
		}
		t.Run("scan/"+name, func(t *testing.T) {
			assertJSONStdout(t, force, func() { scanCmd.Run(scanCmd, nil) })
		})
		t.Run("root/"+name, func(t *testing.T) {
			orig := flagHomeFiles
			flagHomeFiles = true
			t.Cleanup(func() { flagHomeFiles = orig })
			assertJSONStdout(t, force, func() { rootCmd.Run(rootCmd, nil) })
		})
	}
}

// assertJSONStdout runs a --json cleanup (without --dry-run) of a temp file
// and checks stdout is exactly one JSON document, with the prompt, "Aborted."
// and the cleanup summary on stderr.
func assertJSONStdout(t *testing.T, force bool, run func()) {
	t.Helper()
	withVerbose(t, false)
	origEng, origDry, origForce, origLarge, origJSON, origStdin := eng, flagDryRun, flagForce, flagScanLargeFiles, flagJSON, os.Stdin
	t.Cleanup(func() {
		eng, flagDryRun, flagForce, flagScanLargeFiles, flagJSON, os.Stdin = origEng, origDry, origForce, origLarge, origJSON, origStdin
	})

	// Answer the prompt with "no".
	stdin, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = stdin.WriteString("no\n")
	_, _ = stdin.Seek(0, 0)
	os.Stdin = stdin

	file := t.TempDir() + "/a.mp4"
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng = engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{ID: "homedir", Name: "Home Files"}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{{
			Category: "home-large-files", Description: "Large Files",
			Entries: []scan.ScanEntry{{Path: file, Description: "a.mp4", Size: 1}}, TotalSize: 1,
		}}, nil
	}))
	flagJSON, flagDryRun, flagForce, flagScanLargeFiles = true, false, force, true

	var stdout string
	stderr := captureStderr(t, func() { stdout = captureStdout(t, run) })

	dec := json.NewDecoder(strings.NewReader(stdout))
	var summary scan.ScanSummary
	if err := dec.Decode(&summary); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if dec.More() {
		t.Errorf("stdout holds more than one JSON document:\n%s", stdout)
	}
	if len(summary.Categories) != 1 {
		t.Errorf("unexpected JSON: %+v", summary)
	}
	_, statErr := os.Stat(file)
	if force {
		if !strings.Contains(stderr, "Cleanup complete") || !os.IsNotExist(statErr) {
			t.Errorf("forced cleanup: file still there (%v) or no summary on stderr:\n%s", statErr, stderr)
		}
		return
	}
	if !strings.Contains(stderr, "The following items will be permanently deleted:") || !strings.Contains(stderr, "Aborted.") {
		t.Errorf("expected prompt and Aborted. on stderr:\n%s", stderr)
	}
	if statErr != nil {
		t.Errorf("aborted cleanup must not delete: %v", statErr)
	}
}
