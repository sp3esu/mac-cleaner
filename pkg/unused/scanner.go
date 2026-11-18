// Package unused provides a scanner that identifies macOS applications
// not opened in a configurable time period, along with their total disk
// footprint (bundle + ~/Library/ data).
package unused

import (
	"context"
	"encoding/json"
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
// It is used for dependency injection so mdls and PlistBuddy calls can be
// mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- all command names and arguments are hardcoded string literals, no user input
	return cmd.Output()
}

// defaultThreshold is the minimum time since last use for an app to be
// considered unused.
const defaultThreshold = 180 * 24 * time.Hour

// appleBundleIDPrefix identifies Apple-provided applications by their
// bundle identifier. These are skipped because they live in /Applications
// (blocked by the safety system) and require system-level procedures to remove.
const appleBundleIDPrefix = "com.apple."

// mdlsDateLayout is the time layout returned by mdls -raw for date
// attributes such as kMDItemLastUsedDate and kMDItemDateAdded.
const mdlsDateLayout = "2006-01-02 15:04:05 +0000"

// systemLaunchdDirs are the system-wide launchd job directories checked for
// jobs that belong to an app, in addition to ~/Library/LaunchAgents. It is a
// variable so tests can point it at temporary directories.
var systemLaunchdDirs = []string{"/Library/LaunchAgents", "/Library/LaunchDaemons"}

// Limits for the newest-mtime walk over an app's ~/Library data: entries
// deeper than activityWalkDepth below a root are not visited, and a walk
// stops after activityWalkEntries entries.
const (
	activityWalkDepth   = 4
	activityWalkEntries = 5000
)

// Scan discovers applications not opened in 180+ days and returns their
// total disk footprint (bundle + ~/Library/ data). Missing directories
// are silently skipped. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanUnusedApps(home, defaultThreshold, defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanUnusedApps scans application directories for .app bundles that have
// not been opened within the given threshold. Each entry includes the total
// footprint: bundle size + associated ~/Library/ directories.
//
// An app is not reported when any of these holds: Spotlight's last-used
// date is within the threshold; it is an Apple app; it hosts app
// extensions, QuickLook/Spotlight plug-ins or system extensions; its
// ~/Library data was modified within the threshold; it has no last-used
// date but was added within the threshold; one of its executables is
// running; or a launchd job runs a program inside it or is associated
// with its bundle ID. If the process list cannot be read, no app is
// reported and the failure is recorded as a permission issue.
func scanUnusedApps(home string, threshold time.Duration, runner CmdRunner) *scan.CategoryResult {
	appDirs := []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}

	cutoff := time.Now().Add(-threshold)
	plistBuddyPath := "/usr/libexec/PlistBuddy"

	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	// Running processes and launchd jobs are read once per scan, when the
	// first candidate app reaches those checks.
	var (
		usageLoaded bool
		procs       []string
		procsErr    error
		jobs        []launchdJob
	)

	for _, appDir := range appDirs {
		dirEntries, err := os.ReadDir(appDir)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        appDir,
					Description: appDir + " (permission denied)",
				})
			}
			continue
		}

		for _, entry := range dirEntries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}

			appPath := filepath.Join(appDir, entry.Name())

			// Query last-used date via Spotlight metadata.
			lastUsed, err := queryLastUsedDate(appPath, runner)
			if err != nil {
				// mdls failure: skip this app silently.
				continue
			}

			// Skip recently used apps.
			if lastUsed != nil && lastUsed.After(cutoff) {
				continue
			}

			// Extract bundle ID for Library footprint calculation.
			bundleID := extractBundleID(appPath, plistBuddyPath, runner)

			// Skip Apple-provided apps — they are blocked from deletion by
			// the safety system and require system-level removal procedures.
			if isAppleApp(bundleID) {
				continue
			}

			appName := strings.TrimSuffix(entry.Name(), ".app")

			// Apps that host extensions are used without being opened.
			if isExtensionHost(appPath) {
				continue
			}

			// Secondary check: skip if Library data was recently modified.
			if latestMod := libraryLastModified(home, bundleID, appName, cutoff); latestMod.After(cutoff) {
				continue
			}

			// Never opened: fall back to when the app was added.
			if lastUsed == nil {
				if added := queryDateAdded(appPath, runner); added != nil && added.After(cutoff) {
					continue
				}
			}

			if !usageLoaded {
				usageLoaded = true
				procs, procsErr = runningExecutables(runner)
				if procsErr != nil {
					permIssues = append(permIssues, scan.PermissionIssue{
						Path:        "ps",
						Description: "running processes could not be listed, so no app is reported as unused (" + procsErr.Error() + ")",
					})
				} else {
					jobs = loadLaunchdJobs(launchdDirs(home), runner)
				}
			}
			// Fail closed: without a process list, any app may be running.
			if procsErr != nil {
				continue
			}
			if isRunning(appPath, procs) || hasLaunchdJob(appPath, bundleID, jobs) {
				continue
			}

			// Calculate total footprint.
			bundleSize, err := scan.DirSize(appPath)
			if err != nil {
				if os.IsPermission(err) {
					permIssues = append(permIssues, scan.PermissionIssue{
						Path:        appPath,
						Description: appName + " (permission denied)",
					})
				}
				continue
			}

			components := append([]scan.SizeComponent{bundleComponent(appPath, bundleSize)},
				libraryFootprint(home, bundleID, appName)...)
			var size int64
			for _, c := range components {
				size += c.Size
			}

			if size == 0 {
				continue
			}

			desc := formatDescription(appName, lastUsed)

			entries = append(entries, scan.ScanEntry{
				Path:        appPath,
				Description: desc,
				Size:        size,
				Components:  components,
			})
			totalSize += size
		}
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	// Sort by size descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:         "unused-apps",
		Description:      "Unused Applications (180+ days)",
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}

// queryLastUsedDate queries macOS Spotlight for the last-used date of an app.
// Returns nil time pointer for apps that have never been opened.
// Returns an error if mdls fails.
func queryLastUsedDate(appPath string, runner CmdRunner) (*time.Time, error) {
	return queryMdlsDate(appPath, "kMDItemLastUsedDate", runner)
}

// queryDateAdded queries Spotlight for the date an app was added to its
// folder. Returns nil when the date is unknown or mdls fails.
func queryDateAdded(appPath string, runner CmdRunner) *time.Time {
	t, err := queryMdlsDate(appPath, "kMDItemDateAdded", runner)
	if err != nil {
		return nil
	}
	return t
}

// queryMdlsDate reads a date attribute of appPath with mdls. Returns nil
// time pointer when the attribute is "(null)" or empty, and an error if
// mdls fails or the date cannot be parsed.
func queryMdlsDate(appPath, attr string, runner CmdRunner) (*time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, "mdls", "-name", attr, "-raw", appPath)
	if err != nil {
		return nil, fmt.Errorf("mdls failed for %s: %w", appPath, err)
	}

	raw := strings.TrimSpace(string(out))

	// "(null)" means the attribute is not set (e.g. the app was never opened).
	if raw == "(null)" || raw == "" {
		return nil, nil
	}

	t, err := time.Parse(mdlsDateLayout, raw)
	if err != nil {
		return nil, fmt.Errorf("cannot parse date %q: %w", raw, err)
	}

	return &t, nil
}

// extractBundleID reads CFBundleIdentifier from an app's Info.plist.
// Returns empty string on any error.
func extractBundleID(appPath, plistBuddyPath string, runner CmdRunner) string {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, plistBuddyPath, "-c", "Print :CFBundleIdentifier", plistPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// libraryNote labels ~/Library footprint components. They are counted in
// the entry size, but cleanup removes only the app bundle.
const libraryNote = "app data in ~/Library"

// bundleComponent describes the .app bundle's share of an entry's size and
// whether cleanup can delete it (bundles outside the home directory, such
// as /Applications, are blocked by the safety checks at cleanup time).
func bundleComponent(appPath string, size int64) scan.SizeComponent {
	c := scan.SizeComponent{Path: appPath, Size: size, Removed: true, Note: "app bundle"}
	if blocked, reason := safety.IsPathBlocked(appPath); blocked {
		c.Removed = false
		c.Note = "app bundle; cleanup is blocked by safety check: " + reason
	}
	return c
}

// libraryFootprint returns the non-empty ~/Library/ paths associated with
// an app, each with its size. Paths are probed by both bundleID and appName,
// reported with the case they have on disk, and counted once when two
// probes name the same file or directory (APFS is case-insensitive).
// None of them are removed by cleanup.
func libraryFootprint(home, bundleID, appName string) []scan.SizeComponent {
	var components []scan.SizeComponent
	var seen []os.FileInfo
	add := func(p string) {
		p = onDiskCase(p)
		info, err := os.Lstat(p)
		if err != nil {
			return
		}
		for _, s := range seen {
			if os.SameFile(s, info) {
				return
			}
		}
		if size := pathSize(p); size > 0 {
			seen = append(seen, info)
			components = append(components, scan.SizeComponent{Path: p, Size: size, Note: libraryNote})
		}
	}

	// Direct paths to probe by bundleID.
	if bundleID != "" {
		directPaths := []string{
			filepath.Join(home, "Library", "Application Support", bundleID),
			filepath.Join(home, "Library", "Caches", bundleID),
			filepath.Join(home, "Library", "Containers", bundleID),
			filepath.Join(home, "Library", "Saved Application State", bundleID+".savedState"),
			filepath.Join(home, "Library", "HTTPStorages", bundleID),
			filepath.Join(home, "Library", "WebKit", bundleID),
			filepath.Join(home, "Library", "Logs", bundleID),
			filepath.Join(home, "Library", "Preferences", bundleID+".plist"),
			filepath.Join(home, "Library", "Cookies", bundleID+".binarycookies"),
		}

		for _, p := range directPaths {
			add(p)
		}

		// Glob patterns for bundleID.
		globPatterns := []string{
			filepath.Join(home, "Library", "Group Containers", "*"+bundleID+"*"),
			filepath.Join(home, "Library", "Preferences", "ByHost", bundleID+".*.plist"),
			filepath.Join(home, "Library", "LaunchAgents", bundleID+"*.plist"),
		}

		for _, pattern := range globPatterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, m := range matches {
				add(m)
			}
		}
	}

	// Probe by appName (Application Support and Logs).
	if appName != "" {
		namePaths := []string{
			filepath.Join(home, "Library", "Application Support", appName),
			filepath.Join(home, "Library", "Logs", appName),
		}
		for _, p := range namePaths {
			// Avoid double-counting if bundleID == appName.
			if bundleID != "" && bundleID == appName {
				continue
			}
			add(p)
		}
	}

	return components
}

// libraryLastModified returns the most recent modification time across an
// app's ~/Library/ data: its data directories (Application Support, Caches,
// Containers and Containers/<id>/Data, Saved Application State,
// HTTPStorages, WebKit, Logs, and Group Containers named <id> or
// <prefix>.<id>) and its preference files (Preferences/<id>.plist and
// Preferences/ByHost/<id>.*.plist). Directories are walked for their newest
// entry, at most activityWalkDepth levels deep and activityWalkEntries
// entries each, so files written deep inside an old directory count. It
// returns as soon as a time after cutoff is found, and zero time if no
// paths exist.
func libraryLastModified(home, bundleID, appName string, cutoff time.Time) time.Time {
	var paths []string

	if bundleID != "" {
		lib := filepath.Join(home, "Library")
		paths = append(paths,
			filepath.Join(lib, "Preferences", bundleID+".plist"),
			filepath.Join(lib, "Application Support", bundleID),
			filepath.Join(lib, "Caches", bundleID),
			filepath.Join(lib, "Containers", bundleID),
			filepath.Join(lib, "Containers", bundleID, "Data"),
			filepath.Join(lib, "Saved Application State", bundleID+".savedState"),
			filepath.Join(lib, "HTTPStorages", bundleID),
			filepath.Join(lib, "WebKit", bundleID),
			filepath.Join(lib, "Logs", bundleID),
		)
		if byHost, err := filepath.Glob(filepath.Join(lib, "Preferences", "ByHost", bundleID+".*.plist")); err == nil {
			paths = append(paths, byHost...)
		}
		groupDir := filepath.Join(lib, "Group Containers")
		if groups, err := os.ReadDir(groupDir); err == nil {
			for _, g := range groups {
				if g.Name() == bundleID || strings.HasSuffix(g.Name(), "."+bundleID) {
					paths = append(paths, filepath.Join(groupDir, g.Name()))
				}
			}
		}
	}

	if appName != "" && appName != bundleID {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", appName),
			filepath.Join(home, "Library", "Logs", appName),
		)
	}

	var latest time.Time
	for _, p := range paths {
		if mod := newestModTime(p, cutoff, activityWalkDepth, activityWalkEntries); mod.After(latest) {
			latest = mod
		}
		if latest.After(cutoff) {
			break
		}
	}
	return latest
}

// newestModTime returns the newest modification time of root and the
// entries below it, visiting at most maxDepth levels below root and at most
// maxEntries entries. Symlinks are not followed. It stops as soon as a time
// after cutoff is found. Returns zero time if root does not exist.
func newestModTime(root string, cutoff time.Time, maxDepth, maxEntries int) time.Time {
	var newest time.Time
	visited := 0
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Unreadable entries carry no usable time; keep walking.
			return nil
		}
		visited++
		if visited > maxEntries {
			return fs.SkipAll
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		if newest.After(cutoff) {
			return fs.SkipAll
		}
		if d.IsDir() && p != root {
			if rel, err := filepath.Rel(root, p); err == nil && strings.Count(rel, string(filepath.Separator))+1 >= maxDepth {
				return fs.SkipDir
			}
		}
		return nil
	})
	return newest
}

// isExtensionHost reports whether an app bundle carries app extensions
// (Contents/PlugIns/*.appex), QuickLook or Spotlight plug-ins, or system
// extensions. Such apps do their work without being opened, so Spotlight's
// last-used date says nothing about whether they are in use.
func isExtensionHost(appPath string) bool {
	if appex, err := filepath.Glob(filepath.Join(appPath, "Contents", "PlugIns", "*.appex")); err == nil && len(appex) > 0 {
		return true
	}
	for _, dir := range []string{"QuickLook", "Spotlight", "SystemExtensions"} {
		if _, err := os.Stat(filepath.Join(appPath, "Contents", "Library", dir)); err == nil {
			return true
		}
	}
	return false
}

// runningExecutables returns the executable path of every running process,
// read once with `ps -axo comm=`. An empty list is treated as a failure.
func runningExecutables(runner CmdRunner) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, "ps", "-axo", "comm=")
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}
	var procs []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			procs = append(procs, line)
		}
	}
	if len(procs) == 0 {
		return nil, fmt.Errorf("ps returned no processes")
	}
	return procs, nil
}

// insideBundle reports whether path lies inside the app bundle at appPath.
// The comparison ignores case, as APFS does by default.
func insideBundle(path, appPath string) bool {
	prefix := appPath + "/"
	return len(path) > len(prefix) && strings.EqualFold(path[:len(prefix)], prefix)
}

// isRunning reports whether any running executable lies inside appPath.
func isRunning(appPath string, procs []string) bool {
	for _, p := range procs {
		if insideBundle(p, appPath) {
			return true
		}
	}
	return false
}

// launchdJob holds the fields of a launchd job plist that tie it to an app.
type launchdJob struct {
	// programs are Program, ProgramArguments[0] and BundleProgram.
	programs []string
	// bundleIDs are the AssociatedBundleIdentifiers.
	bundleIDs []string
}

// launchdDirs returns the launchd job directories checked for an app's jobs.
func launchdDirs(home string) []string {
	return append([]string{filepath.Join(home, "Library", "LaunchAgents")}, systemLaunchdDirs...)
}

// loadLaunchdJobs reads every *.plist in dirs. Plists that cannot be
// converted or parsed are skipped.
func loadLaunchdJobs(dirs []string, runner CmdRunner) []launchdJob {
	var jobs []launchdJob
	for _, dir := range dirs {
		plists, err := filepath.Glob(filepath.Join(dir, "*.plist"))
		if err != nil {
			continue
		}
		for _, p := range plists {
			if job, ok := readLaunchdJob(p, runner); ok {
				jobs = append(jobs, job)
			}
		}
	}
	return jobs
}

// readLaunchdJob converts a launchd plist to JSON with plutil and extracts
// the fields that tie it to an app. AssociatedBundleIdentifiers may be a
// string or an array of strings.
func readLaunchdJob(path string, runner CmdRunner) (launchdJob, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := runner(ctx, "plutil", "-convert", "json", "-o", "-", path)
	if err != nil {
		return launchdJob{}, false
	}
	var raw struct {
		Program                     string          `json:"Program"`
		ProgramArguments            []string        `json:"ProgramArguments"`
		BundleProgram               string          `json:"BundleProgram"`
		AssociatedBundleIdentifiers json.RawMessage `json:"AssociatedBundleIdentifiers"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return launchdJob{}, false
	}

	var job launchdJob
	for _, p := range []string{raw.Program, raw.BundleProgram} {
		if p != "" {
			job.programs = append(job.programs, p)
		}
	}
	if len(raw.ProgramArguments) > 0 && raw.ProgramArguments[0] != "" {
		job.programs = append(job.programs, raw.ProgramArguments[0])
	}
	if len(raw.AssociatedBundleIdentifiers) > 0 {
		var ids []string
		var id string
		if json.Unmarshal(raw.AssociatedBundleIdentifiers, &ids) == nil {
			job.bundleIDs = ids
		} else if json.Unmarshal(raw.AssociatedBundleIdentifiers, &id) == nil && id != "" {
			job.bundleIDs = []string{id}
		}
	}
	return job, true
}

// hasLaunchdJob reports whether a launchd job runs a program inside appPath
// or is associated with bundleID.
func hasLaunchdJob(appPath, bundleID string, jobs []launchdJob) bool {
	for _, j := range jobs {
		for _, p := range j.programs {
			if insideBundle(p, appPath) {
				return true
			}
		}
		if bundleID == "" {
			continue
		}
		for _, id := range j.bundleIDs {
			if strings.EqualFold(id, bundleID) {
				return true
			}
		}
	}
	return false
}

// onDiskCase returns p with its final element spelled as it is on disk.
// On a case-insensitive volume, os.Stat accepts any case, so paths built
// from an app name (e.g. "Application Support/Discord") may not match the
// directory's real name ("discord"). p is returned unchanged when it exists
// as spelled, when no entry matches, or when the parent cannot be read.
func onDiskCase(p string) string {
	dir, base := filepath.Dir(p), filepath.Base(p)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return p
	}
	match := ""
	for _, e := range entries {
		if e.Name() == base {
			return p
		}
		if match == "" && strings.EqualFold(e.Name(), base) {
			match = e.Name()
		}
	}
	if match == "" {
		return p
	}
	return filepath.Join(dir, match)
}

// pathSize returns the size of a file or directory as reported by
// scan.DirSize. Returns 0 if the path does not exist or cannot be read.
func pathSize(path string) int64 {
	size, err := scan.DirSize(path)
	if err != nil {
		return 0
	}
	return size
}

// isAppleApp reports whether the given bundle identifier belongs to an
// Apple-provided application. An empty bundle ID returns false so that
// apps with unreadable plists are not silently skipped.
func isAppleApp(bundleID string) bool {
	return bundleID != "" && strings.HasPrefix(bundleID, appleBundleIDPrefix)
}

// formatDescription formats the app name with its last-used date.
func formatDescription(appName string, lastUsed *time.Time) string {
	if lastUsed == nil {
		return appName + " (no usage history)"
	}
	return appName + " (last used " + lastUsed.Format("Jan 2006") + ")"
}
