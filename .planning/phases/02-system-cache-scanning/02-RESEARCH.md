# Phase 2: System Cache Scanning - Research

**Researched:** 2026-02-16
**Domain:** Go filesystem scanning, macOS cache directories, dry-run architecture, CLI output formatting
**Confidence:** HIGH

## Summary

Phase 2 builds the first real scanning functionality on top of Phase 1's safety layer and Cobra CLI scaffold. The phase needs three things: (1) a scanning engine that walks directories and calculates sizes, (2) CLI flags (`--system-caches`, `--dry-run`) wired to Cobra, and (3) formatted terminal output showing scan results with a summary. All three are well-understood problems with established Go patterns.

The scan targets are three specific macOS locations: `~/Library/Caches` (user app caches), `~/Library/Logs` (user logs), and the QuickLook thumbnail cache. The first two are straightforward directory enumerations under the user's home directory. QuickLook is more complex -- its cache lives under `/var/folders/<id>/C/com.apple.quicklook.ThumbnailsAgent/`, a per-user system-managed path discoverable via `getconf DARWIN_USER_CACHE_DIR` or the `$TMPDIR` environment variable. The recommended approach is to use `qlmanage -r cache` for QuickLook cleaning (rather than direct deletion) in later phases, but for Phase 2 scanning we just need to locate and size the directory.

The key architectural decision is how to represent scan results. Phase 2 introduces the core data types (`ScanResult`, `CategorySummary`) that every subsequent phase builds on. Getting these types right matters more than any other detail. The scanning engine should live in `pkg/system/` (per the established package convention) and expose a `Scan()` function that returns results without side effects. The safety layer from Phase 1 (`internal/safety`) gates every path before scanning.

**Primary recommendation:** Use `io/fs.WalkDir` for recursive directory size calculation, `os.ReadDir` for top-level cache directory enumeration, and introduce core result types in a new `internal/scan/` package shared across all future category scanners. Wire `--system-caches` and `--dry-run` as persistent boolean flags on the root Cobra command. Format output with `fatih/color` for TTY-aware colored tables.

## Standard Stack

### Core (Phase 2 additions to Phase 1 stack)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| fatih/color | v1.18.0+ | Colored terminal output | Automatic TTY detection, NO_COLOR env var support. User decision: "Colors with TTY auto-detection." |
| io/fs (stdlib) | -- | `WalkDir` for recursive directory traversal | More efficient than `filepath.Walk` (uses `DirEntry` instead of `FileInfo`, avoids extra `os.Lstat` calls). Go 1.16+. |
| os (stdlib) | -- | `ReadDir`, `UserHomeDir`, `Stat` | Top-level directory enumeration, home directory resolution, file existence checks |
| path/filepath (stdlib) | -- | `Join`, `Clean`, `EvalSymlinks` | Path construction and safety-layer integration |
| os/exec (stdlib) | -- | Execute `getconf` for QuickLook cache path | Discover `DARWIN_USER_CACHE_DIR` for QuickLook thumbnail cache location |
| fmt (stdlib) | -- | Formatted output | Size formatting, table output |
| text/tabwriter (stdlib) | -- | Aligned column output | Table-formatted scan results with aligned columns per user decision |

### Not Needed Yet

| Library | Why Deferred |
|---------|-------------|
| Viper | No config file support in Phase 2. Config comes with user-customizable paths later. |
| encoding/json | No JSON output in Phase 2. Comes in Phase 6 (CLI-06). |
| go-humanize (external) | Human-readable sizes can be done with a simple 15-line stdlib function. No need for external dependency. |
| sync/errgroup | Parallel scanning deferred. Phase 2 scans sequentially (only 3 locations). Parallelism benefits Phase 3+ with many more targets. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `io/fs.WalkDir` | `filepath.Walk` | `filepath.Walk` calls `os.Lstat` on every entry, WalkDir uses `DirEntry` which is more efficient. WalkDir also lets you skip directories before reading them. |
| `text/tabwriter` | `tablewriter` (external) | External library adds a dependency for something stdlib handles. `tabwriter` aligns columns with elastic tab stops -- sufficient for this use case. |
| Custom size formatter | `go-humanize` | Adding a dependency for one function (`Bytes()`) is overkill. A 15-line function achieves the same result. |
| `os/exec` for QuickLook path | `$TMPDIR` env var manipulation | Both work. `TMPDIR` is always set on macOS and the QuickLook cache is a sibling directory (`../C/` relative to `$TMPDIR`). Using `TMPDIR` avoids spawning a subprocess. Prefer `TMPDIR`-based approach. |

**Installation:**
```bash
go get github.com/fatih/color@latest
```

## Architecture Patterns

### Recommended Project Structure (Phase 2 additions)

```
mac-cleaner/
├── main.go
├── cmd/
│   └── root.go                    # Add --system-caches, --dry-run flags
├── internal/
│   ├── safety/
│   │   ├── safety.go              # (Phase 1 - unchanged)
│   │   └── safety_test.go         # (Phase 1 - unchanged)
│   └── scan/
│       ├── types.go               # ScanEntry, CategoryResult, ScanSummary types
│       └── size.go                # DirSize(), FormatSize() utility functions
│       └── size_test.go           # Tests for size calculation and formatting
├── pkg/
│   └── system/
│       ├── scanner.go             # Scan() function for system caches
│       └── scanner_test.go        # Tests for system cache scanning
├── go.mod
└── go.sum
```

**Rationale:**
- `internal/scan/` -- Shared types and utilities used by all future scanners (`pkg/system/`, `pkg/browser/`, `pkg/developer/`). In `internal/` because these are implementation details, not public API.
- `pkg/system/scanner.go` -- Category-specific scanning logic. In `pkg/` per the user's decision to have "one package per cleaning category."
- Types in `internal/scan/types.go` -- Central type definitions prevent circular imports when multiple packages need the same result types.

### Pattern 1: Core Scan Result Types

**What:** Define result types that flow through the entire system -- from scanner to reporter to cleaner.

**Why first:** Every phase after Phase 2 builds on these types. Getting them right avoids costly refactors.

```go
// internal/scan/types.go
package scan

// ScanEntry represents a single scannable item (a directory or file).
type ScanEntry struct {
    Path        string // Absolute filesystem path
    Description string // Human-readable name (e.g., "com.apple.Safari")
    Size        int64  // Size in bytes
}

// CategoryResult groups scan entries under a category.
type CategoryResult struct {
    Category    string       // Category identifier (e.g., "system-caches")
    Description string       // Human-readable category name (e.g., "User App Caches")
    Entries     []ScanEntry  // Individual items found
    TotalSize   int64        // Sum of all entry sizes
}

// ScanSummary aggregates results across all scanned categories.
type ScanSummary struct {
    Categories []CategoryResult
    TotalSize  int64
}
```

**Design decisions:**
- `Size` is `int64` (bytes), not formatted string. Formatting is a presentation concern, not a data concern.
- `Category` is a plain string, not an enum. Keeps it simple; new categories (browser, developer) just use new string values.
- `Description` for human-readable display. `Path` for machine use and safety checks.

### Pattern 2: Scanner Function Signature

**What:** Each category package exports a `Scan()` function with consistent signature.

```go
// pkg/system/scanner.go
package system

import "github.com/gregor/mac-cleaner/internal/scan"

// Scan discovers system cache directories and calculates their sizes.
// It returns results for user app caches, user logs, and QuickLook thumbnails.
// Scan never modifies the filesystem.
func Scan() (*scan.CategoryResult, error) {
    // ...
}
```

**Key properties:**
- Returns `(*scan.CategoryResult, error)` -- consistent across all scanners.
- Never modifies the filesystem (scanning is always read-only, regardless of dry-run flag).
- Integrates with safety layer: calls `safety.IsPathBlocked()` before accessing any path.
- Handles missing directories gracefully (directory doesn't exist = skip, not error).

### Pattern 3: Directory Size Calculation with WalkDir

**What:** Calculate total size of a directory recursively using `io/fs.WalkDir`.

```go
// internal/scan/size.go
package scan

import (
    "io/fs"
    "os"
    "path/filepath"
)

// DirSize calculates the total size of all regular files under root.
// Symlinks are not followed. Permission errors are skipped (logged to stderr).
func DirSize(root string) (int64, error) {
    var total int64
    err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            // Skip permission errors, don't abort entire walk
            return nil
        }
        if d.Type().IsRegular() {
            info, err := d.Info()
            if err != nil {
                return nil // Skip files we can't stat
            }
            total += info.Size()
        }
        return nil
    })
    return total, err
}
```

**Key behaviors:**
- Uses `filepath.WalkDir` (not `io/fs.WalkDir`) because we're walking the OS filesystem, not an `fs.FS` interface.
- Does NOT follow symlinks (`WalkDir` skips symlinks by default).
- Permission errors are silently skipped -- the tool reports what it CAN access, not what it cannot (permission handling is Phase 7).
- Only counts regular files (not directories, symlinks, devices).
- Returns `int64` bytes. Formatting happens at the presentation layer.

### Pattern 4: Human-Readable Size Formatting

**What:** Format byte counts as human-readable strings like `du -h`.

```go
// internal/scan/size.go

// FormatSize formats a byte count as a human-readable string.
// Uses decimal (SI) units: B, kB, MB, GB, TB.
// Examples: 0 -> "0 B", 1024 -> "1.0 kB", 5368709120 -> "5.4 GB"
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
    return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
```

**Decision: Use SI units (base 1000), not IEC (base 1024).**

Rationale: macOS Finder and `du -h` use SI units (1 GB = 1,000,000,000 bytes). The user said "like `du -h`" which uses powers of 1000 on macOS. Using IEC (GiB) would confuse users expecting Finder-consistent numbers.

### Pattern 5: QuickLook Cache Discovery

**What:** Locate the QuickLook thumbnail cache using macOS-specific paths.

```go
// pkg/system/scanner.go

// quickLookCacheDir returns the path to the QuickLook thumbnail cache.
// On macOS, this is DARWIN_USER_CACHE_DIR/com.apple.quicklook.ThumbnailsAgent/
// Discovered via $TMPDIR (sibling directory ../C/ relative to $TMPDIR).
func quickLookCacheDir() (string, error) {
    tmpDir := os.Getenv("TMPDIR")
    if tmpDir == "" {
        return "", fmt.Errorf("TMPDIR not set")
    }
    // TMPDIR is /var/folders/<id>/T/
    // DARWIN_USER_CACHE_DIR is /var/folders/<id>/C/
    parent := filepath.Dir(filepath.Clean(tmpDir))
    cacheDir := filepath.Join(parent, "C", "com.apple.quicklook.ThumbnailsAgent")

    if _, err := os.Stat(cacheDir); err != nil {
        return "", err
    }
    return cacheDir, nil
}
```

**Rationale:** Using `$TMPDIR` avoids spawning a subprocess (`getconf DARWIN_USER_CACHE_DIR`). `$TMPDIR` is always set on macOS and points to `/var/folders/<id>/T/`. The QuickLook cache is a sibling under `../C/`.

### Pattern 6: Cobra Flag Wiring

**What:** Add `--system-caches` and `--dry-run` as boolean flags on the root command.

```go
// cmd/root.go additions

var (
    flagDryRun       bool
    flagSystemCaches bool
)

func init() {
    rootCmd.Version = version
    rootCmd.SetVersionTemplate("{{.Version}}\n")

    // Persistent flags available to all subcommands (future-proof)
    rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false,
        "preview what would be removed without deleting")

    // Category flags
    rootCmd.Flags().BoolVar(&flagSystemCaches, "system-caches", false,
        "scan user app caches, logs, and QuickLook thumbnails")
}
```

**Decision: `--dry-run` is a PersistentFlag, `--system-caches` is a local Flag.**

Rationale: `--dry-run` applies globally across all future commands/modes. `--system-caches` is a category selector relevant to the root command's scanning behavior. Making `--dry-run` persistent means future subcommands (if any) inherit it automatically.

**Decision: In Phase 2, scanning IS the only action. `--dry-run` is the implicit default.**

Rationale: Phase 2 has no deletion capability (that's Phase 4). The scan itself is read-only. `--dry-run` in Phase 2 is about explicit user intent ("I only want to see what would be cleaned") and sets up the flag for Phase 4 when actual deletion exists. The tool should accept `--dry-run` and behave identically to without it in Phase 2, except possibly noting in output that no files will be deleted.

### Pattern 7: Output Formatting with Color and Tables

**What:** Display scan results as a formatted table with aligned columns and colored output.

```go
// Example output for: mac-cleaner --system-caches --dry-run
//
// System Caches (dry run)
//
//   User App Caches    ~/Library/Caches
//     com.apple.Safari           245.3 MB
//     com.google.Chrome          189.7 MB
//     com.spotify.client          56.2 MB
//     ... (27 more items)
//
//   User Logs           ~/Library/Logs
//     DiagnosticReports           12.4 MB
//     CrashReporter                3.1 MB
//     ... (5 more items)
//
//   QuickLook Thumbnails
//     ThumbnailsAgent            300.0 kB
//
//   Total: 5.9 GB reclaimable
```

**Implementation notes:**
- Use `tabwriter` for column alignment within each category.
- Use `fatih/color` for category headers (bold), sizes (cyan), and totals (green/bold).
- When piped (non-TTY), colors are automatically disabled by `fatih/color`.
- Per user decision: terse style, no personality, aligned columns, human-readable sizes.

### Anti-Patterns to Avoid

- **Scanning /Library/Caches (system-level) in Phase 2:** The success criteria specifically says "user app caches (~/Library/Caches)". System-level `/Library/Caches` may require elevated permissions and is not in scope.
- **Deleting anything in Phase 2:** Phase 2 is scan-only. Even though `--dry-run` is a flag, there is no non-dry-run path in Phase 2. Deletion comes in Phase 4.
- **Hardcoding home directory path:** Always use `os.UserHomeDir()` to resolve `~`. Never hardcode `/Users/username/`.
- **Following symlinks during size calculation:** `filepath.WalkDir` does not follow symlinks by default. Do not override this behavior.
- **Failing entire scan on one permission error:** Skip inaccessible entries and continue. Report what you CAN access.
- **Putting shared types in `pkg/system/`:** Types used by multiple scanners belong in `internal/scan/`. Putting them in `pkg/system/` creates import cycles when `pkg/browser/` needs the same types.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Recursive directory size | Manual recursion with `os.ReadDir` | `filepath.WalkDir` | Handles symlinks, permissions, edge cases. Battle-tested. |
| Column alignment | Manual string padding | `text/tabwriter` | Elastic tab stops handle variable-width content correctly. |
| TTY detection | `os.Stat(os.Stdout)` tricks | `fatih/color` auto-detection | Handles NO_COLOR, piped output, Windows. Cross-platform. |
| Human-readable sizes | Pulling in `go-humanize` for one function | 15-line `FormatSize()` function | Avoids external dependency for trivial functionality. |
| Home directory expansion | `os.Getenv("HOME")` | `os.UserHomeDir()` | Cross-platform, handles edge cases (no $HOME set). |

**Key insight:** Phase 2 is mostly stdlib code. The only external dependency added is `fatih/color` for terminal output.

## Common Pitfalls

### Pitfall 1: Counting Symlink Targets in Size Calculation
**What goes wrong:** `WalkDir` reports a symlink entry but `d.Info()` returns the target's size, double-counting if the target is elsewhere in the tree.
**Why it happens:** Calling `Info()` on a symlink follows it to the target.
**How to avoid:** Check `d.Type().IsRegular()` before calling `Info()`. Symlinks have type `fs.ModeSymlink`, not regular. `WalkDir` does not follow symlinks into directories, but individual symlink entries still appear.
**Warning signs:** Reported sizes are larger than `du -sh` reports.

### Pitfall 2: TMPDIR Not Set or Different Format
**What goes wrong:** QuickLook cache path calculation fails because `$TMPDIR` is unset or has unexpected format.
**Why it happens:** In some contexts (launchd services, cron jobs), `$TMPDIR` may not be set or may point to `/tmp` instead of `/var/folders/...`.
**How to avoid:** Fall back to `os/exec` calling `getconf DARWIN_USER_CACHE_DIR` if `$TMPDIR` doesn't contain `/var/folders/`. Treat QuickLook as optional -- if discovery fails, skip it and continue.
**Warning signs:** QuickLook section missing from output, or path resolution errors on some machines.

### Pitfall 3: ~/Library/Caches Contains Non-Directory Entries
**What goes wrong:** Scanner assumes every entry in `~/Library/Caches/` is a directory and tries to `WalkDir` into it.
**Why it happens:** `~/Library/Caches/` can contain regular files (e.g., PNG files, database files) at the top level alongside directories.
**How to avoid:** Check `entry.IsDir()` before walking. For regular files, just use `entry.Info().Size()`.
**Warning signs:** Errors like "not a directory" in test output.

### Pitfall 4: Race Condition Between Size Calculation and Display
**What goes wrong:** A cache directory is deleted by another process between scanning and displaying results.
**Why it happens:** macOS system processes actively manage caches. Between scan and display, sizes may change.
**How to avoid:** Accept this as inherent to cache scanning. Sizes are "approximate as of scan time." Do not re-verify sizes before display.
**Warning signs:** Size discrepancies between reported total and actual disk usage.

### Pitfall 5: Very Large Directory Trees Causing Slow Scans
**What goes wrong:** Some `~/Library/Caches/` subdirectories contain millions of small files (e.g., browser caches, npm caches), making `WalkDir` slow.
**Why it happens:** `WalkDir` visits every file. A directory with 100,000 files takes noticeable time to walk.
**How to avoid:** For Phase 2, accept sequential scanning (only 3 locations). Add timeout/progress indication if scanning takes >2 seconds. Phase 3+ can add concurrency.
**Warning signs:** Scan appears to hang when `~/Library/Caches` is very large (>10GB).

### Pitfall 6: du -h vs FormatSize Disagreement
**What goes wrong:** Tool reports "5.4 GB" but `du -sh` says "5.2G" for the same directory.
**Why it happens:** `du` counts disk blocks (which include filesystem overhead), while `WalkDir` + `Info().Size()` counts logical file sizes. Also, `du` on macOS uses 1000-based units while some systems use 1024.
**How to avoid:** Document that sizes are logical file sizes, not disk usage. Use the same SI units as macOS Finder.
**Warning signs:** User complaints that tool "overreports" or "underreports" sizes.

## Code Examples

### Complete Scanner Implementation Pattern

```go
// pkg/system/scanner.go
package system

import (
    "io/fs"
    "os"
    "path/filepath"

    "github.com/gregor/mac-cleaner/internal/safety"
    "github.com/gregor/mac-cleaner/internal/scan"
)

// Scan discovers system cache directories and calculates their sizes.
func Scan() ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("cannot determine home directory: %w", err)
    }

    var results []scan.CategoryResult

    // User App Caches
    cachesDir := filepath.Join(home, "Library", "Caches")
    if cacheResult, err := scanTopLevel(cachesDir, "system-caches", "User App Caches"); err == nil {
        results = append(results, *cacheResult)
    }

    // User Logs
    logsDir := filepath.Join(home, "Library", "Logs")
    if logResult, err := scanTopLevel(logsDir, "system-logs", "User Logs"); err == nil {
        results = append(results, *logResult)
    }

    // QuickLook Thumbnails
    if qlDir, err := quickLookCacheDir(); err == nil {
        if qlResult, err := scanSingleDir(qlDir, "quicklook", "QuickLook Thumbnails"); err == nil {
            results = append(results, *qlResult)
        }
    }

    return results, nil
}

// scanTopLevel enumerates top-level entries in a directory,
// calculates each entry's size, and returns them as a CategoryResult.
func scanTopLevel(dir, category, description string) (*scan.CategoryResult, error) {
    if blocked, reason := safety.IsPathBlocked(dir); blocked {
        safety.WarnBlocked(dir, reason)
        return nil, fmt.Errorf("path blocked: %s", reason)
    }

    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }

    result := &scan.CategoryResult{
        Category:    category,
        Description: description,
    }

    for _, entry := range entries {
        entryPath := filepath.Join(dir, entry.Name())

        if blocked, reason := safety.IsPathBlocked(entryPath); blocked {
            safety.WarnBlocked(entryPath, reason)
            continue
        }

        size, err := scan.DirSize(entryPath)
        if err != nil {
            continue // Skip entries we can't size
        }

        result.Entries = append(result.Entries, scan.ScanEntry{
            Path:        entryPath,
            Description: entry.Name(),
            Size:        size,
        })
        result.TotalSize += size
    }

    return result, nil
}
```

### Test Pattern: Table-Driven Scanner Tests with Temp Directories

```go
// pkg/system/scanner_test.go
package system

import (
    "os"
    "path/filepath"
    "testing"
)

func TestScanTopLevel(t *testing.T) {
    // Create temp directory structure
    tmpDir := t.TempDir()

    // Create subdirectories with known sizes
    subDir := filepath.Join(tmpDir, "com.example.app")
    if err := os.MkdirAll(subDir, 0755); err != nil {
        t.Fatal(err)
    }

    // Write a file with known size
    testFile := filepath.Join(subDir, "cache.db")
    if err := os.WriteFile(testFile, make([]byte, 1024), 0644); err != nil {
        t.Fatal(err)
    }

    result, err := scanTopLevel(tmpDir, "test-category", "Test Category")
    if err != nil {
        t.Fatalf("scanTopLevel() error = %v", err)
    }

    if len(result.Entries) != 1 {
        t.Errorf("got %d entries, want 1", len(result.Entries))
    }

    if result.TotalSize != 1024 {
        t.Errorf("total size = %d, want 1024", result.TotalSize)
    }
}
```

### Test Pattern: Verify Dry-Run Does Not Delete

```go
func TestDryRunPreservesFiles(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "should-still-exist.txt")
    os.WriteFile(testFile, []byte("important data"), 0644)

    // Run scan (scan is always read-only)
    _, err := scanTopLevel(tmpDir, "test", "Test")
    if err != nil {
        t.Fatal(err)
    }

    // Verify file still exists
    if _, err := os.Stat(testFile); err != nil {
        t.Errorf("file was deleted during scan: %v", err)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.Walk()` | `filepath.WalkDir()` (uses `DirEntry`) | Go 1.16 (2021) | WalkDir avoids extra `os.Lstat` calls per entry, ~30% faster for large trees |
| Manual color codes (`\033[31m`) | `fatih/color` with auto-detection | fatih/color v1.0 (2016) | Handles NO_COLOR, TTY detection, Windows. Standard in Go ecosystem. |
| `os.Getenv("HOME")` for home dir | `os.UserHomeDir()` | Go 1.12 (2019) | Cross-platform, handles missing $HOME gracefully |
| `os.ReadDir` returning `[]FileInfo` | `os.ReadDir` returning `[]DirEntry` | Go 1.16 (2021) | `DirEntry` is lazily loaded, more efficient for just names/types |

## Open Questions

1. **Should `~/Library/Caches` entries be sorted by size (largest first) in output?**
   - What we know: User said "table format with aligned columns" and terse style. No explicit sort preference.
   - What's unclear: Whether alphabetical or size-descending is more useful.
   - Recommendation: Sort by size descending. Users care about the biggest space hogs. This matches what CleanMyMac and similar tools do.

2. **Should we show entries with 0 bytes?**
   - What we know: Some cache directories exist but are empty (0 bytes).
   - What's unclear: Whether showing "0 B" entries adds noise or useful information.
   - Recommendation: Hide 0-byte entries from default output. They add noise without actionable information. Show them with `--verbose` (Phase 6).

3. **Should the QuickLook scan include ALL com.apple.quicklook.* directories under DARWIN_USER_CACHE_DIR, or just ThumbnailsAgent?**
   - What we know: The success criteria says "QuickLook thumbnails." There are multiple QuickLook-related directories (ThumbnailsAgent, quicklookd, satellite, etc.).
   - What's unclear: Whether the user considers all QuickLook caches or just the thumbnail-specific one.
   - Recommendation: Scan all `com.apple.quicklook.*` directories under `DARWIN_USER_CACHE_DIR` and aggregate them under a single "QuickLook Thumbnails" category. This is more useful and matches what `qlmanage -r cache` resets.

4. **How to handle the `--system-caches` flag when `--dry-run` is not provided?**
   - What we know: Phase 2 has no deletion capability. Running `mac-cleaner --system-caches` without `--dry-run` would scan and... do nothing.
   - What's unclear: Whether to require `--dry-run` in Phase 2 or treat all Phase 2 runs as implicit dry-run.
   - Recommendation: Treat `--system-caches` (without `--dry-run`) as scan-and-report. The output is the same either way. When `--dry-run` is provided, add a "(dry run)" label to the output header. This avoids confusing users who don't know about the flag yet.

## Sources

### Primary (HIGH confidence)
- [Go filepath.WalkDir](https://pkg.go.dev/path/filepath#WalkDir) -- WalkDir function signature and behavior, symlink handling
- [Go io/fs.WalkDir](https://pkg.go.dev/io/fs#WalkDir) -- WalkDirFunc type, SkipDir/SkipAll, error handling
- [Go os.ReadDir](https://pkg.go.dev/os#ReadDir) -- DirEntry-based directory enumeration
- [Go os.UserHomeDir](https://pkg.go.dev/os#UserHomeDir) -- Home directory resolution on macOS
- [fatih/color GitHub](https://github.com/fatih/color) -- TTY auto-detection, NO_COLOR support, SprintFunc API
- [Cobra User Guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md) -- PersistentFlags vs Flags, BoolVar patterns
- Local macOS verification -- `~/Library/Caches` structure, `~/Library/Logs` structure, QuickLook cache path at `DARWIN_USER_CACHE_DIR/com.apple.quicklook.ThumbnailsAgent/`

### Secondary (MEDIUM confidence)
- [YourBasic Go: Format byte size](https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/) -- SI and IEC size formatting functions
- [mac-cleanup-go GitHub](https://github.com/2ykwang/mac-cleanup-go) -- Reference implementation of Go macOS cleaner with parallel scanning
- [Apple: How to stop and disable Quick Look cache in macOS](https://appleinsider.com/inside/macos/tips/how-to-stop-and-disable-quick-look-cache-in-macos) -- QuickLook cache location and `qlmanage -r cache` command
- [OSXDaily: Clear Quick Look Cache](https://osxdaily.com/2018/08/21/clear-quick-look-cache-mac/) -- QuickLook cache path discovery via `$TMPDIR/../C/`
- [iBoysoft: Library Caches on Mac](https://iboysoft.com/wiki/library-caches-mac.html) -- Which caches are safe to delete

### Verified Locally (HIGH confidence)
- `~/Library/Caches/` contains mix of directories and files (PNG, databases) -- verified on test machine
- `~/Library/Logs/` contains subdirectories and `.log` files -- verified on test machine
- QuickLook cache at `/var/folders/<id>/C/com.apple.quicklook.ThumbnailsAgent/` -- verified via `TMPDIR` manipulation
- `getconf DARWIN_USER_CACHE_DIR` returns `/var/folders/<id>/C/` -- verified
- `qlmanage -r cache` resets the QuickLook thumbnail disk cache -- verified from `qlmanage --help`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- Go stdlib (`io/fs`, `os`, `filepath`) is well-documented and stable. `fatih/color` is the standard Go color library.
- Architecture: HIGH -- Directory scanning with `WalkDir` is a well-understood pattern. Scanner/result type separation is standard Go practice.
- Pitfalls: HIGH -- Verified locally that `~/Library/Caches` contains non-directory entries, QuickLook cache path is discoverable, and `WalkDir` skips symlinks by default.
- macOS paths: HIGH -- All three scan targets verified on local macOS machine.

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, macOS cache locations rarely change)
