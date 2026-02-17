// Package photos provides scanners for Apple Photos and media analysis cache directories.
package photos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Scan discovers and sizes Apple Photos cache directories including Photos app
// caches, media analysis data, iCloud sync caches, and Messages shared photos.
// Missing applications are silently skipped. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanPhotosCaches(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanAnalysisCaches(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanCloudPhotoCaches(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanSyndicationLibrary(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanPhotosCaches scans ~/Library/Containers/com.apple.Photos/Data/Library/Caches/.
// Returns nil if the directory does not exist.
func scanPhotosCaches(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Containers", "com.apple.Photos", "Data", "Library", "Caches")
	return scanSingleDir(dir, "photos-caches", "Photos App Cache")
}

// scanAnalysisCaches scans media analysis cache directories:
//   - ~/Library/Containers/com.apple.mediaanalysisd/Data/Library/Caches/
//   - ~/Library/Containers/com.apple.photoanalysisd/Data/Library/Caches/
//
// Results from both paths are combined into a single CategoryResult.
// Returns nil if neither directory exists.
func scanAnalysisCaches(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "Library", "Caches"),
		filepath.Join(home, "Library", "Containers", "com.apple.photoanalysisd", "Data", "Library", "Caches"),
	}
	return scanMultiDir(paths, "photos-analysis", "Photos Analysis Cache")
}

// scanCloudPhotoCaches scans ~/Library/Containers/com.apple.cloudphotosd/Data/Library/Caches/.
// Returns nil if the directory does not exist.
func scanCloudPhotoCaches(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Containers", "com.apple.cloudphotosd", "Data", "Library", "Caches")
	return scanSingleDir(dir, "photos-icloud-cache", "iCloud Photos Sync Cache")
}

// scanSyndicationLibrary scans ~/Library/Photos/Libraries/Syndication.photoslibrary.
// Returns nil if the directory does not exist.
func scanSyndicationLibrary(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Photos", "Libraries", "Syndication.photoslibrary")
	return scanSingleDir(dir, "photos-syndication", "Messages Shared Photos (Syndication)")
}

// scanSingleDir scans a single directory and returns it as a blob entry.
// Returns nil if the directory does not exist or is empty.
func scanSingleDir(dir, category, description string) *scan.CategoryResult {
	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: description + " (permission denied)",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(dir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: description + " (permission denied)",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    category,
		Description: description,
		Entries: []scan.ScanEntry{
			{
				Path:        dir,
				Description: description,
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
