# Phase 4: App Leftovers & Cleanup Execution - Research

**Researched:** 2026-02-16
**Domain:** macOS orphaned preference detection, iOS backup scanning, Downloads age filtering, file deletion with confirmation prompts
**Confidence:** HIGH

## Summary

Phase 4 introduces two major capabilities: (1) a new scanning category for "app leftovers" (orphaned preferences, old iOS backups, old Downloads files) and (2) the first actual file deletion capability with a confirmation prompt. The scanning side follows established patterns from Phases 2-3: a new `pkg/appleftovers/scanner.go` package exports `Scan() ([]scan.CategoryResult, error)`, wired to a `--app-leftovers` flag in `cmd/root.go`. The deletion side is entirely new and requires careful design -- a confirmation module in `internal/confirm/` that reads stdin, displays what will be removed, and gates all `os.RemoveAll`/`os.Remove` calls behind explicit "yes" input.

The three scanning targets have distinct characteristics. **Orphaned preferences** (APP-01) requires detecting which plist files in `~/Library/Preferences/` belong to apps no longer installed. The recommended approach is to build a set of known bundle IDs by reading `Info.plist` from standard app directories (`/Applications`, `/System/Applications`, etc.) using Go's `os/exec` to call `/usr/libexec/PlistBuddy`, then checking each plist filename against this set. Individual plist files are tiny (most under 100KB, total ~14MB), so the space savings are modest but the cleanup value is organizational. **iOS device backups** (APP-02) are found at `~/Library/Application Support/MobileSync/Backup/` as UUID-named subdirectories, each potentially multi-gigabyte. This is a straightforward `ScanTopLevel`-compatible target. **Old Downloads files** (APP-03) filters `~/Downloads/` entries by modification time against a configurable age threshold (default: 90 days), using `os.FileInfo.ModTime()` with `time.Since()`.

The confirmation prompt (CLI-04) is the critical safety gate. It must display a summary of what will be removed (paths + sizes), then require the user to type "yes" (not just "y") to proceed. This is implemented with `bufio.NewReader(os.Stdin)` and must be testable via dependency injection (pass an `io.Reader` instead of hardcoding `os.Stdin`). The existing `--dry-run` flag already prevents deletion; the new flow is: scan -> display results -> if not dry-run, show confirmation -> if "yes", delete -> show summary.

**Primary recommendation:** Build `pkg/appleftovers/scanner.go` with three sub-scanners following established patterns. Build `internal/confirm/confirm.go` for the confirmation prompt with `io.Reader` injection for testability. Build `internal/cleanup/cleanup.go` for the deletion logic that takes `[]scan.CategoryResult` and removes files, returning a summary. Wire everything through `cmd/root.go` with the new `--app-leftovers` flag and a `runCleanup()` function that orchestrates scan -> confirm -> delete -> summarize.

## Standard Stack

### Core (Phase 4 additions -- no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os (stdlib) | -- | `Remove`, `RemoveAll` for file deletion; `ReadDir`, `Stat` for scanning | Already used in Phases 1-3 |
| os/exec (stdlib) | -- | Execute `/usr/libexec/PlistBuddy` for reading bundle IDs from Info.plist | Same pattern as Docker CLI in Phase 3 |
| bufio (stdlib) | -- | `bufio.NewReader` for reading stdin confirmation input | Standard Go approach for line-oriented input |
| io (stdlib) | -- | `io.Reader` interface for confirmation prompt testability | Dependency injection for stdin |
| time (stdlib) | -- | `time.Since()` for Downloads file age comparison | Standard Go time operations |
| path/filepath (stdlib) | -- | Path construction for preference/backup directories | Already used in Phases 1-3 |
| strings (stdlib) | -- | Plist filename parsing, bundle ID matching | Already used in Phases 1-3 |

### Not Needed

| Library | Why Not |
|---------|---------|
| howett.net/plist (Go plist parser) | We only need plist filenames to extract bundle IDs, not plist content. PlistBuddy is simpler for reading Info.plist bundle IDs. |
| promptui / survey | Overkill for a single yes/no prompt. bufio.NewReader is sufficient. |
| mdfind / Spotlight API | mdfind bulk enumeration takes ~12s (tested). PlistBuddy per-app is ~0.8s for all apps. PlistBuddy wins. |

**Installation:** No new dependencies. Phase 4 uses only stdlib additions to the existing stack.

## Architecture Patterns

### Recommended Project Structure (Phase 4 additions)

```
mac-cleaner/
├── cmd/
│   └── root.go                    # Add --app-leftovers flag + deletion flow
├── pkg/
│   └── appleftovers/
│       ├── scanner.go             # Scan() for orphaned prefs, iOS backups, old Downloads
│       └── scanner_test.go        # Tests with temp dirs, mock app dirs
├── internal/
│   ├── confirm/
│   │   ├── confirm.go             # PromptConfirmation(io.Reader, io.Writer, []scan.CategoryResult) bool
│   │   └── confirm_test.go        # Tests with strings.NewReader for stdin injection
│   ├── cleanup/
│   │   ├── cleanup.go             # Execute([]scan.CategoryResult) CleanupSummary
│   │   └── cleanup_test.go        # Tests with temp dirs, verify files removed
│   ├── safety/                    # (Phase 1 - unchanged)
│   └── scan/                      # (Phase 2 - unchanged, possibly add age-filter helper)
```

### Pattern 1: Orphaned Preferences Scanner

**What:** Enumerates plist files in `~/Library/Preferences/`, builds a set of installed app bundle IDs by reading `Info.plist` from standard app directories, then identifies plists whose domain prefix does not match any known bundle ID.

**When to use:** For APP-01 (orphaned preferences detection).

**Key design decisions:**

1. **Skip `com.apple.*` plists entirely** -- these are system preferences and should never be flagged as orphaned.
2. **Use prefix matching, not exact matching** -- An app with bundle ID `com.example.MyApp` creates prefs like `com.example.MyApp.plist`, `com.example.MyApp.helper.plist`, `com.example.MyApp.SomeExtension.plist`. Match on the bundle ID being a prefix of the plist domain.
3. **Scan multiple app directories** -- `/Applications`, `/Applications/Utilities`, `~/Applications`, `/System/Applications`, `/System/Applications/Utilities`. This covers user-installed, system, and user-scoped apps.
4. **Use PlistBuddy, not mdfind** -- PlistBuddy reading Info.plist from app bundles completes in ~0.8s for all apps. mdfind bulk takes ~12s. PlistBuddy also avoids dependency on Spotlight indexing being up-to-date.

**Example approach:**
```go
// Build set of known bundle IDs
func installedBundleIDs(runner CmdRunner) map[string]bool {
    ids := make(map[string]bool)
    appDirs := []string{
        "/Applications",
        "/Applications/Utilities",
        filepath.Join(home, "Applications"),
        "/System/Applications",
        "/System/Applications/Utilities",
    }
    for _, dir := range appDirs {
        entries, err := os.ReadDir(dir)
        // ...for each .app entry, run PlistBuddy to get CFBundleIdentifier
        // Add to ids map
    }
    return ids
}

// Check if a plist domain matches any installed app
func isOrphaned(domain string, knownIDs map[string]bool) bool {
    // Skip com.apple.* entirely
    if strings.HasPrefix(domain, "com.apple.") { return false }
    // Check if any known ID is a prefix of this domain
    for id := range knownIDs {
        if domain == id || strings.HasPrefix(domain, id+".") {
            return false
        }
    }
    return true
}
```

### Pattern 2: iOS Backup Scanner

**What:** Scans `~/Library/Application Support/MobileSync/Backup/` for subdirectories (each is one device backup). Uses `DirSize` to calculate each backup's size.

**When to use:** For APP-02 (old iOS device backups).

**Key design decisions:**

1. **Use `ScanTopLevel` directly** -- iOS backups follow the directory-of-subdirectories pattern. Each UUID-named subdirectory is one backup.
2. **Return nil if directory doesn't exist** -- Not all Macs have iOS backups. Follow the same graceful-absence pattern as browser/developer scanners.
3. **No age filtering needed** -- All backups are candidates for cleanup. The user decides via the confirmation prompt.

**Location:** `~/Library/Application Support/MobileSync/Backup/`

**Structure:** Each backup is a directory named with the device UDID (e.g., `00006320-001267G15D45802E`). Backups can be multi-gigabyte -- this is likely the highest-value cleanup target in Phase 4.

### Pattern 3: Old Downloads Scanner

**What:** Scans `~/Downloads/` for files and directories older than a configurable threshold (default: 90 days) based on modification time.

**When to use:** For APP-03 (old Downloads cleanup).

**Key design decisions:**

1. **Use modification time (`ModTime`), not creation time** -- Modification time better reflects when the user last interacted with the file. A file downloaded 2 years ago but edited yesterday should not be flagged.
2. **Default age threshold: 90 days** -- Conservative default. Can be made configurable later (Phase 6 CLI flags).
3. **Scan top-level entries only** -- Don't recurse into subdirectories within Downloads. A folder in Downloads is treated as a single unit -- if its top-level modification time is old, the entire folder is a candidate.
4. **Use `Lstat` (not `Stat`) for top-level entries** -- Don't follow symlinks. If a symlink in Downloads points to an important file elsewhere, we should not be measuring or deleting the target.

**Example approach:**
```go
func scanOldDownloads(home string, maxAge time.Duration) *scan.CategoryResult {
    downloadsDir := filepath.Join(home, "Downloads")
    entries, err := os.ReadDir(downloadsDir)
    // ...
    cutoff := time.Now().Add(-maxAge)
    for _, entry := range entries {
        info, _ := entry.Info()
        if info.ModTime().Before(cutoff) {
            // Calculate size and add to results
        }
    }
}
```

### Pattern 4: Confirmation Prompt

**What:** Displays a summary of items to be deleted (paths and sizes), then requires the user to type "yes" to proceed. Uses `io.Reader`/`io.Writer` interfaces for testability.

**When to use:** For CLI-04 (confirmation before deletion).

**Key design decisions:**

1. **Require full "yes" string** -- Not "y" or "Y". This is a destructive operation. Making the user type "yes" prevents accidental confirmation.
2. **Accept `io.Reader` and `io.Writer`** -- Not hardcoded `os.Stdin`/`os.Stdout`. This enables testing with `strings.NewReader("yes\n")` and `bytes.Buffer`.
3. **Display itemized list before prompt** -- Show each entry with path and size so the user knows exactly what will be deleted.
4. **Return bool** -- `true` if user confirmed, `false` otherwise. Caller decides what to do.

**Example:**
```go
func PromptConfirmation(in io.Reader, out io.Writer, results []scan.CategoryResult) bool {
    // Print what will be removed
    fmt.Fprintf(out, "\nThe following items will be permanently deleted:\n\n")
    for _, cat := range results {
        for _, entry := range cat.Entries {
            fmt.Fprintf(out, "  %s  (%s)\n", entry.Path, scan.FormatSize(entry.Size))
        }
    }
    fmt.Fprintf(out, "\nType 'yes' to proceed: ")

    reader := bufio.NewReader(in)
    response, _ := reader.ReadString('\n')
    return strings.TrimSpace(response) == "yes"
}
```

### Pattern 5: Cleanup Execution

**What:** Takes scan results and removes each entry's path using `os.RemoveAll` (for directories) or `os.Remove` (for files). Returns a summary of what was removed.

**When to use:** For the actual deletion after confirmation.

**Key design decisions:**

1. **Check `IsPathBlocked` before every deletion** -- Defense in depth. Even though scan already filtered, re-check at deletion time.
2. **Continue on individual errors** -- If one file fails to delete, log the error and continue with the rest. Don't abort the entire cleanup.
3. **Return structured summary** -- Items removed count, total bytes freed, any errors encountered.
4. **Verify deletion** -- After `os.RemoveAll`, `os.Stat` the path to confirm it's gone. This catches edge cases like files being recreated by background processes.

### Anti-Patterns to Avoid

- **Deleting without re-checking safety:** Always re-validate paths through `safety.IsPathBlocked` at deletion time, not just at scan time. The scan results could be stale.
- **Hardcoding `os.Stdin` in confirmation:** Makes the confirmation prompt untestable. Always accept `io.Reader`.
- **Treating plist domain == bundle ID exactly:** Many apps create multiple plists with suffixed names. Use prefix matching.
- **Using `mdfind` for bundle ID enumeration:** It takes 12+ seconds and depends on Spotlight indexing. PlistBuddy on known app directories is faster and more reliable.
- **Recursing into ~/Downloads:** Only scan top-level entries. Recursing could flag active project files inside subdirectories.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Reading plist file content | Custom binary plist parser | `/usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier"` via `os/exec` | Plist files can be binary or XML. PlistBuddy handles both formats natively. |
| Detecting installed apps | Custom /Applications directory walker with plist parsing | PlistBuddy per .app bundle + known directory list | Apple's tool handles all edge cases (nested apps, localized names, etc.) |
| User input prompt | Custom terminal raw mode reading | `bufio.NewReader(os.Stdin).ReadString('\n')` | Simple line-oriented input is sufficient for yes/no. No need for raw terminal mode. |

**Key insight:** The complexity in this phase is not in any single operation but in the orchestration -- scanning three different kinds of leftovers, presenting combined results, getting confirmation, executing deletion, and reporting results. Each individual piece is simple; the challenge is wiring them together correctly with proper error handling and safety checks.

## Common Pitfalls

### Pitfall 1: False Positive Orphaned Preferences

**What goes wrong:** Flagging preferences from still-installed apps as orphaned, leading users to delete needed configuration.
**Why it happens:** Bundle ID matching is imprecise. An app's Info.plist says `com.example.App` but it creates prefs named `com.example.App.helper`, `com.example.App.SomeExtension`, etc. Also, some apps install to non-standard locations (e.g., `/opt/homebrew/`, `~/bin/`).
**How to avoid:**
- Use prefix matching: if any known bundle ID is a prefix of the plist domain, it's NOT orphaned.
- Always skip `com.apple.*` prefs -- these are macOS system preferences that persist through OS updates.
- Scan all standard app directories, not just `/Applications`.
- Accept that some false positives are inevitable and rely on the confirmation prompt as the safety net.
**Warning signs:** Users reporting that prefs for installed apps are being flagged.

### Pitfall 2: Deleting Active Downloads

**What goes wrong:** Flagging files in `~/Downloads/` that the user actively uses but hasn't modified recently (e.g., reference PDFs, installer DMGs kept for reinstallation).
**Why it happens:** Modification time doesn't capture "last accessed" or "user intent to keep."
**How to avoid:**
- Use a conservative default threshold (90 days).
- The confirmation prompt shows each file individually so users can abort if they see something they want to keep.
- Document that this is based on modification time, not access time.
**Warning signs:** Users complaining about files they want to keep being flagged.

### Pitfall 3: Confirmation Prompt Bypass in Tests

**What goes wrong:** Tests that invoke the deletion path accidentally delete files because the confirmation prompt is bypassed or mocked incorrectly.
**How to avoid:**
- Use `io.Reader` dependency injection -- tests pass `strings.NewReader("no\n")` by default.
- Deletion tests use `t.TempDir()` exclusively -- never real user directories.
- Cleanup function takes explicit paths, never discovers paths on its own.
**Warning signs:** Test cleanup warnings, unexpected file deletions during test runs.

### Pitfall 4: PlistBuddy Not Available

**What goes wrong:** The scanner crashes or returns unexpected results because `/usr/libexec/PlistBuddy` is not found.
**How to avoid:**
- Check that PlistBuddy exists before attempting to use it (like the Docker `exec.LookPath` pattern).
- If PlistBuddy is unavailable, skip orphaned prefs scanning entirely and log a warning.
- PlistBuddy has shipped with every macOS version since 10.0, so this is unlikely but worth guarding against.
**Warning signs:** Errors about "executable not found" on non-standard macOS installations.

### Pitfall 5: Race Conditions During Deletion

**What goes wrong:** A file is deleted between scan and cleanup, causing `os.RemoveAll` to fail with "no such file or directory."
**Why it happens:** Background processes, other users, or the OS may remove temporary files between scan and deletion.
**How to avoid:**
- Treat "file not found" during deletion as success (the file is already gone).
- `os.RemoveAll` already returns nil for non-existent paths, so this is mostly about `os.Remove` for individual files.
**Warning signs:** Sporadic "file not found" errors during cleanup.

### Pitfall 6: Incomplete Deletion Summary

**What goes wrong:** The post-cleanup summary reports incorrect space freed because it uses pre-deletion sizes rather than verifying what was actually removed.
**Why it happens:** If some deletions fail, the total freed is less than the total scanned.
**How to avoid:**
- Track successful deletions separately from attempted deletions.
- Sum sizes only for successfully removed items.
- Report both "attempted" and "actual" in the summary if they differ.
**Warning signs:** Summary showing more space freed than was actually reclaimed.

## Code Examples

### Example 1: CmdRunner Pattern for PlistBuddy (from Phase 3 Docker pattern)

```go
// CmdRunner executes an external command and returns its stdout output.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, name, args...)
    return cmd.Output()
}

// Read bundle ID from an .app bundle
func readBundleID(runner CmdRunner, appPath string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    plistPath := filepath.Join(appPath, "Contents", "Info.plist")
    out, err := runner(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :CFBundleIdentifier", plistPath)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}
```

### Example 2: File Age Filtering for Downloads

```go
func isOlderThan(info os.FileInfo, maxAge time.Duration) bool {
    return time.Since(info.ModTime()) > maxAge
}

// Usage in scanner:
cutoff := time.Now().Add(-maxAge)
for _, entry := range entries {
    info, err := entry.Info()
    if err != nil {
        continue
    }
    if info.ModTime().Before(cutoff) {
        // This entry is old enough to flag
    }
}
```

### Example 3: Confirmation Prompt with Testable IO

```go
func PromptConfirmation(in io.Reader, out io.Writer, results []scan.CategoryResult) bool {
    var totalSize int64
    for _, cat := range results {
        totalSize += cat.TotalSize
    }

    fmt.Fprintf(out, "\nTotal: %s will be permanently deleted.\n", scan.FormatSize(totalSize))
    fmt.Fprintf(out, "Type 'yes' to proceed: ")

    reader := bufio.NewReader(in)
    response, err := reader.ReadString('\n')
    if err != nil {
        return false
    }
    return strings.TrimSpace(response) == "yes"
}

// In test:
func TestConfirmationYes(t *testing.T) {
    in := strings.NewReader("yes\n")
    out := &bytes.Buffer{}
    results := []scan.CategoryResult{{TotalSize: 1000}}

    confirmed := PromptConfirmation(in, out, results)
    if !confirmed {
        t.Error("expected confirmation to succeed with 'yes' input")
    }
}
```

### Example 4: Safe Deletion with Error Continuation

```go
type CleanupResult struct {
    Removed    int
    Failed     int
    BytesFreed int64
    Errors     []error
}

func Execute(results []scan.CategoryResult) CleanupResult {
    var cr CleanupResult
    for _, cat := range results {
        for _, entry := range cat.Entries {
            if blocked, reason := safety.IsPathBlocked(entry.Path); blocked {
                safety.WarnBlocked(entry.Path, reason)
                cr.Failed++
                continue
            }

            err := os.RemoveAll(entry.Path)
            if err != nil {
                cr.Errors = append(cr.Errors, fmt.Errorf("remove %s: %w", entry.Path, err))
                cr.Failed++
                continue
            }
            cr.Removed++
            cr.BytesFreed += entry.Size
        }
    }
    return cr
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| mdfind for app enumeration | PlistBuddy per .app bundle | N/A (design choice) | 15x faster (~0.8s vs ~12s), no Spotlight dependency |
| Access time (atime) for file age | Modification time (mtime) | macOS default noatime mount | atime unreliable on macOS; mtime is consistent |
| Interactive y/n prompt | Full "yes" string required | Best practice for destructive CLIs | Prevents accidental confirmation from muscle memory |

**Deprecated/outdated:**
- `ioutil.ReadDir` -- Deprecated since Go 1.16. Use `os.ReadDir` instead (already used in codebase).
- `ioutil.ReadFile` for plists -- Not needed. We don't parse plist content, only filenames.

## Open Questions

1. **Should orphaned prefs scanning include `~/Library/Application Support/` too?**
   - What we know: `~/Library/Application Support/` contains per-app directories that can also be orphaned. These are typically larger than plist files (some are 100s of MB).
   - What's unclear: The APP-01 requirement says "orphaned preferences from uninstalled apps" which specifically means prefs (plists). Application Support cleanup could be a v2 feature.
   - Recommendation: Stick to `~/Library/Preferences/` for Phase 4 as the requirement specifies. Application Support can be added later.

2. **Should the Downloads age threshold be configurable via flag?**
   - What we know: CLI-08 and CLI-09 (Phase 6) add skip/config flags. A `--downloads-age` flag would fit that pattern.
   - What's unclear: Whether to add it now or defer to Phase 6.
   - Recommendation: Hardcode 90 days for Phase 4. Add `--downloads-age` flag in Phase 6 when the CLI polish happens.

3. **How should the deletion flow interact with `--dry-run`?**
   - What we know: `--dry-run` already exists and prevents deletion. Phase 4 adds confirmation + actual deletion.
   - What's unclear: Should `--dry-run` skip the confirmation prompt entirely (since nothing will be deleted), or show the prompt to preview the UX?
   - Recommendation: Skip the confirmation prompt in dry-run mode. Dry-run means "show what would happen" -- the confirmation is part of the action, not the preview.

4. **What about the PlistBuddy CmdRunner injection pattern for testing?**
   - What we know: Phase 3 used `CmdRunner` for Docker CLI mocking. Same pattern applies to PlistBuddy.
   - What's unclear: Whether to make `Scan()` accept a CmdRunner or use package-level injection.
   - Recommendation: Follow Phase 3's Docker pattern exactly. The `scanOrphanedPrefs` internal function accepts a `CmdRunner`, and the exported `Scan()` passes `defaultRunner`.

5. **Should confirmation be per-category or for all results combined?**
   - What we know: Success criteria says "confirmation prompt before deletion listing exactly what will be removed" (singular prompt).
   - What's unclear: Whether a single prompt listing everything is better UX than per-category prompts.
   - Recommendation: Single combined prompt. Simpler, and Phase 5 (Interactive Mode) will add per-item control.

## Sources

### Primary (HIGH confidence)
- **Codebase analysis** -- Direct reading of all existing source files in `/Users/gregor/projects/mac-clarner/`. All architectural patterns, type definitions, test patterns, and CLI wiring patterns verified from source.
- [Apple Developer - CFBundleIdentifier](https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundleidentifier) -- Bundle identifier format (reverse-DNS UTI)
- [Apple Support - Locate and manage backups](https://support.apple.com/en-us/108809) -- iOS backup location `~/Library/Application Support/MobileSync/Backup/`
- [Go os package](https://pkg.go.dev/os) -- `Remove`, `RemoveAll`, `ReadDir`, `FileInfo.ModTime()`

### Secondary (MEDIUM confidence)
- [Eclectic Light Company - Cleaning out old preference settings](https://eclecticlight.co/2021/09/17/cleaning-out-old-preference-settings-and-other-housekeeping/) -- Orphaned plist detection approach
- [Jamf - How to Find the Bundle ID](https://support.jamf.com/en/articles/11034093-how-to-find-the-bundle-id-of-an-app-on-macos-or-ios) -- PlistBuddy and mdfind approaches
- [Go user confirmation gist](https://gist.github.com/r0l1/3dcbb0c8f6cfe9c66ab8008f55f8f28b) -- bufio.NewReader stdin prompt pattern
- [Cobra GitHub Issue #1007](https://github.com/spf13/cobra/issues/1007) -- Confirmation that Cobra has no built-in prompt support

### Tertiary (LOW confidence)
- **mdfind performance testing** -- Measured locally (12s for bulk enumeration vs 0.8s for PlistBuddy). Results may vary by system.
- **Preference file statistics** -- Measured locally (645 plists, 14MB total, 418 com.apple.*). Typical but not universal.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies, all stdlib patterns already proven in Phases 1-3
- Architecture: HIGH -- Follows exact patterns from existing scanner packages (direct source verification)
- Orphaned prefs detection: MEDIUM -- PlistBuddy approach verified locally but prefix matching heuristic may produce false positives/negatives in edge cases
- Deletion/confirmation: HIGH -- Standard Go patterns (os.Remove, bufio, io.Reader injection), well-documented
- Pitfalls: HIGH -- Based on local testing and codebase analysis

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, macOS paths don't change between minor releases)
