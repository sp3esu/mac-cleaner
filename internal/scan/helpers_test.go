package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan/scantest"
)

// writeFile is a test helper that creates a file with the given size.
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

	result, err := ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	// Total: allocated size of all three files.
	expectedTotal := scantest.OnDisk(t, 100, 500, 300)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}

	// Entries should be sorted by size descending.
	if result.Entries[0].Size < result.Entries[1].Size {
		t.Errorf("entries not sorted by size descending: %d < %d",
			result.Entries[0].Size, result.Entries[1].Size)
	}

	// First entry should be large-cache (two files).
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

	result, err := ScanTopLevel(dir, "test-cat", "Test Category")
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
	result, err := ScanTopLevel("/nonexistent/path/that/does/not/exist", "test", "Test")
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
	writeFile(t, filepath.Join(subDir, "inner.dat"), 5000)

	writeFile(t, filepath.Join(dir, "toplevel.dat"), 150)

	result, err := ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries (1 dir + 1 file), got %d", len(result.Entries))
	}

	// Total: allocated size of subdir/inner.dat plus toplevel.dat.
	expectedTotal := scantest.OnDisk(t, 5000, 150)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}

	// Sorted by size: subdir (5000 bytes) first, then toplevel.dat (150).
	if want := scantest.OnDisk(t, 5000); result.Entries[0].Size != want {
		t.Errorf("expected first entry size %d, got %d", want, result.Entries[0].Size)
	}
	if want := scantest.OnDisk(t, 150); result.Entries[1].Size != want {
		t.Errorf("expected second entry size %d, got %d", want, result.Entries[1].Size)
	}
}

func TestScanTopLevelReportsDeniedSubdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "readable", "a.dat"), 100)
	writeFile(t, filepath.Join(dir, "partial", "ok.dat"), 100)
	locked := filepath.Join(dir, "partial", "locked")
	writeFile(t, filepath.Join(locked, "secret.dat"), 100)
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	result, err := ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}

	if len(result.PermissionIssues) != 1 {
		t.Fatalf("expected 1 permission issue, got %+v", result.PermissionIssues)
	}
	pi := result.PermissionIssues[0]
	if pi.Path != locked {
		t.Errorf("permission issue path = %q, want %q", pi.Path, locked)
	}
	if pi.Description != "locked (permission denied)" {
		t.Errorf("permission issue description = %q, want %q", pi.Description, "locked (permission denied)")
	}

	// The partially readable entry keeps its partial size.
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	want := scantest.OnDisk(t, 100)
	for _, e := range result.Entries {
		if e.Size != want {
			t.Errorf("entry %s size = %d, want %d", e.Description, e.Size, want)
		}
	}
}

func TestScanTopLevelReportsFullyDeniedSubdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "readable", "a.dat"), 100)
	locked := filepath.Join(dir, "locked")
	writeFile(t, filepath.Join(locked, "secret.dat"), 100)
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	result, err := ScanTopLevel(dir, "test-cat", "Test Category")
	if err != nil {
		t.Fatalf("ScanTopLevel: %v", err)
	}
	if len(result.PermissionIssues) != 1 || result.PermissionIssues[0].Path != locked {
		t.Fatalf("expected permission issue for %s, got %+v", locked, result.PermissionIssues)
	}
	// The unreadable dir has no measurable size, so it is not an entry.
	if len(result.Entries) != 1 || result.Entries[0].Description != "readable" {
		t.Errorf("expected only 'readable' entry, got %+v", result.Entries)
	}
}

func TestScanTopLevelExcluding(t *testing.T) {
	dir := t.TempDir()
	// Owned names are matched case-insensitively, as APFS does.
	writeFile(t, filepath.Join(dir, "Yarn", "v6", "pkg.tgz"), 100)
	writeFile(t, filepath.Join(dir, "GOOGLE", "chrome", "Default", "x"), 100)
	writeFile(t, filepath.Join(dir, "GOOGLE", "Other", "y"), 100)
	writeFile(t, filepath.Join(dir, "GOOGLE", "loose.dat"), 100)
	writeFile(t, filepath.Join(dir, "keep", "z"), 100)

	result, err := ScanTopLevelExcluding(dir, "test-cat", "Test Category", []string{"yarn", "Google/Chrome"})
	if err != nil {
		t.Fatalf("ScanTopLevelExcluding: %v", err)
	}

	got := map[string]string{}
	for _, e := range result.Entries {
		got[e.Path] = e.Description
	}
	want := map[string]string{
		filepath.Join(dir, "GOOGLE", "Other"):     filepath.Join("GOOGLE", "Other"),
		filepath.Join(dir, "GOOGLE", "loose.dat"): filepath.Join("GOOGLE", "loose.dat"),
		filepath.Join(dir, "keep"):                "keep",
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for p, d := range want {
		if got[p] != d {
			t.Errorf("entry %s description = %q, want %q (entries: %v)", p, got[p], d, got)
		}
	}
	if wantTotal := scantest.OnDisk(t, 100, 100, 100); result.TotalSize != wantTotal {
		t.Errorf("total = %d, want %d", result.TotalSize, wantTotal)
	}
}

func TestScanTopLevelExcludingAncestorFileIsNotExpanded(t *testing.T) {
	dir := t.TempDir()
	// A regular file named like an owned ancestor cannot contain the owned
	// path, so it is emitted as a normal entry.
	writeFile(t, filepath.Join(dir, "Google"), 100)

	result, err := ScanTopLevelExcluding(dir, "test-cat", "Test Category", []string{"Google/Chrome"})
	if err != nil {
		t.Fatalf("ScanTopLevelExcluding: %v", err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Path != filepath.Join(dir, "Google") {
		t.Errorf("entries = %+v, want only the Google file", result.Entries)
	}
}

func TestScanTopLevelExcludingDeniedAncestor(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	ancestor := filepath.Join(dir, "Google")
	writeFile(t, filepath.Join(ancestor, "Other", "y"), 100)
	if err := os.Chmod(ancestor, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ancestor, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	result, err := ScanTopLevelExcluding(dir, "test-cat", "Test Category", []string{"Google/Chrome"})
	if err != nil {
		t.Fatalf("ScanTopLevelExcluding: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected no entries, got %+v", result.Entries)
	}
	if len(result.PermissionIssues) != 1 || result.PermissionIssues[0].Path != ancestor {
		t.Errorf("expected permission issue for %s, got %+v", ancestor, result.PermissionIssues)
	}
}
