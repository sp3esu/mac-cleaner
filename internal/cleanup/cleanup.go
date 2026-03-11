// Package cleanup provides file and directory removal with safety re-checks.
// It continues on individual errors and returns a structured summary of the
// cleanup operation.
package cleanup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// runGitCleanup runs git gc --aggressive and optionally git lfs prune
// on the given repository. It is a package-level variable for test injection.
var runGitCleanup = defaultRunGitCleanup

// defaultRunGitCleanup runs git gc --aggressive with a 120-second timeout.
// When hasLFS is true and git-lfs is available, it also runs git lfs prune
// (best-effort; lfs prune errors do not fail the cleanup).
func defaultRunGitCleanup(repoPath string, hasLFS bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "gc", "--aggressive") // #nosec G204 -- repoPath is from scanner output, not user input
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git gc: %w", err)
	}

	if hasLFS {
		if _, err := exec.LookPath("git-lfs"); err == nil {
			lfsCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "lfs", "prune") // #nosec G204
			_ = lfsCmd.Run()                                                          // best-effort
		}
	}

	return nil
}

// ProgressFunc is called during cleanup to report progress.
// categoryDesc is the human-readable category name (e.g. "User App Caches").
// entryPath is "" for a category-start event, or the actual path for an entry-level event.
// current is the 1-based item index across all categories; total is the overall item count.
type ProgressFunc func(categoryDesc, entryPath string, current, total int)

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
// re-checked against the safety blocklist before deletion. Entries with
// a "git-gc:" path prefix are handled by running git gc --aggressive
// (and optionally git lfs prune). Other pseudo-paths (e.g. "docker:...")
// are skipped. Errors on individual items do not abort the overall operation.
func Execute(results []scan.CategoryResult, onProgress ProgressFunc) CleanupResult {
	var res CleanupResult

	var total int
	for _, cat := range results {
		total += len(cat.Entries)
	}

	current := 0
	for _, cat := range results {
		if onProgress != nil {
			onProgress(cat.Description, "", current+1, total)
		}
		for _, entry := range cat.Entries {
			current++
			if onProgress != nil {
				onProgress(cat.Description, entry.Path, current, total)
			}

			// Handle git-gc: pseudo-paths by running git gc.
			if repoPath, ok := strings.CutPrefix(entry.Path, "git-gc:"); ok {
				hasLFS := strings.Contains(entry.Description, "[LFS]")
				if err := runGitCleanup(repoPath, hasLFS); err != nil {
					res.Failed++
					res.Errors = append(res.Errors, fmt.Errorf("git cleanup %s: %w", repoPath, err))
				} else {
					res.Removed++
					res.BytesFreed += entry.Size
				}
				continue
			}

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
// Real filesystem paths on macOS always start with "/".
func isPseudoPath(path string) bool {
	return !strings.HasPrefix(path, "/")
}
