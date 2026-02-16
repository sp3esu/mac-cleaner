// Package appleftovers provides scanners for orphaned app preferences,
// iOS device backups, and old Downloads files on macOS.
package appleftovers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gregor/mac-cleaner/internal/safety"
	"github.com/gregor/mac-cleaner/internal/scan"
)

// CmdRunner executes an external command and returns its combined stdout output.
// It is used for dependency injection so PlistBuddy calls can be mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// Scan discovers orphaned app preferences, iOS device backups, and old
// Downloads files. Missing directories are silently skipped. No files are
// modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanOrphanedPrefs(home, "/usr/libexec/PlistBuddy", defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanIOSBackups(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanOldDownloads(home, 90*24*time.Hour); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanOrphanedPrefs finds preference .plist files in ~/Library/Preferences
// that do not match any installed application's bundle ID. com.apple.*
// preferences are always skipped. Returns nil if PlistBuddy is not found
// or the Preferences directory does not exist.
func scanOrphanedPrefs(home, plistBuddyPath string, runner CmdRunner) *scan.CategoryResult {
	// Guard: PlistBuddy must exist.
	if _, err := exec.LookPath(plistBuddyPath); err != nil {
		return nil
	}

	prefsDir := filepath.Join(home, "Library", "Preferences")
	if _, err := os.Stat(prefsDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-orphaned-prefs",
				Description: "Orphaned Preferences",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        prefsDir,
					Description: "Preferences directory (permission denied)",
				}},
			}
		}
		return nil
	}

	// Build set of installed bundle IDs by scanning app directories.
	appDirs := []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
		"/System/Applications",
		"/System/Applications/Utilities",
	}

	installedIDs := make(map[string]bool)
	for _, appDir := range appDirs {
		entries, err := os.ReadDir(appDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}
			plistPath := filepath.Join(appDir, entry.Name(), "Contents", "Info.plist")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			out, err := runner(ctx, plistBuddyPath, "-c", "Print :CFBundleIdentifier", plistPath)
			cancel()

			if err != nil {
				continue
			}

			bundleID := strings.TrimSpace(string(out))
			if bundleID != "" {
				installedIDs[bundleID] = true
			}
		}
	}

	// Read preference files and find orphans.
	prefEntries, err := os.ReadDir(prefsDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-orphaned-prefs",
				Description: "Orphaned Preferences",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        prefsDir,
					Description: "Preferences directory (permission denied)",
				}},
			}
		}
		return nil
	}

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, entry := range prefEntries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".plist") {
			continue
		}

		domain := strings.TrimSuffix(name, ".plist")

		// Never flag com.apple.* as orphaned.
		if strings.HasPrefix(domain, "com.apple.") {
			continue
		}

		// Check if any installed bundle ID matches this domain.
		if isMatchedByInstalledApp(domain, installedIDs) {
			continue
		}

		info, err := os.Lstat(filepath.Join(prefsDir, name))
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        filepath.Join(prefsDir, name),
					Description: domain + " (permission denied)",
				})
			}
			continue
		}

		size := info.Size()
		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        filepath.Join(prefsDir, name),
			Description: domain,
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "app-orphaned-prefs",
		Description:      "Orphaned Preferences",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// isMatchedByInstalledApp checks if a preference domain matches any installed
// bundle ID. A match occurs when the domain equals a bundle ID or starts with
// a bundle ID followed by a dot.
func isMatchedByInstalledApp(domain string, installedIDs map[string]bool) bool {
	for id := range installedIDs {
		if domain == id || strings.HasPrefix(domain, id+".") {
			return true
		}
	}
	return false
}

// scanIOSBackups scans ~/Library/Application Support/MobileSync/Backup for
// iOS device backups. Returns nil if the directory does not exist or has no
// entries.
func scanIOSBackups(home string) *scan.CategoryResult {
	backupDir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")

	if _, err := os.Stat(backupDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-ios-backups",
				Description: "iOS Device Backups",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        backupDir,
					Description: "iOS backups (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(backupDir, "app-ios-backups", "iOS Device Backups")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanOldDownloads scans ~/Downloads for files and directories older than
// maxAge based on modification time. Returns nil if the directory does not
// exist or no old entries are found.
func scanOldDownloads(home string, maxAge time.Duration) *scan.CategoryResult {
	downloadsDir := filepath.Join(home, "Downloads")

	if _, err := os.Stat(downloadsDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-old-downloads",
				Description: "Old Downloads (90+ days)",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        downloadsDir,
					Description: "Downloads directory (permission denied)",
				}},
			}
		}
		return nil
	}

	dirEntries, err := os.ReadDir(downloadsDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-old-downloads",
				Description: "Old Downloads (90+ days)",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        downloadsDir,
					Description: "Downloads directory (permission denied)",
				}},
			}
		}
		return nil
	}

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, entry := range dirEntries {
		info, err := entry.Info()
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        filepath.Join(downloadsDir, entry.Name()),
					Description: entry.Name() + " (permission denied)",
				})
			}
			continue
		}

		if time.Since(info.ModTime()) <= maxAge {
			continue
		}

		var size int64
		entryPath := filepath.Join(downloadsDir, entry.Name())

		if entry.IsDir() {
			s, err := scan.DirSize(entryPath)
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, scan.PermissionIssue{
						Path:        entryPath,
						Description: entry.Name() + " (permission denied)",
					})
				}
				continue
			}
			size = s
		} else {
			size = info.Size()
		}

		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        entryPath,
			Description: entry.Name(),
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "app-old-downloads",
		Description:      "Old Downloads (90+ days)",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}
