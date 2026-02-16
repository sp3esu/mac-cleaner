package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gregor/mac-cleaner/internal/safety"
)

// ScanTopLevel scans the top-level entries of a directory and returns a
// CategoryResult with sized entries sorted largest first. Blocked paths
// are skipped with warnings. Zero-byte entries are excluded.
func ScanTopLevel(dir, category, description string) (*CategoryResult, error) {
	if blocked, reason := safety.IsPathBlocked(dir); blocked {
		safety.WarnBlocked(dir, reason)
		return nil, fmt.Errorf("path blocked: %s", reason)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scanEntries []ScanEntry
	var totalSize int64

	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())

		if blocked, reason := safety.IsPathBlocked(entryPath); blocked {
			safety.WarnBlocked(entryPath, reason)
			continue
		}

		var size int64
		if entry.IsDir() {
			s, err := DirSize(entryPath)
			if err != nil {
				continue
			}
			size = s
		} else {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			size = info.Size()
		}

		if size == 0 {
			continue
		}

		scanEntries = append(scanEntries, ScanEntry{
			Path:        entryPath,
			Description: entry.Name(),
			Size:        size,
		})
		totalSize += size
	}

	// Sort entries by size descending (largest first).
	sort.Slice(scanEntries, func(i, j int) bool {
		return scanEntries[i].Size > scanEntries[j].Size
	})

	return &CategoryResult{
		Category:    category,
		Description: description,
		Entries:     scanEntries,
		TotalSize:   totalSize,
	}, nil
}
