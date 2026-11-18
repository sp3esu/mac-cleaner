package system

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/scan/scantest"
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
	writeFile(t, filepath.Join(subDir, "inner.dat"), 5000)

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

// --- scanQuickLook tests ---

func TestScanQuickLook_MatchingEntries(t *testing.T) {
	dir := t.TempDir()

	// Create matching QuickLook entries.
	qlDir1 := filepath.Join(dir, "com.apple.quicklook.thumbcache")
	os.MkdirAll(qlDir1, 0755)
	writeFile(t, filepath.Join(qlDir1, "data.bin"), 5000)

	qlDir2 := filepath.Join(dir, "com.apple.quicklook.other")
	os.MkdirAll(qlDir2, 0755)
	writeFile(t, filepath.Join(qlDir2, "thumb.dat"), 200)

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	// Entries should be sorted by size descending.
	if result.Entries[0].Size < result.Entries[1].Size {
		t.Error("entries not sorted by size descending")
	}
	if want := scantest.OnDisk(t, 5000); result.Entries[0].Size != want {
		t.Errorf("expected first entry size %d, got %d", want, result.Entries[0].Size)
	}

	expectedTotal := scantest.OnDisk(t, 5000, 200)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, result.TotalSize)
	}

	if result.Category != "quicklook" {
		t.Errorf("expected category 'quicklook', got %q", result.Category)
	}
}

func TestScanQuickLook_NonMatchingIgnored(t *testing.T) {
	dir := t.TempDir()

	// Create non-matching entries only.
	other := filepath.Join(dir, "com.example.something")
	os.MkdirAll(other, 0755)
	writeFile(t, filepath.Join(other, "data.bin"), 300)

	writeFile(t, filepath.Join(dir, "random.txt"), 100)

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for non-matching entries, got %+v", result)
	}
}

func TestScanQuickLook_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty dir, got %+v", result)
	}
}

func TestScanQuickLook_SkipsZeroByte(t *testing.T) {
	dir := t.TempDir()

	// Create an empty QuickLook directory (0 bytes).
	emptyQL := filepath.Join(dir, "com.apple.quicklook.empty")
	os.MkdirAll(emptyQL, 0755)

	// Create a non-empty one to have a result.
	nonEmpty := filepath.Join(dir, "com.apple.quicklook.real")
	os.MkdirAll(nonEmpty, 0755)
	writeFile(t, filepath.Join(nonEmpty, "thumb.dat"), 256)

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Only the non-empty entry should appear.
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (zero-byte skipped), got %d", len(result.Entries))
	}
	if result.Entries[0].Description != "com.apple.quicklook.real" {
		t.Errorf("expected 'com.apple.quicklook.real', got %q", result.Entries[0].Description)
	}
}

func TestScanQuickLook_FileEntries(t *testing.T) {
	dir := t.TempDir()

	// Create a matching file (not directory).
	writeFile(t, filepath.Join(dir, "com.apple.quicklook.data"), 128)

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 128); result.Entries[0].Size != want {
		t.Errorf("expected size %d, got %d", want, result.Entries[0].Size)
	}
}

func TestScanQuickLook_SortedBySizeDescending(t *testing.T) {
	dir := t.TempDir()

	// Create entries with varying sizes.
	names := []struct {
		name string
		size int
	}{
		{"com.apple.quicklook.small", 100},
		{"com.apple.quicklook.large", 900},
		{"com.apple.quicklook.medium", 400},
	}
	for _, n := range names {
		d := filepath.Join(dir, n.name)
		os.MkdirAll(d, 0755)
		writeFile(t, filepath.Join(d, "data.bin"), n.size)
	}

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}

	for i := 0; i < len(result.Entries)-1; i++ {
		if result.Entries[i].Size < result.Entries[i+1].Size {
			t.Errorf("entries not sorted descending at index %d: %d < %d",
				i, result.Entries[i].Size, result.Entries[i+1].Size)
		}
	}
}

// --- quickLookCacheDir tests ---

func TestQuickLookCacheDir_ValidTMPDIR(t *testing.T) {
	base := t.TempDir()

	// Simulate macOS TMPDIR structure: /var/folders/xx/yy/T/
	tDir := filepath.Join(base, "var", "folders", "xx", "yy", "T")
	cDir := filepath.Join(base, "var", "folders", "xx", "yy", "C")
	os.MkdirAll(tDir, 0755)
	os.MkdirAll(cDir, 0755)

	t.Setenv("TMPDIR", tDir+"/")

	got, err := quickLookCacheDir()
	if err != nil {
		t.Fatalf("quickLookCacheDir: %v", err)
	}
	if got != cDir {
		t.Errorf("expected %q, got %q", cDir, got)
	}
}

func TestQuickLookCacheDir_EmptyTMPDIR(t *testing.T) {
	t.Setenv("TMPDIR", "")

	_, err := quickLookCacheDir()
	if err == nil {
		t.Fatal("expected error for empty TMPDIR")
	}
}

func TestQuickLookCacheDir_NonMacOSTMPDIR(t *testing.T) {
	t.Setenv("TMPDIR", "/tmp")

	_, err := quickLookCacheDir()
	if err == nil {
		t.Fatal("expected error for non-macOS TMPDIR")
	}
}

func TestQuickLookCacheDir_MissingSiblingC(t *testing.T) {
	base := t.TempDir()
	tDir := filepath.Join(base, "var", "folders", "xx", "yy", "T")
	os.MkdirAll(tDir, 0755)
	// Intentionally do NOT create the sibling C directory.

	t.Setenv("TMPDIR", tDir+"/")

	_, err := quickLookCacheDir()
	if err == nil {
		t.Fatal("expected error when sibling C dir is missing")
	}
}

func TestQuickLookCacheDir_PassesSafetyCheck(t *testing.T) {
	base := t.TempDir()

	// On macOS, t.TempDir() is under /private/var/folders/ which is
	// allowed by the home containment check. Verify that a valid TMPDIR
	// structure passes the safety check integrated into quickLookCacheDir.
	tDir := filepath.Join(base, "var", "folders", "xx", "yy", "T")
	cDir := filepath.Join(base, "var", "folders", "xx", "yy", "C")
	os.MkdirAll(tDir, 0755)
	os.MkdirAll(cDir, 0755)

	t.Setenv("TMPDIR", tDir+"/")

	got, err := quickLookCacheDir()
	if err != nil {
		t.Fatalf("quickLookCacheDir: %v", err)
	}
	if got != cDir {
		t.Errorf("expected %q, got %q", cDir, got)
	}
}

func TestScanQuickLook_ReportsDeniedSubdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	ql := filepath.Join(dir, "com.apple.quicklook.thumbcache")
	os.MkdirAll(ql, 0755)
	writeFile(t, filepath.Join(ql, "ok.bin"), 100)
	locked := filepath.Join(ql, "locked")
	os.MkdirAll(locked, 0755)
	writeFile(t, filepath.Join(locked, "secret.bin"), 100)
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	result, err := scanQuickLook(dir, "quicklook", "QuickLook Thumbnails")
	if err != nil {
		t.Fatalf("scanQuickLook: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.PermissionIssues) != 1 || result.PermissionIssues[0].Path != locked {
		t.Fatalf("expected permission issue for %s, got %+v", locked, result.PermissionIssues)
	}
	if got := result.PermissionIssues[0].Description; got != "locked (permission denied)" {
		t.Errorf("permission issue description = %q", got)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry with partial size, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 100); result.Entries[0].Size != want {
		t.Errorf("partial size = %d, want %d", result.Entries[0].Size, want)
	}
}

// --- system-caches must not emit caches owned by dedicated scanners ---

// systemCachesPaths runs Scan with HOME set to home and returns the paths
// emitted under system-caches.
func systemCachesPaths(t *testing.T, home string) []string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("TMPDIR", "/tmp") // skip the QuickLook scan
	results, err := Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var paths []string
	for _, r := range results {
		if r.Category != "system-caches" {
			continue
		}
		for _, e := range r.Entries {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func TestScanSystemCachesSkipsOwnedCaches(t *testing.T) {
	home := t.TempDir()
	caches := filepath.Join(home, "Library", "Caches")
	for _, rel := range []string{
		"Homebrew/a",
		"pip/b",
		"Yarn/c", // Yarn v1 casing; owned path is "yarn"
		"Google/Chrome/Default/x",
		"Google/Other/y",
		"com.foo/z",
	} {
		p := filepath.Join(caches, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0755)
		writeFile(t, p, 100)
	}

	got := systemCachesPaths(t, home)
	want := []string{
		filepath.Join(caches, "Google", "Other"),
		filepath.Join(caches, "com.foo"),
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("system-caches paths = %v, want %v", got, want)
	}
}

func TestScanSystemCachesSkipsEveryOwnedCache(t *testing.T) {
	home := t.TempDir()
	caches := filepath.Join(home, "Library", "Caches")
	for _, rel := range ownedCaches {
		p := filepath.Join(caches, filepath.FromSlash(rel), "data.bin")
		os.MkdirAll(filepath.Dir(p), 0755)
		writeFile(t, p, 100)
	}

	if got := systemCachesPaths(t, home); len(got) != 0 {
		t.Errorf("system-caches emitted owned caches: %v", got)
	}
}

func TestOwnedCachesMatchScannerPaths(t *testing.T) {
	// Guard against the list drifting from the dedicated scanners.
	want := []string{
		"com.apple.Safari",
		"Google/Chrome",
		"Firefox",
		"yarn",
		"Homebrew",
		"CocoaPods",
		"pip",
		"Adobe",
		"com.bohemiancoding.sketch3",
		"com.microsoft.teams2",
	}
	if !reflect.DeepEqual(ownedCaches, want) {
		t.Errorf("ownedCaches = %v, want %v", ownedCaches, want)
	}
}
