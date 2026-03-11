package gitrepos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/safety"
)

// mockRunner returns a CmdRunner that produces canned output for git count-objects -v.
func mockRunner(output string, err error) CmdRunner {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte(output), err
	}
}

func TestScanFindsGitDirs(t *testing.T) {
	root := t.TempDir()

	// Create two repos with .git directories.
	repo1 := filepath.Join(root, "projects", "repo1")
	repo2 := filepath.Join(root, "projects", "repo2")
	os.MkdirAll(filepath.Join(repo1, ".git", "objects"), 0755)
	os.MkdirAll(filepath.Join(repo2, ".git", "objects"), 0755)

	// Put some data so DirSize > 0.
	os.WriteFile(filepath.Join(repo1, ".git", "objects", "pack"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(repo2, ".git", "objects", "pack"), make([]byte, 2048), 0644)

	countOutput := "count: 10\nsize: 100\nin-pack: 0\npacks: 0\nsize-pack: 0\nprune-packable: 0\ngarbage: 0\nsize-garbage: 50\n"
	runner := mockRunner(countOutput, nil)

	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 CategoryResult, got %d", len(results))
	}

	cr := results[0]
	if cr.Category != "git-repos" {
		t.Errorf("Category = %q, want %q", cr.Category, "git-repos")
	}
	if len(cr.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cr.Entries))
	}

	// Verify entries contain correct repo paths.
	paths := make(map[string]bool)
	for _, e := range cr.Entries {
		paths[e.Path] = true
	}
	if !paths["git-gc:"+repo1] {
		t.Errorf("missing entry for repo1, got paths: %v", paths)
	}
	if !paths["git-gc:"+repo2] {
		t.Errorf("missing entry for repo2, got paths: %v", paths)
	}
}

func TestSkipSubmodule(t *testing.T) {
	root := t.TempDir()

	// Create a submodule-style .git file (not directory).
	repo := filepath.Join(root, "projects", "myrepo")
	os.MkdirAll(repo, 0755)
	// Write a .git FILE, not directory.
	os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: ../other/.git/modules/myrepo"), 0644)

	runner := mockRunner("", nil)
	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for submodule .git file, got %d categories", len(results))
	}
}

func TestSkipLockedRepo(t *testing.T) {
	root := t.TempDir()

	repo := filepath.Join(root, "projects", "locked-repo")
	gitDir := filepath.Join(repo, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "objects", "dummy"), make([]byte, 512), 0644)
	// Create index.lock to simulate ongoing git operation.
	os.WriteFile(filepath.Join(gitDir, "index.lock"), []byte{}, 0644)

	runner := mockRunner("size: 100\nsize-garbage: 0\n", nil)
	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for locked repo, got %d categories", len(results))
	}
}

func TestParseCountObjects(t *testing.T) {
	output := "count: 1251\nsize: 5492\nin-pack: 0\npacks: 0\nsize-pack: 0\nprune-packable: 0\ngarbage: 0\nsize-garbage: 100\n"
	got := parseCountObjects(output)
	// (5492 + 100) * 1024 = 5726208
	want := int64(5726208)
	if got != want {
		t.Errorf("parseCountObjects() = %d, want %d", got, want)
	}
}

func TestParseCountObjects_Empty(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty string", ""},
		{"malformed", "this is not valid output"},
		{"no size fields", "count: 10\npacks: 0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCountObjects(tt.output)
			if got != 0 {
				t.Errorf("parseCountObjects(%q) = %d, want 0", tt.output, got)
			}
		})
	}
}

func TestDescriptionFormat(t *testing.T) {
	root := t.TempDir()

	repo := filepath.Join(root, "projects", "myapp")
	gitDir := filepath.Join(repo, ".git")
	os.MkdirAll(filepath.Join(gitDir, "objects"), 0755)
	os.WriteFile(filepath.Join(gitDir, "objects", "data"), make([]byte, 4096), 0644)

	countOutput := "size: 200\nsize-garbage: 50\n"
	runner := mockRunner(countOutput, nil)

	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if len(results) == 0 || len(results[0].Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	desc := results[0].Entries[0].Description
	if !strings.Contains(desc, "myapp") {
		t.Errorf("description %q should contain repo name 'myapp'", desc)
	}
	if !strings.Contains(desc, ".git") {
		t.Errorf("description %q should contain '.git'", desc)
	}
	if !strings.Contains(desc, "reclaimable") {
		t.Errorf("description %q should contain 'reclaimable'", desc)
	}
}

func TestDescriptionFormat_WithLFS(t *testing.T) {
	root := t.TempDir()

	repo := filepath.Join(root, "projects", "lfs-repo")
	gitDir := filepath.Join(repo, ".git")
	os.MkdirAll(filepath.Join(gitDir, "lfs"), 0755)
	os.MkdirAll(filepath.Join(gitDir, "objects"), 0755)
	os.WriteFile(filepath.Join(gitDir, "objects", "data"), make([]byte, 1024), 0644)

	countOutput := "size: 100\nsize-garbage: 0\n"
	runner := mockRunner(countOutput, nil)

	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if len(results) == 0 || len(results[0].Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	desc := results[0].Entries[0].Description
	if !strings.Contains(desc, "[LFS]") {
		t.Errorf("description %q should contain '[LFS]'", desc)
	}
}

func TestLFSDetection(t *testing.T) {
	root := t.TempDir()

	repo := filepath.Join(root, "projects", "lfs-project")
	gitDir := filepath.Join(repo, ".git")
	os.MkdirAll(filepath.Join(gitDir, "lfs", "objects"), 0755)
	os.MkdirAll(filepath.Join(gitDir, "objects"), 0755)
	os.WriteFile(filepath.Join(gitDir, "objects", "data"), make([]byte, 512), 0644)

	runner := mockRunner("size: 50\nsize-garbage: 0\n", nil)

	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if len(results) == 0 || len(results[0].Entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	desc := results[0].Entries[0].Description
	if !strings.Contains(desc, "[LFS]") {
		t.Errorf("description should contain [LFS] for repo with .git/lfs, got: %q", desc)
	}
}

func TestGitNotInstalled(t *testing.T) {
	// Override lookPath to simulate git not being found.
	origLookPath := lookPathFn
	lookPathFn = func(name string) (string, error) {
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	t.Cleanup(func() { lookPathFn = origLookPath })

	results, err := scanWithRunner(mockRunner("", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results when git not installed, got %d categories", len(results))
	}
}

func TestEntryPathPrefix(t *testing.T) {
	root := t.TempDir()

	repo := filepath.Join(root, "projects", "prefixed")
	os.MkdirAll(filepath.Join(repo, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(repo, ".git", "objects", "data"), make([]byte, 256), 0644)

	runner := mockRunner("size: 10\nsize-garbage: 0\n", nil)

	results, err := scanFrom(root, runner)
	if err != nil {
		t.Fatalf("scanFrom() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one category result")
	}

	for _, e := range results[0].Entries {
		if !strings.HasPrefix(e.Path, "git-gc:") {
			t.Errorf("entry path %q should have 'git-gc:' prefix", e.Path)
		}
		repoPath := strings.TrimPrefix(e.Path, "git-gc:")
		if !strings.HasPrefix(repoPath, "/") {
			t.Errorf("repo path %q should be absolute", repoPath)
		}
	}
}

func TestRiskForCategory_GitRepos(t *testing.T) {
	got := safety.RiskForCategory("git-repos")
	if got != "moderate" {
		t.Errorf("RiskForCategory('git-repos') = %q, want %q", got, "moderate")
	}
}
