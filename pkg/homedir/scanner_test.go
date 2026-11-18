package homedir

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// writeFile creates a file with the given size in bytes.
func writeFile(t *testing.T, path string, sizeBytes int) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdirAll(%s): %v", dir, err)
	}
	if err := os.WriteFile(path, make([]byte, sizeBytes), 0644); err != nil {
		t.Fatalf("writeFile(%s): %v", path, err)
	}
}

func TestLargeFiles(t *testing.T) {
	root := t.TempDir()
	// Create a 150MB file (above the 100MB threshold).
	largeFile := filepath.Join(root, "big.dmg")
	writeFile(t, largeFile, 150*1024*1024)

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	var found bool
	for _, cr := range results {
		if cr.Category == "home-large-files" {
			found = true
			if len(cr.Entries) != 1 {
				t.Fatalf("expected 1 large file entry, got %d", len(cr.Entries))
			}
			if cr.Entries[0].Path != largeFile {
				t.Errorf("expected path %s, got %s", largeFile, cr.Entries[0].Path)
			}
			if cr.Entries[0].Size != 150*1024*1024 {
				t.Errorf("expected size %d, got %d", 150*1024*1024, cr.Entries[0].Size)
			}
		}
	}
	if !found {
		t.Error("home-large-files category not found")
	}
}

func TestOldFiles(t *testing.T) {
	root := t.TempDir()
	// Create a 15MB file (above 10MB min) and set it to 200 days old.
	oldFile := filepath.Join(root, "old-report.pdf")
	writeFile(t, oldFile, 15*1024*1024)
	oldTime := time.Now().Add(-200 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	var found bool
	for _, cr := range results {
		if cr.Category == "home-old-files" {
			found = true
			if len(cr.Entries) != 1 {
				t.Fatalf("expected 1 old file entry, got %d", len(cr.Entries))
			}
			if cr.Entries[0].Path != oldFile {
				t.Errorf("expected path %s, got %s", oldFile, cr.Entries[0].Path)
			}
			// Description should contain age annotation.
			if cr.Entries[0].Description == "" {
				t.Error("expected description with age annotation")
			}
		}
	}
	if !found {
		t.Error("home-old-files category not found")
	}
}

func TestExclusions(t *testing.T) {
	root := t.TempDir()
	// Create large files inside directories that should be excluded.
	excludedDirs := []string{"Library", "Applications", ".Trash", "Downloads"}
	for _, dir := range excludedDirs {
		writeFile(t, filepath.Join(root, dir, "big.bin"), 150*1024*1024)
	}

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	for _, cr := range results {
		for _, e := range cr.Entries {
			for _, dir := range excludedDirs {
				prefix := filepath.Join(root, dir)
				if e.Path == prefix || len(e.Path) > len(prefix) && e.Path[:len(prefix)+1] == prefix+string(filepath.Separator) {
					t.Errorf("file in excluded dir %s should not be reported: %s", dir, e.Path)
				}
			}
		}
	}
}

func TestSymlinkSkip(t *testing.T) {
	root := t.TempDir()
	// Create a real large file in a subdir.
	realFile := filepath.Join(root, "subdir", "real.bin")
	writeFile(t, realFile, 150*1024*1024)

	// Create a symlink to it at root level.
	linkPath := filepath.Join(root, "link.bin")
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	for _, cr := range results {
		for _, e := range cr.Entries {
			if e.Path == linkPath {
				t.Errorf("symlink should not be reported: %s", linkPath)
			}
		}
	}
}

func TestDualCategory(t *testing.T) {
	root := t.TempDir()
	// Create a 200MB file that is also 1 year old -> both categories.
	dualFile := filepath.Join(root, "huge-old.iso")
	writeFile(t, dualFile, 200*1024*1024)
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(dualFile, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	foundLarge := false
	foundOld := false
	for _, cr := range results {
		for _, e := range cr.Entries {
			if e.Path == dualFile {
				if cr.Category == "home-large-files" {
					foundLarge = true
				}
				if cr.Category == "home-old-files" {
					foundOld = true
				}
			}
		}
	}
	if !foundLarge {
		t.Error("200MB old file should appear in home-large-files")
	}
	if !foundOld {
		t.Error("200MB old file should appear in home-old-files")
	}
}

func TestOldFileMinSize(t *testing.T) {
	root := t.TempDir()
	// Create a 5KB old file -- below the 10MB minimum for old files.
	smallOldFile := filepath.Join(root, "tiny-old.txt")
	writeFile(t, smallOldFile, 5*1024)
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(smallOldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	for _, cr := range results {
		if cr.Category == "home-old-files" {
			for _, e := range cr.Entries {
				if e.Path == smallOldFile {
					t.Error("5KB old file should NOT appear in home-old-files (below 10MB threshold)")
				}
			}
		}
	}
}

func TestEmptyResults(t *testing.T) {
	root := t.TempDir()
	// Empty directory -- no files to find.

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty results for empty dir, got %d categories", len(results))
	}
}

func TestSortOrder(t *testing.T) {
	root := t.TempDir()
	// Create multiple large files of different sizes.
	writeFile(t, filepath.Join(root, "small.bin"), 110*1024*1024)
	writeFile(t, filepath.Join(root, "medium.bin"), 200*1024*1024)
	writeFile(t, filepath.Join(root, "large.bin"), 300*1024*1024)

	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom error: %v", err)
	}

	for _, cr := range results {
		if cr.Category == "home-large-files" {
			if len(cr.Entries) != 3 {
				t.Fatalf("expected 3 large file entries, got %d", len(cr.Entries))
			}
			// Should be sorted by size descending.
			if !sort.SliceIsSorted(cr.Entries, func(i, j int) bool {
				return cr.Entries[i].Size > cr.Entries[j].Size
			}) {
				t.Error("large files should be sorted by size descending")
			}
			if cr.Entries[0].Size != 300*1024*1024 {
				t.Errorf("first entry should be 300MB, got %d", cr.Entries[0].Size)
			}
		}
	}
}
