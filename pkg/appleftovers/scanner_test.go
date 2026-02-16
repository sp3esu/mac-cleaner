package appleftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gregor/mac-cleaner/internal/scan"
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

// --- Orphaned Preferences tests ---

func TestScanOrphanedPrefs(t *testing.T) {
	home := t.TempDir()

	// Create Preferences directory with plist files.
	prefsDir := filepath.Join(home, "Library", "Preferences")
	writeFile(t, filepath.Join(prefsDir, "com.example.removed.plist"), 500)
	writeFile(t, filepath.Join(prefsDir, "com.apple.finder.plist"), 300)
	writeFile(t, filepath.Join(prefsDir, "com.known.app.plist"), 200)
	writeFile(t, filepath.Join(prefsDir, "com.known.app.helper.plist"), 100)

	// Create a fake app directory with one .app that returns "com.known.app".
	appDir := filepath.Join(home, "Applications")
	writeFile(t, filepath.Join(appDir, "KnownApp.app", "Contents", "Info.plist"), 10)

	// Mock runner: returns "com.known.app" for any PlistBuddy call.
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("com.known.app\n"), nil
	}

	// Create a fake PlistBuddy so LookPath succeeds.
	fakeBin := t.TempDir()
	fakePB := filepath.Join(fakeBin, "PlistBuddy")
	if err := os.WriteFile(fakePB, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	result := scanOrphanedPrefs(home, fakePB, runner)
	if result == nil {
		t.Fatal("expected non-nil result for orphaned prefs")
	}

	if result.Category != "app-orphaned-prefs" {
		t.Errorf("expected category 'app-orphaned-prefs', got %q", result.Category)
	}

	// com.apple.finder should be skipped (apple prefix).
	// com.known.app should be skipped (matches installed app).
	// com.known.app.helper should be skipped (prefix match with com.known.app).
	// Only com.example.removed should be flagged.
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 orphaned entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Description != "com.example.removed" {
		t.Errorf("expected orphaned entry 'com.example.removed', got %q", result.Entries[0].Description)
	}

	if result.Entries[0].Size != 500 {
		t.Errorf("expected size 500, got %d", result.Entries[0].Size)
	}
}

func TestScanOrphanedPrefsNoPlistBuddy(t *testing.T) {
	home := t.TempDir()
	prefsDir := filepath.Join(home, "Library", "Preferences")
	writeFile(t, filepath.Join(prefsDir, "com.example.removed.plist"), 500)

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		t.Fatal("runner should not be called when PlistBuddy is not found")
		return nil, nil
	}

	// Pass a path that does not exist.
	result := scanOrphanedPrefs(home, "/nonexistent/PlistBuddy", runner)
	if result != nil {
		t.Fatal("expected nil when PlistBuddy is not found")
	}
}

func TestScanOrphanedPrefsApplePrefixSkipped(t *testing.T) {
	home := t.TempDir()

	// Create Preferences directory with various com.apple.* plist files.
	prefsDir := filepath.Join(home, "Library", "Preferences")
	appleDomains := []string{
		"com.apple.finder",
		"com.apple.Safari",
		"com.apple.dock",
		"com.apple.systempreferences",
		"com.apple.Terminal",
		"com.apple.dt.Xcode",
	}

	for _, domain := range appleDomains {
		writeFile(t, filepath.Join(prefsDir, domain+".plist"), 100)
	}

	// Create a fake PlistBuddy so LookPath succeeds.
	fakeBin := t.TempDir()
	fakePB := filepath.Join(fakeBin, "PlistBuddy")
	if err := os.WriteFile(fakePB, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// No apps installed -- but all prefs should still be skipped.
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("no bundle ID")
	}

	result := scanOrphanedPrefs(home, fakePB, runner)
	if result != nil {
		t.Fatal("expected nil -- all com.apple.* prefs should be skipped")
	}
}

func TestScanOrphanedPrefsNoPrefsDir(t *testing.T) {
	home := t.TempDir()

	fakeBin := t.TempDir()
	fakePB := filepath.Join(fakeBin, "PlistBuddy")
	if err := os.WriteFile(fakePB, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, nil
	}

	result := scanOrphanedPrefs(home, fakePB, runner)
	if result == nil {
		// No Preferences dir, should return nil.
	} else {
		t.Fatal("expected nil when Preferences directory is missing")
	}
}

// --- iOS Backups tests ---

func TestScanIOSBackups(t *testing.T) {
	home := t.TempDir()
	backupDir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")

	// Create two UUID-named backup directories with files inside.
	writeFile(t, filepath.Join(backupDir, "AAAA-BBBB-CCCC-DDDD", "Manifest.db"), 3000)
	writeFile(t, filepath.Join(backupDir, "AAAA-BBBB-CCCC-DDDD", "files", "data.bin"), 2000)
	writeFile(t, filepath.Join(backupDir, "EEEE-FFFF-1111-2222", "Manifest.db"), 1000)

	result := scanIOSBackups(home)
	if result == nil {
		t.Fatal("expected non-nil result for iOS backups")
	}

	if result.Category != "app-ios-backups" {
		t.Errorf("expected category 'app-ios-backups', got %q", result.Category)
	}
	if result.Description != "iOS Device Backups" {
		t.Errorf("expected description 'iOS Device Backups', got %q", result.Description)
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	// Entries should be sorted by size descending.
	if result.Entries[0].Size != 5000 {
		t.Errorf("expected first entry size 5000, got %d", result.Entries[0].Size)
	}
	if result.Entries[1].Size != 1000 {
		t.Errorf("expected second entry size 1000, got %d", result.Entries[1].Size)
	}

	expectedTotal := int64(6000)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

func TestScanIOSBackupsMissing(t *testing.T) {
	home := t.TempDir()
	result := scanIOSBackups(home)
	if result != nil {
		t.Fatal("expected nil for missing iOS backup directory")
	}
}

func TestScanIOSBackupsEmptyDir(t *testing.T) {
	home := t.TempDir()
	backupDir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	os.MkdirAll(backupDir, 0755)

	result := scanIOSBackups(home)
	if result != nil {
		t.Fatal("expected nil for empty iOS backup directory")
	}
}

// --- Old Downloads tests ---

func TestScanOldDownloads(t *testing.T) {
	home := t.TempDir()
	downloadsDir := filepath.Join(home, "Downloads")

	// Create files with various ages.
	writeFile(t, filepath.Join(downloadsDir, "old-large.dmg"), 5000)
	writeFile(t, filepath.Join(downloadsDir, "old-small.zip"), 1000)
	writeFile(t, filepath.Join(downloadsDir, "recent.pdf"), 2000)

	// Make "old" files actually old (120 days ago).
	oldTime := time.Now().Add(-120 * 24 * time.Hour)
	os.Chtimes(filepath.Join(downloadsDir, "old-large.dmg"), oldTime, oldTime)
	os.Chtimes(filepath.Join(downloadsDir, "old-small.zip"), oldTime, oldTime)
	// recent.pdf keeps its current time (just created).

	maxAge := 90 * 24 * time.Hour
	result := scanOldDownloads(home, maxAge)
	if result == nil {
		t.Fatal("expected non-nil result for old downloads")
	}

	if result.Category != "app-old-downloads" {
		t.Errorf("expected category 'app-old-downloads', got %q", result.Category)
	}
	if result.Description != "Old Downloads (90+ days)" {
		t.Errorf("expected description 'Old Downloads (90+ days)', got %q", result.Description)
	}

	// Only old-large.dmg and old-small.zip should be returned.
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	// Sorted by size descending.
	if result.Entries[0].Description != "old-large.dmg" {
		t.Errorf("expected first entry 'old-large.dmg', got %q", result.Entries[0].Description)
	}
	if result.Entries[0].Size != 5000 {
		t.Errorf("expected first entry size 5000, got %d", result.Entries[0].Size)
	}
	if result.Entries[1].Description != "old-small.zip" {
		t.Errorf("expected second entry 'old-small.zip', got %q", result.Entries[1].Description)
	}

	expectedTotal := int64(6000)
	if result.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, result.TotalSize)
	}
}

func TestScanOldDownloadsSkipsRecent(t *testing.T) {
	home := t.TempDir()
	downloadsDir := filepath.Join(home, "Downloads")

	// All files are recent (just created).
	writeFile(t, filepath.Join(downloadsDir, "recent1.pdf"), 1000)
	writeFile(t, filepath.Join(downloadsDir, "recent2.zip"), 2000)

	maxAge := 90 * 24 * time.Hour
	result := scanOldDownloads(home, maxAge)
	if result != nil {
		t.Fatal("expected nil when all downloads are recent")
	}
}

func TestScanOldDownloadsMissing(t *testing.T) {
	home := t.TempDir()
	result := scanOldDownloads(home, 90*24*time.Hour)
	if result != nil {
		t.Fatal("expected nil for missing Downloads directory")
	}
}

func TestScanOldDownloadsWithDirectories(t *testing.T) {
	home := t.TempDir()
	downloadsDir := filepath.Join(home, "Downloads")

	// Create an old directory with files inside.
	writeFile(t, filepath.Join(downloadsDir, "old-project", "file1.txt"), 1500)
	writeFile(t, filepath.Join(downloadsDir, "old-project", "file2.txt"), 500)

	// Make directory and contents old.
	oldTime := time.Now().Add(-120 * 24 * time.Hour)
	os.Chtimes(filepath.Join(downloadsDir, "old-project"), oldTime, oldTime)
	os.Chtimes(filepath.Join(downloadsDir, "old-project", "file1.txt"), oldTime, oldTime)
	os.Chtimes(filepath.Join(downloadsDir, "old-project", "file2.txt"), oldTime, oldTime)

	maxAge := 90 * 24 * time.Hour
	result := scanOldDownloads(home, maxAge)
	if result == nil {
		t.Fatal("expected non-nil result for old directory in Downloads")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (directory), got %d", len(result.Entries))
	}

	if result.Entries[0].Description != "old-project" {
		t.Errorf("expected entry 'old-project', got %q", result.Entries[0].Description)
	}

	// Directory size should be sum of its files.
	expectedSize := int64(2000)
	if result.Entries[0].Size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, result.Entries[0].Size)
	}
}

func TestScanOldDownloadsSkipsZeroByte(t *testing.T) {
	home := t.TempDir()
	downloadsDir := filepath.Join(home, "Downloads")

	// Create a zero-byte old file.
	writeFile(t, filepath.Join(downloadsDir, "empty.txt"), 0)
	oldTime := time.Now().Add(-120 * 24 * time.Hour)
	os.Chtimes(filepath.Join(downloadsDir, "empty.txt"), oldTime, oldTime)

	maxAge := 90 * 24 * time.Hour
	result := scanOldDownloads(home, maxAge)
	if result != nil {
		t.Fatal("expected nil -- zero-byte entries should be excluded")
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create iOS backups.
	backupDir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	writeFile(t, filepath.Join(backupDir, "device-1", "Manifest.db"), 1000)

	// Create old downloads.
	downloadsDir := filepath.Join(home, "Downloads")
	writeFile(t, filepath.Join(downloadsDir, "old.dmg"), 2000)
	oldTime := time.Now().Add(-120 * 24 * time.Hour)
	os.Chtimes(filepath.Join(downloadsDir, "old.dmg"), oldTime, oldTime)

	// Call private helpers directly (Scan() uses os.UserHomeDir()).
	var results []scan.CategoryResult

	// Skip orphaned prefs (requires PlistBuddy mock setup).
	if cr := scanIOSBackups(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanOldDownloads(home, 90*24*time.Hour); cr != nil {
		results = append(results, *cr)
	}

	// Expect iOS backups + old downloads.
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Category != "app-ios-backups" {
		t.Errorf("expected first result 'app-ios-backups', got %q", results[0].Category)
	}
	if results[1].Category != "app-old-downloads" {
		t.Errorf("expected second result 'app-old-downloads', got %q", results[1].Category)
	}
}
