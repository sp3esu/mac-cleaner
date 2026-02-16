package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// writeFile is a test helper that creates a file with the given size,
// creating parent directories as needed.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func TestScanSafariMissing(t *testing.T) {
	home := t.TempDir()
	result := scanSafari(home)
	if result != nil {
		t.Fatal("expected nil for missing Safari cache")
	}
}

func TestScanSafariWithData(t *testing.T) {
	home := t.TempDir()
	safariDir := filepath.Join(home, "Library", "Caches", "com.apple.Safari")
	writeFile(t, filepath.Join(safariDir, "cache.db"), 1000)
	writeFile(t, filepath.Join(safariDir, "Webpage Previews", "thumb.jpg"), 500)

	result := scanSafari(home)
	if result == nil {
		t.Fatal("expected non-nil result for Safari with data")
	}

	if result.Category != "browser-safari" {
		t.Errorf("expected category 'browser-safari', got %q", result.Category)
	}
	if result.Description != "Safari Cache" {
		t.Errorf("expected description 'Safari Cache', got %q", result.Description)
	}

	expectedSize := int64(1500)
	if result.TotalSize != expectedSize {
		t.Errorf("expected total size %d, got %d", expectedSize, result.TotalSize)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Description != "com.apple.Safari" {
		t.Errorf("expected entry description 'com.apple.Safari', got %q", result.Entries[0].Description)
	}
}

func TestScanSafariEmptyDir(t *testing.T) {
	home := t.TempDir()
	safariDir := filepath.Join(home, "Library", "Caches", "com.apple.Safari")
	os.MkdirAll(safariDir, 0755)

	result := scanSafari(home)
	if result != nil {
		t.Fatal("expected nil for empty Safari cache directory")
	}
}

func TestScanChromeMissing(t *testing.T) {
	home := t.TempDir()
	result := scanChrome(home)
	if result != nil {
		t.Fatal("expected nil for missing Chrome cache")
	}
}

func TestScanChromeWithData(t *testing.T) {
	home := t.TempDir()
	chromeDir := filepath.Join(home, "Library", "Caches", "Google", "Chrome")
	writeFile(t, filepath.Join(chromeDir, "Default", "Cache", "data_0"), 800)

	result := scanChrome(home)
	if result == nil {
		t.Fatal("expected non-nil result for Chrome with data")
	}

	if result.Category != "browser-chrome" {
		t.Errorf("expected category 'browser-chrome', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Description != "Chrome (Default)" {
		t.Errorf("expected 'Chrome (Default)', got %q", result.Entries[0].Description)
	}
	if result.Entries[0].Size != 800 {
		t.Errorf("expected size 800, got %d", result.Entries[0].Size)
	}
}

func TestScanChromeMultipleProfiles(t *testing.T) {
	home := t.TempDir()
	chromeDir := filepath.Join(home, "Library", "Caches", "Google", "Chrome")
	writeFile(t, filepath.Join(chromeDir, "Default", "Cache", "data_0"), 500)
	writeFile(t, filepath.Join(chromeDir, "Profile 1", "Cache", "data_0"), 300)

	result := scanChrome(home)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries (Default + Profile 1), got %d", len(result.Entries))
	}

	// Should be sorted by size descending: Default (500) first.
	if result.Entries[0].Size != 500 {
		t.Errorf("expected first entry size 500, got %d", result.Entries[0].Size)
	}
	if result.Entries[1].Size != 300 {
		t.Errorf("expected second entry size 300, got %d", result.Entries[1].Size)
	}

	expectedTotal := int64(800)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}
}

func TestScanChromeSkipsZeroByte(t *testing.T) {
	home := t.TempDir()
	chromeDir := filepath.Join(home, "Library", "Caches", "Google", "Chrome")
	// Create a non-empty profile and an empty one.
	writeFile(t, filepath.Join(chromeDir, "Default", "Cache", "data_0"), 500)
	os.MkdirAll(filepath.Join(chromeDir, "EmptyProfile"), 0755)

	result := scanChrome(home)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (empty profile skipped), got %d", len(result.Entries))
	}
}

func TestScanFirefoxMissing(t *testing.T) {
	home := t.TempDir()
	result := scanFirefox(home)
	if result != nil {
		t.Fatal("expected nil for missing Firefox cache")
	}
}

func TestScanFirefoxWithData(t *testing.T) {
	home := t.TempDir()
	firefoxDir := filepath.Join(home, "Library", "Caches", "Firefox")
	writeFile(t, filepath.Join(firefoxDir, "Profiles", "abc123.default", "cache2", "entries", "data.bin"), 700)

	result := scanFirefox(home)
	if result == nil {
		t.Fatal("expected non-nil result for Firefox with data")
	}

	if result.Category != "browser-firefox" {
		t.Errorf("expected category 'browser-firefox', got %q", result.Category)
	}
	if result.Description != "Firefox Cache" {
		t.Errorf("expected description 'Firefox Cache', got %q", result.Description)
	}

	if len(result.Entries) < 1 {
		t.Fatal("expected at least 1 entry")
	}
	if result.TotalSize != 700 {
		t.Errorf("expected total size 700, got %d", result.TotalSize)
	}
}

func TestScanFirefoxEmptyDir(t *testing.T) {
	home := t.TempDir()
	firefoxDir := filepath.Join(home, "Library", "Caches", "Firefox")
	os.MkdirAll(firefoxDir, 0755)

	result := scanFirefox(home)
	if result != nil {
		t.Fatal("expected nil for empty Firefox cache directory")
	}
}

func TestScanIntegration(t *testing.T) {
	// Use a temp dir that simulates a home with Chrome and Firefox but no Safari.
	home := t.TempDir()

	// Chrome with one profile.
	chromeDir := filepath.Join(home, "Library", "Caches", "Google", "Chrome")
	writeFile(t, filepath.Join(chromeDir, "Default", "Cache", "data_0"), 400)

	// Firefox with a profile.
	firefoxDir := filepath.Join(home, "Library", "Caches", "Firefox")
	writeFile(t, filepath.Join(firefoxDir, "Profiles", "test.default", "cache2", "entries.bin"), 300)

	// Call the private helpers directly since Scan() uses os.UserHomeDir().
	var results []scan.CategoryResult
	if cr := scanSafari(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanChrome(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanFirefox(home); cr != nil {
		results = append(results, *cr)
	}

	// Safari silently skipped (no cache dir).
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Chrome + Firefox), got %d", len(results))
	}

	if results[0].Category != "browser-chrome" {
		t.Errorf("expected first result 'browser-chrome', got %q", results[0].Category)
	}
	if results[1].Category != "browser-firefox" {
		t.Errorf("expected second result 'browser-firefox', got %q", results[1].Category)
	}
}

func TestScanEmptyHome(t *testing.T) {
	home := t.TempDir()

	// Call the private helpers directly.
	var results []scan.CategoryResult
	if cr := scanSafari(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanChrome(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanFirefox(home); cr != nil {
		results = append(results, *cr)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty home, got %d", len(results))
	}
}
