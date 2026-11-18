package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// FileDiskUsage returns the bytes the filesystem has allocated for a file,
// as du reports them (st_blocks * 512). Sparse files therefore count only
// their allocated blocks, and small files are rounded up to whole blocks.
// When info carries no *syscall.Stat_t it falls back to info.Size().
func FileDiskUsage(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

// inodeKey identifies a file on disk independently of the path used to
// reach it, so hard links to the same data can be counted once.
type inodeKey struct {
	dev int64
	ino uint64
}

// DirUsage returns the allocated size in bytes (see FileDiskUsage) of all
// regular files under root. Files with more than one hard link are counted
// once per (device, inode) pair within this call. Symlinks are not followed
// or counted. Every path that cannot be read because of a permission error
// (including root's own directory listing) is skipped and appended to
// denied, so callers can report it while keeping the partial size. Other
// per-entry errors are skipped silently. err is non-nil only when root
// itself cannot be stat'ed, in which case size is 0.
func DirUsage(root string) (size int64, denied []string, err error) {
	// Check that the root exists before walking.
	if _, err := os.Lstat(root); err != nil {
		return 0, nil, err
	}

	seen := make(map[inodeKey]struct{})

	// The walk callback never returns an error, so WalkDir cannot fail.
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we cannot access. Propagating errors here would
			// abort the entire scan for a single bad entry, which is
			// undesirable for a cleanup tool. Permission errors are
			// recorded so the caller can tell the user.
			if os.IsPermission(err) {
				denied = append(denied, path)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			// Skip files whose info we cannot read.
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			key := inodeKey{dev: int64(st.Dev), ino: uint64(st.Ino)}
			if _, dup := seen[key]; dup {
				return nil
			}
			seen[key] = struct{}{}
		}
		size += FileDiskUsage(info)
		return nil
	})

	return size, denied, nil
}

// DirSize returns the allocated disk usage in bytes of all regular files
// under root (or of root itself when it is a regular file), as du reports
// it. Hard-linked files are counted once per root. Symlinks are not
// followed or counted. Permission-denied entries are skipped silently; use
// DirUsage to learn which paths were denied. Returns 0 and an error if root
// does not exist.
func DirSize(root string) (int64, error) {
	size, _, err := DirUsage(root)
	return size, err
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
