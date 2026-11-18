package unused

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/scan/scantest"
)

// sumSizes returns the total size of the given components.
func sumSizes(components []scan.SizeComponent) int64 {
	var n int64
	for _, c := range components {
		n += c.Size
	}
	return n
}

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

// backdate sets the access and modification times of root and everything
// under it to when. Changing a child's times does not touch its parent's
// mtime, so the walk order does not matter.
func backdate(t *testing.T, root string, when time.Time) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(p, when, when)
	})
	if err != nil {
		t.Fatalf("backdate %s: %v", root, err)
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
		// Default: only launchd is running, unless a test sets "ps".
		if name == "ps" {
			return []byte("/sbin/launchd\n"), nil
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
	backdate(t, cacheDir, oldTime)

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

	// Size should include bundle (100 + 5000) + Library caches (2000), on disk.
	expectedSize := scantest.OnDisk(t, 100, 5000, 2000)
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
		backdate(t, d, oldTime)
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

	// Bundle (1000) + Caches (5000) + AppSupport (3000) + Containers (2000), on disk.
	expectedSize := scantest.OnDisk(t, 1000, 5000, 3000, 2000)
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
	if want := scantest.OnDisk(t, 2000); result.Entries[0].Size != want {
		t.Errorf("expected size %d, got %d", want, result.Entries[0].Size)
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

	size := sumSizes(libraryFootprint(home, "com.test.app", "TestApp"))

	// Should include Caches (1000) + AppSupport (2000) directories on disk,
	// plus the Preferences plist (500), which pathSize sizes by file length.
	if want := scantest.OnDisk(t, 1000, 2000) + 500; size != want {
		t.Errorf("expected library footprint %d, got %d", want, size)
	}
}

func TestLibraryFootprint_EmptyBundleID(t *testing.T) {
	home := t.TempDir()

	// Create only appName-based paths.
	writeFile(t, filepath.Join(home, "Library", "Application Support", "MyApp", "data"), 1500)
	writeFile(t, filepath.Join(home, "Library", "Logs", "MyApp", "log.txt"), 500)

	size := sumSizes(libraryFootprint(home, "", "MyApp"))

	if want := scantest.OnDisk(t, 1500, 500); size != want {
		t.Errorf("expected library footprint %d, got %d", want, size)
	}
}

func TestLibraryFootprint_NoPaths(t *testing.T) {
	home := t.TempDir()

	size := sumSizes(libraryFootprint(home, "com.nonexistent.app", "NonExistent"))

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

		result := libraryLastModified(home, "com.test.app", "TestApp", time.Now().Add(-defaultThreshold))
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

		result := libraryLastModified(home, "com.nonexistent.app", "NonExistent", time.Now().Add(-defaultThreshold))
		if !result.IsZero() {
			t.Errorf("expected zero time, got %v", result)
		}
	})

	t.Run("checks appName paths when bundleID differs", func(t *testing.T) {
		home := t.TempDir()

		writeFile(t, filepath.Join(home, "Library", "Application Support", "MyApp", "data"), 100)

		result := libraryLastModified(home, "com.other.id", "MyApp", time.Now().Add(-defaultThreshold))
		if result.IsZero() {
			t.Fatal("expected non-zero time from appName path")
		}
	})

	t.Run("skips appName paths when equal to bundleID", func(t *testing.T) {
		home := t.TempDir()

		// Only create an appName-based path (same as bundleID).
		writeFile(t, filepath.Join(home, "Library", "Application Support", "SameName", "data"), 100)

		// bundleID == appName → appName paths skipped, but bundleID path matches.
		result := libraryLastModified(home, "SameName", "SameName", time.Now().Add(-defaultThreshold))
		if result.IsZero() {
			t.Fatal("expected non-zero time from bundleID path")
		}
	})
}

func TestIsAppleApp(t *testing.T) {
	tests := []struct {
		bundleID string
		want     bool
	}{
		{"com.apple.Safari", true},
		{"com.apple.garageband", true},
		{"com.apple.", true},
		{"com.example.app", false},
		{"", false},
		{"com.appleish.app", false},
		{"COM.APPLE.Safari", false},
	}

	for _, tt := range tests {
		t.Run(tt.bundleID, func(t *testing.T) {
			if got := isAppleApp(tt.bundleID); got != tt.want {
				t.Errorf("isAppleApp(%q) = %v, want %v", tt.bundleID, got, tt.want)
			}
		})
	}
}

func TestScanUnusedApps_AppleAppSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create an unused Apple app.
	writeFile(t, filepath.Join(appDir, "GarageBand.app", "Contents", "MacOS", "GarageBand"), 5000)

	oldDate := time.Now().Add(-365 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "GarageBand.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "GarageBand.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.apple.garageband\n")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result != nil {
		t.Fatal("expected nil result: Apple apps should be skipped")
	}
}

func TestScanUnusedApps_AppleAppSkippedMixed(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// Create an unused Apple app and a third-party app.
	writeFile(t, filepath.Join(appDir, "GarageBand.app", "Contents", "MacOS", "GarageBand"), 5000)
	writeFile(t, filepath.Join(appDir, "OldApp.app", "Contents", "MacOS", "OldApp"), 3000)

	oldDate := time.Now().Add(-365 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}

	// GarageBand — Apple app.
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "GarageBand.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}
	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "GarageBand.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.apple.garageband\n")}

	// OldApp — third-party app.
	mdlsKey = "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "OldApp.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}
	plistKey = "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "OldApp.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{output: []byte("com.example.oldapp\n")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result: third-party app should be detected")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Path != filepath.Join(appDir, "OldApp.app") {
		t.Errorf("expected OldApp.app, got %q", result.Entries[0].Path)
	}
}

func TestScanUnusedApps_UnknownBundleIDNotSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")

	// App where PlistBuddy fails — empty bundleID should not be skipped.
	writeFile(t, filepath.Join(appDir, "Mystery.app", "Contents", "MacOS", "Mystery"), 2000)

	oldDate := time.Now().Add(-200 * 24 * time.Hour).Format(mdlsDateLayout)

	responses := map[string]mockResponse{}
	mdlsKey := "mdls -name kMDItemLastUsedDate -raw " + filepath.Join(appDir, "Mystery.app")
	responses[mdlsKey] = mockResponse{output: []byte(oldDate)}

	plistKey := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
		filepath.Join(appDir, "Mystery.app", "Contents", "Info.plist")
	responses[plistKey] = mockResponse{err: fmt.Errorf("plist not found")}

	runner := newMockRunner(responses)

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil {
		t.Fatal("expected non-nil result: app with unknown bundleID should not be skipped")
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Path != filepath.Join(appDir, "Mystery.app") {
		t.Errorf("expected Mystery.app, got %q", result.Entries[0].Path)
	}
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
	backdate(t, cacheDir, oldTime)

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

func TestScanUnusedApps_ComponentsExplainSize(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	appPath := filepath.Join(appDir, "BigData.app")

	writeFile(t, filepath.Join(appPath, "Contents", "MacOS", "BigData"), 1000)
	containers := filepath.Join(home, "Library", "Containers", "com.example.bigdata")
	caches := filepath.Join(home, "Library", "Caches", "com.example.bigdata")
	writeFile(t, filepath.Join(containers, "Data", "disk.raw"), 9000)
	writeFile(t, filepath.Join(caches, "data.db"), 500)
	oldTime := time.Now().Add(-200 * 24 * time.Hour)
	backdate(t, containers, oldTime)
	backdate(t, caches, oldTime)

	runner := newMockRunner(map[string]mockResponse{
		"mdls -name kMDItemLastUsedDate -raw " + appPath: {output: []byte(oldTime.Format(mdlsDateLayout))},
		"/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " +
			filepath.Join(appPath, "Contents", "Info.plist"): {output: []byte("com.example.bigdata\n")},
	})

	result := scanUnusedApps(home, defaultThreshold, runner)
	if result == nil || len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %+v", result)
	}
	e := result.Entries[0]
	if got := sumSizes(e.Components); got != e.Size {
		t.Errorf("components sum %d != entry size %d", got, e.Size)
	}
	if want := scantest.OnDisk(t, 1000, 9000, 500); e.Size != want {
		t.Errorf("expected size %d, got %d", want, e.Size)
	}

	byPath := map[string]scan.SizeComponent{}
	for _, c := range e.Components {
		byPath[c.Path] = c
	}
	if len(byPath) != 3 {
		t.Fatalf("expected 3 components (bundle, Containers, Caches), got %+v", e.Components)
	}
	bundleSize := scantest.OnDisk(t, 1000)
	if b := byPath[appPath]; !b.Removed || b.Size != bundleSize {
		t.Errorf("bundle component = %+v, want Removed=true Size=%d", b, bundleSize)
	}
	for _, p := range []string{containers, caches} {
		c, ok := byPath[p]
		if !ok {
			t.Fatalf("missing component for %s", p)
		}
		if c.Removed {
			t.Errorf("library component %s must not be marked removed", p)
		}
		if c.Note != libraryNote {
			t.Errorf("library component note = %q, want %q", c.Note, libraryNote)
		}
	}
	if got := e.RemovedSize(); got != bundleSize {
		t.Errorf("RemovedSize = %d, want %d (bundle only)", got, bundleSize)
	}
}

func TestBundleComponent(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	t.Run("under home is removed", func(t *testing.T) {
		p := filepath.Join(home, "Applications", "Foo.app")
		c := bundleComponent(p, 42)
		if !c.Removed || c.Size != 42 || c.Path != p || c.Note != "app bundle" {
			t.Errorf("got %+v", c)
		}
	})

	t.Run("system Applications is blocked", func(t *testing.T) {
		c := bundleComponent("/Applications/Foo.app", 42)
		if c.Removed {
			t.Error("bundle in /Applications must not be marked removed")
		}
		if !strings.Contains(c.Note, "blocked by safety check") {
			t.Errorf("note should explain the block, got %q", c.Note)
		}
	})
}

func TestLibraryFootprint_ComponentsNotRemoved(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "Library", "Caches", "com.test.app", "data"), 1000)
	writeFile(t, filepath.Join(home, "Library", "Group Containers", "XYZ.com.test.app", "db"), 300)
	writeFile(t, filepath.Join(home, "Library", "LaunchAgents", "com.test.app.helper.plist"), 20)
	// Empty directory: must not produce a component.
	if err := os.MkdirAll(filepath.Join(home, "Library", "Logs", "com.test.app"), 0755); err != nil {
		t.Fatal(err)
	}

	comps := libraryFootprint(home, "com.test.app", "TestApp")
	if len(comps) != 3 {
		t.Fatalf("expected 3 non-empty components, got %+v", comps)
	}
	// Directories are sized on disk; the LaunchAgents plist (20) by file length.
	if want := scantest.OnDisk(t, 1000, 300) + 20; sumSizes(comps) != want {
		t.Errorf("expected total %d, got %d", want, sumSizes(comps))
	}
	for _, c := range comps {
		if c.Removed || c.Size == 0 {
			t.Errorf("unexpected component %+v", c)
		}
	}
}

// addApp creates a minimal .app bundle under appDir and registers the mdls
// (kMDItemLastUsedDate) and PlistBuddy responses for it. An empty bundleID
// makes PlistBuddy fail. It returns the bundle path.
func addApp(t *testing.T, responses map[string]mockResponse, appDir, name, bundleID, lastUsed string) string {
	t.Helper()
	appPath := filepath.Join(appDir, name+".app")
	writeFile(t, filepath.Join(appPath, "Contents", "MacOS", name), 1000)
	responses["mdls -name kMDItemLastUsedDate -raw "+appPath] = mockResponse{output: []byte(lastUsed)}
	key := "/usr/libexec/PlistBuddy -c Print :CFBundleIdentifier " + filepath.Join(appPath, "Contents", "Info.plist")
	if bundleID == "" {
		responses[key] = mockResponse{err: fmt.Errorf("no plist")}
	} else {
		responses[key] = mockResponse{output: []byte(bundleID + "\n")}
	}
	return appPath
}

// longAgo is well before the default 180-day cutoff.
var longAgo = time.Now().Add(-400 * 24 * time.Hour)

// entryPaths returns the entry paths of a scan result (nil-safe).
func entryPaths(cr *scan.CategoryResult) []string {
	if cr == nil {
		return nil
	}
	var paths []string
	for _, e := range cr.Entries {
		paths = append(paths, e.Path)
	}
	return paths
}

func TestScanUnusedApps_RunningProcessSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{}
	running := addApp(t, responses, appDir, "Magnet", "com.example.magnet", longAgo.Format(mdlsDateLayout))
	// "Mag.app" shares a name prefix with "Magnet.app" but is not running.
	idle := addApp(t, responses, appDir, "Mag", "com.example.mag", longAgo.Format(mdlsDateLayout))

	psCalls := 0
	mock := newMockRunner(responses)
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "ps" {
			psCalls++
			if strings.Join(args, " ") != "-axo comm=" {
				t.Errorf("unexpected ps args %q", args)
			}
			return []byte("/sbin/launchd\n" + running + "/Contents/MacOS/Magnet\n/usr/bin/zsh\n"), nil
		}
		return mock(ctx, name, args...)
	}

	result := scanUnusedApps(home, defaultThreshold, runner)
	got := entryPaths(result)
	if len(got) != 1 || got[0] != idle {
		t.Fatalf("expected only %s flagged, got %v", idle, got)
	}
	if psCalls != 1 {
		t.Errorf("ps should run once per scan, ran %d times", psCalls)
	}
}

func TestScanUnusedApps_PsFailureFlagsNothing(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{"ps": {err: fmt.Errorf("ps: operation not permitted")}}
	addApp(t, responses, appDir, "OldApp", "com.example.oldapp", longAgo.Format(mdlsDateLayout))
	addApp(t, responses, appDir, "Other", "", longAgo.Format(mdlsDateLayout))

	result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses))
	if result == nil {
		t.Fatal("expected a result carrying the ps failure note")
	}
	if len(result.Entries) != 0 || result.TotalSize != 0 {
		t.Errorf("no app may be flagged when ps fails, got %v", entryPaths(result))
	}
	if len(result.PermissionIssues) != 1 || result.PermissionIssues[0].Path != "ps" {
		t.Fatalf("expected one ps note, got %+v", result.PermissionIssues)
	}
	if !strings.Contains(result.PermissionIssues[0].Description, "not permitted") {
		t.Errorf("note should carry the ps error, got %q", result.PermissionIssues[0].Description)
	}
}

func TestScanUnusedApps_UserLaunchAgentIntoBundleSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{}
	appPath := addApp(t, responses, appDir, "Updater", "com.example.updater", longAgo.Format(mdlsDateLayout))

	plist := filepath.Join(home, "Library", "LaunchAgents", "com.vendor.agent.plist")
	writeFile(t, plist, 10)
	responses["plutil -convert json -o - "+plist] = mockResponse{
		output: []byte(`{"Label":"com.vendor.agent","ProgramArguments":["` + appPath + `/Contents/MacOS/agent","--bg"]}`),
	}

	if result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses)); result != nil {
		t.Fatalf("app with a LaunchAgent into its bundle must be skipped, got %v", entryPaths(result))
	}
}

func TestScanUnusedApps_RecentPreferencesSkipped(t *testing.T) {
	const id = "com.example.prefs"
	tests := []struct {
		name string
		rel  []string // path under ~/Library written now
	}{
		{"preferences plist", []string{"Preferences", id + ".plist"}},
		{"ByHost plist", []string{"Preferences", "ByHost", id + ".0F3A-UUID.plist"}},
		{"group container", []string{"Group Containers", "TEAM123." + id, "Library", "db.sqlite"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			appDir := filepath.Join(home, "Applications")
			responses := map[string]mockResponse{}
			addApp(t, responses, appDir, "Prefs", id, longAgo.Format(mdlsDateLayout))

			p := filepath.Join(append([]string{home, "Library"}, tt.rel...)...)
			writeFile(t, p, 100)
			// Only the leaf is fresh: backdate every directory above it.
			backdate(t, filepath.Join(home, "Library"), longAgo)
			now := time.Now()
			if err := os.Chtimes(p, now, now); err != nil {
				t.Fatal(err)
			}

			if result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses)); result != nil {
				t.Fatalf("app with %s written now must be skipped, got %v", tt.name, entryPaths(result))
			}
		})
	}
}

func TestScanUnusedApps_GroupContainerNameMustMatch(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{}
	appPath := addApp(t, responses, appDir, "Prefs", "com.example.prefs", longAgo.Format(mdlsDateLayout))
	// A different app's group container whose name merely contains the ID.
	writeFile(t, filepath.Join(home, "Library", "Group Containers", "TEAM.com.example.prefs.other", "db"), 100)

	got := entryPaths(scanUnusedApps(home, defaultThreshold, newMockRunner(responses)))
	if len(got) != 1 || got[0] != appPath {
		t.Fatalf("unrelated group container must not count as activity, got %v", got)
	}
}

func TestScanUnusedApps_NestedContainerWriteSkipped(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{}
	addApp(t, responses, appDir, "Docker", "com.example.docker", longAgo.Format(mdlsDateLayout))

	container := filepath.Join(home, "Library", "Containers", "com.example.docker")
	disk := filepath.Join(container, "Data", "vms", "0", "data", "disk.raw")
	writeFile(t, disk, 4096)
	// Mirror Docker.raw: every directory is old, only the disk image is
	// being written. Its depth below Containers/<id> is 5.
	backdate(t, container, longAgo)
	now := time.Now()
	if err := os.Chtimes(disk, now, now); err != nil {
		t.Fatal(err)
	}

	if result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses)); result != nil {
		t.Fatalf("app whose container data is written now must be skipped, got %v", entryPaths(result))
	}
}

func TestScanUnusedApps_DateAddedFallback(t *testing.T) {
	tests := []struct {
		name      string
		dateAdded mockResponse
		wantFlag  bool
	}{
		{"added today", mockResponse{output: []byte(time.Now().Add(-time.Hour).Format(mdlsDateLayout))}, false},
		{"added long ago", mockResponse{output: []byte(longAgo.Format(mdlsDateLayout))}, true},
		{"date added null", mockResponse{output: []byte("(null)")}, true},
		{"mdls fails", mockResponse{err: fmt.Errorf("mdls failed")}, true},
		{"unparseable", mockResponse{output: []byte("garbage")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			appDir := filepath.Join(home, "Applications")
			responses := map[string]mockResponse{}
			appPath := addApp(t, responses, appDir, "Fresh", "com.example.fresh", "(null)")
			responses["mdls -name kMDItemDateAdded -raw "+appPath] = tt.dateAdded

			result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses))
			if !tt.wantFlag {
				if result != nil {
					t.Fatalf("app added within the threshold must be skipped, got %v", entryPaths(result))
				}
				return
			}
			if result == nil || len(result.Entries) != 1 {
				t.Fatalf("expected app flagged, got %v", entryPaths(result))
			}
			if d := result.Entries[0].Description; d != "Fresh (no usage history)" {
				t.Errorf("description = %q, want 'Fresh (no usage history)'", d)
			}
		})
	}
}

func TestScanUnusedApps_ExtensionHostSkipped(t *testing.T) {
	tests := []struct {
		name string
		rel  string // file created inside the bundle
		skip bool
	}{
		{"appex plugin", "Contents/PlugIns/Preview.appex/Contents/Info.plist", true},
		{"QuickLook generator", "Contents/Library/QuickLook/x.qlgenerator/Contents/Info.plist", true},
		{"Spotlight importer", "Contents/Library/Spotlight/x.mdimporter/Contents/Info.plist", true},
		{"system extension", "Contents/Library/SystemExtensions/x.systemextension/Contents/Info.plist", true},
		{"non-appex plugin", "Contents/PlugIns/codec.bundle/Contents/Info.plist", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			appDir := filepath.Join(home, "Applications")
			responses := map[string]mockResponse{}
			appPath := addApp(t, responses, appDir, "Host", "com.example.host", longAgo.Format(mdlsDateLayout))
			writeFile(t, filepath.Join(appPath, filepath.FromSlash(tt.rel)), 10)

			got := entryPaths(scanUnusedApps(home, defaultThreshold, newMockRunner(responses)))
			if tt.skip && len(got) != 0 {
				t.Fatalf("extension host must be skipped, got %v", got)
			}
			if !tt.skip && (len(got) != 1 || got[0] != appPath) {
				t.Fatalf("expected %s flagged, got %v", appPath, got)
			}
		})
	}
}

func TestLibraryFootprint_ResolvesOnDiskCase(t *testing.T) {
	home := t.TempDir()
	onDisk := filepath.Join(home, "Library", "Application Support", "discord")
	writeFile(t, filepath.Join(onDisk, "Cache", "data_0"), 1000)

	comps := libraryFootprint(home, "com.hnc.Discord", "Discord")
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %+v", comps)
	}
	if comps[0].Path != onDisk {
		t.Errorf("component path = %q, want on-disk case %q", comps[0].Path, onDisk)
	}
	if comps[0].Size != 1000 || comps[0].Removed {
		t.Errorf("unexpected component %+v", comps[0])
	}
}

func TestLibraryFootprint_SameDirCountedOnce(t *testing.T) {
	home := t.TempDir()
	// bundleID "myapp" and appName "MyApp" name the same directory on a
	// case-insensitive volume (and after case resolution on any volume).
	writeFile(t, filepath.Join(home, "Library", "Application Support", "MyApp", "data"), 1000)
	writeFile(t, filepath.Join(home, "Library", "Logs", "myapp", "log.txt"), 200)

	comps := libraryFootprint(home, "myapp", "MyApp")
	if got := sumSizes(comps); got != 1200 {
		t.Errorf("expected 1200 (each dir once), got %d from %+v", got, comps)
	}
	if len(comps) != 2 {
		t.Errorf("expected 2 components, got %+v", comps)
	}
}

func TestPathSize(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "prefs.plist")
	writeFile(t, file, 1234)
	writeFile(t, filepath.Join(dir, "sub", "a"), 100)

	if got, want := pathSize(file), mustDirSize(t, file); got != want || got == 0 {
		t.Errorf("pathSize(file) = %d, want scan.DirSize %d", got, want)
	}
	if got, want := pathSize(filepath.Join(dir, "sub")), mustDirSize(t, filepath.Join(dir, "sub")); got != want {
		t.Errorf("pathSize(dir) = %d, want %d", got, want)
	}
	if got := pathSize(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("pathSize(missing) = %d, want 0", got)
	}
}

func mustDirSize(t *testing.T, p string) int64 {
	t.Helper()
	n, err := scan.DirSize(p)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestScanUnusedApps_GenuinelyUnusedStillFlagged(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	const id = "com.example.stale"
	responses := map[string]mockResponse{
		"ps": {output: []byte("/sbin/launchd\n/Applications/Other.app/Contents/MacOS/Other\n")},
	}
	appPath := addApp(t, responses, appDir, "Stale", id, longAgo.Format(mdlsDateLayout))

	lib := filepath.Join(home, "Library")
	prefs := filepath.Join(lib, "Preferences", id+".plist")
	caches := filepath.Join(lib, "Caches", id)
	container := filepath.Join(lib, "Containers", id)
	writeFile(t, prefs, 300)
	writeFile(t, filepath.Join(caches, "a", "b.db"), 500)
	writeFile(t, filepath.Join(container, "Data", "Library", "x.db"), 2000)
	agent := filepath.Join(lib, "LaunchAgents", "com.vendor.other.plist")
	writeFile(t, agent, 10)
	responses["plutil -convert json -o - "+agent] = mockResponse{
		output: []byte(`{"Label":"com.vendor.other","Program":"/Applications/Other.app/Contents/MacOS/Other"}`),
	}
	backdate(t, lib, longAgo)

	result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses))
	if result == nil || len(result.Entries) != 1 {
		t.Fatalf("expected the stale app flagged, got %v", entryPaths(result))
	}
	e := result.Entries[0]
	if e.Path != appPath || e.Description != "Stale (last used "+longAgo.Format("Jan 2006")+")" {
		t.Errorf("unexpected entry %+v", e)
	}
	want := map[string]struct {
		size    int64
		removed bool
	}{
		appPath:   {1000, true}, // bundle under home: cleanup removes it
		container: {2000, false},
		caches:    {500, false},
		prefs:     {300, false},
	}
	if len(e.Components) != len(want) {
		t.Fatalf("expected %d components, got %+v", len(want), e.Components)
	}
	for _, c := range e.Components {
		w, ok := want[c.Path]
		if !ok {
			t.Errorf("unexpected component %+v", c)
			continue
		}
		if c.Size != w.size || c.Removed != w.removed {
			t.Errorf("component %s = size %d removed %v, want %d %v", c.Path, c.Size, c.Removed, w.size, w.removed)
		}
	}
	if e.Size != 3800 || e.RemovedSize() != 1000 {
		t.Errorf("size %d removed %d, want 3800 and 1000", e.Size, e.RemovedSize())
	}
}

// withSystemLaunchdDirs points the system launchd directories at dirs for
// the duration of a test.
func withSystemLaunchdDirs(t *testing.T, dirs ...string) {
	t.Helper()
	orig := systemLaunchdDirs
	systemLaunchdDirs = dirs
	t.Cleanup(func() { systemLaunchdDirs = orig })
}

func TestScanUnusedApps_LaunchdJobs(t *testing.T) {
	const id = "com.example.vm"
	tests := []struct {
		name     string
		json     func(appPath string) string
		plutilOK bool
		skip     bool
	}{
		{"Program inside bundle", func(a string) string {
			return `{"Label":"x","Program":"` + a + `/Contents/Library/LaunchServices/helper"}`
		}, true, true},
		{"ProgramArguments[0] inside bundle", func(a string) string {
			return `{"Label":"x","ProgramArguments":["` + a + `/Contents/MacOS/vmnetd","-d"]}`
		}, true, true},
		{"absolute BundleProgram inside bundle", func(a string) string {
			return `{"Label":"x","BundleProgram":"` + a + `/Contents/MacOS/agent"}`
		}, true, true},
		{"AssociatedBundleIdentifiers string", func(string) string {
			return `{"Label":"x","Program":"/Library/PrivilegedHelperTools/x","AssociatedBundleIdentifiers":"` + id + `"}`
		}, true, true},
		{"AssociatedBundleIdentifiers array", func(string) string {
			return `{"Label":"x","Program":"/Library/PrivilegedHelperTools/x","AssociatedBundleIdentifiers":["com.other","` + id + `"]}`
		}, true, true},
		{"unrelated job", func(string) string {
			return `{"Label":"x","ProgramArguments":["/Library/PrivilegedHelperTools/x"],"AssociatedBundleIdentifiers":["com.other"]}`
		}, true, false},
		{"relative BundleProgram", func(string) string {
			return `{"Label":"x","BundleProgram":"Contents/MacOS/agent"}`
		}, true, false},
		{"malformed JSON", func(string) string { return `{not json` }, true, false},
		{"plutil fails", func(string) string { return "" }, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			daemons := filepath.Join(home, "sys", "LaunchDaemons")
			withSystemLaunchdDirs(t, filepath.Join(home, "sys", "LaunchAgents"), daemons)
			appDir := filepath.Join(home, "Applications")
			responses := map[string]mockResponse{}
			appPath := addApp(t, responses, appDir, "VM", id, longAgo.Format(mdlsDateLayout))

			plist := filepath.Join(daemons, "com.vendor.vmnetd.plist")
			writeFile(t, plist, 10)
			key := "plutil -convert json -o - " + plist
			if tt.plutilOK {
				responses[key] = mockResponse{output: []byte(tt.json(appPath))}
			} else {
				responses[key] = mockResponse{err: fmt.Errorf("plutil: invalid plist")}
			}

			got := entryPaths(scanUnusedApps(home, defaultThreshold, newMockRunner(responses)))
			if tt.skip && len(got) != 0 {
				t.Fatalf("app with a matching launchd job must be skipped, got %v", got)
			}
			if !tt.skip && (len(got) != 1 || got[0] != appPath) {
				t.Fatalf("expected %s flagged, got %v", appPath, got)
			}
		})
	}
}

func TestScanUnusedApps_EmptyPsOutputFlagsNothing(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "Applications")
	responses := map[string]mockResponse{"ps": {output: []byte("\n")}}
	addApp(t, responses, appDir, "OldApp", "com.example.oldapp", longAgo.Format(mdlsDateLayout))

	result := scanUnusedApps(home, defaultThreshold, newMockRunner(responses))
	if result == nil || len(result.Entries) != 0 || len(result.PermissionIssues) != 1 {
		t.Fatalf("empty ps output must fail closed with a note, got %+v", result)
	}
}

func TestIsRunning(t *testing.T) {
	procs := []string{"/Applications/Foo.app/Contents/MacOS/Foo", "/usr/bin/zsh"}
	tests := []struct {
		app  string
		want bool
	}{
		{"/Applications/Foo.app", true},
		{"/applications/foo.app", true}, // APFS is case-insensitive
		{"/Applications/Fo.app", false},
		{"/Applications/Foo.app/Contents/MacOS/Foo", false}, // the bundle itself, not a path inside
		{"/Applications/Bar.app", false},
	}
	for _, tt := range tests {
		if got := isRunning(tt.app, procs); got != tt.want {
			t.Errorf("isRunning(%q) = %v, want %v", tt.app, got, tt.want)
		}
	}
}

func TestNewestModTime(t *testing.T) {
	cutoff := time.Now().Add(-defaultThreshold)

	// tree creates root/<rel> (a file), backdates everything, then marks
	// only the file as modified now.
	tree := func(t *testing.T, rel ...string) (string, string) {
		t.Helper()
		root := t.TempDir()
		f := filepath.Join(append([]string{root}, rel...)...)
		writeFile(t, f, 10)
		backdate(t, root, longAgo)
		now := time.Now()
		if err := os.Chtimes(f, now, now); err != nil {
			t.Fatal(err)
		}
		return root, f
	}

	t.Run("finds a fresh file at the depth cap", func(t *testing.T) {
		root, _ := tree(t, "a", "b", "c", "new.raw") // depth 4
		if got := newestModTime(root, cutoff, 4, 5000); !got.After(cutoff) {
			t.Errorf("expected fresh mtime, got %v", got)
		}
	})

	t.Run("does not descend past the depth cap", func(t *testing.T) {
		root, _ := tree(t, "a", "b", "c", "d", "new.raw") // depth 5
		got := newestModTime(root, cutoff, 4, 5000)
		if got.After(cutoff) {
			t.Errorf("entry below the depth cap must not be visited, got %v", got)
		}
		if got.IsZero() {
			t.Error("expected the old mtimes of visited entries, got zero")
		}
	})

	t.Run("stops after the entry cap", func(t *testing.T) {
		root := t.TempDir()
		for i := 0; i < 10; i++ {
			writeFile(t, filepath.Join(root, fmt.Sprintf("a%02d", i)), 1)
		}
		writeFile(t, filepath.Join(root, "z-new"), 1) // visited last (lexical order)
		backdate(t, root, longAgo)
		now := time.Now()
		if err := os.Chtimes(filepath.Join(root, "z-new"), now, now); err != nil {
			t.Fatal(err)
		}
		if got := newestModTime(root, cutoff, 4, 5); got.After(cutoff) {
			t.Errorf("entries past the cap must not be visited, got %v", got)
		}
		if got := newestModTime(root, cutoff, 4, 5000); !got.After(cutoff) {
			t.Errorf("with a large cap the fresh file must be found, got %v", got)
		}
	})

	t.Run("regular file root", func(t *testing.T) {
		_, f := tree(t, "only")
		if got := newestModTime(f, cutoff, 4, 5000); !got.After(cutoff) {
			t.Errorf("expected the file's own fresh mtime, got %v", got)
		}
	})

	t.Run("missing root is zero", func(t *testing.T) {
		if got := newestModTime(filepath.Join(t.TempDir(), "nope"), cutoff, 4, 5000); !got.IsZero() {
			t.Errorf("expected zero, got %v", got)
		}
	})

	t.Run("unreadable subdirectory is skipped", func(t *testing.T) {
		root, _ := tree(t, "locked", "new.raw")
		locked := filepath.Join(root, "locked")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0755) })
		got := newestModTime(root, cutoff, 4, 5000)
		if got.After(cutoff) || got.IsZero() {
			t.Errorf("expected only the old visible mtimes, got %v", got)
		}
	})
}

func TestOnDiskCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "discord"), 0755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		in, want string
	}{
		{filepath.Join(dir, "discord"), filepath.Join(dir, "discord")},
		{filepath.Join(dir, "Discord"), filepath.Join(dir, "discord")},
		{filepath.Join(dir, "DISCORD"), filepath.Join(dir, "discord")},
		{filepath.Join(dir, "Slack"), filepath.Join(dir, "Slack")},                           // no match
		{filepath.Join(dir, "missing", "Discord"), filepath.Join(dir, "missing", "Discord")}, // unreadable parent
	}
	for _, tt := range tests {
		if got := onDiskCase(tt.in); got != tt.want {
			t.Errorf("onDiskCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLibraryLastModified_ContainerDataBeyondTopLevel(t *testing.T) {
	home := t.TempDir()
	container := filepath.Join(home, "Library", "Containers", "com.example.docker")
	disk := filepath.Join(container, "Data", "vms", "0", "data", "Docker.raw")
	writeFile(t, disk, 10)
	backdate(t, container, longAgo)
	now := time.Now()
	if err := os.Chtimes(disk, now, now); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().Add(-defaultThreshold)
	if got := libraryLastModified(home, "com.example.docker", "Docker", cutoff); !got.After(cutoff) {
		t.Errorf("Docker.raw written now (depth 4 below Containers/<id>/Data) must count, got %v", got)
	}
}

func TestHasLaunchdJob(t *testing.T) {
	jobs := []launchdJob{{programs: []string{"/Library/PrivilegedHelperTools/x"}, bundleIDs: []string{"com.example.app"}}}
	if hasLaunchdJob("/Applications/App.app", "", jobs) {
		t.Error("an app without a bundle ID must only match by program path")
	}
	if !hasLaunchdJob("/Applications/App.app", "COM.example.APP", jobs) {
		t.Error("bundle IDs should match case-insensitively")
	}
	if hasLaunchdJob("/Applications/App.app", "com.example.other", nil) {
		t.Error("no jobs, no match")
	}
}
