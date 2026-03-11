// Package gitrepos discovers git repositories and reports reclaimable space.
package gitrepos

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/walk"
)

// CmdRunner executes an external command and returns its combined output.
// It is used for dependency injection so git CLI calls can be mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// lookPathFn wraps exec.LookPath for test injection.
var lookPathFn = exec.LookPath

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- command names and arguments are hardcoded string literals
	return cmd.CombinedOutput()
}

// Scan discovers git repositories under the home directory and reports
// their .git directory sizes and estimated reclaimable space.
func Scan() ([]scan.CategoryResult, error) {
	return scanWithRunner(defaultRunner)
}

func scanWithRunner(runner CmdRunner) ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("gitrepos: resolve home dir: %w", err)
	}

	if _, err := lookPathFn("git"); err != nil {
		return nil, nil // git not installed, skip silently
	}

	return scanFrom(home, runner)
}

func scanFrom(root string, runner CmdRunner) ([]scan.CategoryResult, error) {
	var entries []scan.ScanEntry

	err := walk.Walk(walk.Options{
		Root:     root,
		MaxDepth: 5,
		// Do NOT exclude ".git" -- we want to find it via OnDir.
		Exclude: []string{"node_modules", "vendor"},
		ExcludeAbs: []string{
			filepath.Join(root, "Library"),
			filepath.Join(root, "Applications"),
		},
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			if d.Name() != ".git" {
				return true // continue descent
			}
			if !d.IsDir() {
				return false // submodule/worktree .git file -- skip
			}

			// Check for lock file (concurrent operation).
			lockFile := filepath.Join(path, "index.lock")
			if _, err := os.Stat(lockFile); err == nil {
				return false // operation in progress, skip
			}

			repoRoot := filepath.Dir(path)
			gitDirSize, err := scan.DirSize(path)
			if err != nil || gitDirSize == 0 {
				return false
			}

			// Get reclaimable estimate.
			reclaimable := countObjectsReclaimable(runner, repoRoot)

			// Check for LFS.
			hasLFS := false
			lfsDir := filepath.Join(path, "lfs")
			if info, err := os.Stat(lfsDir); err == nil && info.IsDir() {
				hasLFS = true
			}

			// Build description.
			desc := filepath.Base(repoRoot) + " (.git " + scan.FormatSize(gitDirSize) + ")"
			if reclaimable > 0 {
				desc += " ~" + scan.FormatSize(reclaimable) + " reclaimable"
			}
			if hasLFS {
				desc += " [LFS]"
			}

			entries = append(entries, scan.ScanEntry{
				Path:        "git-gc:" + repoRoot,
				Description: desc,
				Size:        reclaimable,
			})

			return false // don't descend into .git
		},
	})
	if err != nil {
		return nil, fmt.Errorf("gitrepos: walk: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	var totalSize int64
	for _, e := range entries {
		totalSize += e.Size
	}

	cr := scan.CategoryResult{
		Category:    "git-repos",
		Description: fmt.Sprintf("Git Repositories (%d repos, ~%s reclaimable)", len(entries), scan.FormatSize(totalSize)),
		Entries:     entries,
		TotalSize:   totalSize,
	}
	cr.SetRiskLevels(safety.RiskForCategory)

	return []scan.CategoryResult{cr}, nil
}

// countObjectsReclaimable runs git count-objects -v and returns estimated
// reclaimable bytes (sum of size + size-garbage fields, converted from KiB).
func countObjectsReclaimable(runner CmdRunner, repoPath string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runner(ctx, "git", "-C", repoPath, "count-objects", "-v")
	if err != nil {
		return 0
	}

	return parseCountObjects(string(out))
}

// parseCountObjects parses git count-objects -v output and returns
// estimated reclaimable bytes. The "size" and "size-garbage" fields
// are in KiB, so they are multiplied by 1024.
func parseCountObjects(output string) int64 {
	var reclaimable int64
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "size", "size-garbage":
			n, _ := strconv.ParseInt(val, 10, 64)
			reclaimable += n * 1024 // KiB to bytes
		}
	}
	return reclaimable
}
