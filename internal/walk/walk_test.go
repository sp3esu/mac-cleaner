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

func TestOnFileCallback(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "a/b")
	writeFile(t, filepath.Join(root, "top.txt"), 10)
	writeFile(t, filepath.Join(root, "a", "mid.txt"), 10)
	writeFile(t, filepath.Join(root, "a", "b", "deep.txt"), 10)

	type fileInfo struct {
		rel   string
		depth int
	}
	var files []fileInfo
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		OnFile: func(path string, d fs.DirEntry, depth int) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, fileInfo{rel: rel, depth: depth})
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// Check depths are correct.
	depthMap := make(map[string]int)
	for _, f := range files {
		depthMap[f.rel] = f.depth
	}
	if depthMap["top.txt"] != 1 {
		t.Errorf("top.txt depth = %d, want 1", depthMap["top.txt"])
	}
	if depthMap[filepath.Join("a", "mid.txt")] != 2 {
		t.Errorf("a/mid.txt depth = %d, want 2", depthMap[filepath.Join("a", "mid.txt")])
	}
	if depthMap[filepath.Join("a", "b", "deep.txt")] != 3 {
		t.Errorf("a/b/deep.txt depth = %d, want 3", depthMap[filepath.Join("a", "b", "deep.txt")])
	}
}

func TestOnFileMaxDepth(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "a/b/c")
	writeFile(t, filepath.Join(root, "top.txt"), 10)
	writeFile(t, filepath.Join(root, "a", "mid.txt"), 10)
	writeFile(t, filepath.Join(root, "a", "b", "deep.txt"), 10)
	writeFile(t, filepath.Join(root, "a", "b", "c", "tooDeep.txt"), 10)

	var files []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 2,
		OnFile: func(path string, d fs.DirEntry, depth int) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, rel)
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	found := make(map[string]bool)
	for _, f := range files {
		found[f] = true
	}

	if !found["top.txt"] {
		t.Error("expected top.txt at depth 1")
	}
	if !found[filepath.Join("a", "mid.txt")] {
		t.Error("expected a/mid.txt at depth 2")
	}
	if found[filepath.Join("a", "b", "deep.txt")] {
		t.Error("should not see a/b/deep.txt beyond MaxDepth=2")
	}
	if found[filepath.Join("a", "b", "c", "tooDeep.txt")] {
		t.Error("should not see a/b/c/tooDeep.txt beyond MaxDepth=2")
	}
}

func TestOnFileExclusions(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "Library")
	mkdirAll(t, root, "Library/Caches")
	mkdirAll(t, root, "project/node_modules")
	mkdirAll(t, root, "project/src")
	writeFile(t, filepath.Join(root, "Library", "Caches", "data.bin"), 10)
	writeFile(t, filepath.Join(root, "project", "node_modules", "pkg.js"), 10)
	writeFile(t, filepath.Join(root, "project", "src", "main.go"), 10)

	var files []string
	err := Walk(Options{
		Root:       root,
		MaxDepth:   5,
		Exclude:    []string{"node_modules"},
		ExcludeAbs: []string{libDir},
		OnFile: func(path string, d fs.DirEntry, depth int) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, rel)
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	found := make(map[string]bool)
	for _, f := range files {
		found[f] = true
	}

	// File inside excluded abs path should NOT appear.
	if found[filepath.Join("Library", "Caches", "data.bin")] {
		t.Error("file in excluded abs path Library/ should not be reported")
	}
	// File inside excluded dir name should NOT appear.
	if found[filepath.Join("project", "node_modules", "pkg.js")] {
		t.Error("file in excluded dir node_modules/ should not be reported")
	}
	// File in non-excluded dir SHOULD appear.
	if !found[filepath.Join("project", "src", "main.go")] {
		t.Error("file in project/src/ should be reported")
	}
}

func TestOnFileNilBackwardCompat(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "a")
	writeFile(t, filepath.Join(root, "a", "file.txt"), 10)

	var dirs []string
	err := Walk(Options{
		Root:     root,
		MaxDepth: 5,
		// OnFile is nil -- should not panic.
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			rel, _ := filepath.Rel(root, path)
			dirs = append(dirs, rel)
			return true
		},
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	// Dirs should still be visited.
	found := make(map[string]bool)
	for _, d := range dirs {
		found[d] = true
	}
	if !found["a"] {
		t.Error("OnDir should still work when OnFile is nil")
	}
}

// writeFile creates a file with the given size in bytes.
func writeFile(t *testing.T, path string, sizeBytes int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, sizeBytes), 0644); err != nil {
		t.Fatalf("writeFile(%s): %v", path, err)
	}
}

// mkdirAll creates a nested directory structure under root.
func mkdirAll(t *testing.T, root, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, path), 0755); err != nil {
		t.Fatalf("mkdirAll(%s): %v", path, err)
	}
}
