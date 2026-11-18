// Package cleanup provides file and directory removal with safety re-checks.
// It continues on individual errors and returns a structured summary of the
// cleanup operation.
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	// Removed is the number of items successfully removed (or, for git
	// repositories, successfully repacked).
	Removed int
	// Skipped is the number of items deliberately left alone without an
	// error: non-filesystem entries (docker:, tmutil:), paths that no longer
	// exist, paths already removed with an earlier entry in the same run,
	// and git repositories with an operation in progress.
	Skipped int
	// Failed is the number of items that failed removal, including paths
	// blocked by the safety check.
	Failed int
	// BytesFreed is the number of bytes cleanup actually freed: the removed
	// size (ScanEntry.RemovedSize) of each deleted path, counted once, plus
	// the measured shrink of each repacked .git directory.
	BytesFreed int64
	// Errors holds individual error details for failed items.
	Errors []error
}

// Execute removes all entries from the given scan results. Each path is
// re-checked against the safety blocklist before deletion. Entries with
// a "git-gc:" path prefix are handled by running git gc --aggressive
// (and optionally git lfs prune). Other pseudo-paths (e.g. "docker:...")
// are skipped. Errors on individual items do not abort the overall operation.
// Only entry.Path is ever removed: paths listed in entry.Components other
// than entry.Path are counted in Size but left in place, and BytesFreed
// credits entry.RemovedSize() rather than entry.Size. An entry whose path is
// missing, or equals or lies under a path already removed in this run
// (compared with scan.PathKey), is skipped and credited 0 bytes.
func Execute(results []scan.CategoryResult, onProgress ProgressFunc) CleanupResult {
	var res CleanupResult

	var total int
	for _, cat := range results {
		total += len(cat.Entries)
	}

	// removed maps the PathKey of each path removed in this run to the bytes
	// credited for it.
	removed := map[string]int64{}

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
				gitCleanup(&res, repoPath, strings.Contains(entry.Description, "[LFS]"))
				continue
			}

			// Skip pseudo-paths that are informational only.
			if isPseudoPath(entry.Path) {
				res.Skipped++
				continue
			}

			// Already deleted with an earlier entry (same path or an
			// ancestor). Checked before the safety check, which cannot
			// resolve a path whose parent directories are gone.
			key := scan.PathKey(entry.Path)
			if coveredByRemoved(removed, key) {
				res.Skipped++
				continue
			}

			// Re-check safety at deletion time.
			if blocked, reason := safety.IsPathBlocked(entry.Path); blocked {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Errorf("blocked: %s (%s)", entry.Path, reason))
				continue
			}

			if _, err := os.Lstat(entry.Path); errors.Is(err, fs.ErrNotExist) {
				res.Skipped++
				continue
			}

			if err := os.RemoveAll(entry.Path); err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Errorf("remove %s: %w", entry.Path, err))
				continue
			}

			freed := entry.RemovedSize() - takeRemovedBelow(removed, key)
			if freed < 0 {
				freed = 0
			}
			removed[key] = freed
			res.Removed++
			res.BytesFreed += freed
		}
	}

	return res
}

// gitCleanup runs git gc on repoPath after re-checking it, and records the
// outcome in res. BytesFreed is credited with the measured shrink of the
// .git directory, never with the scan's count-objects estimate.
func gitCleanup(res *CleanupResult, repoPath string, hasLFS bool) {
	if blocked, reason := safety.IsPathBlocked(repoPath); blocked {
		res.Failed++
		res.Errors = append(res.Errors, fmt.Errorf("blocked: %s (%s)", repoPath, reason))
		return
	}

	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Lstat(gitDir); errors.Is(err, fs.ErrNotExist) {
		res.Skipped++ // repository removed since the scan
		return
	}
	// Another git operation (or gc) is running: leave the repository alone.
	for _, lock := range []string{"index.lock", "gc.pid"} {
		if _, err := os.Lstat(filepath.Join(gitDir, lock)); err == nil {
			res.Skipped++
			return
		}
	}

	before, _ := scan.DirSize(gitDir)
	if err := runGitCleanup(repoPath, hasLFS); err != nil {
		res.Failed++
		res.Errors = append(res.Errors, fmt.Errorf("git cleanup %s: %w", repoPath, err))
		return
	}
	after, _ := scan.DirSize(gitDir)

	res.Removed++
	if before > after {
		res.BytesFreed += before - after
	}
}

// coveredByRemoved reports whether key, or any of its ancestor directories,
// was removed earlier in this run.
func coveredByRemoved(removed map[string]int64, key string) bool {
	for p := key; ; {
		if _, ok := removed[p]; ok {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// takeRemovedBelow returns the bytes already credited for paths removed
// inside key earlier in this run, and forgets them: key's own size already
// includes them, and key now covers them.
func takeRemovedBelow(removed map[string]int64, key string) int64 {
	prefix := key + string(filepath.Separator)
	var n int64
	for k, size := range removed {
		if strings.HasPrefix(k, prefix) {
			n += size
			delete(removed, k)
		}
	}
	return n
}

// isPseudoPath returns true for paths that represent non-filesystem entries
// (e.g. Docker resource identifiers like "docker:BuildCache").
// Real filesystem paths on macOS always start with "/".
func isPseudoPath(path string) bool {
	return !strings.HasPrefix(path, "/")
}
