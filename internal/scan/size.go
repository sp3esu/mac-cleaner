package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DirSize returns the total size in bytes of all regular files under root.
// Symlinks are not followed or counted. Permission-denied entries are
// skipped silently. Returns 0 and an error if root does not exist.
func DirSize(root string) (int64, error) {
	// Check that the root exists before walking.
	if _, err := os.Lstat(root); err != nil {
		return 0, err
	}

	var total int64

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we cannot access (permission denied, etc.)
			return nil
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				// Skip files whose info we cannot read.
				return nil
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

// FormatSize formats a byte count as a human-readable string using SI units
// (base 1000) to match macOS Finder convention.
// Examples: 0 -> "0 B", 1500 -> "1.5 kB", 1000000 -> "1.0 MB".
func FormatSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"kB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}
