package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/scan/scantest"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "one byte", bytes: 1, want: "1 B"},
		{name: "999 bytes", bytes: 999, want: "999 B"},
		{name: "1000 bytes is 1.0 kB", bytes: 1000, want: "1.0 kB"},
		{name: "1500 bytes is 1.5 kB", bytes: 1500, want: "1.5 kB"},
		{name: "1 MB", bytes: 1000000, want: "1.0 MB"},
		{name: "1.5 MB", bytes: 1500000, want: "1.5 MB"},
		{name: "1 GB", bytes: 1000000000, want: "1.0 GB"},
		{name: "5.4 GB", bytes: 5368709120, want: "5.4 GB"},
		{name: "1 TB", bytes: 1000000000000, want: "1.0 TB"},
		{name: "1 PB", bytes: 1000000000000000, want: "1.0 PB"},
		{name: "1 EB", bytes: 1000000000000000000, want: "1.0 EB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestDirSizeEmptyDir(t *testing.T) {
	dir := t.TempDir()
	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize(%q) unexpected error: %v", dir, err)
	}
	if size != 0 {
		t.Errorf("DirSize(empty) = %d, want 0", size)
	}
}

func TestDirSizeSingleFile(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 1024)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), data, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize(%q) unexpected error: %v", dir, err)
	}
	if want := scantest.OnDisk(t, 1024); size != want {
		t.Errorf("DirSize(single 1024-byte file) = %d, want %d", size, want)
	}
}

func TestDirSizeNestedDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create nested structure:
	// dir/
	//   a.txt (100 bytes)
	//   sub/
	//     b.txt (200 bytes)
	//     deep/
	//       c.txt (300 bytes)
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0755); err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 100), 0644); err != nil {
		t.Fatalf("failed to write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), make([]byte, 200), 0644); err != nil {
		t.Fatalf("failed to write b.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "deep", "c.txt"), make([]byte, 300), 0644); err != nil {
		t.Fatalf("failed to write c.txt: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize(%q) unexpected error: %v", dir, err)
	}
	want := scantest.OnDisk(t, 100, 200, 300)
	if size != want {
		t.Errorf("DirSize(nested) = %d, want %d", size, want)
	}
}

func TestDirSizeSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()

	// Create a real file (500 bytes)
	realFile := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(realFile, make([]byte, 500), 0644); err != nil {
		t.Fatalf("failed to write real.txt: %v", err)
	}

	// Create a symlink to the real file — should NOT be counted
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(realFile, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize(%q) unexpected error: %v", dir, err)
	}
	// Only the real file should be counted, not the symlink
	if want := scantest.OnDisk(t, 500); size != want {
		t.Errorf("DirSize(with symlink) = %d, want %d (symlink should be skipped)", size, want)
	}
}

func TestDirSizeNonExistent(t *testing.T) {
	size, err := DirSize("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("DirSize(nonexistent) expected error, got nil")
	}
	if size != 0 {
		t.Errorf("DirSize(nonexistent) = %d, want 0", size)
	}
}

func TestDirSizePermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()

	// Create a readable file (100 bytes)
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), make([]byte, 100), 0644); err != nil {
		t.Fatalf("failed to write ok.txt: %v", err)
	}

	// Create a subdirectory with no read permission
	denied := filepath.Join(dir, "denied")
	if err := os.MkdirAll(denied, 0755); err != nil {
		t.Fatalf("failed to create denied dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(denied, "secret.txt"), make([]byte, 200), 0644); err != nil {
		t.Fatalf("failed to write secret.txt: %v", err)
	}
	// Remove read+execute permission so WalkDir cannot enter
	if err := os.Chmod(denied, 0000); err != nil {
		t.Fatalf("failed to chmod denied dir: %v", err)
	}
	// Restore permission on cleanup so TempDir can be removed
	t.Cleanup(func() { os.Chmod(denied, 0755) })

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize should not return error for permission-denied entries, got: %v", err)
	}
	// Only the readable file should be counted
	if want := scantest.OnDisk(t, 100); size != want {
		t.Errorf("DirSize(with permission-denied subdir) = %d, want %d", size, want)
	}
}

// allocated returns the bytes the filesystem has allocated for path, as du
// reports them (st_blocks * 512).
func allocated(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("lstat %s: no syscall.Stat_t", path)
	}
	return st.Blocks * 512
}

func TestDirSizeSparseFileCountsAllocatedBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sparse.raw")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	const logical = 8 << 30 // 8 GiB
	if err := f.Truncate(logical); err != nil {
		f.Close()
		t.Fatalf("truncate: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize: %v", err)
	}
	if size >= logical/2 {
		t.Errorf("DirSize(sparse 8 GiB) = %d, want much smaller than %d", size, int64(logical))
	}
	if want := allocated(t, path); size != want {
		t.Errorf("DirSize(sparse) = %d, want allocated %d", size, want)
	}
}

func TestDirSizeSmallFileRoundsUpToBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(path, []byte{1}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize: %v", err)
	}
	want := allocated(t, path)
	if want < 512 {
		t.Fatalf("filesystem allocated %d bytes for a 1-byte file; expected at least one block", want)
	}
	if size != want {
		t.Errorf("DirSize(1-byte file) = %d, want %d (allocated blocks)", size, want)
	}
}

func TestDirSizeHardLinkCountedOnce(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "blob")
	if err := os.WriteFile(orig, make([]byte, 10<<20), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Link(orig, filepath.Join(dir, "record-blob")); err != nil {
		t.Fatalf("link: %v", err)
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize: %v", err)
	}
	if want := allocated(t, orig); size != want {
		t.Errorf("DirSize(file + hard link) = %d, want %d (counted once)", size, want)
	}
}

func TestDirSizeSeparateCopiesCountedTwice(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, make([]byte, 1<<20), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	size, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize: %v", err)
	}
	if want := allocated(t, a) + allocated(t, b); size != want {
		t.Errorf("DirSize(two copies) = %d, want %d", size, want)
	}
}

func TestDirSizeRegularFileRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, make([]byte, 5000), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	size, err := DirSize(path)
	if err != nil {
		t.Fatalf("DirSize(file root): %v", err)
	}
	if want := allocated(t, path); size != want {
		t.Errorf("DirSize(file root) = %d, want %d", size, want)
	}
}

func TestDirUsageReportsDeniedSubdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.bin")
	if err := os.WriteFile(ok, make([]byte, 1<<20), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	denied := filepath.Join(dir, "denied")
	if err := os.MkdirAll(denied, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(denied, "secret.bin"), make([]byte, 1<<20), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(denied, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	size, deniedPaths, err := DirUsage(dir)
	if err != nil {
		t.Fatalf("DirUsage: %v", err)
	}
	if want := allocated(t, ok); size != want {
		t.Errorf("DirUsage size = %d, want partial size %d", size, want)
	}
	if len(deniedPaths) != 1 || deniedPaths[0] != denied {
		t.Errorf("DirUsage denied = %v, want [%s]", deniedPaths, denied)
	}
}

func TestDirUsageDeniedRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	root := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(root, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0755) }) // #nosec G302 -- test cleanup restores a test dir

	size, deniedPaths, err := DirUsage(root)
	if err != nil {
		t.Fatalf("DirUsage: %v", err)
	}
	if size != 0 {
		t.Errorf("DirUsage(denied root) size = %d, want 0", size)
	}
	if len(deniedPaths) != 1 || deniedPaths[0] != root {
		t.Errorf("DirUsage(denied root) denied = %v, want [%s]", deniedPaths, root)
	}
}

func TestDirUsageNonExistent(t *testing.T) {
	size, denied, err := DirUsage("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("DirUsage(nonexistent) expected error, got nil")
	}
	if size != 0 || len(denied) != 0 {
		t.Errorf("DirUsage(nonexistent) = (%d, %v), want (0, [])", size, denied)
	}
}

// fakeInfo is an fs.FileInfo whose Sys() is not a *syscall.Stat_t.
type fakeInfo struct{ size int64 }

func (f fakeInfo) Name() string       { return "fake" }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() fs.FileMode  { return 0 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

func TestFileDiskUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if got, want := FileDiskUsage(info), allocated(t, path); got != want {
		t.Errorf("FileDiskUsage = %d, want %d", got, want)
	}

	if got := FileDiskUsage(fakeInfo{size: 1234}); got != 1234 {
		t.Errorf("FileDiskUsage(no Stat_t) = %d, want 1234 (falls back to Size)", got)
	}
}
