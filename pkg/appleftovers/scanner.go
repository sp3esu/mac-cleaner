// Package appleftovers provides scanners for orphaned app preferences,
// iOS device backups, and old Downloads files on macOS.
package appleftovers

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CmdRunner executes an external command and returns its combined stdout output.
// It is used for dependency injection so PlistBuddy calls can be mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- all command names and arguments are hardcoded string literals, no user input
	return cmd.Output()
}

// recentPrefsWindow: a preference file modified this recently belongs to
// software that is still running, so it is never reported as orphaned.
const recentPrefsWindow = 90 * 24 * time.Hour

// nestedBundleMaxDepth bounds how many directory levels below each of an
// app's Contents/{MacOS,Library,Frameworks,PlugIns} are searched for
// helper bundles (Frameworks/X.framework/Versions/A/Helpers/Y.app is 5).
const nestedBundleMaxDepth = 5

// Bounds for judging a Downloads directory's age by its newest descendant.
// A directory that cannot be fully checked within them is treated as recent.
const (
	downloadsMaxDepth   = 16
	downloadsMaxEntries = 50000
)

// systemAppDirs are the system-wide directories searched for installed
// apps (plus one vendor subfolder level below each). ~/Applications is
// added per scan. Package variables so tests can isolate them.
var (
	systemAppDirs = []string{
		"/Applications",
		"/Applications/Utilities",
		"/System/Applications",
		"/System/Applications/Utilities",
	}
	// libraryAppSupportDir holds apps installed by vendors outside the app
	// folders, e.g. Microsoft/MAU2.0/Microsoft AutoUpdate.app.
	libraryAppSupportDir = "/Library/Application Support"
)

// nestedBundleDirs are the Contents subdirectories searched for helper,
// login-item and extension bundles.
var nestedBundleDirs = []string{"MacOS", "Library", "Frameworks", "PlugIns"}

// systemPrefDomains lists macOS-owned preference domains (lowercase) that
// carry neither a com.apple. nor a systemgroup. prefix.
var systemPrefDomains = map[string]bool{
	"org.cups.printingprefs": true, // CUPS printing preferences
}

// Scan discovers orphaned app preferences, iOS device backups, and old
// Downloads files. Missing directories are silently skipped. No files are
// modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanOrphanedPrefs(home, "/usr/libexec/PlistBuddy", defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanIOSBackups(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanOldDownloads(home, 90*24*time.Hour); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanOrphanedPrefs finds preference .plist files in ~/Library/Preferences
// that do not match any installed application's bundle ID. macOS system
// domains (see isSystemDomain) and files modified within recentPrefsWindow
// are always skipped. Installed IDs come from top-level apps, their nested
// helper bundles, and apps under /Library/Application Support. If any
// bundle ID cannot be read, no entries are reported (fail closed) and a
// PermissionIssue explains why. Returns nil if PlistBuddy is not found or
// the Preferences directory does not exist.
func scanOrphanedPrefs(home, plistBuddyPath string, runner CmdRunner) *scan.CategoryResult {
	// Guard: PlistBuddy must exist.
	if _, err := exec.LookPath(plistBuddyPath); err != nil {
		return nil
	}

	prefsDir := filepath.Join(home, "Library", "Preferences")
	if _, err := os.Stat(prefsDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-orphaned-prefs",
				Description: "Orphaned Preferences",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        prefsDir,
					Description: "Preferences directory (permission denied)",
				}},
			}
		}
		return nil
	}

	installedIDs, failedPlist := installedBundleIDs(home, plistBuddyPath, runner)
	if failedPlist != "" {
		return &scan.CategoryResult{
			Category:    "app-orphaned-prefs",
			Description: "Orphaned Preferences",
			PermissionIssues: []scan.PermissionIssue{{
				Path:        failedPlist,
				Description: "bundle ID unreadable, so the installed-app set could not be built; orphaned preferences not reported",
			}},
		}
	}

	// Read preference files and find orphans.
	prefEntries, err := os.ReadDir(prefsDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-orphaned-prefs",
				Description: "Orphaned Preferences",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        prefsDir,
					Description: "Preferences directory (permission denied)",
				}},
			}
		}
		return nil
	}

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64
	now := time.Now()

	for _, entry := range prefEntries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".plist") {
			continue
		}

		domain := strings.TrimSuffix(name, ".plist")

		// Never flag macOS system domains as orphaned.
		if isSystemDomain(domain) {
			continue
		}

		// Check if any installed bundle ID matches this domain.
		if isMatchedByInstalledApp(domain, installedIDs) {
			continue
		}

		path := filepath.Join(prefsDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        path,
					Description: domain + " (permission denied)",
				})
			}
			continue
		}

		// Recently written: something installed still uses it.
		if now.Sub(info.ModTime()) < recentPrefsWindow {
			continue
		}

		size, err := scan.DirSize(path)
		if err != nil || size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        path,
			Description: domain,
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "app-orphaned-prefs",
		Description:      "Orphaned Preferences",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// isSystemDomain reports whether a preference domain belongs to macOS:
// it is not reverse-DNS (no dot, e.g. loginwindow, pbs, MobileMeAccounts),
// is a dot-file (.GlobalPreferences), contains com.apple., starts with
// systemgroup., or is listed in systemPrefDomains.
func isSystemDomain(domain string) bool {
	d := strings.ToLower(domain)
	return !strings.Contains(d, ".") ||
		strings.HasPrefix(d, ".") ||
		strings.Contains(d, "com.apple.") ||
		strings.HasPrefix(d, "systemgroup.") ||
		systemPrefDomains[d]
}

// installedBundleIDs returns the lowercase bundle IDs of all installed apps
// and their nested helper bundles. If a bundle's Info.plist exists but its
// CFBundleIdentifier cannot be read, it stops and returns that plist path.
func installedBundleIDs(home, plistBuddyPath string, runner CmdRunner) (map[string]bool, string) {
	ids := make(map[string]bool)
	for _, app := range findAppBundles(home) {
		for _, plist := range bundlePlists(app) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			out, err := runner(ctx, plistBuddyPath, "-c", "Print :CFBundleIdentifier", plist)
			cancel()

			bundleID := strings.TrimSpace(string(out))
			if err != nil || bundleID == "" {
				return nil, plist
			}
			ids[strings.ToLower(bundleID)] = true
		}
	}
	return ids, ""
}

// findAppBundles lists .app bundles in the app directories (and one
// vendor subfolder below each), and one or two levels below
// /Library/Application Support.
func findAppBundles(home string) []string {
	var apps []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			apps = append(apps, p)
		}
	}
	// appsIn adds the .app bundles in dir, descending depth more levels
	// into plain (non-.app) subdirectories.
	var appsIn func(dir string, depth int)
	appsIn = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if !isDirOrDirLink(p, e) {
				continue
			}
			if strings.HasSuffix(e.Name(), ".app") {
				add(p)
			} else if depth > 0 {
				appsIn(p, depth-1)
			}
		}
	}
	appDirs := append(append([]string(nil), systemAppDirs...), filepath.Join(home, "Applications"))
	for _, dir := range appDirs {
		appsIn(dir, 1)
	}
	// /Library/Application Support/*/*.app and */*/*.app.
	if entries, err := os.ReadDir(libraryAppSupportDir); err == nil {
		for _, e := range entries {
			p := filepath.Join(libraryAppSupportDir, e.Name())
			if isDirOrDirLink(p, e) {
				appsIn(p, 1)
			}
		}
	}
	return apps
}

// isDirOrDirLink reports whether e (found at p) is a directory or a symlink
// to one. Apps are often installed as symlinks (nix-darwin, home-manager,
// Apple's cryptex links such as /Applications/Safari.app), and must still
// count as installed. The search depth is bounded, so link cycles end.
func isDirOrDirLink(p string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// bundlePlists returns the Info.plist of app and of every .app or .appex
// nested in its Contents/{MacOS,Library,Frameworks,PlugIns}, up to
// nestedBundleMaxDepth levels down. Bundles without an Info.plist are
// skipped.
func bundlePlists(app string) []string {
	var plists []string
	addPlist := func(bundle string) {
		p := filepath.Join(bundle, "Contents", "Info.plist")
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			plists = append(plists, p)
		}
	}
	addPlist(app)
	for _, sub := range nestedBundleDirs {
		root := filepath.Join(app, "Contents", sub)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if rel != "." && strings.Count(rel, string(filepath.Separator))+1 > nestedBundleMaxDepth {
				return fs.SkipDir
			}
			if rel != "." && (strings.HasSuffix(d.Name(), ".app") || strings.HasSuffix(d.Name(), ".appex")) {
				addPlist(path)
			}
			return nil
		})
	}
	return plists
}

// isMatchedByInstalledApp checks if a preference domain matches any installed
// bundle ID (lowercase). A match occurs when the domain equals a bundle ID or
// starts with a bundle ID followed by a dot, compared case-insensitively.
func isMatchedByInstalledApp(domain string, installedIDs map[string]bool) bool {
	domain = strings.ToLower(domain)
	for id := range installedIDs {
		if domain == id || strings.HasPrefix(domain, id+".") {
			return true
		}
	}
	return false
}

// scanIOSBackups scans ~/Library/Application Support/MobileSync/Backup for
// iOS device backups. Returns nil if the directory does not exist or has no
// entries.
func scanIOSBackups(home string) *scan.CategoryResult {
	backupDir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")

	if _, err := os.Stat(backupDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-ios-backups",
				Description: "iOS Device Backups",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        backupDir,
					Description: "iOS backups (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(backupDir, "app-ios-backups", "iOS Device Backups")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanOldDownloads scans ~/Downloads for files and directories older than
// maxAge based on modification time. A directory counts as old only if
// nothing inside it was modified within maxAge (see hasRecentActivity).
// Returns nil if the directory does not exist or no old entries are found.
func scanOldDownloads(home string, maxAge time.Duration) *scan.CategoryResult {
	downloadsDir := filepath.Join(home, "Downloads")

	if _, err := os.Stat(downloadsDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-old-downloads",
				Description: "Old Downloads (90+ days)",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        downloadsDir,
					Description: "Downloads directory (permission denied)",
				}},
			}
		}
		return nil
	}

	dirEntries, err := os.ReadDir(downloadsDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "app-old-downloads",
				Description: "Old Downloads (90+ days)",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        downloadsDir,
					Description: "Downloads directory (permission denied)",
				}},
			}
		}
		return nil
	}

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64
	cutoff := time.Now().Add(-maxAge)

	for _, entry := range dirEntries {
		info, err := entry.Info()
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        filepath.Join(downloadsDir, entry.Name()),
					Description: entry.Name() + " (permission denied)",
				})
			}
			continue
		}

		if time.Since(info.ModTime()) <= maxAge {
			continue
		}

		entryPath := filepath.Join(downloadsDir, entry.Name())

		if entry.IsDir() && hasRecentActivity(entryPath, cutoff, downloadsMaxDepth, downloadsMaxEntries) {
			continue
		}

		size, err := scan.DirSize(entryPath)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        entryPath,
					Description: entry.Name() + " (permission denied)",
				})
			}
			continue
		}

		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        entryPath,
			Description: entry.Name(),
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "app-old-downloads",
		Description:      "Old Downloads (90+ days)",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// hasRecentActivity reports whether root or anything below it was modified
// after cutoff. It stops at the first such entry. It fails safe: if the
// walk hits an error, goes deeper than maxDepth levels, or visits more than
// maxEntries entries, the tree could not be fully checked and it reports
// true.
func hasRecentActivity(root string, cutoff time.Time, maxDepth, maxEntries int) bool {
	recent := false
	visited := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			recent = true
			return fs.SkipAll
		}
		visited++
		if visited > maxEntries {
			recent = true
			return fs.SkipAll
		}
		if rel, _ := filepath.Rel(root, path); rel != "." && strings.Count(rel, string(filepath.Separator))+1 > maxDepth {
			recent = true
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil || info.ModTime().After(cutoff) {
			recent = true
			return fs.SkipAll
		}
		return nil
	})
	return recent
}
