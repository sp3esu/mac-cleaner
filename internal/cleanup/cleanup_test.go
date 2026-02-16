package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

func TestExecuteRemovesFiles(t *testing.T) {
	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "file1.txt")
	f2 := filepath.Join(tmp, "file2.txt")
	os.WriteFile(f1, []byte("hello"), 0644)
	os.WriteFile(f2, []byte("world!"), 0644)

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: f1, Description: "file1", Size: 5},
				{Path: f2, Description: "file2", Size: 6},
			},
			TotalSize: 11,
		},
	}

	res := Execute(results)

	if res.Removed != 2 {
		t.Errorf("Removed = %d, want 2", res.Removed)
	}
	if res.Failed != 0 {
		t.Errorf("Failed = %d, want 0", res.Failed)
	}
	if res.BytesFreed != 11 {
		t.Errorf("BytesFreed = %d, want 11", res.BytesFreed)
	}

	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("file1 should be deleted")
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Errorf("file2 should be deleted")
	}
}

func TestExecuteRemovesDirectories(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "dir", "subdir")
	os.MkdirAll(nested, 0755)
	os.WriteFile(filepath.Join(nested, "file.txt"), []byte("data"), 0644)

	topDir := filepath.Join(tmp, "dir")
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: topDir, Description: "dir", Size: 4},
			},
			TotalSize: 4,
		},
	}

	res := Execute(results)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	if res.BytesFreed != 4 {
		t.Errorf("BytesFreed = %d, want 4", res.BytesFreed)
	}
	if _, err := os.Stat(topDir); !os.IsNotExist(err) {
		t.Errorf("directory should be deleted")
	}
}

func TestExecuteContinuesOnError(t *testing.T) {
	tmp := t.TempDir()

	// Create a valid file that can be removed.
	validFile := filepath.Join(tmp, "valid.txt")
	os.WriteFile(validFile, []byte("ok"), 0644)

	// Create a read-only directory with a file inside, then try to remove
	// the file through a path that will fail (file inside read-only dir).
	roDir := filepath.Join(tmp, "readonly")
	os.MkdirAll(roDir, 0755)
	roFile := filepath.Join(roDir, "locked.txt")
	os.WriteFile(roFile, []byte("locked"), 0644)
	os.Chmod(roDir, 0555) // make dir read-only so file inside cannot be removed
	t.Cleanup(func() { os.Chmod(roDir, 0755) })

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: roFile, Description: "locked", Size: 6},
				{Path: validFile, Description: "valid", Size: 2},
			},
			TotalSize: 8,
		},
	}

	res := Execute(results)

	// The valid file should still be removed even though the locked one failed.
	if _, err := os.Stat(validFile); !os.IsNotExist(err) {
		t.Errorf("valid file should be deleted")
	}
	if res.Removed < 1 {
		t.Errorf("Removed should be at least 1, got %d", res.Removed)
	}
	if res.Failed < 1 {
		t.Errorf("Failed should be at least 1, got %d", res.Failed)
	}
}

func TestExecuteBlockedPath(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: "/System/foo", Description: "system-foo", Size: 100},
			},
			TotalSize: 100,
		},
	}

	res := Execute(results)

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0 (blocked path)", res.Removed)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (blocked path)", res.Failed)
	}
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0 (blocked path)", res.BytesFreed)
	}
}

func TestExecuteAlreadyGone(t *testing.T) {
	// os.RemoveAll returns nil for nonexistent paths, so this counts as Removed.
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/definitely-does-not-exist-abc123", Description: "gone", Size: 50},
			},
			TotalSize: 50,
		},
	}

	res := Execute(results)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (already gone counts as removed)", res.Removed)
	}
	if res.BytesFreed != 50 {
		t.Errorf("BytesFreed = %d, want 50", res.BytesFreed)
	}
}

func TestExecuteEmptyResults(t *testing.T) {
	res := Execute([]scan.CategoryResult{})

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0", res.Removed)
	}
	if res.Failed != 0 {
		t.Errorf("Failed = %d, want 0", res.Failed)
	}
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0", res.BytesFreed)
	}
}

func TestExecutePseudoPath(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "docker",
			Description: "Docker",
			Entries: []scan.ScanEntry{
				{Path: "docker:BuildCache", Description: "Build Cache", Size: 1000},
			},
			TotalSize: 1000,
		},
	}

	res := Execute(results)

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0 (pseudo-path skipped)", res.Removed)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (pseudo-path counted as failed)", res.Failed)
	}
}
