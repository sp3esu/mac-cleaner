// Package walk provides a reusable recursive directory walker with depth
// limits, name-based and absolute-path exclusions, and a per-directory
// callback. It is designed for scanning home directories for project
// caches and is reused across multiple scanning phases.
package walk

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Options configures a recursive directory walk.
type Options struct {
	// Root is the starting directory (walked at depth 0).
	Root string
	// MaxDepth is the maximum depth to descend (1 = immediate children of Root).
	MaxDepth int
	// Exclude lists base directory names to skip at any depth.
	Exclude []string
	// ExcludeAbs lists absolute path prefixes to skip.
	ExcludeAbs []string
	// OnDir is called for each directory visited. Return false to skip
	// descending into the directory.
	OnDir func(path string, d fs.DirEntry, depth int) bool
}

// Walk recursively traverses directories under opts.Root, respecting depth
// limits, exclusion lists, and the OnDir callback. Only directories are
// visited; regular files are ignored. Permission errors are skipped
// silently. Symlinks are not followed (filepath.WalkDir default).
func Walk(opts Options) error {
	// Resolve root to a clean absolute path for reliable depth calculation.
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return err
	}
	rootLen := len(root)

	// Build exclude set for O(1) lookups.
	excludeSet := make(map[string]bool, len(opts.Exclude))
	for _, name := range opts.Exclude {
		excludeSet[name] = true
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// On permission or I/O errors, skip the entry and continue.
		if err != nil {
			return nil
		}

		// Only process directories.
		if !d.IsDir() {
			return nil
		}

		// Calculate depth relative to root: root itself is depth 0.
		depth := 0
		if len(path) > rootLen {
			rel := path[rootLen+1:] // skip the separator
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}

		// Enforce max depth.
		if opts.MaxDepth > 0 && depth > opts.MaxDepth {
			return fs.SkipDir
		}

		// Skip root itself from exclusion checks but still call OnDir.
		if depth > 0 {
			name := d.Name()

			// Skip dotfile directories at depth 1 (direct children of root).
			// At deeper levels, dotfile dirs like .build and .venv are allowed.
			if depth == 1 && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}

			// Check exclude-by-name list.
			if excludeSet[name] {
				return fs.SkipDir
			}

			// Check exclude-by-absolute-path prefixes.
			for _, prefix := range opts.ExcludeAbs {
				if path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator)) {
					return fs.SkipDir
				}
			}
		}

		// Call OnDir callback.
		if opts.OnDir != nil {
			if !opts.OnDir(path, d, depth) {
				return fs.SkipDir
			}
		}

		return nil
	})
}
