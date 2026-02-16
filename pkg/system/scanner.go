// Package system provides scanners for macOS system-level cache directories.
package system

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Scan discovers and sizes system cache directories. It scans
// ~/Library/Caches, ~/Library/Logs, and QuickLook thumbnail caches.
// Blocked paths are skipped with stderr warnings. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	// User App Caches
	if cr, err := scan.ScanTopLevel(filepath.Join(home, "Library", "Caches"), "system-caches", "User App Caches"); err == nil && cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		if len(cr.Entries) > 0 || len(cr.PermissionIssues) > 0 {
			results = append(results, *cr)
		}
	}

	// User Logs
	if cr, err := scan.ScanTopLevel(filepath.Join(home, "Library", "Logs"), "system-logs", "User Logs"); err == nil && cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		if len(cr.Entries) > 0 || len(cr.PermissionIssues) > 0 {
			results = append(results, *cr)
		}
	}

	// QuickLook Thumbnails
	if cacheDir, err := quickLookCacheDir(); err == nil {
		if cr, err := scanQuickLook(cacheDir, "quicklook", "QuickLook Thumbnails"); err == nil && cr != nil {
			cr.SetRiskLevels(safety.RiskForCategory)
			results = append(results, *cr)
		}
	}

	return results, nil
}

// quickLookCacheDir derives the per-user QuickLook cache directory from
// $TMPDIR. On macOS, TMPDIR is typically /var/folders/XX/YY/T/, and the
// cache directory is the sibling "C" directory.
func quickLookCacheDir() (string, error) {
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		return "", fmt.Errorf("TMPDIR not set")
	}
	if !strings.Contains(tmpDir, "/var/folders/") {
		return "", fmt.Errorf("TMPDIR does not look like macOS per-user temp: %s", tmpDir)
	}

	parent := filepath.Dir(filepath.Clean(tmpDir))
	cacheDir := filepath.Join(parent, "C")

	if _, err := os.Stat(cacheDir); err != nil {
		return "", fmt.Errorf("QuickLook cache dir not found: %w", err)
	}

	return cacheDir, nil
}

// scanQuickLook scans a per-user cache directory for QuickLook-related
// entries (directories matching "com.apple.quicklook.*") and aggregates
// them into a single CategoryResult.
func scanQuickLook(cacheParent, category, description string) (*scan.CategoryResult, error) {
	entries, err := os.ReadDir(cacheParent)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []scan.PermissionIssue{{
					Path:        cacheParent,
					Description: description + " (permission denied)",
				}},
			}, nil
		}
		return nil, err
	}

	var scanEntries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "com.apple.quicklook.") {
			continue
		}

		entryPath := filepath.Join(cacheParent, entry.Name())

		var size int64
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
			info, err := entry.Info()
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, scan.PermissionIssue{
						Path:        entryPath,
						Description: entry.Name() + " (permission denied)",
					})
				}
				continue
			}
			size = info.Size()
		}

		if size == 0 {
			continue
		}

		scanEntries = append(scanEntries, scan.ScanEntry{
			Path:        entryPath,
			Description: entry.Name(),
			Size:        size,
		})
		totalSize += size
	}

	if len(scanEntries) == 0 && len(permIssues) == 0 {
		return nil, nil
	}

	sort.Slice(scanEntries, func(i, j int) bool {
		return scanEntries[i].Size > scanEntries[j].Size
	})

	return &scan.CategoryResult{
		Category:         category,
		Description:      description,
		Entries:          scanEntries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}, nil
}
