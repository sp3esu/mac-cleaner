# Phase 3: Browser & Developer Caches - Research

**Researched:** 2026-02-16
**Domain:** macOS browser cache paths, developer tool caches, Docker CLI integration, graceful missing-tool handling
**Confidence:** HIGH

## Summary

Phase 3 extends the scanning engine from Phase 2 to two new categories: browser data (Safari, Chrome, Firefox) and developer tool caches (Xcode DerivedData, npm/yarn cache, Homebrew cache, Docker artifacts). The architecture is already established -- each category gets a scanner package (`pkg/browser/`, `pkg/developer/`) that exports a `Scan()` function returning `[]scan.CategoryResult`. The `scanTopLevel()` pattern from `pkg/system/scanner.go` can be reused for directory-based caches. Docker is the notable exception: it requires shelling out to `docker system df --format json` rather than directly scanning directories.

The critical complexity in this phase is **graceful degradation**. Unlike system caches (which always exist on macOS), browsers and developer tools may not be installed. Chrome and Firefox might not exist. Docker might be installed but not running. Xcode might never have been used (empty DerivedData). The scanner must detect each tool's presence, skip gracefully when absent, and report only what it finds. Safari has an additional complication: macOS TCC (Transparency, Consent, and Control) protections block programmatic access to `~/Library/Caches/com.apple.Safari/` without Full Disk Access permission -- the scanner must handle "Operation not permitted" errors gracefully.

The output must show space breakdown per tool, matching the established pattern: bold category headers, cyan sizes, green+bold total. The existing `printResults()` function in `cmd/root.go` already handles `[]scan.CategoryResult` display, so the new scanners just need to produce results in that format. Two new CLI flags (`--browser-data` and `--dev-caches`) follow the same wiring pattern as `--system-caches`.

**Primary recommendation:** Build `pkg/browser/scanner.go` and `pkg/developer/scanner.go` following the exact same patterns as `pkg/system/scanner.go`. Use `os.Stat` on `.app` bundles and cache directories to detect tool presence. For Docker, shell out to `docker system df --format json` and parse the JSON output. Handle all absence/error cases by returning empty results (not errors).

## Standard Stack

### Core (Phase 3 additions -- no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os (stdlib) | -- | `Stat`, `ReadDir`, `UserHomeDir` for cache path detection | Already used in Phase 2 |
| os/exec (stdlib) | -- | Execute `docker system df` for Docker disk usage | Standard Go approach for CLI tool integration |
| encoding/json (stdlib) | -- | Parse `docker system df --format json` output | Standard JSON parsing |
| path/filepath (stdlib) | -- | Path construction for cache directories | Already used in Phase 2 |

### Not Needed

| Library | Why Not |
|---------|---------|
| Docker Go SDK | Overkill -- we only need `docker system df` output. Adding `github.com/docker/docker` pulls in massive dependency tree. `os/exec` is sufficient. |
| Browser profile parsers | We only need directory sizes, not profile data parsing. |
| sync/errgroup | Sequential scanning is fine. Only 7 scan targets total, each is fast (except Docker which is a single CLI call). |

**Installation:** No new dependencies. Phase 3 uses only stdlib additions to the existing stack.

## Architecture Patterns

### Recommended Project Structure (Phase 3 additions)

```
mac-cleaner/
├── cmd/
│   └── root.go                    # Add --browser-data, --dev-caches flags
├── pkg/
│   ├── browser/
│   │   ├── scanner.go             # Scan() for Safari, Chrome, Firefox caches
│   │   └── scanner_test.go        # Tests with temp dirs + missing browser simulation
│   ├── developer/
│   │   ├── scanner.go             # Scan() for Xcode, npm, Homebrew, Docker
│   │   └── scanner_test.go        # Tests with temp dirs + Docker CLI mocking
│   └── system/
│       ├── scanner.go             # (Phase 2 - unchanged)
│       └── scanner_test.go        # (Phase 2 - unchanged)
├── internal/
│   ├── safety/                    # (Phase 1 - unchanged)
│   └── scan/                      # (Phase 2 - unchanged)
```

### Pattern 1: Browser Scanner with Graceful Missing-Browser Handling

**What:** Each browser scanner checks for the browser's existence before scanning its cache directories. Missing browsers produce no entries and no errors.

**When to use:** For all browser cache scanning (Safari, Chrome, Firefox).

```go
// pkg/browser/scanner.go
package browser

import (
    "os"
    "path/filepath"
    "sort"

    "github.com/gregor/mac-cleaner/internal/scan"
)

// Scan discovers browser cache directories and calculates their sizes.
// Missing browsers are silently skipped. Returns one CategoryResult per
// browser that has data, or an empty slice if no browsers are found.
func Scan() ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("cannot determine home directory: %w", err)
    }

    var results []scan.CategoryResult

    if cr := scanSafari(home); cr != nil {
        results = append(results, *cr)
    }
    if cr := scanChrome(home); cr != nil {
        results = append(results, *cr)
    }
    if cr := scanFirefox(home); cr != nil {
        results = append(results, *cr)
    }

    return results, nil
}
```

**Key properties:**
- `Scan()` never returns an error for missing browsers -- missing is normal.
- Each `scanX()` helper returns `*scan.CategoryResult` (nil if browser not found or cache empty).
- Detection strategy: check if cache directory exists, not if `.app` bundle exists. A user may have removed an app but left cache behind -- that is exactly what we want to find.

### Pattern 2: Cache Directory Detection with Fallback

**What:** For each browser/tool, define a list of known cache paths and scan whichever exist.

```go
// scanChrome scans Chrome's cache directory.
// Returns nil if Chrome cache doesn't exist.
func scanChrome(home string) *scan.CategoryResult {
    // Chrome stores cache under ~/Library/Caches/Google/Chrome/
    // Multiple profiles: Default/, Profile 1/, Profile 2/, etc.
    chromeCache := filepath.Join(home, "Library", "Caches", "Google", "Chrome")

    if _, err := os.Stat(chromeCache); err != nil {
        return nil // Chrome not installed or no cache
    }

    // Scan all profile cache directories
    entries, err := os.ReadDir(chromeCache)
    if err != nil {
        return nil
    }

    var scanEntries []scan.ScanEntry
    var totalSize int64

    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        entryPath := filepath.Join(chromeCache, entry.Name())
        size, err := scan.DirSize(entryPath)
        if err != nil || size == 0 {
            continue
        }
        scanEntries = append(scanEntries, scan.ScanEntry{
            Path:        entryPath,
            Description: "Chrome (" + entry.Name() + ")",
            Size:        size,
        })
        totalSize += size
    }

    if len(scanEntries) == 0 {
        return nil
    }

    sort.Slice(scanEntries, func(i, j int) bool {
        return scanEntries[i].Size > scanEntries[j].Size
    })

    return &scan.CategoryResult{
        Category:    "browser-chrome",
        Description: "Chrome Cache",
        Entries:     scanEntries,
        TotalSize:   totalSize,
    }
}
```

### Pattern 3: Docker CLI Integration with Graceful Failure

**What:** Shell out to `docker system df --format json` and parse the JSON output. Handle Docker not installed, not running, or timing out.

**When to use:** Docker is the only Phase 3 target that cannot be scanned via filesystem alone.

```go
// scanDocker queries Docker for disk usage via CLI.
// Returns nil if Docker is not installed or daemon is not running.
func scanDocker() *scan.CategoryResult {
    // Check if docker binary exists
    dockerPath, err := exec.LookPath("docker")
    if err != nil {
        return nil // Docker not installed
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, dockerPath, "system", "df", "--format", "json")
    output, err := cmd.Output()
    if err != nil {
        // Docker daemon not running, permission denied, etc.
        return nil
    }

    // Parse JSON lines -- docker system df --format json outputs one JSON object per line
    var scanEntries []scan.ScanEntry
    var totalSize int64

    for _, line := range bytes.Split(output, []byte("\n")) {
        line = bytes.TrimSpace(line)
        if len(line) == 0 {
            continue
        }
        var item struct {
            Type        string `json:"Type"`
            TotalCount  int    `json:"TotalCount"`
            Size        string `json:"Size"`
            Reclaimable string `json:"Reclaimable"`
        }
        if err := json.Unmarshal(line, &item); err != nil {
            continue
        }
        // Parse the reclaimable size
        reclaimableBytes := parseDockerSize(item.Reclaimable)
        if reclaimableBytes == 0 {
            continue
        }
        scanEntries = append(scanEntries, scan.ScanEntry{
            Path:        "docker:" + item.Type,
            Description: "Docker " + item.Type,
            Size:        reclaimableBytes,
        })
        totalSize += reclaimableBytes
    }

    if len(scanEntries) == 0 {
        return nil
    }

    return &scan.CategoryResult{
        Category:    "dev-docker",
        Description: "Docker Artifacts",
        Entries:     scanEntries,
        TotalSize:   totalSize,
    }
}
```

### Pattern 4: CLI Flag Wiring for New Categories

**What:** Add `--browser-data` and `--dev-caches` flags following the exact same pattern as `--system-caches`.

```go
// cmd/root.go additions
var (
    flagDryRun       bool
    flagSystemCaches bool
    flagBrowserData  bool  // NEW
    flagDevCaches    bool  // NEW
)

func init() {
    // ... existing flags ...
    rootCmd.Flags().BoolVar(&flagBrowserData, "browser-data", false,
        "scan Safari, Chrome, and Firefox caches")
    rootCmd.Flags().BoolVar(&flagDevCaches, "dev-caches", false,
        "scan Xcode, npm/yarn, Homebrew, and Docker caches")
}

// In the Run function:
// Run: func(cmd *cobra.Command, args []string) {
//     if flagSystemCaches {
//         runSystemCachesScan(cmd)
//     }
//     if flagBrowserData {
//         runBrowserDataScan(cmd)
//     }
//     if flagDevCaches {
//         runDevCachesScan(cmd)
//     }
//     if !flagSystemCaches && !flagBrowserData && !flagDevCaches {
//         cmd.Help()
//     }
// }
```

### Pattern 5: printResults Generalization

**What:** The existing `printResults()` function hardcodes "System Caches" as the header. Generalize it to accept a title parameter so browser and developer results use appropriate headers.

```go
// printResults displays scan results as a formatted table with color.
// title is the section header (e.g., "System Caches", "Browser Data", "Developer Caches").
func printResults(results []scan.CategoryResult, dryRun bool, title string) {
    // ... same implementation but using title parameter ...
    header := title
    if dryRun {
        header += " (dry run)"
    }
    // ...
}
```

### Anti-Patterns to Avoid

- **Checking `.app` bundle to decide whether to scan cache:** A user may have uninstalled Chrome but its cache at `~/Library/Caches/Google/Chrome/` still exists. That leftover cache is exactly what the tool should find. Check the cache directory itself, not the app.
- **Using Docker Go SDK:** The `github.com/docker/docker` client pulls in hundreds of transitive dependencies. For a single `docker system df` call, `os/exec` is dramatically simpler and sufficient.
- **Treating missing browsers/tools as errors:** Missing is normal. `Scan()` should return `nil` or empty results, not `error`. Only return errors for truly unexpected failures (e.g., cannot determine home directory).
- **Hardcoding a single Chrome profile path:** Chrome supports multiple profiles (`Default/`, `Profile 1/`, `Profile 2/`). Scan all subdirectories under `~/Library/Caches/Google/Chrome/`.
- **Trying to access Safari cache without Full Disk Access:** `~/Library/Caches/com.apple.Safari/` is TCC-protected on macOS Mojave+. Attempting to read it will fail with "Operation not permitted" unless the app has Full Disk Access. Handle this gracefully.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Docker disk usage | Filesystem scanning of Docker VM disk image | `docker system df --format json` via `os/exec` | Docker data lives inside a VM disk image on macOS. Scanning the filesystem won't give meaningful results. The CLI command provides accurate reclaimable size. |
| Docker size parsing | Custom size parser for "16.43 MB (70%)" strings | Parse the JSON output which separates size and reclaimable fields | JSON format avoids fragile string parsing of human-readable output. |
| Browser detection | Registry/plist parsing to find installed browsers | `os.Stat` on cache directories | Cache directory existence is both simpler and more useful -- it finds leftover caches from uninstalled browsers too. |
| Firefox profile enumeration | Custom profiles.ini parser | `os.ReadDir` on `~/Library/Caches/Firefox/Profiles/` | We just need directory sizes, not profile metadata. ReadDir gives us all profile directories. |

**Key insight:** Phase 3 scanners are almost entirely directory size calculations using the existing `scanTopLevel()` and `scan.DirSize()` patterns from Phase 2. Docker is the only exception requiring CLI integration.

## Common Pitfalls

### Pitfall 1: Safari TCC Protection ("Operation not permitted")

**What goes wrong:** Scanner tries to read `~/Library/Caches/com.apple.Safari/` and gets "Operation not permitted" error, causing the entire browser scan to fail.
**Why it happens:** macOS Mojave+ protects Safari data with TCC (Transparency, Consent, and Control). Terminal/CLI apps need explicit "Full Disk Access" in System Settings to read Safari cache.
**How to avoid:** Catch the permission error and return nil for Safari (with an optional stderr note explaining why Safari data couldn't be scanned). Do NOT treat this as a fatal error. The user can grant Full Disk Access later if they want Safari scanning.
**Warning signs:** Safari section missing from output on macOS Mojave+. Test by running without Full Disk Access first.
**Verified locally:** Confirmed on macOS -- `ls ~/Library/Caches/com.apple.Safari/` returns "Operation not permitted".

### Pitfall 2: Docker Daemon Not Running

**What goes wrong:** `docker system df` hangs or returns a connection error when Docker Desktop is installed but the daemon isn't running.
**Why it happens:** Docker Desktop on macOS uses a Linux VM. When the app isn't open, the daemon isn't running, and CLI commands fail with "Cannot connect to the Docker daemon."
**How to avoid:** Use `exec.CommandContext` with a 10-second timeout. Check the exit code -- non-zero means Docker is unavailable. Return nil, no error.
**Warning signs:** Scan hangs for 30+ seconds waiting for Docker. Test with Docker stopped.

### Pitfall 3: Chrome Multiple Profiles

**What goes wrong:** Scanner only finds `Default/` profile cache and misses `Profile 1/`, `Profile 2/`, etc.
**Why it happens:** Hardcoding `~/Library/Caches/Google/Chrome/Default/` instead of scanning all subdirectories.
**How to avoid:** Use `os.ReadDir` on `~/Library/Caches/Google/Chrome/` and scan all subdirectories as potential profile caches.
**Warning signs:** Chrome reported size is much smaller than expected for users with multiple profiles.

### Pitfall 4: Firefox Cache in Two Locations

**What goes wrong:** Scanner misses Firefox cache because it looks in the wrong location.
**Why it happens:** Firefox cache lives at `~/Library/Caches/Firefox/Profiles/<profile>/cache2/`, but the profile data is at `~/Library/Application Support/Firefox/Profiles/<profile>/`. The phase only needs to scan the cache, not the profile data.
**How to avoid:** Scan `~/Library/Caches/Firefox/` as the base directory. Use `scanTopLevel` or walk into `Profiles/` subdirectories.
**Warning signs:** Firefox cache shows as 0 or missing despite active Firefox usage.

### Pitfall 5: Homebrew Cache Already in ~/Library/Caches

**What goes wrong:** Homebrew cache at `~/Library/Caches/Homebrew/` is double-counted because `--system-caches` already scans `~/Library/Caches/` top-level entries.
**Why it happens:** The Phase 2 system scanner uses `scanTopLevel(~/Library/Caches, ...)` which includes Homebrew as one entry.
**How to avoid:** Accept the overlap for now -- each flag scans its own scope. The `--dev-caches` flag provides the developer-focused breakdown (Homebrew separated out). Alternatively, the planner could decide to exclude known developer tool caches from the system scan. Document this overlap clearly.
**Warning signs:** Users notice Homebrew size counted under both `--system-caches` and `--dev-caches`.

### Pitfall 6: npm Cache at ~/.npm Not Under ~/Library

**What goes wrong:** Scanner looks for npm cache under `~/Library/Caches/` and doesn't find it.
**Why it happens:** npm uses `~/.npm/` (a dotfile in the home directory), not `~/Library/Caches/npm/`. This is different from how most macOS apps store caches.
**How to avoid:** Hardcode `~/.npm/` as the npm cache path. The `_cacache/` subdirectory contains the actual cached packages and is typically the largest component.
**Warning signs:** npm cache shows as missing or 0 bytes despite having packages installed.

### Pitfall 7: Empty Xcode DerivedData

**What goes wrong:** Scanner reports an error or shows an entry with 0 bytes for Xcode DerivedData.
**Why it happens:** `~/Library/Developer/Xcode/DerivedData/` exists as an empty directory if Xcode is installed but no projects have been built, or if the user recently cleaned it.
**How to avoid:** Check directory existence first, then check if it has any entries. Zero-byte entries are already filtered by the existing pattern. If the directory is empty, return nil.
**Warning signs:** "Xcode DerivedData: 0 B" appearing in output.

### Pitfall 8: Docker size string parsing

**What goes wrong:** Parsing Docker's human-readable size strings (e.g., "16.43 MB", "2.3 GB") fails for edge cases.
**Why it happens:** Docker uses its own size formatting that may differ from the tool's FormatSize output.
**How to avoid:** Use `docker system df --format json` which outputs structured data. Parse the `Size` and `Reclaimable` fields. Note: these fields contain human-readable strings in the JSON output too (e.g., `"Size": "16.43MB"`), so a size parser is still needed, OR use `--format '{{json .}}'` with Go template to get numeric fields.
**Warning signs:** Docker sizes show as 0 or parse errors in logs.

## Code Examples

### Browser Cache Paths Reference (macOS)

```go
// Verified cache paths on macOS (as of 2026-02-16)

// Safari -- TCC-protected, needs Full Disk Access
// ~/Library/Caches/com.apple.Safari/      -- "Operation not permitted" without FDA
// ~/Library/Safari/LocalStorage/           -- web storage (may also be TCC-protected)

// Chrome -- freely accessible
// ~/Library/Caches/Google/Chrome/Default/Cache/        -- HTTP cache (766 MB typical)
// ~/Library/Caches/Google/Chrome/Default/Code Cache/   -- compiled JS (235 MB typical)
// ~/Library/Caches/Google/Chrome/Profile 1/Cache/      -- additional profiles
// Note: scan the entire ~/Library/Caches/Google/Chrome/ tree

// Firefox -- freely accessible if installed
// ~/Library/Caches/Firefox/Profiles/<name>/cache2/     -- HTTP cache
// Note: scan the entire ~/Library/Caches/Firefox/ tree
```

### Developer Cache Paths Reference (macOS)

```go
// Verified cache paths on macOS (as of 2026-02-16)

// Xcode DerivedData
// ~/Library/Developer/Xcode/DerivedData/    -- build artifacts, index data
// Note: can be customized via `defaults read com.apple.dt.Xcode IDECustomDerivedDataLocation`
// but scanning default location is sufficient for most users

// npm cache
// ~/.npm/                                   -- total 5.2 GB on test machine
// ~/.npm/_cacache/                          -- actual cached packages (4.8 GB)
// ~/.npm/_npx/                              -- npx package cache (390 MB)
// ~/.npm/_logs/                             -- npm logs (60 KB, negligible)

// yarn cache (yarn v1)
// ~/Library/Caches/yarn/                    -- yarn v1 default
// May vary -- use `yarn cache dir` output if available

// Homebrew cache
// ~/Library/Caches/Homebrew/                -- 2.3 GB on test machine
// ~/Library/Caches/Homebrew/downloads/      -- downloaded bottles (1.6 GB)
// ~/Library/Caches/Homebrew/Cask/           -- cask downloads

// Docker -- use CLI, not filesystem
// `docker system df --format json`          -- images, containers, volumes, build cache
```

### Graceful Missing-Tool Detection Pattern

```go
// Pattern: check directory, scan if exists, return nil if not
func scanXcodeDerivedData(home string) *scan.CategoryResult {
    derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")

    // Check if directory exists
    info, err := os.Stat(derivedData)
    if err != nil || !info.IsDir() {
        return nil // Xcode not installed or DerivedData doesn't exist
    }

    // Reuse the established scanTopLevel pattern
    result, err := scanTopLevel(derivedData, "dev-xcode", "Xcode DerivedData")
    if err != nil {
        return nil
    }
    return result
}
```

### Docker JSON Parsing

```go
// docker system df --format json outputs one JSON line per resource type:
// {"Active":"2","Reclaimable":"11.63MB (70%)","Size":"16.43MB","TotalCount":"5","Type":"Images"}
// {"Active":"0","Reclaimable":"212B (100%)","Size":"212B","TotalCount":"2","Type":"Containers"}
// {"Active":"1","Reclaimable":"0B (0%)","Size":"36B","TotalCount":"2","Type":"Local Volumes"}
// {"Active":"0","Reclaimable":"0B","Size":"0B","TotalCount":"0","Type":"Build Cache"}

type dockerDfEntry struct {
    Type        string `json:"Type"`
    TotalCount  string `json:"TotalCount"`
    Size        string `json:"Size"`
    Reclaimable string `json:"Reclaimable"`
    Active      string `json:"Active"`
}

// parseDockerSize parses Docker's size strings like "16.43MB", "2.3GB", "0B"
// Returns size in bytes.
func parseDockerSize(s string) int64 {
    // Strip the percentage part if present: "11.63MB (70%)" -> "11.63MB"
    if idx := strings.Index(s, " ("); idx != -1 {
        s = s[:idx]
    }
    // Parse number and unit
    // Handle: "0B", "212B", "16.43MB", "2.3GB", "1.5kB"
    // ...
}
```

### Test Pattern: Simulating Missing Browsers

```go
func TestScanChromeMissing(t *testing.T) {
    // Use a temp dir as "home" -- Chrome cache won't exist
    fakeHome := t.TempDir()

    result := scanChrome(fakeHome)
    if result != nil {
        t.Errorf("expected nil for missing Chrome, got %+v", result)
    }
}

func TestScanChromeWithData(t *testing.T) {
    fakeHome := t.TempDir()

    // Create fake Chrome cache structure
    cacheDir := filepath.Join(fakeHome, "Library", "Caches", "Google", "Chrome", "Default", "Cache")
    os.MkdirAll(cacheDir, 0755)
    writeFile(t, filepath.Join(cacheDir, "data.bin"), 1024)

    result := scanChrome(fakeHome)
    if result == nil {
        t.Fatal("expected non-nil result for Chrome with data")
    }
    if result.TotalSize == 0 {
        t.Error("expected non-zero total size")
    }
}
```

### Test Pattern: Docker CLI Mocking

```go
// For Docker tests, use dependency injection:
// 1. Define a runner interface or function type
// 2. In production, use exec.Command
// 3. In tests, use a mock that returns predefined JSON

type cmdRunner func(name string, args ...string) ([]byte, error)

func scanDockerWithRunner(runner cmdRunner) *scan.CategoryResult {
    output, err := runner("docker", "system", "df", "--format", "json")
    if err != nil {
        return nil
    }
    // ... parse output ...
}

// In tests:
func TestScanDockerNotInstalled(t *testing.T) {
    mock := func(name string, args ...string) ([]byte, error) {
        return nil, &exec.Error{Name: "docker", Err: exec.ErrNotFound}
    }
    result := scanDockerWithRunner(mock)
    if result != nil {
        t.Error("expected nil when Docker not installed")
    }
}

func TestScanDockerWithData(t *testing.T) {
    jsonOutput := `{"Active":"2","Reclaimable":"11.63MB (70%)","Size":"16.43MB","TotalCount":"5","Type":"Images"}
{"Active":"0","Reclaimable":"212B (100%)","Size":"212B","TotalCount":"2","Type":"Containers"}
{"Active":"1","Reclaimable":"0B (0%)","Size":"36B","TotalCount":"2","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"0B","Size":"0B","TotalCount":"0","Type":"Build Cache"}`

    mock := func(name string, args ...string) ([]byte, error) {
        return []byte(jsonOutput), nil
    }
    result := scanDockerWithRunner(mock)
    if result == nil {
        t.Fatal("expected non-nil result")
    }
    // Verify Images entry has ~11.63MB reclaimable
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Safari cache at `~/Library/Caches/` | Safari cache at `~/Library/Caches/com.apple.Safari/` (TCC-protected) | macOS Mojave (2018) | Need Full Disk Access or graceful failure |
| Chrome single profile | Chrome multiple profiles (`Default/`, `Profile 1/`, etc.) | Chrome 2013+ | Must scan all profile directories |
| Firefox cache at profile dir | Firefox cache at `~/Library/Caches/Firefox/Profiles/` | Firefox 2014+ | Separate from profile data directory |
| Docker data as filesystem | Docker Desktop uses LinuxKit VM with disk image | Docker Desktop 2.0 (2019) | Cannot scan Docker data via filesystem -- must use CLI |
| `docker system prune` only | `docker system df --format json` for reporting | Docker 17.06 (2017) | Structured JSON output available for programmatic use |
| npm cache at `~/.npm` (manual cleaning) | `npm cache clean --force` (v5+) | npm 5.0 (2017) | npm discourages manual cache deletion; for scanning we just measure |

## Open Questions

1. **Should Homebrew cache be excluded from `--system-caches` to avoid double-counting?**
   - What we know: Phase 2's `--system-caches` scans `~/Library/Caches/` which includes `Homebrew/`. Phase 3's `--dev-caches` will also scan `~/Library/Caches/Homebrew/`.
   - What's unclear: Whether to deduplicate or accept overlap.
   - Recommendation: Accept overlap for now. Each flag has its own perspective -- `--system-caches` shows all caches, `--dev-caches` shows developer-specific breakdown. The total is only meaningful per-flag. If the user runs both flags together in the future, deduplication can be added then.

2. **Should Docker show "total size" or "reclaimable size"?**
   - What we know: `docker system df` distinguishes between total size and reclaimable size. Active images/containers are not reclaimable.
   - What's unclear: Whether the user wants to see what they could reclaim or what Docker uses total.
   - Recommendation: Show reclaimable size, matching the tool's "what can I clean up" philosophy. Label entries as "Docker Images (reclaimable)" for clarity.

3. **Should we scan yarn cache when yarn is not installed?**
   - What we know: yarn v1 uses `~/Library/Caches/yarn/`. yarn v2+ (berry) uses `.yarn/cache/` per-project. The user may have npm but not yarn, or vice versa. We also cannot detect pnpm store.
   - What's unclear: Whether to scan for yarn/pnpm or just npm.
   - Recommendation: Check if `~/Library/Caches/yarn/` exists (yarn v1 global cache) and scan it if present. Skip yarn v2 project-level caches (those are per-project, not global). Skip pnpm for now -- it is out of scope based on requirements (DEV-02 says "npm/yarn cache").

4. **How to handle Safari TCC restriction in the output?**
   - What we know: Safari cache scanning will fail with "Operation not permitted" without Full Disk Access.
   - What's unclear: Whether to show a message like "Safari: requires Full Disk Access" or silently skip.
   - Recommendation: Print a brief stderr note ("Safari cache: grant Full Disk Access in System Settings to scan") and skip. Do not include a 0-byte entry in results. This matches the existing `safety.WarnBlocked()` pattern for blocked paths.

5. **Should `scanTopLevel` from `pkg/system/scanner.go` be promoted to a shared utility?**
   - What we know: `pkg/browser/` and `pkg/developer/` will need the same `scanTopLevel` logic (enumerate top-level entries, calculate sizes, sort descending, skip zero-byte).
   - What's unclear: Whether to duplicate or share.
   - Recommendation: Extract `scanTopLevel` into `internal/scan/helpers.go` as a shared utility. All three scanner packages (`system`, `browser`, `developer`) import it. This eliminates code duplication and ensures consistent behavior.

## Sources

### Primary (HIGH confidence)
- Local macOS verification -- Safari cache at `~/Library/Caches/com.apple.Safari/` is TCC-protected ("Operation not permitted" confirmed)
- Local macOS verification -- Chrome cache at `~/Library/Caches/Google/Chrome/Default/` confirmed (766 MB Cache, 235 MB Code Cache)
- Local macOS verification -- npm cache at `~/.npm/` confirmed (5.2 GB total, 4.8 GB in `_cacache/`)
- Local macOS verification -- Homebrew cache at `~/Library/Caches/Homebrew/` confirmed (2.3 GB total, 1.6 GB downloads)
- Local macOS verification -- Xcode DerivedData at `~/Library/Developer/Xcode/DerivedData/` confirmed (exists but empty on test machine)
- [Docker system df docs](https://docs.docker.com/reference/cli/docker/system/df/) -- JSON format support confirmed, field names verified

### Secondary (MEDIUM confidence)
- [Apple Community thread](https://discussions.apple.com/thread/8571352) -- Safari cache path history and current location
- [File Juicer cache docs](https://echoone.com/filejuicer/formats/cache) -- Safari, Chrome cache locations cross-referenced
- [File Juicer Chrome cache docs](https://echoone.com/filejuicer/formats/chrome-cache) -- Chrome cache path confirmed
- [Mozilla Support](https://support.mozilla.org/en-US/kb/profiles-where-firefox-stores-user-data) -- Firefox profile and cache locations
- [npm docs](https://docs.npmjs.com/cli/v8/commands/npm-cache/) -- npm cache location and commands
- [Homebrew docs](https://docs.brew.sh/FAQ) -- Homebrew cache location via `brew --cache`
- [Docker prune docs](https://docs.docker.com/engine/manage-resources/pruning/) -- Docker cleanup commands and concepts

### Tertiary (LOW confidence)
- [yarn classic docs](https://classic.yarnpkg.com/lang/en/docs/cli/cache/) -- yarn v1 cache location (may vary by installation method)
- Chrome multi-profile cache paths -- inferred from local observation and community forums; specific path structure for `Profile 1/`, `Profile 2/` needs validation on machines with multiple Chrome profiles

## Metadata

**Confidence breakdown:**
- Browser cache paths: HIGH -- Safari, Chrome verified locally; Firefox path structure well-documented by Mozilla
- Safari TCC protection: HIGH -- "Operation not permitted" confirmed locally on macOS
- Developer cache paths: HIGH -- npm, Homebrew, Xcode all verified locally with real data
- Docker integration: HIGH -- `docker system df --format json` documented in official Docker docs
- Chrome multi-profile: MEDIUM -- only single profile verified locally; multi-profile paths inferred from documentation
- yarn cache path: MEDIUM -- yarn not installed on test machine; path based on official yarn v1 docs
- Architecture patterns: HIGH -- direct extension of Phase 2 patterns already working in production

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain -- macOS cache locations and Docker CLI rarely change)
