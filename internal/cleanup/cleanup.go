// Package cleanup provides file and directory removal with safety re-checks.
// It continues on individual errors and returns a structured summary of the
// cleanup operation.
package cleanup

import (
	"fmt"
	"os"
	"strings"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CleanupResult summarises the outcome of a cleanup operation.
type CleanupResult struct {
	// Removed is the number of items successfully removed.
	Removed int
	// Failed is the number of items that failed removal.
	Failed int
	// BytesFreed is the total size in bytes of successfully removed items.
	BytesFreed int64
	// Errors holds individual error details for failed items.
	Errors []error
}

// Execute removes all entries from the given scan results. Each path is
// re-checked against the safety blocklist before deletion. Pseudo-paths
// (e.g. "docker:...") are skipped. Errors on individual items do not
// abort the overall operation.
func Execute(results []scan.CategoryResult) CleanupResult {
	var res CleanupResult

	for _, cat := range results {
		for _, entry := range cat.Entries {
			// Skip pseudo-paths that are informational only.
			if isPseudoPath(entry.Path) {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Errorf("skip non-filesystem path: %s", entry.Path))
				continue
			}

			// Re-check safety at deletion time.
			if blocked, reason := safety.IsPathBlocked(entry.Path); blocked {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Errorf("blocked: %s (%s)", entry.Path, reason))
				continue
			}

			err := os.RemoveAll(entry.Path)
			if err != nil && !os.IsNotExist(err) {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Errorf("remove %s: %w", entry.Path, err))
				continue
			}

			res.Removed++
			res.BytesFreed += entry.Size
		}
	}

	return res
}

// isPseudoPath returns true for paths that represent non-filesystem entries
// (e.g. Docker resource identifiers like "docker:BuildCache").
func isPseudoPath(path string) bool {
	return strings.Contains(path, ":")
}
