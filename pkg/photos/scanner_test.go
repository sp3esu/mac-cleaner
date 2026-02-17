package photos

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

// --- Photos Caches tests ---

func TestScanPhotosCachesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanPhotosCaches(home)
	if result != nil {
		t.Fatal("expected nil for missing Photos caches")
	}
}

func TestScanPhotosCachesEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanPhotosCaches(home)
	if result != nil {
		t.Fatal("expected nil for empty Photos caches directory")
	}
}

func TestScanPhotosCachesWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(dir, "com.apple.Photos", "cache.db"), 5000)

	result := scanPhotosCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for Photos caches with data")
	}
	if result.Category != "photos-caches" {
		t.Errorf("expected category 'photos-caches', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 5000 {
		t.Errorf("expected total size 5000, got %d", result.TotalSize)
	}
}

func TestScanPhotosCachesPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// Remove read permission on the parent to prevent stat.
	parent := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanPhotosCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Analysis Caches tests ---

func TestScanAnalysisCachesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanAnalysisCaches(home)
	if result != nil {
		t.Fatal("expected nil for missing analysis caches")
	}
}

func TestScanAnalysisCachesEmpty(t *testing.T) {
	home := t.TempDir()
	dir1 := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches")
	dir2 := filepath.Join(home, "Library", "Containers", "com.apple.photoanalysisd", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanAnalysisCaches(home)
	if result != nil {
		t.Fatal("expected nil for empty analysis caches directories")
	}
}

func TestScanAnalysisCachesWithData(t *testing.T) {
	home := t.TempDir()
	dir1 := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches")
	dir2 := filepath.Join(home, "Library", "Containers", "com.apple.photoanalysisd", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(dir1, "model.mlmodelc"), 8000)
	writeFile(t, filepath.Join(dir2, "faces.db"), 4000)

	result := scanAnalysisCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for analysis caches with data")
	}
	if result.Category != "photos-analysis" {
		t.Errorf("expected category 'photos-analysis', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 12000 {
		t.Errorf("expected total size 12000, got %d", result.TotalSize)
	}
}

func TestScanAnalysisCachesPartial(t *testing.T) {
	home := t.TempDir()
	// Only mediaanalysisd exists.
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(dir, "model.mlmodelc"), 6000)

	result := scanAnalysisCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for partial analysis caches")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 6000 {
		t.Errorf("expected total size 6000, got %d", result.TotalSize)
	}
}

func TestScanAnalysisCachesPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanAnalysisCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Cloud Photo Caches tests ---

func TestScanCloudPhotoCachesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanCloudPhotoCaches(home)
	if result != nil {
		t.Fatal("expected nil for missing cloud photo caches")
	}
}

func TestScanCloudPhotoCachesEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.cloudphotosd", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanCloudPhotoCaches(home)
	if result != nil {
		t.Fatal("expected nil for empty cloud photo caches directory")
	}
}

func TestScanCloudPhotoCachesWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.cloudphotosd", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(dir, "sync.db"), 7000)

	result := scanCloudPhotoCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for cloud photo caches with data")
	}
	if result.Category != "photos-icloud-cache" {
		t.Errorf("expected category 'photos-icloud-cache', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 7000 {
		t.Errorf("expected total size 7000, got %d", result.TotalSize)
	}
}

func TestScanCloudPhotoCachesPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.cloudphotosd", "Data", "Library", "Caches")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Containers", "com.apple.cloudphotosd", "Data", "Library")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanCloudPhotoCaches(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Syndication Library tests ---

func TestScanSyndicationLibraryMissing(t *testing.T) {
	home := t.TempDir()
	result := scanSyndicationLibrary(home)
	if result != nil {
		t.Fatal("expected nil for missing syndication library")
	}
}

func TestScanSyndicationLibraryEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Photos", "Libraries", "Syndication.photoslibrary")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanSyndicationLibrary(home)
	if result != nil {
		t.Fatal("expected nil for empty syndication library directory")
	}
}

func TestScanSyndicationLibraryWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Photos", "Libraries", "Syndication.photoslibrary")
	writeFile(t, filepath.Join(dir, "database", "Photos.sqlite"), 3000)
	writeFile(t, filepath.Join(dir, "resources", "media", "photo1.jpg"), 2000)

	result := scanSyndicationLibrary(home)
	if result == nil {
		t.Fatal("expected non-nil result for syndication library with data")
	}
	if result.Category != "photos-syndication" {
		t.Errorf("expected category 'photos-syndication', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 5000 {
		t.Errorf("expected total size 5000, got %d", result.TotalSize)
	}
}

func TestScanSyndicationLibraryPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Photos", "Libraries", "Syndication.photoslibrary")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Photos", "Libraries")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanSyndicationLibrary(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create Photos caches.
	photosDir := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(photosDir, "cache.db"), 1000)

	// Create mediaanalysisd caches.
	analysisDir := filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches")
	writeFile(t, filepath.Join(analysisDir, "model.bin"), 2000)

	// No cloudphotosd, no syndication -- should be silently skipped.

	var results []scan.CategoryResult
	if cr := scanPhotosCaches(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanAnalysisCaches(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanCloudPhotoCaches(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanSyndicationLibrary(home); cr != nil {
		results = append(results, *cr)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Photos + Analysis), got %d", len(results))
	}
	if results[0].Category != "photos-caches" {
		t.Errorf("expected first result 'photos-caches', got %q", results[0].Category)
	}
	if results[1].Category != "photos-analysis" {
		t.Errorf("expected second result 'photos-analysis', got %q", results[1].Category)
	}
}
