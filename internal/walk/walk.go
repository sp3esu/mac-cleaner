// Package walk provides a reusable recursive directory walker with depth
// limits, name-based and absolute-path exclusions, and a per-directory
// callback.
package walk

import "io/fs"

// Options configures a recursive directory walk.
type Options struct {
	// Root is the starting directory.
	Root string
	// MaxDepth is the maximum depth to descend (1 = immediate children of Root).
	MaxDepth int
	// Exclude lists base directory names to skip at any depth.
	Exclude []string
	// ExcludeAbs lists absolute path prefixes to skip.
	ExcludeAbs []string
	// OnDir is called for each directory visited. Return false to skip descent.
	OnDir func(path string, d fs.DirEntry, depth int) bool
}

// Walk recursively traverses directories under opts.Root.
func Walk(opts Options) error {
	return nil
}
