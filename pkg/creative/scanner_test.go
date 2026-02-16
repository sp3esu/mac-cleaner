package creative

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

// --- Adobe Caches tests ---

func TestScanAdobeCachesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanAdobeCaches(home)
	if result != nil {
		t.Fatal("expected nil for missing Adobe Caches")
	}
}

func TestScanAdobeCachesWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "Adobe")
	writeFile(t, filepath.Join(dir, "Photoshop", "cache.db"), 3000)
	writeFile(t, filepath.Join(dir, "Premiere Pro", "cache.db"), 5000)

	result := scanAdobeCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for Adobe Caches with data")
	}
	if result.Category != "creative-adobe" {
		t.Errorf("expected category 'creative-adobe', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 8000 {
		t.Errorf("expected total size 8000, got %d", result.TotalSize)
	}
}

func TestScanAdobeCachesEmptyDir(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "Adobe")
	os.MkdirAll(dir, 0755)

	result := scanAdobeCaches(home)
	if result != nil {
		t.Fatal("expected nil for empty Adobe Caches directory")
	}
}

// --- Adobe Media Cache tests ---

func TestScanAdobeMediaCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanAdobeMediaCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Adobe Media Cache")
	}
}

func TestScanAdobeMediaCacheWithData(t *testing.T) {
	home := t.TempDir()
	cacheFiles := filepath.Join(home, "Library", "Application Support", "Adobe", "Common", "Media Cache Files")
	cache := filepath.Join(home, "Library", "Application Support", "Adobe", "Common", "Media Cache")
	writeFile(t, filepath.Join(cacheFiles, "peak.pek"), 4000)
	writeFile(t, filepath.Join(cache, "index.db"), 2000)

	result := scanAdobeMediaCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Adobe Media Cache with data")
	}
	if result.Category != "creative-adobe-media" {
		t.Errorf("expected category 'creative-adobe-media', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 6000 {
		t.Errorf("expected total size 6000, got %d", result.TotalSize)
	}
}

func TestScanAdobeMediaCachePartial(t *testing.T) {
	home := t.TempDir()
	// Only one of the two directories exists.
	cacheFiles := filepath.Join(home, "Library", "Application Support", "Adobe", "Common", "Media Cache Files")
	writeFile(t, filepath.Join(cacheFiles, "peak.pek"), 3000)

	result := scanAdobeMediaCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for partial Adobe Media Cache")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 3000 {
		t.Errorf("expected total size 3000, got %d", result.TotalSize)
	}
}

// --- Sketch Cache tests ---

func TestScanSketchCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanSketchCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Sketch Cache")
	}
}

func TestScanSketchCacheWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "com.bohemiancoding.sketch3")
	writeFile(t, filepath.Join(dir, "thumbnails", "thumb1.png"), 1000)
	writeFile(t, filepath.Join(dir, "thumbnails", "thumb2.png"), 2000)

	result := scanSketchCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Sketch Cache with data")
	}
	if result.Category != "creative-sketch" {
		t.Errorf("expected category 'creative-sketch', got %q", result.Category)
	}
	// Sketch cache is treated as a single blob.
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 3000 {
		t.Errorf("expected total size 3000, got %d", result.TotalSize)
	}
}

// --- Figma Cache tests ---

func TestScanFigmaCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanFigmaCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Figma Cache")
	}
}

func TestScanFigmaCacheWithData(t *testing.T) {
	home := t.TempDir()
	profile := filepath.Join(home, "Library", "Application Support", "Figma", "DesktopProfile")
	desktop := filepath.Join(home, "Library", "Application Support", "Figma", "Desktop")
	writeFile(t, filepath.Join(profile, "Cache", "data_0"), 2000)
	writeFile(t, filepath.Join(desktop, "plugin_cache", "plugin.js"), 1000)

	result := scanFigmaCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Figma Cache with data")
	}
	if result.Category != "creative-figma" {
		t.Errorf("expected category 'creative-figma', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 3000 {
		t.Errorf("expected total size 3000, got %d", result.TotalSize)
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create Adobe caches.
	adobeDir := filepath.Join(home, "Library", "Caches", "Adobe")
	writeFile(t, filepath.Join(adobeDir, "Photoshop", "cache.db"), 1000)

	// Create Sketch cache.
	sketchDir := filepath.Join(home, "Library", "Caches", "com.bohemiancoding.sketch3")
	writeFile(t, filepath.Join(sketchDir, "thumb.png"), 500)

	// No Figma, no Adobe Media Cache -- should be silently skipped.

	var results []scan.CategoryResult
	if cr := scanAdobeCaches(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanAdobeMediaCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanSketchCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanFigmaCache(home); cr != nil {
		results = append(results, *cr)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Adobe + Sketch), got %d", len(results))
	}
	if results[0].Category != "creative-adobe" {
		t.Errorf("expected first result 'creative-adobe', got %q", results[0].Category)
	}
	if results[1].Category != "creative-sketch" {
		t.Errorf("expected second result 'creative-sketch', got %q", results[1].Category)
	}
}
