package walk

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestMaxDepth(t *testing.T) {
	root := t.TempDir()
	// Create nested dirs: root/a/b/c/d (depths 1,2,3,4)
	mkdirAll(t, root, "a/b/c/d")

	var visited []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 2,
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	// Should visit root (.), a (depth 1), b (depth 2) but NOT c or d
	sort.Strings(visited)
	for _, v := range visited {
		if v == filepath.Join("a", "b", "c") || v == filepath.Join("a", "b", "c", "d") {
			t.Errorf("visited dir beyond MaxDepth=2: %s", v)
		}
	}
	// Must visit a and a/b
	found := make(map[string]bool)
	for _, v := range visited {
		found[v] = true
	}
	if !found["a"] {
		t.Error("did not visit 'a' at depth 1")
	}
	if !found[filepath.Join("a", "b")] {
		t.Error("did not visit 'a/b' at depth 2")
	}
}

func TestExcludeByName(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "project/node_modules/pkg")
	mkdirAll(t, root, "project/.git/objects")
	mkdirAll(t, root, "project/src")

	var visited []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		Exclude:  []string{"node_modules", ".git"},
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	for _, v := range visited {
		base := filepath.Base(v)
		if base == "node_modules" || base == ".git" {
			t.Errorf("visited excluded dir: %s", v)
		}
	}
}

func TestExcludeByAbsPath(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "Library")
	mkdirAll(t, root, "Library/Caches/stuff")
	mkdirAll(t, root, "code/project")

	var visited []string
	err := Walk(Options{
		Root:       root,
		MaxDepth:   5,
		ExcludeAbs: []string{libDir},
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	for _, v := range visited {
		if v == "Library" || v == filepath.Join("Library", "Caches") {
			t.Errorf("visited excluded abs path dir: %s", v)
		}
	}
}

func TestDotfileDirs(t *testing.T) {
	root := t.TempDir()
	// Depth 1 dotfile dirs should be skipped
	mkdirAll(t, root, ".config/stuff")
	mkdirAll(t, root, ".local/share")
	// Depth 2+ dotfile dirs like .build should be visited
	mkdirAll(t, root, "project/.build/debug")
	mkdirAll(t, root, "project/src")

	var visited []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	found := make(map[string]bool)
	for _, v := range visited {
		found[v] = true
	}

	// Depth 1 dotfile dirs should NOT be visited
	if found[".config"] {
		t.Error("visited .config at depth 1 (should be skipped)")
	}
	if found[".local"] {
		t.Error("visited .local at depth 1 (should be skipped)")
	}

	// Depth 2 .build SHOULD be visited
	if !found[filepath.Join("project", ".build")] {
		t.Error("did not visit project/.build at depth 2 (should be allowed)")
	}
}

func TestOnDirCallbackSkip(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "a/deep/nested")
	mkdirAll(t, root, "b/other")

	var visited []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			// Don't descend into "a"
			return d.Name() != "a"
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	for _, v := range visited {
		if v == filepath.Join("a", "deep") || v == filepath.Join("a", "deep", "nested") {
			t.Errorf("descended into 'a' despite callback returning false: %s", v)
		}
	}
}

func TestPermissionError(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("permission test requires unix")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission denial as root")
	}

	root := t.TempDir()
	mkdirAll(t, root, "readable/child")
	unreadable := filepath.Join(root, "unreadable")
	mkdirAll(t, root, "unreadable/secret")
	os.Chmod(unreadable, 0000)
	t.Cleanup(func() { os.Chmod(unreadable, 0755) })

	var visited []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk should not fail on permission errors: %v", err)
	}

	found := make(map[string]bool)
	for _, v := range visited {
		found[v] = true
	}
	if !found["readable"] {
		t.Error("did not visit readable dir")
	}
}

// mkdirAll creates a nested directory structure under root.
func mkdirAll(t *testing.T, root, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, path), 0755); err != nil {
		t.Fatalf("mkdirAll(%s): %v", path, err)
	}
}
