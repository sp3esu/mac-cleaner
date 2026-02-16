package scan

import (
	"os"
	"path/filepath"
	"testing"
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
	if size != 1024 {
		t.Errorf("DirSize(single 1024-byte file) = %d, want 1024", size)
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
	want := int64(600) // 100 + 200 + 300
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
	if size != 500 {
		t.Errorf("DirSize(with symlink) = %d, want 500 (symlink should be skipped)", size)
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
	if size != 100 {
		t.Errorf("DirSize(with permission-denied subdir) = %d, want 100", size)
	}
}
