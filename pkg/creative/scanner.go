// Package creative provides scanners for creative application cache directories.
package creative

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Scan discovers and sizes creative application cache directories for Adobe,
// Sketch, and Figma. Missing applications are silently skipped. No files are
// modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanAdobeCaches(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanAdobeMediaCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanSketchCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanFigmaCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanAdobeCaches scans ~/Library/Caches/Adobe/.
// Returns nil if the directory does not exist.
func scanAdobeCaches(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Caches", "Adobe")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "creative-adobe",
				Description: "Adobe Caches",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Adobe Caches (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(dir, "creative-adobe", "Adobe Caches")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanAdobeMediaCache scans Adobe media cache directories:
//   - ~/Library/Application Support/Adobe/Common/Media Cache Files/
//   - ~/Library/Application Support/Adobe/Common/Media Cache/
//
// Results from both paths are combined into a single CategoryResult.
// Returns nil if neither directory exists.
func scanAdobeMediaCache(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "Adobe", "Common", "Media Cache Files"),
		filepath.Join(home, "Library", "Application Support", "Adobe", "Common", "Media Cache"),
	}

	return scanMultiDir(paths, "creative-adobe-media", "Adobe Media Cache")
}

// scanSketchCache scans ~/Library/Caches/com.bohemiancoding.sketch3/.
// Returns nil if the directory does not exist.
func scanSketchCache(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Caches", "com.bohemiancoding.sketch3")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "creative-sketch",
				Description: "Sketch Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Sketch Cache (permission denied)",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(dir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "creative-sketch",
				Description: "Sketch Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Sketch Cache (permission denied)",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    "creative-sketch",
		Description: "Sketch Cache",
		Entries: []scan.ScanEntry{
			{
				Path:        dir,
				Description: "Sketch",
				Size:        size,
			},
		},
		TotalSize: size,
	}
}

// scanFigmaCache scans Figma cache directories:
//   - ~/Library/Application Support/Figma/DesktopProfile/
//   - ~/Library/Application Support/Figma/Desktop/
//
// Results from both paths are combined into a single CategoryResult.
// Returns nil if neither directory exists.
func scanFigmaCache(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "Application Support", "Figma", "DesktopProfile"),
		filepath.Join(home, "Library", "Application Support", "Figma", "Desktop"),
	}

	return scanMultiDir(paths, "creative-figma", "Figma Cache")
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
