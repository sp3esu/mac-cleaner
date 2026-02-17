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

	// Create associated Library data.
	writeFile(t, filepath.Join(home, "Library", "Caches", "com.example.oldapp", "cache.db"), 2000)

	oldDate := time.Now().Add(-365 * 24 * time.Hour).Format(mdlsDateLayout)

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

	// Multiple Library directories.
	writeFile(t, filepath.Join(home, "Library", "Caches", "com.example.bigdata", "data.db"), 5000)
	writeFile(t, filepath.Join(home, "Library", "Application Support", "com.example.bigdata", "store.db"), 3000)
	writeFile(t, filepath.Join(home, "Library", "Containers", "com.example.bigdata", "Data", "db.sqlite"), 2000)

	oldDate := time.Now().Add(-200 * 24 * time.Hour).Format(mdlsDateLayout)

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
