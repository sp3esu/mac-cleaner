// Package unused provides a scanner that identifies macOS applications
// not opened in a configurable time period, along with their total disk
// footprint (bundle + ~/Library/ data).
package unused

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CmdRunner executes an external command and returns its combined stdout output.
// It is used for dependency injection so mdls and PlistBuddy calls can be
// mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- all command names and arguments are hardcoded string literals, no user input
	return cmd.Output()
}

// defaultThreshold is the minimum time since last use for an app to be
// considered unused.
const defaultThreshold = 180 * 24 * time.Hour

// mdlsDateLayout is the time layout returned by mdls -raw for kMDItemLastUsedDate.
const mdlsDateLayout = "2006-01-02 15:04:05 +0000"

// Scan discovers applications not opened in 180+ days and returns their
// total disk footprint (bundle + ~/Library/ data). Missing directories
// are silently skipped. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanUnusedApps(home, defaultThreshold, defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanUnusedApps scans application directories for .app bundles that have
// not been opened within the given threshold. Each entry includes the total
// footprint: bundle size + associated ~/Library/ directories.
func scanUnusedApps(home string, threshold time.Duration, runner CmdRunner) *scan.CategoryResult {
	appDirs := []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}

	cutoff := time.Now().Add(-threshold)
	plistBuddyPath := "/usr/libexec/PlistBuddy"

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, appDir := range appDirs {
		dirEntries, err := os.ReadDir(appDir)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        appDir,
					Description: appDir + " (permission denied)",
				})
			}
			continue
		}

		for _, entry := range dirEntries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}

			appPath := filepath.Join(appDir, entry.Name())

			// Query last-used date via Spotlight metadata.
			lastUsed, err := queryLastUsedDate(appPath, runner)
			if err != nil {
				// mdls failure: skip this app silently.
				continue
			}

			// Skip recently used apps.
			if lastUsed != nil && lastUsed.After(cutoff) {
				continue
			}

			// Extract bundle ID for Library footprint calculation.
			bundleID := extractBundleID(appPath, plistBuddyPath, runner)

			appName := strings.TrimSuffix(entry.Name(), ".app")

			// Secondary check: skip if Library data was recently modified.
			if latestMod := libraryLastModified(home, bundleID, appName); !latestMod.IsZero() && latestMod.After(cutoff) {
				continue
			}

			// Calculate total footprint.
			bundleSize, err := scan.DirSize(appPath)
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, scan.PermissionIssue{
						Path:        appPath,
						Description: appName + " (permission denied)",
					})
				}
				continue
			}

			libSize := libraryFootprint(home, bundleID, appName)
			size := bundleSize + libSize

			if size == 0 {
				continue
			}

			desc := formatDescription(appName, lastUsed)

			entries = append(entries, scan.ScanEntry{
				Path:        appPath,
				Description: desc,
				Size:        size,
			})
			totalSize += size
		}
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "unused-apps",
		Description:      "Unused Applications (180+ days)",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// queryLastUsedDate queries macOS Spotlight for the last-used date of an app.
// Returns nil time pointer for apps that have never been opened.
// Returns an error if mdls fails.
func queryLastUsedDate(appPath string, runner CmdRunner) (*time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, "mdls", "-name", "kMDItemLastUsedDate", "-raw", appPath)
	if err != nil {
		return nil, fmt.Errorf("mdls failed for %s: %w", appPath, err)
	}

	raw := strings.TrimSpace(string(out))

	// "(null)" means the app has never been opened.
	if raw == "(null)" || raw == "" {
		return nil, nil
	}

	t, err := time.Parse(mdlsDateLayout, raw)
	if err != nil {
		return nil, fmt.Errorf("cannot parse date %q: %w", raw, err)
	}

	return &t, nil
}

// extractBundleID reads CFBundleIdentifier from an app's Info.plist.
// Returns empty string on any error.
func extractBundleID(appPath, plistBuddyPath string, runner CmdRunner) string {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, plistBuddyPath, "-c", "Print :CFBundleIdentifier", plistPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// libraryFootprint calculates the total size of an app's associated
// ~/Library/ directories. Paths are probed by both bundleID and appName.
func libraryFootprint(home, bundleID, appName string) int64 {
	var total int64

	// Direct paths to probe by bundleID.
	if bundleID != "" {
		directPaths := []string{
			filepath.Join(home, "Library", "Application Support", bundleID),
			filepath.Join(home, "Library", "Caches", bundleID),
			filepath.Join(home, "Library", "Containers", bundleID),
			filepath.Join(home, "Library", "Saved Application State", bundleID+".savedState"),
			filepath.Join(home, "Library", "HTTPStorages", bundleID),
			filepath.Join(home, "Library", "WebKit", bundleID),
			filepath.Join(home, "Library", "Logs", bundleID),
			filepath.Join(home, "Library", "Preferences", bundleID+".plist"),
			filepath.Join(home, "Library", "Cookies", bundleID+".binarycookies"),
		}

		for _, p := range directPaths {
			total += pathSize(p)
		}

		// Glob patterns for bundleID.
		globPatterns := []string{
			filepath.Join(home, "Library", "Group Containers", "*"+bundleID+"*"),
			filepath.Join(home, "Library", "Preferences", "ByHost", bundleID+".*.plist"),
			filepath.Join(home, "Library", "LaunchAgents", bundleID+"*.plist"),
		}

		for _, pattern := range globPatterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, m := range matches {
				total += pathSize(m)
			}
		}
	}

	// Probe by appName (Application Support and Logs).
	if appName != "" {
		namePaths := []string{
			filepath.Join(home, "Library", "Application Support", appName),
			filepath.Join(home, "Library", "Logs", appName),
		}
		for _, p := range namePaths {
			// Avoid double-counting if bundleID == appName.
			if bundleID != "" && bundleID == appName {
				continue
			}
			total += pathSize(p)
		}
	}

	return total
}

// libraryLastModified returns the most recent modification time across an
// app's ~/Library/ data directories. Only top-level directory mtimes are
// checked (no recursive walk). Returns zero time if no paths exist.
func libraryLastModified(home, bundleID, appName string) time.Time {
	var paths []string

	if bundleID != "" {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", bundleID),
			filepath.Join(home, "Library", "Caches", bundleID),
			filepath.Join(home, "Library", "Containers", bundleID),
			filepath.Join(home, "Library", "Saved Application State", bundleID+".savedState"),
			filepath.Join(home, "Library", "HTTPStorages", bundleID),
			filepath.Join(home, "Library", "WebKit", bundleID),
			filepath.Join(home, "Library", "Logs", bundleID),
		)
	}

	if appName != "" && appName != bundleID {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", appName),
			filepath.Join(home, "Library", "Logs", appName),
		)
	}

	var latest time.Time
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if mod := info.ModTime(); mod.After(latest) {
			latest = mod
		}
	}
	return latest
}

// pathSize returns the size of a file or directory. Returns 0 if the path
// does not exist or cannot be read.
func pathSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}

	if !info.IsDir() {
		return info.Size()
	}

	size, err := scan.DirSize(path)
	if err != nil {
		return 0
	}
	return size
}

// formatDescription formats the app name with its last-used date.
func formatDescription(appName string, lastUsed *time.Time) string {
	if lastUsed == nil {
		return appName + " (no usage history)"
	}
	return appName + " (last used " + lastUsed.Format("Jan 2006") + ")"
}
