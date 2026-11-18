// Package scantest provides test helpers for asserting scan sizes, which
// are allocated disk usage rather than logical file length.
package scantest

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

// OnDisk returns the bytes the test's temporary filesystem allocates for
// regular, non-sparse files of the given lengths, summed. Tests use it to
// turn "a 100-byte file plus a 200-byte file" into the size a scanner
// reports for them, without hard-coding the filesystem block size.
func OnDisk(t testing.TB, sizes ...int) int64 {
	t.Helper()
	dir := t.TempDir()
	var total int64
	for i, n := range sizes {
		path := filepath.Join(dir, strconv.Itoa(i))
		if err := os.WriteFile(path, make([]byte, n), 0o600); err != nil {
			t.Fatalf("scantest.OnDisk: write %s: %v", path, err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("scantest.OnDisk: lstat %s: %v", path, err)
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			total += info.Size()
			continue
		}
		total += st.Blocks * 512
	}
	return total
}
