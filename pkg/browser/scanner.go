// Package browser provides scanners for macOS browser cache directories.
package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gregor/mac-cleaner/internal/safety"
	"github.com/gregor/mac-cleaner/internal/scan"
)

// Scan discovers and sizes browser cache directories for Safari, Chrome,
// and Firefox. Missing browsers are silently skipped. Permission failures
// are collected as PermissionIssue structs. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanSafari(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanChrome(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanFirefox(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanSafari scans the Safari cache directory. Returns nil if Safari is
// not installed or the cache directory does not exist. Returns a
// CategoryResult with PermissionIssue if TCC (Full Disk Access)
// permission prevents access.
func scanSafari(home string) *scan.CategoryResult {
	safariDir := filepath.Join(home, "Library", "Caches", "com.apple.Safari")

	_, err := os.Stat(safariDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "browser-safari",
				Description: "Safari Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        safariDir,
					Description: "Safari cache requires Full Disk Access",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(safariDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "browser-safari",
				Description: "Safari Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        safariDir,
					Description: "Safari cache requires Full Disk Access",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    "browser-safari",
		Description: "Safari Cache",
		Entries: []scan.ScanEntry{
			{
				Path:        safariDir,
				Description: "com.apple.Safari",
				Size:        size,
			},
		},
		TotalSize: size,
	}
}

// scanChrome scans Chrome cache directories including all user profiles
// (Default, Profile 1, Profile 2, etc.). Returns nil if Chrome cache
// directory does not exist.
func scanChrome(home string) *scan.CategoryResult {
	chromeDir := filepath.Join(home, "Library", "Caches", "Google", "Chrome")

	if _, err := os.Stat(chromeDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "browser-chrome",
				Description: "Chrome Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        chromeDir,
					Description: "Chrome cache (permission denied)",
				}},
			}
		}
		return nil
	}

	entries, err := os.ReadDir(chromeDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "browser-chrome",
				Description: "Chrome Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        chromeDir,
					Description: "Chrome cache (permission denied)",
				}},
			}
		}
		return nil
	}

	var scanEntries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(chromeDir, entry.Name())
		size, err := scan.DirSize(entryPath)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        entryPath,
					Description: fmt.Sprintf("Chrome (%s) (permission denied)", entry.Name()),
				})
			}
			continue
		}

		if size == 0 {
			continue
		}

		scanEntries = append(scanEntries, scan.ScanEntry{
			Path:        entryPath,
			Description: fmt.Sprintf("Chrome (%s)", entry.Name()),
			Size:        size,
		})
		totalSize += size
	}

	if len(scanEntries) == 0 && len(permIssues) == 0 {
		return nil
	}

	sort.Slice(scanEntries, func(i, j int) bool {
		return scanEntries[i].Size > scanEntries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "browser-chrome",
		Description:      "Chrome Cache",
		Entries:          scanEntries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// scanFirefox scans the Firefox cache directory. Returns nil if Firefox
// cache directory does not exist. Uses the shared ScanTopLevel helper
// since Firefox caches follow the standard directory-of-subdirectories pattern.
func scanFirefox(home string) *scan.CategoryResult {
	firefoxDir := filepath.Join(home, "Library", "Caches", "Firefox")

	if _, err := os.Stat(firefoxDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "browser-firefox",
				Description: "Firefox Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        firefoxDir,
					Description: "Firefox cache (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(firefoxDir, "browser-firefox", "Firefox Cache")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}
