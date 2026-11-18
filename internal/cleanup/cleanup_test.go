package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	if res.Removed != 0 || res.Skipped != 1 || res.Failed != 0 {
		t.Errorf("Removed=%d Skipped=%d Failed=%d, want 0/1/0 (missing path is skipped)", res.Removed, res.Skipped, res.Failed)
	}
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0 (nothing was there to free)", res.BytesFreed)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected no errors, got %v", res.Errors)
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
	if res.Skipped != 1 || res.Failed != 0 || len(res.Errors) != 0 {
		t.Errorf("Skipped=%d Failed=%d Errors=%v, want 1/0/none (pseudo-path is informational)", res.Skipped, res.Failed, res.Errors)
	}
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0", res.BytesFreed)
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

// fakeRepo creates a directory with a .git directory holding size bytes of
// objects, under a temp dir that passes the safety check.
func fakeRepo(t *testing.T, size int) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "myrepo")
	objects := filepath.Join(repo, ".git", "objects")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objects, "pack"), make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

// stubGitCleanup replaces runGitCleanup for the duration of a test.
func stubGitCleanup(t *testing.T, fn func(repoPath string, hasLFS bool) error) {
	t.Helper()
	orig := runGitCleanup
	runGitCleanup = fn
	t.Cleanup(func() { runGitCleanup = orig })
}

func gitResults(repoPath, desc string, size int64) []scan.CategoryResult {
	return []scan.CategoryResult{{
		Category:    "git-repos",
		Description: "Git Repositories",
		Entries:     []scan.ScanEntry{{Path: "git-gc:" + repoPath, Description: desc, Size: size}},
		TotalSize:   size,
	}}
}

func TestGitGCPseudoPath(t *testing.T) {
	repo := fakeRepo(t, 100)
	var calledPath string
	var calledLFS bool
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		calledPath = repoPath
		calledLFS = hasLFS
		return nil
	})

	res := Execute(gitResults(repo, "myrepo (.git 50 MB)", 5000), nil)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	if res.Failed != 0 {
		t.Errorf("Failed = %d, want 0", res.Failed)
	}
	// The stub changes nothing on disk, so nothing was freed, whatever the
	// count-objects estimate said.
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0 (measured, not the 5000 estimate)", res.BytesFreed)
	}
	if calledPath != repo {
		t.Errorf("runGitCleanup called with path %q, want %q", calledPath, repo)
	}
	if calledLFS {
		t.Errorf("runGitCleanup called with hasLFS=true, want false")
	}
}

func TestGitGC_BytesFreedIsMeasuredDelta(t *testing.T) {
	repo := fakeRepo(t, 100)
	garbage := filepath.Join(repo, ".git", "objects", "garbage")
	if err := os.WriteFile(garbage, make([]byte, 700), 0o600); err != nil {
		t.Fatal(err)
	}
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		// Simulate gc pruning one file inside .git.
		return os.Remove(filepath.Join(repoPath, ".git", "objects", "garbage"))
	})

	res := Execute(gitResults(repo, "myrepo", 5000), nil)
	if res.Removed != 1 || res.Failed != 0 {
		t.Fatalf("Removed=%d Failed=%d, want 1/0 (errors: %v)", res.Removed, res.Failed, res.Errors)
	}
	if res.BytesFreed != 700 {
		t.Errorf("BytesFreed = %d, want 700 (measured .git shrink)", res.BytesFreed)
	}
}

func TestGitGC_GrowthClampedAtZero(t *testing.T) {
	repo := fakeRepo(t, 100)
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		return os.WriteFile(filepath.Join(repoPath, ".git", "objects", "new"), make([]byte, 300), 0o600)
	})

	res := Execute(gitResults(repo, "myrepo", 5000), nil)
	if res.Removed != 1 || res.BytesFreed != 0 {
		t.Errorf("Removed=%d BytesFreed=%d, want 1/0 (growth is not negative freed space)", res.Removed, res.BytesFreed)
	}
}

func TestGitGC_BlockedPathNeverRunsGit(t *testing.T) {
	called := false
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		called = true
		return nil
	})

	res := Execute(gitResults("/System/Library", "system", 5000), nil)
	if called {
		t.Error("runGitCleanup invoked on a blocked path")
	}
	if res.Failed != 1 || res.Removed != 0 || res.BytesFreed != 0 {
		t.Errorf("Failed=%d Removed=%d BytesFreed=%d, want 1/0/0", res.Failed, res.Removed, res.BytesFreed)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error(), "blocked") {
		t.Errorf("expected a blocked error, got %v", res.Errors)
	}
}

func TestGitGC_SkippedWhileGitIsBusy(t *testing.T) {
	for _, lock := range []string{"index.lock", "gc.pid"} {
		t.Run(lock, func(t *testing.T) {
			repo := fakeRepo(t, 100)
			if err := os.WriteFile(filepath.Join(repo, ".git", lock), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			called := false
			stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
				called = true
				return nil
			})

			res := Execute(gitResults(repo, "myrepo", 5000), nil)
			if called {
				t.Errorf("runGitCleanup invoked while %s exists", lock)
			}
			if res.Skipped != 1 || res.Removed != 0 || res.Failed != 0 || res.BytesFreed != 0 {
				t.Errorf("Skipped=%d Removed=%d Failed=%d BytesFreed=%d, want 1/0/0/0", res.Skipped, res.Removed, res.Failed, res.BytesFreed)
			}
		})
	}
}

func TestGitGC_MissingRepoSkipped(t *testing.T) {
	called := false
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		called = true
		return nil
	})

	res := Execute(gitResults(filepath.Join(t.TempDir(), "gone"), "gone", 5000), nil)
	if called {
		t.Error("runGitCleanup invoked on a repository that no longer exists")
	}
	if res.Skipped != 1 || res.Failed != 0 {
		t.Errorf("Skipped=%d Failed=%d, want 1/0", res.Skipped, res.Failed)
	}
}

func TestGitGCPseudoPath_Failure(t *testing.T) {
	repo := fakeRepo(t, 100)
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		return fmt.Errorf("git gc failed")
	})

	res := Execute(gitResults(repo, "broken", 1000), nil)

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0", res.Removed)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1", res.Failed)
	}
	if res.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0", res.BytesFreed)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(res.Errors))
	}
	if !strings.Contains(res.Errors[0].Error(), "git cleanup") {
		t.Errorf("error %q should contain 'git cleanup'", res.Errors[0])
	}
}

func TestGitGCWithLFS(t *testing.T) {
	repo := fakeRepo(t, 100)
	var calledLFS bool
	stubGitCleanup(t, func(repoPath string, hasLFS bool) error {
		calledLFS = hasLFS
		return nil
	})

	res := Execute(gitResults(repo, "lfsrepo (.git 100 MB) [LFS]", 10000), nil)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	if !calledLFS {
		t.Errorf("runGitCleanup called with hasLFS=false, want true (description contains [LFS])")
	}
}

func TestRegularPathsUnchanged(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "regular.txt")
	os.WriteFile(f, []byte("data"), 0644)

	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test",
			Entries: []scan.ScanEntry{
				{Path: f, Description: "regular", Size: 4},
			},
			TotalSize: 4,
		},
	}

	res := Execute(results, nil)

	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Errorf("file should be deleted via os.RemoveAll")
	}
}

func TestDockerPseudoPathStillSkipped(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "docker",
			Description: "Docker",
			Entries: []scan.ScanEntry{
				{Path: "docker:BuildCache", Description: "Build Cache", Size: 2000},
			},
			TotalSize: 2000,
		},
	}

	res := Execute(results, nil)

	if res.Removed != 0 {
		t.Errorf("Removed = %d, want 0 (docker pseudo-path should still be skipped)", res.Removed)
	}
	if res.Skipped != 1 || res.Failed != 0 {
		t.Errorf("Skipped=%d Failed=%d, want 1/0", res.Skipped, res.Failed)
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

// TestExecuteRemovesOnlyEntryPath pins the contract that scanners rely on
// when they set SizeComponent.Removed: cleanup deletes entry.Path only, and
// any other counted component paths are left in place.
func TestExecuteRemovesOnlyEntryPath(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "App.app")
	lib := filepath.Join(tmp, "Library", "Containers", "com.example.app")
	os.MkdirAll(bundle, 0755)
	os.MkdirAll(lib, 0755)
	os.WriteFile(filepath.Join(bundle, "bin"), make([]byte, 10), 0644)
	os.WriteFile(filepath.Join(lib, "data"), make([]byte, 20), 0644)

	results := []scan.CategoryResult{{
		Category: "unused-apps",
		Entries: []scan.ScanEntry{{
			Path: bundle,
			Size: 30,
			Components: []scan.SizeComponent{
				{Path: bundle, Size: 10, Removed: true},
				{Path: lib, Size: 20},
			},
		}},
	}}

	res := Execute(results, nil)
	if res.Removed != 1 || res.Failed != 0 {
		t.Fatalf("Removed=%d Failed=%d, want 1/0 (errors: %v)", res.Removed, res.Failed, res.Errors)
	}
	if _, err := os.Stat(bundle); !os.IsNotExist(err) {
		t.Error("entry path (component marked Removed) should be deleted")
	}
	if _, err := os.Stat(filepath.Join(lib, "data")); err != nil {
		t.Errorf("component not marked Removed must be left in place: %v", err)
	}
}

// mkFile creates a file of n bytes at path, creating parent directories.
func mkFile(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExecute_NestedEntriesCreditedOnce(t *testing.T) {
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "Library", "Caches", "Google")
	inner := filepath.Join(outer, "Chrome", "Default")
	mkFile(t, filepath.Join(inner, "data"), 1000)

	results := []scan.CategoryResult{
		{Category: "system-caches", Entries: []scan.ScanEntry{{Path: outer, Size: 1000}}},
		{Category: "browser-chrome", Entries: []scan.ScanEntry{{Path: inner, Size: 1000}}},
	}
	res := Execute(results, nil)
	if res.Removed != 1 || res.Skipped != 1 || res.Failed != 0 {
		t.Errorf("Removed=%d Skipped=%d Failed=%d, want 1/1/0", res.Removed, res.Skipped, res.Failed)
	}
	if res.BytesFreed != 1000 {
		t.Errorf("BytesFreed = %d, want 1000 (outer only)", res.BytesFreed)
	}
	if _, err := os.Stat(outer); !os.IsNotExist(err) {
		t.Error("outer directory should be deleted")
	}
}

func TestExecute_InnerFirstThenOuter(t *testing.T) {
	// When the inner entry comes first both exist and both are removed. The
	// outer path's size already includes the inner one, so the bytes
	// credited for the inner are not credited again.
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "a")
	inner := filepath.Join(outer, "b")
	mkFile(t, filepath.Join(inner, "f"), 10)

	results := []scan.CategoryResult{
		{Category: "x", Entries: []scan.ScanEntry{{Path: inner, Size: 10}}},
		{Category: "y", Entries: []scan.ScanEntry{{Path: outer, Size: 10}}},
	}
	res := Execute(results, nil)
	if res.Removed != 2 || res.Failed != 0 {
		t.Errorf("Removed=%d Failed=%d, want 2/0", res.Removed, res.Failed)
	}
	if res.BytesFreed != 10 {
		t.Errorf("BytesFreed = %d, want 10 (inner bytes credited once)", res.BytesFreed)
	}
}

func TestExecute_SamePathInTwoCategoriesCreditedOnce(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "Movies", "a.mp4")
	mkFile(t, f, 1000)

	entry := scan.ScanEntry{Path: f, Size: 1000}
	results := []scan.CategoryResult{
		{Category: "home-large-files", Entries: []scan.ScanEntry{entry}},
		{Category: "home-old-files", Entries: []scan.ScanEntry{entry}},
	}
	res := Execute(results, nil)
	if res.Removed != 1 || res.Skipped != 1 || res.Failed != 0 {
		t.Errorf("Removed=%d Skipped=%d Failed=%d, want 1/1/0", res.Removed, res.Skipped, res.Failed)
	}
	if res.BytesFreed != 1000 {
		t.Errorf("BytesFreed = %d, want 1000", res.BytesFreed)
	}
}

func TestExecute_SamePathDifferentCaseCreditedOnce(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "Caches", "Yarn")
	mkFile(t, filepath.Join(dir, "f"), 10)

	results := []scan.CategoryResult{
		{Category: "system-caches", Entries: []scan.ScanEntry{{Path: dir, Size: 10}}},
		{Category: "dev-yarn", Entries: []scan.ScanEntry{{Path: filepath.Join(tmp, "Caches", "yarn"), Size: 10}}},
	}
	res := Execute(results, nil)
	if res.Removed != 1 || res.Skipped != 1 || res.BytesFreed != 10 {
		t.Errorf("Removed=%d Skipped=%d BytesFreed=%d, want 1/1/10", res.Removed, res.Skipped, res.BytesFreed)
	}
}

func TestExecute_UnusedAppCreditsRemovedSizeOnly(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "Applications", "X.app")
	lib := filepath.Join(tmp, "Library", "Application Support", "X")
	mkFile(t, filepath.Join(bundle, "bin"), 100)
	mkFile(t, filepath.Join(lib, "data"), 900)

	results := []scan.CategoryResult{{
		Category: "unused-apps",
		Entries: []scan.ScanEntry{{
			Path: bundle,
			Size: 1000,
			Components: []scan.SizeComponent{
				{Path: bundle, Size: 100, Removed: true},
				{Path: lib, Size: 900, Note: "app data in ~/Library"},
			},
		}},
	}}
	res := Execute(results, nil)
	if res.Removed != 1 || res.Failed != 0 {
		t.Fatalf("Removed=%d Failed=%d, want 1/0 (errors: %v)", res.Removed, res.Failed, res.Errors)
	}
	if res.BytesFreed != 100 {
		t.Errorf("BytesFreed = %d, want 100 (bundle only)", res.BytesFreed)
	}
	if _, err := os.Stat(filepath.Join(lib, "data")); err != nil {
		t.Errorf("Library data must be left in place: %v", err)
	}
}

func TestExecute_DockerAndTmutilSkipped(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "dev-docker", Entries: []scan.ScanEntry{{
			Path: "docker:Images", Size: 20_000_000_000,
			Components: []scan.SizeComponent{{Path: "docker:Images", Size: 20_000_000_000}},
		}}},
		{Category: "sysdata-timemachine", Entries: []scan.ScanEntry{{
			Path:       "tmutil:snapshot:com.apple.TimeMachine.2026-11-18-000000.local",
			Components: []scan.SizeComponent{{Path: "tmutil:snapshot:com.apple.TimeMachine.2026-11-18-000000.local"}},
		}}},
	}
	res := Execute(results, nil)
	if res.Skipped != 2 || res.Failed != 0 || res.Removed != 0 || res.BytesFreed != 0 || len(res.Errors) != 0 {
		t.Errorf("Skipped=%d Failed=%d Removed=%d BytesFreed=%d Errors=%v, want 2/0/0/0/none",
			res.Skipped, res.Failed, res.Removed, res.BytesFreed, res.Errors)
	}
}

func TestExecute_BlockedBundleStaysFailed(t *testing.T) {
	results := []scan.CategoryResult{{
		Category: "unused-apps",
		Entries: []scan.ScanEntry{{
			Path: "/Applications/Docker.app", Size: 71_000,
			Components: []scan.SizeComponent{
				{Path: "/Applications/Docker.app", Size: 2_000},
				{Path: "/Users/x/Library/Containers/com.docker.docker", Size: 69_000},
			},
		}},
	}}
	res := Execute(results, nil)
	if res.Failed != 1 || res.Removed != 0 || res.Skipped != 0 || res.BytesFreed != 0 {
		t.Errorf("Failed=%d Removed=%d Skipped=%d BytesFreed=%d, want 1/0/0/0", res.Failed, res.Removed, res.Skipped, res.BytesFreed)
	}
}
