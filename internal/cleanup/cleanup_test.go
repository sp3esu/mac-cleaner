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

	res := Execute(results, nil)

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

	res := Execute(results, nil)

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

	res := Execute(results, nil)

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

	res := Execute(results, nil)

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
	// Use a path under a temp dir (which is under the home or /private/var/folders/)
	// so it passes the home containment check. The path itself does not exist.
	tmp := t.TempDir()
	gonePath := filepath.Join(tmp, "definitely-does-not-exist-abc123")

	// os.RemoveAll returns nil for nonexistent paths, so this counts as Removed.
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: gonePath, Description: "gone", Size: 50},
			},
			TotalSize: 50,
		},
	}

	res := Execute(results, nil)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (already gone counts as removed)", res.Removed)
	}
	if res.BytesFreed != 50 {
		t.Errorf("BytesFreed = %d, want 50", res.BytesFreed)
	}
}

func TestExecuteEmptyResults(t *testing.T) {
	res := Execute([]scan.CategoryResult{}, nil)

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

	res := Execute(results, nil)

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0 (pseudo-path skipped)", res.Removed)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (pseudo-path counted as failed)", res.Failed)
	}
}

func TestExecuteProgressCallback(t *testing.T) {
	tmp := t.TempDir()
	f1 := filepath.Join(tmp, "a.txt")
	f2 := filepath.Join(tmp, "b.txt")
	os.WriteFile(f1, []byte("aaa"), 0644)
	os.WriteFile(f2, []byte("bbb"), 0644)

	results := []scan.CategoryResult{
		{
			Category:    "cat-a",
			Description: "Category A",
			Entries: []scan.ScanEntry{
				{Path: f1, Description: "file-a", Size: 3},
			},
			TotalSize: 3,
		},
		{
			Category:    "cat-b",
			Description: "Category B",
			Entries: []scan.ScanEntry{
				{Path: f2, Description: "file-b", Size: 3},
			},
			TotalSize: 3,
		},
	}

	type call struct {
		categoryDesc string
		entryPath    string
		current      int
		total        int
	}
	var calls []call
	cb := func(categoryDesc, entryPath string, current, total int) {
		calls = append(calls, call{categoryDesc, entryPath, current, total})
	}

	Execute(results, cb)

	// Expect 4 calls: category-start A, entry A, category-start B, entry B.
	if len(calls) != 4 {
		t.Fatalf("expected 4 callback calls, got %d", len(calls))
	}

	// Category-start for A: entryPath="", current=1, total=2.
	if calls[0].categoryDesc != "Category A" || calls[0].entryPath != "" || calls[0].current != 1 || calls[0].total != 2 {
		t.Errorf("call[0] = %+v, want category-start for Category A (1/2)", calls[0])
	}
	// Entry for A: entryPath=f1, current=1, total=2.
	if calls[1].categoryDesc != "Category A" || calls[1].entryPath != f1 || calls[1].current != 1 || calls[1].total != 2 {
		t.Errorf("call[1] = %+v, want entry for Category A", calls[1])
	}
	// Category-start for B: entryPath="", current=2, total=2.
	if calls[2].categoryDesc != "Category B" || calls[2].entryPath != "" || calls[2].current != 2 || calls[2].total != 2 {
		t.Errorf("call[2] = %+v, want category-start for Category B (2/2)", calls[2])
	}
	// Entry for B: entryPath=f2, current=2, total=2.
	if calls[3].categoryDesc != "Category B" || calls[3].entryPath != f2 || calls[3].current != 2 || calls[3].total != 2 {
		t.Errorf("call[3] = %+v, want entry for Category B", calls[3])
	}
}

func TestIsPseudoPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "docker pseudo-path", path: "docker:BuildCache", want: true},
		{name: "empty string", path: "", want: true},
		{name: "relative path", path: "relative/path", want: true},
		{name: "absolute path", path: "/Users/foo/bar", want: false},
		{name: "path with colon", path: "/Users/foo/my:file", want: false},
		{name: "root path", path: "/", want: false},
		{name: "tmp path", path: "/tmp/test", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPseudoPath(tt.path); got != tt.want {
				t.Errorf("isPseudoPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExecuteProgressCallbackNil(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "file.txt")
	os.WriteFile(f, []byte("data"), 0644)

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: f, Description: "file", Size: 4},
			},
			TotalSize: 4,
		},
	}

	// Should not panic with nil callback.
	res := Execute(results, nil)
	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
}
