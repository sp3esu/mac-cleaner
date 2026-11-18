package appleftovers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// isolateAppRoots points the system-wide app search locations at empty
// temp dirs, so tests do not read the real /Applications.
func isolateAppRoots(t *testing.T) (appDir, supportRoot string) {
	t.Helper()
	appDir = t.TempDir()
	supportRoot = filepath.Join(t.TempDir(), "Library", "Application Support")
	if err := os.MkdirAll(supportRoot, 0755); err != nil {
		t.Fatal(err)
	}
	origDirs, origSupport := systemAppDirs, libraryAppSupportDir
	systemAppDirs = []string{appDir}
	libraryAppSupportDir = supportRoot
	t.Cleanup(func() {
		systemAppDirs, libraryAppSupportDir = origDirs, origSupport
	})
	return appDir, supportRoot
}

// fakePlistBuddy creates an executable stub so exec.LookPath succeeds.
func fakePlistBuddy(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "PlistBuddy")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

// fileRunner is a PlistBuddy stub whose "bundle ID" is the content of the
// Info.plist it is asked about. It records every plist it reads.
type fileRunner struct {
	read []string
	fail string // Info.plist path that makes the call fail
}

func (r *fileRunner) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	p := args[len(args)-1]
	r.read = append(r.read, p)
	if r.fail != "" && p == r.fail {
		return nil, errors.New("exit status 1")
	}
	return os.ReadFile(p) // #nosec G304 -- test-only, temp paths
}

// writeBundle creates bundle/Contents/Info.plist holding id.
func writeBundle(t *testing.T, bundle, id string) string {
	t.Helper()
	p := filepath.Join(bundle, "Contents", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(id+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

// writePref writes an old (200-day) preference plist for domain.
func writePref(t *testing.T, home, domain string) {
	t.Helper()
	p := filepath.Join(home, "Library", "Preferences", domain+".plist")
	writeFile(t, p, 100)
	old := time.Now().Add(-200 * 24 * time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
}

func flaggedDomains(t *testing.T, home string, runner CmdRunner) []string {
	t.Helper()
	cr := scanOrphanedPrefs(home, fakePlistBuddy(t), runner)
	if cr == nil {
		return nil
	}
	if len(cr.PermissionIssues) > 0 {
		t.Fatalf("unexpected issues: %+v", cr.PermissionIssues)
	}
	var got []string
	for _, e := range cr.Entries {
		got = append(got, e.Description)
	}
	sort.Strings(got)
	return got
}

func TestOrphanedPrefs_SystemDomainsNeverFlagged(t *testing.T) {
	isolateAppRoots(t)
	home := t.TempDir()
	system := []string{
		".GlobalPreferences", ".GlobalPreferences_m", "loginwindow", "pbs",
		"MobileMeAccounts", "systemgroup.com.apple.x", "org.cups.PrintingPrefs",
		"group.com.apple.notes", "ContextStoreAgent",
	}
	for _, d := range system {
		writePref(t, home, d)
	}
	writePref(t, home, "com.example.gone")

	r := &fileRunner{}
	got := flaggedDomains(t, home, r.run)
	if strings.Join(got, ",") != "com.example.gone" {
		t.Errorf("flagged = %v, want only com.example.gone", got)
	}
}

func TestOrphanedPrefs_NestedBundlesRecognised(t *testing.T) {
	appDir, _ := isolateAppRoots(t)
	home := t.TempDir()
	host := filepath.Join(appDir, "Docker.app")
	writeBundle(t, host, "com.docker.docker")
	writeBundle(t, filepath.Join(host, "Contents", "MacOS", "Docker Desktop.app"), "com.electron.dockerdesktop")
	writeBundle(t, filepath.Join(host, "Contents", "Library", "LoginItems", "Login.app"), "com.vendor.login")
	writeBundle(t, filepath.Join(host, "Contents", "Frameworks", "ZoomAutoUpdater.app"), "us.zoom.ZoomAutoUpdater")
	writeBundle(t, filepath.Join(host, "Contents", "Frameworks", "Kit.framework", "Versions", "A", "Helpers", "Helper.app"), "com.vendor.fwhelper")
	writeBundle(t, filepath.Join(host, "Contents", "PlugIns", "Share.appex"), "com.vendor.share")
	// An app in a vendor folder one level below an app dir.
	writeBundle(t, filepath.Join(appDir, "Vendor", "Tool.app"), "com.vendor.tool")
	// A nested .app with no Info.plist is not a readable bundle: skipped, not a failure.
	if err := os.MkdirAll(filepath.Join(host, "Contents", "MacOS", "Broken.app"), 0755); err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{
		"com.electron.dockerdesktop", "com.vendor.login", "us.zoom.ZoomAutoUpdater",
		"com.vendor.fwhelper", "com.vendor.share", "com.vendor.tool",
		"com.vendor.TOOL.helper", // matching is case-insensitive
		"com.example.gone",
	} {
		writePref(t, home, d)
	}

	r := &fileRunner{}
	got := flaggedDomains(t, home, r.run)
	if strings.Join(got, ",") != "com.example.gone" {
		t.Errorf("flagged = %v, want only com.example.gone", got)
	}
}

func TestOrphanedPrefs_NestedSearchIsBounded(t *testing.T) {
	appDir, _ := isolateAppRoots(t)
	home := t.TempDir()
	host := filepath.Join(appDir, "Host.app")
	writeBundle(t, host, "com.host")
	// Outside the searched Contents subdirectories.
	outside := writeBundle(t, filepath.Join(host, "Contents", "Resources", "Res.app"), "com.host.res")
	// Deeper than the depth bound below Contents/Frameworks.
	deep := writeBundle(t, filepath.Join(host, "Contents", "Frameworks", "a", "b", "c", "d", "e", "Deep.app"), "com.deep")
	writePref(t, home, "com.example.gone")

	r := &fileRunner{}
	flaggedDomains(t, home, r.run)
	for _, p := range r.read {
		if p == outside || p == deep {
			t.Errorf("read %s, which is outside the bounded nested search", p)
		}
	}
}

func TestOrphanedPrefs_ApplicationSupportApps(t *testing.T) {
	_, support := isolateAppRoots(t)
	home := t.TempDir()
	writeBundle(t, filepath.Join(support, "Microsoft", "MAU2.0", "Microsoft AutoUpdate.app"), "com.microsoft.autoupdate2")
	writeBundle(t, filepath.Join(support, "Microsoft", "MAU2.0", "Microsoft AutoUpdate.app", "Contents", "MacOS", "Microsoft Update Assistant.app"), "com.microsoft.autoupdate.fba")
	writeBundle(t, filepath.Join(support, "Vendor", "Agent.app"), "com.vendor.agent")
	for _, d := range []string{"com.microsoft.autoupdate2", "com.microsoft.autoupdate.fba", "com.vendor.agent", "com.example.gone"} {
		writePref(t, home, d)
	}

	r := &fileRunner{}
	got := flaggedDomains(t, home, r.run)
	if strings.Join(got, ",") != "com.example.gone" {
		t.Errorf("flagged = %v, want only com.example.gone", got)
	}
}

// TestOrphanedPrefs_SymlinkedAppsRecognised: apps installed as symlinks
// (nix-darwin, home-manager, cryptex links) count as installed, in every
// searched location. Links to files and dangling links are ignored.
func TestOrphanedPrefs_SymlinkedAppsRecognised(t *testing.T) {
	appDir, support := isolateAppRoots(t)
	home := t.TempDir()
	store := t.TempDir()
	link := func(target, name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	}
	bundle := func(name, id string) string {
		p := filepath.Join(store, name)
		writeBundle(t, p, id)
		return p
	}
	// Symlinked .app in a system app dir and in ~/Applications.
	link(bundle("Sys.app", "com.vendor.sys"), filepath.Join(appDir, "Sys.app"))
	link(bundle("User.app", "com.vendor.user"), filepath.Join(home, "Applications", "User.app"))
	// A symlinked vendor folder holding a real .app.
	vendor := filepath.Join(store, "VendorDir")
	writeBundle(t, filepath.Join(vendor, "Tool.app"), "com.vendor.tool")
	link(vendor, filepath.Join(appDir, "Vendor"))
	// A symlinked vendor folder under /Library/Application Support.
	agentDir := filepath.Join(store, "AgentDir")
	writeBundle(t, filepath.Join(agentDir, "Agent.app"), "com.vendor.agent")
	link(agentDir, filepath.Join(support, "Agent"))
	// A link to a regular file and a dangling link: not apps, no failure.
	plain := filepath.Join(store, "plain")
	writeFile(t, plain, 10)
	link(plain, filepath.Join(appDir, "File.app"))
	link(filepath.Join(store, "missing.app"), filepath.Join(appDir, "Dangling.app"))
	// A link cycle: the bounded depth stops the search.
	link(appDir, filepath.Join(appDir, "Loop"))

	for _, d := range []string{"com.vendor.sys", "com.vendor.user", "com.vendor.tool", "com.vendor.agent", "com.example.gone"} {
		writePref(t, home, d)
	}

	r := &fileRunner{}
	got := flaggedDomains(t, home, r.run)
	if strings.Join(got, ",") != "com.example.gone" {
		t.Errorf("flagged = %v (read %v), want only com.example.gone", got, r.read)
	}
}

func TestIsDirOrDirLink(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	file := filepath.Join(dir, "file")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, 10)
	for name, target := range map[string]string{"dirlink": sub, "filelink": file, "dangling": filepath.Join(dir, "nope")} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"sub": true, "dirlink": true, "file": false, "filelink": false, "dangling": false}
	for _, e := range entries {
		if got := isDirOrDirLink(filepath.Join(dir, e.Name()), e); got != want[e.Name()] {
			t.Errorf("isDirOrDirLink(%s) = %v, want %v", e.Name(), got, want[e.Name()])
		}
	}
}

func TestOrphanedPrefs_RecentlyModifiedNotFlagged(t *testing.T) {
	isolateAppRoots(t)
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "Library", "Preferences", "com.example.active.plist"), 100) // mtime now
	p := filepath.Join(home, "Library", "Preferences", "com.example.recent.plist")
	writeFile(t, p, 100)
	recent := time.Now().Add(-(recentPrefsWindow - 24*time.Hour))
	if err := os.Chtimes(p, recent, recent); err != nil {
		t.Fatal(err)
	}
	writePref(t, home, "com.example.gone")

	r := &fileRunner{}
	got := flaggedDomains(t, home, r.run)
	if strings.Join(got, ",") != "com.example.gone" {
		t.Errorf("flagged = %v, want only com.example.gone", got)
	}
}

func TestOrphanedPrefs_PlistBuddyFailureFailsClosed(t *testing.T) {
	appDir, _ := isolateAppRoots(t)
	home := t.TempDir()
	writeBundle(t, filepath.Join(appDir, "Good.app"), "com.good")
	bad := writeBundle(t, filepath.Join(appDir, "Bad.app"), "com.bad")
	writePref(t, home, "com.bad")
	writePref(t, home, "com.example.gone")

	for name, runner := range map[string]CmdRunner{
		"error": (&fileRunner{fail: bad}).run,
		"empty output": func(ctx context.Context, n string, args ...string) ([]byte, error) {
			if args[len(args)-1] == bad {
				return []byte("  \n"), nil
			}
			return []byte("com.good\n"), nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			cr := scanOrphanedPrefs(home, fakePlistBuddy(t), runner)
			if cr == nil {
				t.Fatal("expected a result carrying the issue")
			}
			if len(cr.Entries) != 0 || cr.TotalSize != 0 {
				t.Errorf("expected no entries, got %+v", cr.Entries)
			}
			if len(cr.PermissionIssues) != 1 || cr.PermissionIssues[0].Path != bad ||
				!strings.Contains(cr.PermissionIssues[0].Description, "installed") {
				t.Errorf("unexpected issues: %+v", cr.PermissionIssues)
			}
		})
	}
}

func TestIsSystemDomain(t *testing.T) {
	for _, d := range []string{"loginwindow", ".GlobalPreferences", "com.apple.finder", "group.com.apple.x", "systemgroup.foo", "org.cups.PrintingPrefs", "ORG.CUPS.printingprefs"} {
		if !isSystemDomain(d) {
			t.Errorf("isSystemDomain(%q) = false", d)
		}
	}
	for _, d := range []string{"com.example.app", "org.cups.other", "us.zoom.xos"} {
		if isSystemDomain(d) {
			t.Errorf("isSystemDomain(%q) = true", d)
		}
	}
}

// --- Old Downloads: directory age uses the newest descendant ---

func backdateAll(t *testing.T, root string, days int) {
	t.Helper()
	old := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var paths []string
	if err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err == nil {
			paths = append(paths, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Children first, so touching them cannot bump a parent afterwards.
	for i := len(paths) - 1; i >= 0; i-- {
		if err := os.Chtimes(paths[i], old, old); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScanOldDownloads_DirWithFreshNestedFileNotFlagged(t *testing.T) {
	home := t.TempDir()
	dl := filepath.Join(home, "Downloads")
	fresh := filepath.Join(dl, "project-export", "src", "main.go")
	writeFile(t, fresh, 4096)
	writeFile(t, filepath.Join(dl, "project-export", "README.md"), 4096)
	writeFile(t, filepath.Join(dl, "old-export", "a", "b.txt"), 4096)
	backdateAll(t, dl, 200)
	now := time.Now()
	if err := os.Chtimes(fresh, now, now); err != nil {
		t.Fatal(err)
	}

	cr := scanOldDownloads(home, 90*24*time.Hour)
	if cr == nil {
		t.Fatal("expected old-export to be flagged")
	}
	var got []string
	for _, e := range cr.Entries {
		got = append(got, e.Description)
	}
	if strings.Join(got, ",") != "old-export" {
		t.Errorf("flagged = %v, want only old-export", got)
	}
}

func TestHasRecentActivity(t *testing.T) {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	root := t.TempDir()
	tree := filepath.Join(root, "tree")
	writeFile(t, filepath.Join(tree, "a", "b", "c.txt"), 10)
	writeFile(t, filepath.Join(tree, "d.txt"), 10)
	backdateAll(t, tree, 200)

	if hasRecentActivity(tree, cutoff, 5, 100) {
		t.Error("fully old tree reported as recent")
	}
	// Depth bound reached before the whole tree was seen: fail safe.
	if !hasRecentActivity(tree, cutoff, 1, 100) {
		t.Error("tree deeper than maxDepth must count as recent")
	}
	// Entry cap reached: fail safe.
	if !hasRecentActivity(tree, cutoff, 5, 2) {
		t.Error("tree larger than maxEntries must count as recent")
	}
	// Missing root: fail safe.
	if !hasRecentActivity(filepath.Join(root, "missing"), cutoff, 5, 100) {
		t.Error("unreadable tree must count as recent")
	}
	// A fresh deep file stops the walk.
	now := time.Now()
	if err := os.Chtimes(filepath.Join(tree, "a", "b", "c.txt"), now, now); err != nil {
		t.Fatal(err)
	}
	if !hasRecentActivity(tree, cutoff, 5, 100) {
		t.Error("tree with a fresh nested file reported as old")
	}
}

func TestScanOldDownloads_SymlinkEntrySkipped(t *testing.T) {
	home := t.TempDir()
	dl := filepath.Join(home, "Downloads")
	target := filepath.Join(home, "elsewhere.bin")
	writeFile(t, target, 4096)
	if err := os.MkdirAll(dl, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dl, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	// Backdate the link itself (Go has no lutimes wrapper).
	stamp := time.Now().Add(-200 * 24 * time.Hour).Format("200601021504")
	if out, err := exec.CommandContext(context.Background(), "touch", "-h", "-t", stamp, link).CombinedOutput(); err != nil { // #nosec G204 -- test-only, fixed args
		t.Skipf("touch -h: %v: %s", err, out)
	}
	if cr := scanOldDownloads(home, 90*24*time.Hour); cr != nil {
		t.Errorf("symlink should count as 0 bytes and be skipped, got %+v", cr.Entries)
	}
}
