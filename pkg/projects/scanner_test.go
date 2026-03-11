package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

func TestScanNodeModules(t *testing.T) {
	root := t.TempDir()
	pkg := map[string]string{"name": "my-node-app"}
	pkgJSON, _ := json.Marshal(pkg)
	projDir := filepath.Join(root, "myapp")
	os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(projDir, "package.json"), pkgJSON, 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category, got nil")
	}
	if len(found.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(found.Entries))
	}
	if found.Entries[0].Size == 0 {
		t.Error("expected nonzero size")
	}
}

func TestScanSwiftBuild(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "swiftapp")
	os.MkdirAll(filepath.Join(projDir, ".build"), 0755)
	os.WriteFile(filepath.Join(projDir, ".build", "artifact"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(projDir, "Package.swift"), []byte(`// swift-tools-version:5.5
let package = Package(name: "SwiftApp")
`), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-swift")
	if found == nil {
		t.Fatal("expected proj-swift category")
	}
	if len(found.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(found.Entries))
	}
}

func TestScanRustTarget(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "rustapp")
	os.MkdirAll(filepath.Join(projDir, "target"), 0755)
	os.WriteFile(filepath.Join(projDir, "target", "binary"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(projDir, "Cargo.toml"), []byte(`[package]
name = "rustapp"
version = "0.1.0"
`), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-rust")
	if found == nil {
		t.Fatal("expected proj-rust category")
	}
}

func TestScanPythonVenv(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "pyapp")
	venvDir := filepath.Join(projDir, "venv")
	os.MkdirAll(venvDir, 0755)
	os.WriteFile(filepath.Join(venvDir, "pyvenv.cfg"), []byte("home = /usr/bin/python3"), 0644)
	os.WriteFile(filepath.Join(venvDir, "data.bin"), make([]byte, 1024), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-python")
	if found == nil {
		t.Fatal("expected proj-python category")
	}
}

func TestScanPythonDotVenv(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "pyapp2")
	venvDir := filepath.Join(projDir, ".venv")
	os.MkdirAll(venvDir, 0755)
	os.WriteFile(filepath.Join(venvDir, "pyvenv.cfg"), []byte("home = /usr/bin/python3"), 0644)
	os.WriteFile(filepath.Join(venvDir, "data.bin"), make([]byte, 1024), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-python")
	if found == nil {
		t.Fatal("expected proj-python category")
	}
}

func TestScanJavaBuild(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "javaapp")
	os.MkdirAll(filepath.Join(projDir, "build"), 0755)
	os.WriteFile(filepath.Join(projDir, "build", "output.jar"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(projDir, "build.gradle"), []byte("apply plugin: 'java'"), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-java")
	if found == nil {
		t.Fatal("expected proj-java category")
	}
}

func TestNoFalsePositive(t *testing.T) {
	root := t.TempDir()
	// Create "build" dir without any manifest
	os.MkdirAll(filepath.Join(root, "somedir", "build"), 0755)
	os.WriteFile(filepath.Join(root, "somedir", "build", "file.txt"), make([]byte, 512), 0644)

	results := scanWithRoot(t, root)
	for _, r := range results {
		if r.Category == "proj-java" {
			t.Errorf("false positive: detected proj-java without manifest")
		}
	}
}

func TestStalenessMarker(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "oldapp")
	os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
	manifestPath := filepath.Join(projDir, "package.json")
	os.WriteFile(manifestPath, []byte(`{"name":"oldapp"}`), 0644)

	// Set manifest mtime to 8 months ago
	staleTime := time.Now().Add(-8 * 30 * 24 * time.Hour)
	os.Chtimes(manifestPath, staleTime, staleTime)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if len(found.Entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}
	if !strings.Contains(found.Entries[0].Description, "\u26a0") {
		t.Errorf("stale entry should contain warning symbol, got: %s", found.Entries[0].Description)
	}
}

func TestRecentProject(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "newapp")
	os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{"name":"newapp"}`), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if strings.Contains(found.Entries[0].Description, "\u26a0") {
		t.Errorf("recent entry should NOT contain warning symbol, got: %s", found.Entries[0].Description)
	}
}

func TestAggregateSummary(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"app1", "app2", "app3"} {
		projDir := filepath.Join(root, name)
		os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
		os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
		os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{"name":"`+name+`"}`), 0644)
	}

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if len(found.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(found.Entries))
	}
	// Description should contain count
	if !strings.Contains(found.Description, "3 dirs") {
		t.Errorf("category description should contain count, got: %s", found.Description)
	}
}

func TestProjectNameFromPackageJson(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "mydir")
	os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{"name":"custom-name"}`), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if !strings.Contains(found.Entries[0].Description, "custom-name") {
		t.Errorf("expected project name from package.json, got: %s", found.Entries[0].Description)
	}
}

func TestProjectNameFallback(t *testing.T) {
	root := t.TempDir()
	projDir := filepath.Join(root, "my-project")
	os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(projDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
	// package.json with no name field
	os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{}`), 0644)

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if !strings.Contains(found.Entries[0].Description, "my-project") {
		t.Errorf("expected fallback to parent dir name, got: %s", found.Entries[0].Description)
	}
}

func TestSinglePassWalk(t *testing.T) {
	root := t.TempDir()

	// Create multiple project types
	nodeDir := filepath.Join(root, "nodeapp")
	os.MkdirAll(filepath.Join(nodeDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(nodeDir, "node_modules", "dep.js"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte(`{"name":"nodeapp"}`), 0644)

	rustDir := filepath.Join(root, "rustapp")
	os.MkdirAll(filepath.Join(rustDir, "target"), 0755)
	os.WriteFile(filepath.Join(rustDir, "target", "binary"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(rustDir, "Cargo.toml"), []byte("[package]\nname = \"rustapp\"\n"), 0644)

	results := scanWithRoot(t, root)

	categories := make(map[string]bool)
	for _, r := range results {
		categories[r.Category] = true
	}
	if !categories["proj-node"] {
		t.Error("missing proj-node from single pass")
	}
	if !categories["proj-rust"] {
		t.Error("missing proj-rust from single pass")
	}
}

func TestEntriesSortedBySizeDesc(t *testing.T) {
	root := t.TempDir()

	// Create 3 node projects with different sizes
	sizes := map[string]int{"small": 100, "medium": 5000, "large": 50000}
	for name, size := range sizes {
		projDir := filepath.Join(root, name)
		os.MkdirAll(filepath.Join(projDir, "node_modules"), 0755)
		os.WriteFile(filepath.Join(projDir, "node_modules", "data"), make([]byte, size), 0644)
		os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{"name":"`+name+`"}`), 0644)
	}

	results := scanWithRoot(t, root)
	found := findCategory(results, "proj-node")
	if found == nil {
		t.Fatal("expected proj-node category")
	}
	if len(found.Entries) < 2 {
		t.Fatal("need at least 2 entries to test sorting")
	}

	sorted := sort.SliceIsSorted(found.Entries, func(i, j int) bool {
		return found.Entries[i].Size > found.Entries[j].Size
	})
	if !sorted {
		t.Error("entries not sorted by size descending")
	}
}

// scanWithRoot is a test helper that scans from a custom root instead of home dir.
func scanWithRoot(t *testing.T, root string) []scan.CategoryResult {
	t.Helper()
	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom(%s): %v", root, err)
	}
	return results
}

func findCategory(results []scan.CategoryResult, category string) *scan.CategoryResult {
	for i := range results {
		if results[i].Category == category {
			return &results[i]
		}
	}
	return nil
}
