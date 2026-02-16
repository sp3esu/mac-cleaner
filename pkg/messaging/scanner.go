// Package messaging provides scanners for messaging application cache directories.
package messaging

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Scan discovers and sizes messaging application cache directories for Slack,
// Discord, Microsoft Teams, and Zoom. Missing applications are silently
// skipped. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanSlackCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanDiscordCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanTeamsCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanZoomCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanSlackCache scans Slack cache directories:
//   - ~/Library/Application Support/Slack/Cache/
//   - ~/Library/Application Support/Slack/Service Worker/CacheStorage/
//
// Returns nil if neither directory exists.
func scanSlackCache(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "Slack", "Cache"),
		filepath.Join(home, "Library", "Application Support", "Slack", "Service Worker", "CacheStorage"),
	}

	return scanMultiDir(paths, "msg-slack", "Slack Cache")
}

// scanDiscordCache scans Discord cache directories:
//   - ~/Library/Application Support/discord/Cache/
//   - ~/Library/Application Support/discord/Code Cache/
//
// Returns nil if neither directory exists.
func scanDiscordCache(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "discord", "Cache"),
		filepath.Join(home, "Library", "Application Support", "discord", "Code Cache"),
	}

	return scanMultiDir(paths, "msg-discord", "Discord Cache")
}

// scanTeamsCache scans Microsoft Teams cache directories:
//   - ~/Library/Application Support/Microsoft/Teams/Cache/
//   - ~/Library/Caches/com.microsoft.teams2/
//
// Returns nil if neither directory exists.
func scanTeamsCache(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "Microsoft", "Teams", "Cache"),
		filepath.Join(home, "Library", "Caches", "com.microsoft.teams2"),
	}

	return scanMultiDir(paths, "msg-teams", "Microsoft Teams Cache")
}

// scanZoomCache scans ~/Library/Application Support/zoom.us/data/.
// Returns nil if the directory does not exist.
func scanZoomCache(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Application Support", "zoom.us", "data")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "msg-zoom",
				Description: "Zoom Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Zoom Cache (permission denied)",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(dir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "msg-zoom",
				Description: "Zoom Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Zoom Cache (permission denied)",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    "msg-zoom",
		Description: "Zoom Cache",
		Entries: []scan.ScanEntry{
			{
				Path:        dir,
				Description: "Zoom",
				Size:        size,
			},
		},
		TotalSize: size,
	}
}

// scanMultiDir scans multiple directories and combines them into a single
// CategoryResult. Each existing directory becomes a single blob entry with
// its total size. Returns nil if no directories exist or all are empty.
func scanMultiDir(paths []string, category, description string) *scan.CategoryResult {
	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, dir := range paths {
		if _, err := os.Stat(dir); err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        dir,
					Description: description + " (permission denied)",
				})
			}
			continue
		}

		size, err := scan.DirSize(dir)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        dir,
					Description: description + " (permission denied)",
				})
			}
			continue
		}

		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        dir,
			Description: filepath.Base(dir),
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:         category,
		Description:      description,
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}
