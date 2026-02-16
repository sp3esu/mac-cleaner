package developer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// writeFile is a test helper that creates a file with the given size,
// creating parent directories as needed.
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

// --- Xcode DerivedData tests ---

func TestScanXcodeMissing(t *testing.T) {
	home := t.TempDir()
	result := scanXcodeDerivedData(home)
	if result != nil {
		t.Fatal("expected nil for missing Xcode DerivedData")
	}
}

func TestScanXcodeWithData(t *testing.T) {
	home := t.TempDir()
	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	writeFile(t, filepath.Join(derivedData, "MyApp-abc123", "Build", "Products", "app.o"), 1000)
	writeFile(t, filepath.Join(derivedData, "OtherApp-def456", "Build", "Products", "lib.o"), 500)

	result := scanXcodeDerivedData(home)
	if result == nil {
		t.Fatal("expected non-nil result for Xcode with data")
	}

	if result.Category != "dev-xcode" {
		t.Errorf("expected category 'dev-xcode', got %q", result.Category)
	}
	if result.Description != "Xcode DerivedData" {
		t.Errorf("expected description 'Xcode DerivedData', got %q", result.Description)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	expectedTotal := int64(1500)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}

	// Entries should be sorted by size descending.
	if result.Entries[0].Size != 1000 {
		t.Errorf("expected first entry size 1000, got %d", result.Entries[0].Size)
	}
}

func TestScanXcodeEmptyDir(t *testing.T) {
	home := t.TempDir()
	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	os.MkdirAll(derivedData, 0755)

	result := scanXcodeDerivedData(home)
	if result != nil {
		t.Fatal("expected nil for empty DerivedData directory")
	}
}

// --- npm cache tests ---

func TestScanNpmMissing(t *testing.T) {
	home := t.TempDir()
	result := scanNpmCache(home)
	if result != nil {
		t.Fatal("expected nil for missing npm cache")
	}
}

func TestScanNpmWithData(t *testing.T) {
	home := t.TempDir()
	npmDir := filepath.Join(home, ".npm")
	writeFile(t, filepath.Join(npmDir, "_cacache", "content-v2", "sha512", "pkg.tgz"), 2000)
	writeFile(t, filepath.Join(npmDir, "_logs", "debug.log"), 100)

	result := scanNpmCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for npm with data")
	}

	if result.Category != "dev-npm" {
		t.Errorf("expected category 'dev-npm', got %q", result.Category)
	}
	if result.Description != "npm Cache" {
		t.Errorf("expected description 'npm Cache', got %q", result.Description)
	}

	expectedTotal := int64(2100)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

// --- yarn cache tests ---

func TestScanYarnMissing(t *testing.T) {
	home := t.TempDir()
	result := scanYarnCache(home)
	if result != nil {
		t.Fatal("expected nil for missing yarn cache")
	}
}

func TestScanYarnWithData(t *testing.T) {
	home := t.TempDir()
	yarnDir := filepath.Join(home, "Library", "Caches", "yarn")
	writeFile(t, filepath.Join(yarnDir, "v6", ".tmp", "pkg1.tgz"), 3000)
	writeFile(t, filepath.Join(yarnDir, "v6", ".tmp", "pkg2.tgz"), 1500)

	result := scanYarnCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for yarn with data")
	}

	if result.Category != "dev-yarn" {
		t.Errorf("expected category 'dev-yarn', got %q", result.Category)
	}
	if result.Description != "Yarn Cache" {
		t.Errorf("expected description 'Yarn Cache', got %q", result.Description)
	}

	// Yarn cache is treated as a single blob.
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}

	expectedTotal := int64(4500)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

// --- Homebrew cache tests ---

func TestScanHomebrewMissing(t *testing.T) {
	home := t.TempDir()
	result := scanHomebrew(home)
	if result != nil {
		t.Fatal("expected nil for missing Homebrew cache")
	}
}

func TestScanHomebrewWithData(t *testing.T) {
	home := t.TempDir()
	brewDir := filepath.Join(home, "Library", "Caches", "Homebrew")
	writeFile(t, filepath.Join(brewDir, "downloads", "pkg1.bottle.tar.gz"), 5000)
	writeFile(t, filepath.Join(brewDir, "Cask", "firefox.dmg"), 8000)

	result := scanHomebrew(home)
	if result == nil {
		t.Fatal("expected non-nil result for Homebrew with data")
	}

	if result.Category != "dev-homebrew" {
		t.Errorf("expected category 'dev-homebrew', got %q", result.Category)
	}
	if result.Description != "Homebrew Cache" {
		t.Errorf("expected description 'Homebrew Cache', got %q", result.Description)
	}

	expectedTotal := int64(13000)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

// --- Docker tests ---

func TestScanDockerNotInstalled(t *testing.T) {
	// Use a runner that should never be called.
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		t.Fatal("runner should not be called when docker is not installed")
		return nil, nil
	}

	// Save PATH and set to empty to simulate docker not being installed.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	result := scanDocker(runner)
	if result != nil {
		t.Fatal("expected nil when docker is not installed")
	}
}

func TestScanDockerDaemonStopped(t *testing.T) {
	fakeDockerPath(t)

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("Cannot connect to the Docker daemon")
	}

	result := scanDocker(runner)
	if result != nil {
		t.Fatal("expected nil when Docker daemon is not running")
	}
}

// fakeDockerPath creates a temporary directory with a fake docker executable
// and prepends it to PATH so exec.LookPath("docker") succeeds.
func fakeDockerPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	dockerPath := filepath.Join(dir, "docker")
	if err := os.WriteFile(dockerPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("create fake docker: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestScanDockerWithData(t *testing.T) {
	fakeDockerPath(t)

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		output := `{"Type":"Images","TotalCount":"5","Active":"2","Size":"2.3GB","Reclaimable":"1.2GB (52%)"}
{"Type":"Containers","TotalCount":"3","Active":"1","Size":"500MB","Reclaimable":"300MB (60%)"}
{"Type":"Local Volumes","TotalCount":"2","Active":"1","Size":"1GB","Reclaimable":"500MB (50%)"}
{"Type":"Build Cache","TotalCount":"10","Active":"0","Size":"3.5GB","Reclaimable":"3.5GB"}`
		return []byte(output), nil
	}

	result := scanDocker(runner)
	if result == nil {
		t.Fatal("expected non-nil result for Docker with data")
	}

	if result.Category != "dev-docker" {
		t.Errorf("expected category 'dev-docker', got %q", result.Category)
	}
	if result.Description != "Docker Reclaimable" {
		t.Errorf("expected description 'Docker Reclaimable', got %q", result.Description)
	}

	if len(result.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result.Entries))
	}

	// Entries should be sorted by size descending.
	// Build Cache (3.5GB) > Images (1.2GB) > Local Volumes (500MB) > Containers (300MB)
	if result.Entries[0].Description != "Docker Build Cache" {
		t.Errorf("expected first entry 'Docker Build Cache', got %q", result.Entries[0].Description)
	}

	// Check total is sum of all reclaimable amounts.
	expectedTotal := int64(3500000000) + int64(1200000000) + int64(500000000) + int64(300000000)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

func TestScanDockerEmptyOutput(t *testing.T) {
	fakeDockerPath(t)

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	result := scanDocker(runner)
	if result != nil {
		t.Fatal("expected nil for empty Docker output")
	}
}

func TestScanDockerAllZero(t *testing.T) {
	fakeDockerPath(t)

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		output := `{"Type":"Images","TotalCount":"0","Active":"0","Size":"0B","Reclaimable":"0B"}
{"Type":"Containers","TotalCount":"0","Active":"0","Size":"0B","Reclaimable":"0B"}`
		return []byte(output), nil
	}

	result := scanDocker(runner)
	if result != nil {
		t.Fatal("expected nil when all Docker reclaimable sizes are 0B")
	}
}

// --- parseDockerSize tests ---

func TestParseDockerSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0B", 0},
		{"100B", 100},
		{"1.5kB", 1500},
		{"1.5KB", 1500},
		{"16.43MB", 16430000},
		{"2.3GB", 2300000000},
		{"1TB", 1000000000000},
		{"1.2GB (52%)", 1200000000},
		{"300MB (60%)", 300000000},
		{"3.5GB", 3500000000},
		{"500MB (50%)", 500000000},
		{"", 0},
		{"invalid", 0},
		{"  ", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDockerSize(tt.input)
			if got != tt.expected {
				t.Errorf("parseDockerSize(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create Xcode DerivedData with a project.
	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	writeFile(t, filepath.Join(derivedData, "MyApp-abc123", "Build", "app.o"), 1000)

	// Create npm cache.
	npmDir := filepath.Join(home, ".npm")
	writeFile(t, filepath.Join(npmDir, "_cacache", "content", "pkg.tgz"), 2000)

	// No yarn, no Homebrew -- should be silently skipped.

	// Call private helpers directly (Scan() uses os.UserHomeDir()).
	var results []scan.CategoryResult
	if cr := scanXcodeDerivedData(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanNpmCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanYarnCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanHomebrew(home); cr != nil {
		results = append(results, *cr)
	}

	// Expect Xcode + npm, yarn and Homebrew silently skipped.
	if len(results) != 2 {
		t.Fatalf("expected 2 results (Xcode + npm), got %d", len(results))
	}

	if results[0].Category != "dev-xcode" {
		t.Errorf("expected first result 'dev-xcode', got %q", results[0].Category)
	}
	if results[1].Category != "dev-npm" {
		t.Errorf("expected second result 'dev-npm', got %q", results[1].Category)
	}
}
