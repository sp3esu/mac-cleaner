package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sp3esu/mac-cleaner/internal/safety"
)

// ScanTopLevel scans the top-level entries of a directory and returns a
// CategoryResult with sized entries sorted largest first. Blocked paths
// are skipped with warnings. Zero-byte entries are excluded. Sizes are
// allocated disk usage (see DirUsage); unreadable paths inside an entry are
// reported as PermissionIssues while the entry keeps its partial size.
func ScanTopLevel(dir, category, description string) (*CategoryResult, error) {
	return ScanTopLevelExcluding(dir, category, description, nil)
}

// ScanTopLevelExcluding is ScanTopLevel minus the paths in owned, which are
// slash-separated paths relative to dir that another scanner reports (and
// cleans) on its own. Names are compared case-insensitively, as APFS does.
// An entry equal to an owned path is skipped. An entry that is a strict
// ancestor of an owned path is never emitted itself; its children are
// emitted instead (recursively applying the same rule), so deleting a
// system-caches entry can never delete an owned path.
func ScanTopLevelExcluding(dir, category, description string, owned []string) (*CategoryResult, error) {
	if blocked, reason := safety.IsPathBlocked(dir); blocked {
		safety.WarnBlocked(dir, reason)
		return nil, fmt.Errorf("path blocked: %s", reason)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsPermission(err) {
			return &CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []PermissionIssue{{
					Path:        dir,
					Description: description + " (permission denied)",
				}},
			}, nil
		}
		return nil, err
	}

	scanEntries, permIssues := scanEntriesExcluding(dir, "", entries, owned)

	var totalSize int64
	for _, e := range scanEntries {
		totalSize += e.Size
	}

	// Sort entries by size descending (largest first).
	sort.Slice(scanEntries, func(i, j int) bool {
		return scanEntries[i].Size > scanEntries[j].Size
	})

	return &CategoryResult{
		Category:         category,
		Description:      description,
		Entries:          scanEntries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}, nil
}

// scanEntriesExcluding sizes the given entries of dir, skipping or expanding
// those that match owned (see ScanTopLevelExcluding). rel is dir's path
// relative to the scan root and prefixes each entry's description.
func scanEntriesExcluding(dir, rel string, entries []os.DirEntry, owned []string) ([]ScanEntry, []PermissionIssue) {
	var scanEntries []ScanEntry
	var permIssues []PermissionIssue

	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())
		entryRel := filepath.Join(rel, entry.Name())

		isOwned, nested := matchOwned(entry.Name(), owned)
		if isOwned {
			continue
		}

		if blocked, reason := safety.IsPathBlocked(entryPath); blocked {
			safety.WarnBlocked(entryPath, reason)
			continue
		}

		if len(nested) > 0 && entry.IsDir() {
			children, err := os.ReadDir(entryPath)
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, PermissionIssue{
						Path:        entryPath,
						Description: entry.Name() + " (permission denied)",
					})
				}
				continue
			}
			e, p := scanEntriesExcluding(entryPath, entryRel, children, nested)
			scanEntries = append(scanEntries, e...)
			permIssues = append(permIssues, p...)
			continue
		}

		var size int64
		if entry.IsDir() {
			s, denied, err := DirUsage(entryPath)
			for _, d := range denied {
				permIssues = append(permIssues, PermissionIssue{
					Path:        d,
					Description: filepath.Base(d) + " (permission denied)",
				})
			}
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, PermissionIssue{
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
					permIssues = append(permIssues, PermissionIssue{
						Path:        entryPath,
						Description: entry.Name() + " (permission denied)",
					})
				}
				continue
			}
			size = FileDiskUsage(info)
		}

		if size == 0 {
			continue
		}

		scanEntries = append(scanEntries, ScanEntry{
			Path:        entryPath,
			Description: entryRel,
			Size:        size,
		})
	}

	return scanEntries, permIssues
}

// matchOwned reports whether name equals one of the owned paths, and
// returns the remainders of the owned paths that lie below name (non-empty
// when name is a strict ancestor of an owned path). Comparison is
// case-insensitive.
func matchOwned(name string, owned []string) (isOwned bool, nested []string) {
	for _, o := range owned {
		first, rest, hasRest := strings.Cut(o, "/")
		if !strings.EqualFold(first, name) {
			continue
		}
		if !hasRest {
			return true, nil
		}
		nested = append(nested, rest)
	}
	return false, nested
}
