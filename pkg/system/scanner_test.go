package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// writeFile is a test helper that creates a file with the given size.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func TestScanTopLevel(t *testing.T) {
	dir := t.TempDir()

	// Create two subdirectories with files of known sizes.
	smallDir := filepath.Join(dir, "small-cache")
	largeDir := filepath.Join(dir, "large-cache")
	os.MkdirAll(smallDir, 0755)
	os.MkdirAll(largeDir, 0755)

	writeFile(t, filepath.Join(smallDir, "a.dat"), 100)
	writeFile(t, filepath.Join(largeDir, "b.dat"), 500)
	writeFile(t, filepath.Join(largeDir, "c.dat"), 300)

	result, err := scan.ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	// Total: small=100, large=800.
	expectedTotal := int64(100 + 500 + 300)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}

	// Entries should be sorted by size descending.
	if result.Entries[0].Size < result.Entries[1].Size {
		t.Errorf("entries not sorted by size descending: %d < %d",
			result.Entries[0].Size, result.Entries[1].Size)
	}

	// First entry should be large-cache (800 bytes).
	if result.Entries[0].Description != "large-cache" {
		t.Errorf("expected first entry 'large-cache', got %q", result.Entries[0].Description)
	}
}

func TestScanTopLevelSkipsZeroBytes(t *testing.T) {
	dir := t.TempDir()

	// Create an empty subdirectory (0 bytes).
	emptyDir := filepath.Join(dir, "empty-cache")
	os.MkdirAll(emptyDir, 0755)

	// Create a non-empty subdirectory so result is not nil.
	nonEmpty := filepath.Join(dir, "non-empty")
	os.MkdirAll(nonEmpty, 0755)
	writeFile(t, filepath.Join(nonEmpty, "data.bin"), 50)

	result, err := scan.ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Only the non-empty directory should appear.
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (zero-byte skipped), got %d", len(result.Entries))
	}
	if result.Entries[0].Description != "non-empty" {
		t.Errorf("expected 'non-empty', got %q", result.Entries[0].Description)
	}
}

func TestScanTopLevelNonExistent(t *testing.T) {
	result, err := scan.ScanTopLevel("/nonexistent/path/that/does/not/exist", "test", "Test")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if result != nil {
		t.Fatal("expected nil result for non-existent path")
	}
}

func TestScanTopLevelHandlesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a mix of files and directories at top level.
	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)
	writeFile(t, filepath.Join(subDir, "inner.dat"), 200)

	writeFile(t, filepath.Join(dir, "toplevel.dat"), 150)

	result, err := scan.ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries (1 dir + 1 file), got %d", len(result.Entries))
	}

	// Total: subdir=200, toplevel.dat=150
	expectedTotal := int64(200 + 150)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}

	// Sorted by size: subdir (200) first, then toplevel.dat (150).
	if result.Entries[0].Size != 200 {
		t.Errorf("expected first entry size 200, got %d", result.Entries[0].Size)
	}
	if result.Entries[1].Size != 150 {
		t.Errorf("expected second entry size 150, got %d", result.Entries[1].Size)
	}
}

func TestScanPreservesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create files to verify they are not deleted by scanning.
	subDir := filepath.Join(dir, "cache-dir")
	os.MkdirAll(subDir, 0755)
	filePath := filepath.Join(subDir, "important.dat")
	writeFile(t, filePath, 1024)

	topFile := filepath.Join(dir, "top.log")
	writeFile(t, topFile, 512)

	// Run scan.
	_, err := scan.ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}

	// Verify all files still exist.
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file deleted after scan: %s", filePath)
	}
	if _, err := os.Stat(topFile); err != nil {
		t.Errorf("file deleted after scan: %s", topFile)
	}

	// Verify directory still exists.
	if _, err := os.Stat(subDir); err != nil {
		t.Errorf("directory deleted after scan: %s", subDir)
	}
}

func TestScanTopLevelCategoryFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.dat"), 100)

	result, err := scan.ScanTopLevel(dir, "my-category", "My Description")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}

	if result.Category != "my-category" {
		t.Errorf("expected category 'my-category', got %q", result.Category)
	}
	if result.Description != "My Description" {
		t.Errorf("expected description 'My Description', got %q", result.Description)
	}
}
