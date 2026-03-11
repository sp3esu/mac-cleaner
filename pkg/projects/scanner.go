// Package projects scans for project build cache directories (node_modules,
// .build, target, venv/.venv, build) across the home directory.
package projects

import (
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Scan discovers project build caches under the user's home directory.
func Scan() ([]scan.CategoryResult, error) {
	return nil, nil
}

// scanFrom is the internal implementation that accepts a custom root for testing.
func scanFrom(root string) ([]scan.CategoryResult, error) {
	return nil, nil
}
