package messaging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/scan/scantest"
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

// --- Slack Cache tests ---

func TestScanSlackCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanSlackCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Slack Cache")
	}
}

func TestScanSlackCacheWithData(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, "Library", "Application Support", "Slack", "Cache")
	swDir := filepath.Join(home, "Library", "Application Support", "Slack", "Service Worker", "CacheStorage")
	writeFile(t, filepath.Join(cacheDir, "data_0"), 3000)
	writeFile(t, filepath.Join(swDir, "sw_cache.db"), 2000)

	result := scanSlackCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Slack Cache with data")
	}
	if result.Category != "msg-slack" {
		t.Errorf("expected category 'msg-slack', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 3000, 2000); result.TotalSize != want {
		t.Errorf("expected total size %d, got %d", want, result.TotalSize)
	}
}

func TestScanSlackCachePartial(t *testing.T) {
	home := t.TempDir()
	// Only main cache exists.
	cacheDir := filepath.Join(home, "Library", "Application Support", "Slack", "Cache")
	writeFile(t, filepath.Join(cacheDir, "data_0"), 1500)

	result := scanSlackCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for partial Slack Cache")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 1500); result.TotalSize != want {
		t.Errorf("expected total size %d, got %d", want, result.TotalSize)
	}
}

// --- Discord Cache tests ---

func TestScanDiscordCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanDiscordCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Discord Cache")
	}
}

func TestScanDiscordCacheWithData(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, "Library", "Application Support", "discord", "Cache")
	codeDir := filepath.Join(home, "Library", "Application Support", "discord", "Code Cache")
	writeFile(t, filepath.Join(cacheDir, "data_1"), 4000)
	writeFile(t, filepath.Join(codeDir, "js", "code.js"), 1000)

	result := scanDiscordCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Discord Cache with data")
	}
	if result.Category != "msg-discord" {
		t.Errorf("expected category 'msg-discord', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 4000, 1000); result.TotalSize != want {
		t.Errorf("expected total size %d, got %d", want, result.TotalSize)
	}
}

// --- Teams Cache tests ---

func TestScanTeamsCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanTeamsCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Teams Cache")
	}
}

func TestScanTeamsCacheWithData(t *testing.T) {
	home := t.TempDir()
	teamsDir := filepath.Join(home, "Library", "Application Support", "Microsoft", "Teams", "Cache")
	teams2Dir := filepath.Join(home, "Library", "Caches", "com.microsoft.teams2")
	writeFile(t, filepath.Join(teamsDir, "data_0"), 2000)
	writeFile(t, filepath.Join(teams2Dir, "cache.db"), 3000)

	result := scanTeamsCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Teams Cache with data")
	}
	if result.Category != "msg-teams" {
		t.Errorf("expected category 'msg-teams', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 2000, 3000); result.TotalSize != want {
		t.Errorf("expected total size %d, got %d", want, result.TotalSize)
	}
}

// --- Zoom Cache tests ---

func TestScanZoomCacheMissing(t *testing.T) {
	home := t.TempDir()
	result := scanZoomCache(home)
	if result != nil {
		t.Fatal("expected nil for missing Zoom Cache")
	}
}

func TestScanZoomCacheWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Application Support", "zoom.us", "data")
	writeFile(t, filepath.Join(dir, "meeting_cache", "data.bin"), 2500)

	result := scanZoomCache(home)
	if result == nil {
		t.Fatal("expected non-nil result for Zoom Cache with data")
	}
	if result.Category != "msg-zoom" {
		t.Errorf("expected category 'msg-zoom', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if want := scantest.OnDisk(t, 2500); result.TotalSize != want {
		t.Errorf("expected total size %d, got %d", want, result.TotalSize)
	}
}

func TestScanZoomCacheEmptyDir(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Application Support", "zoom.us", "data")
	os.MkdirAll(dir, 0755)

	result := scanZoomCache(home)
	if result != nil {
		t.Fatal("expected nil for empty Zoom cache directory")
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create Slack cache.
	slackDir := filepath.Join(home, "Library", "Application Support", "Slack", "Cache")
	writeFile(t, filepath.Join(slackDir, "data_0"), 1000)

	// Create Zoom cache.
	zoomDir := filepath.Join(home, "Library", "Application Support", "zoom.us", "data")
	writeFile(t, filepath.Join(zoomDir, "data.bin"), 500)

	// No Discord, no Teams -- should be silently skipped.

	var results []scan.CategoryResult
	if cr := scanSlackCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanDiscordCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanTeamsCache(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanZoomCache(home); cr != nil {
		results = append(results, *cr)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Slack + Zoom), got %d", len(results))
	}
	if results[0].Category != "msg-slack" {
		t.Errorf("expected first result 'msg-slack', got %q", results[0].Category)
	}
	if results[1].Category != "msg-zoom" {
		t.Errorf("expected second result 'msg-zoom', got %q", results[1].Category)
	}
}
