// Package homedir scans the user's home directory for large and old files
// that may be candidates for cleanup. It produces two categories:
// "home-large-files" for files >= 100 MB and "home-old-files" for files
// that are both >= 180 days old and >= 10 MB.
package homedir

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/walk"
)

const (
	// defaultSizeThreshold is the minimum file size to be considered "large" (100 MB).
	defaultSizeThreshold = 100 * 1024 * 1024
	// defaultAgeThreshold is the minimum age to be considered "old" (180 days).
	defaultAgeThreshold = 180 * 24 * time.Hour
	// defaultOldFileMinSize is the minimum file size for the "old files" category (10 MB).
	defaultOldFileMinSize = 10 * 1024 * 1024
)

// Scan discovers large and old files under the user's home directory.
// It returns one CategoryResult per category that has at least one entry.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("homedir: resolve home dir: %w", err)
	}
	return scanFrom(home)
}

// scanFrom is the internal implementation that accepts a custom root,
// enabling tests to use temporary directories.
func scanFrom(root string) ([]scan.CategoryResult, error) {
	var largeFiles []scan.ScanEntry
	var oldFiles []scan.ScanEntry
	now := time.Now()

	err := walk.Walk(walk.Options{
		Root:     root,
		MaxDepth: 5,
		Exclude:  []string{"node_modules", ".build", "target", "build", "venv", ".venv"},
		ExcludeAbs: []string{
			filepath.Join(root, "Library"),
			filepath.Join(root, "Applications"),
			filepath.Join(root, ".Trash"),
			filepath.Join(root, "Downloads"),
		},
		OnFile: func(path string, d fs.DirEntry, depth int) {
			// Skip symlinks.
			if d.Type()&fs.ModeSymlink != 0 {
				return
			}

			info, err := d.Info()
			if err != nil {
				return
			}

			size := info.Size()
			mtime := info.ModTime()
			age := now.Sub(mtime)

			// Check large file threshold.
			if size >= defaultSizeThreshold {
				largeFiles = append(largeFiles, scan.ScanEntry{
					Path:        path,
					Description: filepath.Base(path),
					Size:        size,
				})
			}

			// Check old file threshold (must also meet minimum size).
			if age >= defaultAgeThreshold && size >= defaultOldFileMinSize {
				oldFiles = append(oldFiles, scan.ScanEntry{
					Path:        path,
					Description: filepath.Base(path) + "  " + formatAge(mtime),
					Size:        size,
				})
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("homedir: walk: %w", err)
	}

	var results []scan.CategoryResult

	if len(largeFiles) > 0 {
		// Sort by size descending.
		sort.Slice(largeFiles, func(i, j int) bool {
			return largeFiles[i].Size > largeFiles[j].Size
		})

		var totalSize int64
		for _, e := range largeFiles {
			totalSize += e.Size
		}

		cr := scan.CategoryResult{
			Category:    "home-large-files",
			Description: fmt.Sprintf("Large Files >100 MB (%d files, %s)", len(largeFiles), scan.FormatSize(totalSize)),
			Entries:     largeFiles,
			TotalSize:   totalSize,
		}
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, cr)
	}

	if len(oldFiles) > 0 {
		// Sort by size descending.
		sort.Slice(oldFiles, func(i, j int) bool {
			return oldFiles[i].Size > oldFiles[j].Size
		})

		var totalSize int64
		for _, e := range oldFiles {
			totalSize += e.Size
		}

		cr := scan.CategoryResult{
			Category:    "home-old-files",
			Description: fmt.Sprintf("Old Files >180 days (%d files, %s)", len(oldFiles), scan.FormatSize(totalSize)),
			Entries:     oldFiles,
			TotalSize:   totalSize,
		}
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, cr)
	}

	return results, nil
}

// formatAge formats a time as a human-readable age string.
func formatAge(mtime time.Time) string {
	d := time.Since(mtime)
	days := int(d.Hours() / 24)
	if days < 1 {
		return "<1d"
	}
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	years := months / 12
	return fmt.Sprintf("%dy", years)
}
