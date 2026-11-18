package homedir

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// backdate sets path's atime and mtime to the given number of days ago.
func backdate(t *testing.T, path string, days int) {
	t.Helper()
	old := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes(%s): %v", path, err)
	}
}

// allocated returns the bytes allocated on disk for path.
func allocated(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Sys().(*syscall.Stat_t).Blocks * 512
}

func entryPaths(t *testing.T, root string) map[string][]string {
	t.Helper()
	results, err := scanFrom(root)
	if err != nil {
		t.Fatalf("scanFrom: %v", err)
	}
	got := map[string][]string{}
	for _, cr := range results {
		for _, e := range cr.Entries {
			rel, _ := filepath.Rel(root, e.Path)
			got[cr.Category] = append(got[cr.Category], rel)
		}
	}
	return got
}

// TestProtectedLocationsNotFlagged covers package bundles, VCS metadata and
// installed-tool directories: their files must never be offered for
// deletion, however large or old they are.
func TestProtectedLocationsNotFlagged(t *testing.T) {
	root := t.TempDir()
	const size = 101 * 1024 * 1024
	protected := []string{
		"Pictures/Photos Library.photoslibrary/database/Photos.sqlite",
		"Music/Music/Music Library.musiclibrary/Library.musicdb",
		"Music/Music/Media.localized/Music/song.m4a",
		"repo/.git/objects/pack/pack-abc.pack",
		"go/bin/gopls",
		"bin/tool",
		"X.app/Contents/big",
		"Movies/TV/Media.localized/v.m4v",
	}
	// Other package suffixes (matched case-insensitively) and VCS dirs. These
	// are only big enough (>= 10 MiB, 400 days old) to be "old files".
	protectedOld := []string{
		"Movies/Project.FCPBUNDLE/render.mov",
		"Music/Song.logicx/Alternatives/000/ProjectData",
		"VMs/Win.pvm/disk.hdd",
		"VMs/Linux.utm/Data/disk.qcow2",
		"VMs/Old.vmwarevm/disk.vmdk",
		"Disk.sparsebundle/bands/0",
		"code/Lib.framework/Versions/A/Lib",
		"code/Plug.bundle/Contents/MacOS/Plug",
		"Movies/iMovie Library.imovielibrary/clip.mov",
		"Movies/TV Library.tvlibrary/db",
		"code/hgrepo/.hg/store/data.i",
		"code/svnrepo/.SVN/pristine/blob",
	}
	for _, rel := range protected {
		p := filepath.Join(root, rel)
		writeFile(t, p, size)
		backdate(t, p, 400)
	}
	for _, rel := range protectedOld {
		p := filepath.Join(root, rel)
		writeFile(t, p, 11*1024*1024)
		backdate(t, p, 400)
	}
	protected = append(protected, protectedOld...)
	iso := filepath.Join(root, "Documents", "big.iso")
	writeFile(t, iso, size)
	backdate(t, iso, 400)

	got := entryPaths(t, root)
	for cat, paths := range got {
		for _, p := range paths {
			for _, rel := range protected {
				if p == rel {
					t.Errorf("%s: protected file %s was flagged", cat, rel)
				}
			}
		}
	}
	for _, cat := range []string{"home-large-files", "home-old-files"} {
		if len(got[cat]) != 1 || got[cat][0] != filepath.Join("Documents", "big.iso") {
			t.Errorf("%s = %v, want only Documents/big.iso", cat, got[cat])
		}
	}
}

// TestPackageSuffixDirIsPruned checks the walk does not descend into a
// package directory at all (not just that matching files are filtered).
func TestPackageSuffixDirIsPruned(t *testing.T) {
	for _, name := range []string{"A.photoslibrary", "A.PhotosLibrary", "B.app", ".git", ".Hg"} {
		if !skipDir(name) {
			t.Errorf("skipDir(%q) = false, want true", name)
		}
	}
	// A name that is only a suffix (".app") is not a package.
	for _, name := range []string{"Documents", "photoslibrary", "app", ".app", ".photoslibrary", "git", "my.git.backup", "Movies"} {
		if skipDir(name) {
			t.Errorf("skipDir(%q) = true, want false", name)
		}
	}
}

// TestRootNamedLikePackageIsWalked: the scan root itself is always walked,
// even when its name ends in a package suffix.
func TestRootNamedLikePackageIsWalked(t *testing.T) {
	root := filepath.Join(t.TempDir(), "home.bundle")
	iso := filepath.Join(root, "Documents", "big.iso")
	writeFile(t, iso, 101*1024*1024)
	got := entryPaths(t, root)
	if len(got["home-large-files"]) != 1 {
		t.Errorf("home-large-files = %v, want Documents/big.iso", got["home-large-files"])
	}
}

// TestSparseFileNotLarge: thresholds compare allocated bytes, so a file
// with a large logical length but almost nothing on disk is not "large".
func TestSparseFileNotLarge(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "Documents", "sparse.img")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 4096)); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(200 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	backdate(t, p, 400)
	if a := allocated(t, p); a >= defaultOldFileMinSize {
		t.Skipf("filesystem does not create sparse files (allocated %d)", a)
	}

	got := entryPaths(t, root)
	for cat, paths := range got {
		for _, rel := range paths {
			if strings.HasSuffix(rel, "sparse.img") {
				t.Errorf("%s: sparse file with ~0 allocated bytes was flagged", cat)
			}
		}
	}
}

// TestEntrySizeIsAllocated: the reported size is the allocated size.
func TestEntrySizeIsAllocated(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "Documents", "odd.bin")
	writeFile(t, p, 150*1024*1024+1) // not a multiple of the block size
	results, err := scanFrom(root)
	if err != nil {
		t.Fatal(err)
	}
	want := allocated(t, p)
	if len(results) == 0 || len(results[0].Entries) != 1 {
		t.Fatalf("expected one large file, got %+v", results)
	}
	if got := results[0].Entries[0].Size; got != want {
		t.Errorf("Size = %d, want allocated %d", got, want)
	}
}

// fakeInfo is a FileInfo without a *syscall.Stat_t.
type fakeInfo struct{ os.FileInfo }

func (fakeInfo) Size() int64 { return 1234 }
func (fakeInfo) Sys() any    { return nil }

func TestAllocatedSizeFallback(t *testing.T) {
	if got := allocatedSize(fakeInfo{}); got != 1234 {
		t.Errorf("allocatedSize without Stat_t = %d, want logical size 1234", got)
	}
}
