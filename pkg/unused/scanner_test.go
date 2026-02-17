package unused

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

// mockRunner returns a CmdRunner that maps commands to predefined responses.
// The key is "command arg0 arg1 ..." joined by space.
type mockResponse struct {
	output []byte
	err    error
}

func newMockRunner(responses map[string]mockResponse) CmdRunner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Try exact match first.
		key := name
		for _, a := range args {
			key += " " + a
		}
		if resp, ok := responses[key]; ok {
			return resp.output, resp.err
		}
		// Fallback: match by command name only.
		if resp, ok := responses[name]; ok {
			return resp.output, resp.err
		}
		return nil, fmt.Errorf("mock: no response for %q", key)
	}
}

func TestScanUnusedApps_UnusedDetected(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create a .app bundle with some content.
	writeFile(t, filepath.Join(appDir, "OldApp.app", "Contents", "Info.plist"), 100)
	writeFile(t, filepath.Join(appDir, "OldApp.app", "Contents", "MacOS", "OldApp"), 5000)

	// Create associated Library data (backdated so the Library mod check doesn't skip).
	cacheDir := filepath.Join(home, "Library", "Caches", "com.example.oldapp")
	writeFile(t, filepath.Join(cacheDir, "cache.db"), 2000)
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	os.Chtimes(cacheDir, oldTime, oldTime)

	oldDate := oldTime.Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "OldApp.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "OldApp.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.oldapp\n")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result for unused app")
	}

	if result.Category != "unused-apps" {
		t.Errorf("expected category 'unused-apps', got %q", result.Category)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != filepath.Join(appDir, "OldApp.app") {
		t.Errorf("expected path %q, got %q", filepath.Join(appDir, "OldApp.app"), entry.Path)
	}

	// Size should include bundle (5100) + Library caches (2000) = 7100.
	expectedSize := int64(7100)
	if entry.Size != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, entry.Size)
	}

	if result.TotalSize != expectedSize {
		t.Errorf("expected total size %d, got %d", expectedSize, result.TotalSize)
	}
}

func TestScanUnusedApps_RecentAppSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	writeFile(t, filepath.Join(appDir, "RecentApp.app", "Contents", "MacOS", "RecentApp"), 5000)

	// App was used yesterday.
	recentDate := time.Now().Add(-24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "RecentApp.app")
	responses[mdlsKey] = mockResponse{output: []byte(recentDate)}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil result when all apps are recent")
	}
}

func TestScanUnusedApps_NeverOpened(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	writeFile(t, filepath.Join(appDir, "NeverUsed.app", "Contents", "MacOS", "NeverUsed"), 3000)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "NeverUsed.app")
	responses[mdlsKey] = mockResponse{output: []byte("(null)")}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "NeverUsed.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.neverused\n")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result for never-opened app")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Description != "NeverUsed (no usage history)" {
		t.Errorf("expected description containing 'no usage history', got %q", entry.Description)
	}
}

func TestScanUnusedApps_LibraryFootprintIncluded(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Minimal app bundle.
	writeFile(t, filepath.Join(appDir, "BigData.app", "Contents", "MacOS", "BigData"), 1000)

	// Multiple Library directories (backdated so the Library mod check doesn't skip).
	libDirs := []string{
		filepath.Join(home, "Library", "Caches", "com.example.bigdata"),
		filepath.Join(home, "Library", "Application Support", "com.example.bigdata"),
		filepath.Join(home, "Library", "Containers", "com.example.bigdata"),
	}
	writeFile(t, filepath.Join(libDirs[0], "data.db"), 5000)
	writeFile(t, filepath.Join(libDirs[1], "store.db"), 3000)
	writeFile(t, filepath.Join(libDirs[2], "Data", "db.sqlite"), 2000)
	oldTime := time.Now().Add(-200 * 24 * time.Hour)
	for _, d := range libDirs {
		os.Chtimes(d, oldTime, oldTime)
	}

	oldDate := oldTime.Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "BigData.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "BigData.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.bigdata\n")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Bundle (1000) + Caches (5000) + AppSupport (3000) + Containers (2000) = 11000
	expectedSize := int64(11000)
	if result.Entries[0].Size != expectedSize {
		t.Errorf("expected size %d (bundle + Library), got %d", expectedSize, result.Entries[0].Size)
	}
}

func TestScanUnusedApps_MdlsErrorSkipsApp(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	writeFile(t, filepath.Join(appDir, "Broken.app", "Contents", "MacOS", "Broken"), 1000)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "Broken.app")
	responses[mdlsKey] = mockResponse{err: fmt.Errorf("mdls not available")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil when mdls fails for all apps")
	}
}

func TestScanUnusedApps_PlistBuddyErrorStillScans(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	writeFile(t, filepath.Join(appDir, "NoPlist.app", "Contents", "MacOS", "NoPlist"), 2000)

	oldDate := time.Now().Add(-200 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "NoPlist.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	// PlistBuddy fails -- bundleID will be empty, but app should still appear.
	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "NoPlist.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{err: fmt.Errorf("plist not found")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result even when PlistBuddy fails")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	// Size should be bundle only (no Library data without bundleID).
	if result.Entries[0].Size != 2000 {
		t.Errorf("expected size 2000, got %d", result.Entries[0].Size)
	}
}

func TestScanUnusedApps_EmptyAppDirReturnsNil(t *testing.T) {
	home := t.TempDir()

	// Create empty ~/Applications dir.
	if err := os.MkdirAll(filepath.Join(home, "Applications"), 0755); err != nil {
		t.Fatal(err)
	}

	runner := newMockRunner(map[string]mockResponse{})

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil for empty app directory")
	}
}

func TestScanUnusedApps_MissingAppDirReturnsNil(t *testing.T) {
	home := t.TempDir()
	// No ~/Applications dir at all.

	runner := newMockRunner(map[string]mockResponse{})

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil when app directory doesn't exist")
	}
}

func TestScanUnusedApps_SortedBySizeDescending(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create two unused apps of different sizes.
	writeFile(t, filepath.Join(appDir, "SmallApp.app", "Contents", "MacOS", "SmallApp"), 1000)
	writeFile(t, filepath.Join(appDir, "BigApp.app", "Contents", "MacOS", "BigApp"), 5000)

	oldDate := time.Now().Add(-200 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	for _, name := range []string{"SmallApp", "BigApp"} {
		mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, name+".app")
		responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

		plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
			filepath.Join(appDir, name+".app", "Contents", "Info.plist")
		responses[plistKey] = mockResponse{err: fmt.Errorf("no plist")}
	}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}

	if result.Entries[0].Size < result.Entries[1].Size {
		t.Errorf("entries not sorted by size descending: %d < %d",
			result.Entries[0].Size, result.Entries[1].Size)
	}
}

func TestScanUnusedApps_PermissionErrorCollected(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create app dir, then make it unreadable to trigger ReadDir permission error.
	writeFile(t, filepath.Join(appDir, "SomeApp.app", "Contents", "MacOS", "SomeApp"), 1000)

	os.Chmod(appDir, 0000)
	t.Cleanup(func() {
		os.Chmod(appDir, 0755)
	})

	runner := newMockRunner(map[string]mockResponse{})

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result with permission issues")
	}

	if len(result.PermissionIssues) != 1 {
		t.Errorf("expected 1 permission issue, got %d", len(result.PermissionIssues))
	}

	if result.PermissionIssues[0].Path != appDir {
		t.Errorf("expected permission issue path %q, got %q", appDir, result.PermissionIssues[0].Path)
	}
}

func TestScanUnusedApps_DateParsingEdgeCases(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	writeFile(t, filepath.Join(appDir, "Weird.app", "Contents", "MacOS", "Weird"), 1000)

	tests := []struct {
		name     string
		mdlsOut  string
		wantNil  bool
		wantDesc string
	}{
		{
			name:    "empty string",
			mdlsOut: "",
			wantNil: false, // treated as never opened
		},
		{
			name:    "invalid date format",
			mdlsOut: "not-a-date",
			wantNil: true, // parse error → skip
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mockResponse{}
			mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "Weird.app")
			responses[mdlsKey] = mockResponse{output: []byte(tt.mdlsOut)}

			plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
				filepath.Join(appDir, "Weird.app", "Contents", "Info.plist")
			responses[plistKey] = mockResponse{err: fmt.Errorf("no plist")}

			runner := newMockRunner(responses)
			result := scanUnusedApps(home, defaultThreshold, runner)

			if tt.wantNil {
				if result != nil {
					t.Fatal("expected nil result for invalid date")
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
			}
		})
	}
}

func TestScanUnusedApps_NonAppEntriesSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create non-.app entries.
	writeFile(t, filepath.Join(appDir, "readme.txt"), 100)
	writeFile(t, filepath.Join(appDir, "SomeDir", "file.txt"), 100)

	runner := newMockRunner(map[string]mockResponse{})

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil when no .app bundles exist")
	}
}

func TestFormatDescription(t *testing.T) {
	t.Run("no usage history", func(t *testing.T) {
		desc := formatDescription("SomeApp", nil)
		if desc != "SomeApp (no usage history)" {
			t.Errorf("unexpected description: %q", desc)
		}
	})

	t.Run("with date", func(t *testing.T) {
		date := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		desc := formatDescription("SomeApp", &date)
		if desc != "SomeApp (last used Jan 2024)" {
			t.Errorf("unexpected description: %q", desc)
		}
	})
}

func TestQueryLastUsedDate(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		expected := time.Date(2024, 5, 14, 9, 23, 41, 0, time.UTC)
		runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("2024-05-14 09:23:41 +0000"), nil
		}

		result, err := queryLastUsedDate("/some/app.app", runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if !result.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, *result)
		}
	})

	t.Run("null response", func(t *testing.T) {
		runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("(null)"), nil
		}

		result, err := queryLastUsedDate("/some/app.app", runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for never-opened app")
		}
	})

	t.Run("command error", func(t *testing.T) {
		runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("command failed")
		}

		_, err := queryLastUsedDate("/some/app.app", runner)
		if err == nil {
			t.Fatal("expected error when command fails")
		}
	})
}

func TestLibraryFootprint(t *testing.T) {
	home := t.TempDir()

	// Create various Library directories.
	writeFile(t, filepath.Join(home, "Library", "Caches", "com.test.app", "data"), 1000)
	writeFile(t, filepath.Join(home, "Library", "Application Support", "com.test.app", "db"), 2000)
	writeFile(t, filepath.Join(home, "Library", "Preferences", "com.test.app.plist"), 500)

	size := libraryFootprint(home, "com.test.app", "TestApp")

	// Should include Caches (1000) + AppSupport (2000) + Preferences plist (500) = 3500
	if size != 3500 {
		t.Errorf("expected library footprint 3500, got %d", size)
	}
}

func TestLibraryFootprint_EmptyBundleID(t *testing.T) {
	home := t.TempDir()

	// Create only appName-based paths.
	writeFile(t, filepath.Join(home, "Library", "Application Support", "MyApp", "data"), 1500)
	writeFile(t, filepath.Join(home, "Library", "Logs", "MyApp", "log.txt"), 500)

	size := libraryFootprint(home, "", "MyApp")

	if size != 2000 {
		t.Errorf("expected library footprint 2000, got %d", size)
	}
}

func TestLibraryFootprint_NoPaths(t *testing.T) {
	home := t.TempDir()

	size := libraryFootprint(home, "com.nonexistent.app", "NonExistent")

	if size != 0 {
		t.Errorf("expected 0 for nonexistent paths, got %d", size)
	}
}

func TestLibraryLastModified(t *testing.T) {
	t.Run("returns latest mod time across Library dirs", func(t *testing.T) {
		home := t.TempDir()

		// Create two Library dirs; one older, one newer.
		oldDir := filepath.Join(home, "Library", "Caches", "com.test.app")
		newDir := filepath.Join(home, "Library", "Application Support", "com.test.app")

		writeFile(t, filepath.Join(oldDir, "data"), 100)
		writeFile(t, filepath.Join(newDir, "db"), 100)

		oldTime := time.Now().Add(-365 * 24 * time.Hour)
		os.Chtimes(oldDir, oldTime, oldTime)

		result := libraryLastModified(home, "com.test.app", "TestApp")
		if result.IsZero() {
			t.Fatal("expected non-zero time")
		}

		// The newer directory was just created, so its mod time should be very recent.
		if time.Since(result) > time.Minute {
			t.Errorf("expected recent mod time, got %v ago", time.Since(result))
		}
	})

	t.Run("returns zero for nonexistent paths", func(t *testing.T) {
		home := t.TempDir()

		result := libraryLastModified(home, "com.nonexistent.app", "NonExistent")
		if !result.IsZero() {
			t.Errorf("expected zero time, got %v", result)
		}
	})

	t.Run("checks appName paths when bundleID differs", func(t *testing.T) {
		home := t.TempDir()

		writeFile(t, filepath.Join(home, "Library", "Application Support", "MyApp", "data"), 100)

		result := libraryLastModified(home, "com.other.id", "MyApp")
		if result.IsZero() {
			t.Fatal("expected non-zero time from appName path")
		}
	})

	t.Run("skips appName paths when equal to bundleID", func(t *testing.T) {
		home := t.TempDir()

		// Only create an appName-based path (same as bundleID).
		writeFile(t, filepath.Join(home, "Library", "Application Support", "SameName", "data"), 100)

		// bundleID == appName → appName paths skipped, but bundleID path matches.
		result := libraryLastModified(home, "SameName", "SameName")
		if result.IsZero() {
			t.Fatal("expected non-zero time from bundleID path")
		}
	})
}

func TestScanUnusedApps_RecentLibraryDataSkipsApp(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create an app bundle.
	writeFile(t, filepath.Join(appDir, "FalsePositive.app", "Contents", "MacOS", "FalsePositive"), 5000)

	// mdls says it was never used (null).
	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "FalsePositive.app")
	responses[mdlsKey] = mockResponse{output: []byte("(null)")}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "FalsePositive.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.falsepositive\n")}

	// But the app's Library data was recently modified (created just now).
	writeFile(t, filepath.Join(home, "Library", "Caches", "com.example.falsepositive", "recent.db"), 1000)

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil result: app with recent Library data should be skipped")
	}
}

func TestScanUnusedApps_OldLibraryDataStillDetected(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create an app bundle.
	writeFile(t, filepath.Join(appDir, "TrulyOld.app", "Contents", "MacOS", "TrulyOld"), 3000)

	// mdls says it was used a long time ago.
	oldDate := time.Now().Add(-365 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "TrulyOld.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "TrulyOld.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.trulyold\n")}

	// Library data exists but is old.
	cacheDir := filepath.Join(home, "Library", "Caches", "com.example.trulyold")
	writeFile(t, filepath.Join(cacheDir, "old.db"), 2000)
	oldTime := time.Now().Add(-365 * 24 * time.Hour)
	os.Chtimes(cacheDir, oldTime, oldTime)

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result for app with old Library data")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Path != filepath.Join(appDir, "TrulyOld.app") {
		t.Errorf("expected TrulyOld.app, got %q", result.Entries[0].Path)
	}
}
