# Phase 7: Safety Enforcement - Research

**Researched:** 2026-02-16
**Domain:** Risk categorization, macOS permission handling, graceful error reporting
**Confidence:** HIGH

## Summary

Phase 7 adds two capabilities to the existing mac-cleaner tool: (1) risk-level categorization for every scan item (SAFE-04), and (2) graceful permission error reporting without aborting scans (SAFE-03). Both requirements are well-scoped additions to the existing architecture. The tool already has the SIP/swap safety layer (Phase 1), four working scanners, interactive/flag-based modes, and cleanup execution. Phase 7 layers risk metadata on top of scan results and hardens error paths.

Risk categorization requires adding a `RiskLevel` field to `scan.ScanEntry` (the per-item type, not `CategoryResult`). Each scanner assigns risk levels at scan time based on a hardcoded mapping of category IDs to risk levels. The display layer then highlights risky items in the output and the confirmation/interactive flow warns about risky items before proceeding. Permission error reporting requires wrapping existing scanner error paths to collect permission-denied entries into a new `PermissionIssue` structure that is displayed after scanning completes, without aborting the scan.

The key architectural insight is that risk levels are a property of the *item type* (what category it belongs to), not the individual file. All entries in `dev-xcode` are "risky" because Xcode DerivedData regenerates but its deletion can cause multi-hour rebuilds. All entries in `system-caches` are "safe" because user app caches regenerate automatically. This makes the mapping a simple category-to-risk lookup, not per-file analysis.

**Primary recommendation:** Add a `RiskLevel string` field to `ScanEntry` with three values: `"safe"`, `"moderate"`, `"risky"`. Each scanner sets this field based on its category. The display layer uses color coding (green=safe, yellow=moderate, red=risky) and the deletion flow requires explicit confirmation for results containing risky items. Permission errors are collected into a `[]PermissionIssue` slice and reported to stderr after scanning, without failing the overall scan.

## Standard Stack

### Core (no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os (stdlib) | -- | `os.IsPermission(err)` for detecting permission-denied errors | Already used throughout codebase. Checks both EACCES and EPERM. |
| errors (stdlib) | -- | `errors.Is(err, fs.ErrPermission)` as modern alternative to `os.IsPermission` | Preferred over `os.IsPermission` per Go documentation. |
| fmt (stdlib) | -- | Stderr output for permission warnings | Already used for all error output. |
| fatih/color v1.18.0 | already in go.mod | Color coding: green (safe), yellow (moderate), red (risky) | Already used for bold headers, cyan sizes, green totals. |

### Not Needed

| Library | Why Not |
|---------|---------|
| Any new dependency | Risk categorization and permission error reporting are pure logic changes to existing code. No new libraries required. |

**Installation:** No new dependencies.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| String risk levels ("safe"/"moderate"/"risky") | Integer enum (0/1/2) or Go iota const | Strings are human-readable in JSON output without a mapping layer. The prior decision says "Category field is plain string (not enum) for extensibility" -- risk level should follow the same pattern. |
| Per-item risk on ScanEntry | Per-category risk on CategoryResult | Per-item granularity allows future mixed-risk categories (e.g., some browser items safe, some moderate). Even if current mapping is 1:1 with categories, the field should live on the item. |
| Hardcoded risk mapping | Config file risk mapping | Prior decision: "Core safety protections are hardcoded -- no config can override them." Risk categorization is a safety feature and should follow the same pattern. |

## Architecture Patterns

### Recommended Changes (Phase 7)

```
mac-cleaner/
├── internal/
│   └── scan/
│       └── types.go           # Add: RiskLevel field to ScanEntry
│                               #       PermissionIssue type
│                               #       Risk level constants
│   └── safety/
│       └── risk.go            # NEW: RiskForCategory() mapping function
│       └── risk_test.go       # NEW: Table-driven tests for risk mapping
├── pkg/
│   └── system/scanner.go      # Modify: Set RiskLevel on each ScanEntry
│   └── browser/scanner.go     # Modify: Set RiskLevel on each ScanEntry
│   └── developer/scanner.go   # Modify: Set RiskLevel on each ScanEntry
│   └── appleftovers/scanner.go # Modify: Set RiskLevel on each ScanEntry
│                               #          Collect permission errors
├── cmd/
│   └── root.go                # Modify: Display risk colors, warn on risky items
│                               #          Show permission issues after scan
```

### Pattern 1: Risk Level on ScanEntry

**What:** Add a `RiskLevel` string field to the existing `ScanEntry` type. Each scanner sets it when creating entries.

**When to use:** Every scan entry gets a risk level. No entry should have an empty risk level.

**Key design decisions:**

1. **Three risk levels: "safe", "moderate", "risky"** -- This matches the requirements exactly. "safe" items regenerate automatically (caches, thumbnails). "moderate" items regenerate but may cause temporary inconvenience (browser caches clear sessions, Homebrew re-downloads). "risky" items may cause significant disruption if deleted (Xcode DerivedData triggers multi-hour rebuilds, orphaned preferences may belong to running apps, iOS backups cannot be recovered).

2. **Risk level set at entry creation, not post-hoc** -- Each scanner knows its domain and assigns risk directly. This is simpler and more reliable than a post-scan mapping pass. The `safety.RiskForCategory()` helper provides a central lookup that all scanners call.

3. **Risk level included in JSON output** -- The `json:"risk_level"` tag ensures automation consumers see the risk categorization.

4. **Risk level constants in scan package** -- Define `RiskSafe`, `RiskModerate`, `RiskRisky` as string constants to prevent typos. Keep them in `scan/types.go` alongside the existing types.

**Example:**
```go
// internal/scan/types.go
const (
    RiskSafe     = "safe"
    RiskModerate = "moderate"
    RiskRisky    = "risky"
)

type ScanEntry struct {
    Path        string `json:"path"`
    Description string `json:"description"`
    Size        int64  `json:"size"`
    RiskLevel   string `json:"risk_level"`
}
```

### Pattern 2: Category-to-Risk Mapping

**What:** A central function that maps category IDs to risk levels. Each scanner calls this when creating entries.

**When to use:** When creating ScanEntry values. The scanner knows its category ID and calls `safety.RiskForCategory("dev-xcode")` to get `"risky"`.

**Recommended risk mapping:**

| Category ID | Risk Level | Rationale |
|-------------|-----------|-----------|
| `system-caches` | safe | User app caches regenerate automatically on next app launch |
| `system-logs` | safe | Log files regenerate; old logs have no functional impact |
| `quicklook` | safe | Thumbnail cache regenerates on next file preview |
| `browser-safari` | moderate | Cache regenerates but may clear session data; TCC may block access |
| `browser-chrome` | moderate | Cache regenerates but clears session data across profiles |
| `browser-firefox` | moderate | Cache regenerates but clears session data |
| `dev-xcode` | risky | DerivedData deletion triggers full project rebuilds (can take hours) |
| `dev-npm` | moderate | npm cache can be rebuilt with `npm cache verify`; slow on large projects |
| `dev-yarn` | moderate | Yarn cache can be rebuilt; slow on large projects |
| `dev-homebrew` | moderate | Homebrew re-downloads on next install; slow on metered connections |
| `dev-docker` | risky | Docker images/containers may need to be rebuilt from scratch |
| `app-orphaned-prefs` | risky | Preferences may belong to running apps or contain user settings |
| `app-ios-backups` | risky | iOS backups cannot be regenerated; may contain irreplaceable data |
| `app-old-downloads` | moderate | User files older than 90 days; may still be wanted |

**Key design decisions:**

1. **Function, not map literal** -- Use `RiskForCategory(categoryID string) string` that returns a risk level. Internally it uses a map, but the function encapsulates the lookup and provides a safe default (`RiskModerate`) for unknown categories. This is defensive.

2. **Place in safety package** -- The `internal/safety` package already houses safety-related logic (path blocking). Risk categorization is a safety concern. Adding `risk.go` keeps safety logic together.

3. **Default to "moderate" for unknown categories** -- If a new scanner is added in a future version and the developer forgets to add a risk mapping, defaulting to "moderate" is safer than "safe" (which might skip user warnings) or "risky" (which would over-warn).

**Example:**
```go
// internal/safety/risk.go
package safety

import "github.com/gregor/mac-cleaner/internal/scan"

var categoryRisk = map[string]string{
    "system-caches":     scan.RiskSafe,
    "system-logs":       scan.RiskSafe,
    "quicklook":         scan.RiskSafe,
    "browser-safari":    scan.RiskModerate,
    "browser-chrome":    scan.RiskModerate,
    "browser-firefox":   scan.RiskModerate,
    "dev-xcode":         scan.RiskRisky,
    "dev-npm":           scan.RiskModerate,
    "dev-yarn":          scan.RiskModerate,
    "dev-homebrew":      scan.RiskModerate,
    "dev-docker":        scan.RiskRisky,
    "app-orphaned-prefs": scan.RiskRisky,
    "app-ios-backups":   scan.RiskRisky,
    "app-old-downloads": scan.RiskModerate,
}

// RiskForCategory returns the risk level for a given category ID.
// Unknown categories default to "moderate" for safety.
func RiskForCategory(categoryID string) string {
    if level, ok := categoryRisk[categoryID]; ok {
        return level
    }
    return scan.RiskModerate
}
```

### Pattern 3: Scanner Integration

**What:** Each scanner sets `RiskLevel` on every `ScanEntry` it creates, using `safety.RiskForCategory()`.

**When to use:** In every scanner function that creates `ScanEntry` values.

**Key design decision:** The category ID is already known at entry creation time (it is being set on `CategoryResult.Category`). Pass the same category ID to `RiskForCategory()`.

**Two approaches (recommend Approach B):**

**Approach A: Each scanner sets risk on every entry individually.**
```go
// In each scanner:
scanEntries = append(scanEntries, scan.ScanEntry{
    Path:        entryPath,
    Description: entry.Name(),
    Size:        size,
    RiskLevel:   safety.RiskForCategory("dev-xcode"),
})
```
Pro: Explicit. Con: Repetitive across 14 scanners, easy to forget.

**Approach B: Set risk level after entry creation, centrally per category.**
Add a helper that stamps risk levels onto a `CategoryResult`:
```go
// internal/scan/helpers.go or types.go
func (cr *CategoryResult) SetRiskLevels(riskFn func(string) string) {
    level := riskFn(cr.Category)
    for i := range cr.Entries {
        cr.Entries[i].RiskLevel = level
    }
}
```
Each scanner calls `cr.SetRiskLevels(safety.RiskForCategory)` once after building the result. This is less error-prone and keeps scanner code cleaner.

**Recommended: Approach B.** It is less invasive (one line per CategoryResult rather than one line per ScanEntry), and it guarantees every entry in a category gets the same risk level.

### Pattern 4: Permission Error Collection (SAFE-03)

**What:** When a scanner encounters a permission error while scanning a directory, it records the error rather than silently swallowing it. After all scanning completes, the tool reports all permission issues to stderr.

**When to use:** During directory scanning when `os.ReadDir`, `os.Stat`, or `filepath.WalkDir` encounters EACCES or EPERM errors.

**Key design decisions:**

1. **New `PermissionIssue` type** -- A simple struct with `Path` and `Description` fields. Collected into a slice and reported after scanning.

2. **Scanners return permission issues alongside results** -- The scanner function signatures change from `Scan() ([]scan.CategoryResult, error)` to either:
   - (a) Return `([]scan.CategoryResult, []scan.PermissionIssue, error)` -- explicit but breaks all callers.
   - (b) Add `PermissionIssues []PermissionIssue` field to a new `ScanResult` wrapper type.
   - (c) Collect permission issues in a package-level or passed-in collector.

   **Recommendation: (a) with a helper.** Change scanner signatures to return permission issues. This is explicit, testable, and the callers (4 runner functions in root.go) are easy to update. Alternatively, add `PermissionIssues` to `ScanSummary` to keep the interface cleaner. The simplest approach is to add a `PermissionIssues` field to the returned data.

   **Simplest viable approach:** Add `PermissionIssues []PermissionIssue` as a field on `CategoryResult`. Each category can report which paths it could not access. This requires no signature change. The display layer iterates over permission issues from all categories and reports them.

3. **Report at the end, not inline** -- Permission issues are collected during scanning and reported after all results are printed. This keeps the scan output clean and groups all permission information together. Format: `Note: Could not access 2 paths (permission denied):` followed by the list.

4. **Never request Full Disk Access** -- The tool detects TCC-blocked paths (Safari cache, etc.) and reports them as permission issues. It does NOT prompt the user to grant Full Disk Access. It does NOT fail. It simply reports what it could not access and moves on.

5. **Safari already handles TCC** -- The existing `scanSafari()` function already checks `os.IsPermission(err)` and prints a stderr hint. This pattern should be generalized: instead of each scanner printing its own ad-hoc permission message, all scanners feed into the centralized `PermissionIssue` collection.

**Example:**
```go
// internal/scan/types.go
type PermissionIssue struct {
    Path        string `json:"path"`
    Description string `json:"description"`
}
```

**Current state of permission handling in scanners:**

| Scanner | Current Behavior | Needed Change |
|---------|-----------------|---------------|
| `scan.ScanTopLevel` | Calls `safety.WarnBlocked` for blocked paths, silently continues for other errors | Detect `os.IsPermission` errors and collect into PermissionIssue |
| `scan.DirSize` | Silently skips permission-denied entries in WalkDir callback | This is correct for size calculation; the DirSize caller should also check top-level access |
| `scanSafari` | Prints TCC hint to stderr on permission error | Convert to PermissionIssue; remove ad-hoc stderr print |
| `scanChrome` | Silently continues on ReadDir error | Detect permission error, collect into PermissionIssue |
| `scanFirefox` | Silently continues on Stat error | Detect permission error, collect into PermissionIssue |
| `scanXcodeDerivedData` | Silently continues on Stat error | Detect permission error, collect into PermissionIssue |
| `scanNpmCache` | Silently continues on Stat error | Detect permission error, collect into PermissionIssue |
| `scanYarnCache` | Silently continues on Stat/DirSize error | Detect permission error, collect into PermissionIssue |
| `scanHomebrew` | Silently continues on Stat error | Detect permission error, collect into PermissionIssue |
| `scanDocker` | Silently continues on Docker CLI failure | No change needed (Docker errors are not permission errors in the filesystem sense) |
| `scanOrphanedPrefs` | Silently continues on ReadDir/Stat errors | Detect permission error on Preferences dir, collect into PermissionIssue |
| `scanIOSBackups` | Silently continues on Stat error | Detect permission error, collect into PermissionIssue |
| `scanOldDownloads` | Silently continues on Stat/ReadDir error | Detect permission error, collect into PermissionIssue |

### Pattern 5: Risk-Aware Display

**What:** The output layer uses color coding to indicate risk levels and adds a warning section for risky items.

**When to use:** In `printResults` (human-readable output) and `printJSON` (structured output).

**Key design decisions:**

1. **Color coding in human-readable output:**
   - Safe items: default text (no extra color) or green description
   - Moderate items: yellow risk indicator
   - Risky items: red risk indicator + bold description

2. **Risk indicator in tabwriter:** Add a risk tag next to each entry's description. Example:
   ```
   Xcode DerivedData
     MyProject-abc123def         [risky]     5.2 GB
     OtherProject-xyz789         [risky]     2.1 GB
   ```
   Or more subtly, color the size differently for risky items.

3. **Risky items warning in confirmation prompt:** Before the "Type 'yes' to proceed" prompt, add a warning line if any risky items are in the removal set:
   ```
   WARNING: 3 risky items selected for removal (Xcode DerivedData, iOS Backups).
   These items may be difficult or impossible to recover.
   ```

4. **Interactive walkthrough shows risk level:** In the per-item keep/remove prompt, display the risk level:
   ```
   [3/14] MyProject-abc123def  5.2 GB  [risky]
   keep or remove? [k/r]:
   ```

5. **JSON output includes risk_level field:** Already covered by the `json:"risk_level"` tag on ScanEntry. No additional display logic needed for JSON mode.

### Pattern 6: Permission Issue Display

**What:** After scanning completes, report all permission issues to stderr.

**When to use:** After all scanners have run and results have been printed.

**Example output:**
```
Note: 2 paths could not be accessed (permission denied):
  ~/Library/Caches/com.apple.Safari — Safari cache requires Full Disk Access
  ~/Library/Mail — Mail data requires Full Disk Access
```

**Key design decisions:**

1. **Output to stderr** -- Permission issues are informational, not scan results. They go to stderr so they do not contaminate JSON output.

2. **Include in JSON output as separate field** -- When `--json` is used, permission issues appear in the JSON under a `"permission_issues"` key. This lets automation consumers know what was skipped.

3. **Brief, actionable descriptions** -- Each permission issue includes a short explanation of why access was denied and what the user could do (e.g., "requires Full Disk Access"). But the tool itself never requests or prompts for Full Disk Access.

### Anti-Patterns to Avoid

- **Prompting for Full Disk Access:** The tool should NEVER ask the user to enable Full Disk Access. It reports what it cannot access and moves on. Users can choose to grant access independently.
- **Using sudo or escalated privileges:** The tool runs with standard user permissions only. No `sudo`, no privilege escalation.
- **Aborting scan on permission error:** A permission error in one scanner must not prevent other scanners from running. Each scanner is independent.
- **Changing risk levels via configuration:** Risk levels are hardcoded safety classifications. Following the established pattern: "Core safety protections are hardcoded -- no config can override them."
- **Empty risk level strings:** Every ScanEntry must have a non-empty RiskLevel. The `SetRiskLevels` pattern ensures this.
- **Risk level on CategoryResult instead of ScanEntry:** Risk is a per-entry property for future extensibility, even though current mapping is per-category.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Permission detection | Custom errno checking | `os.IsPermission(err)` or `errors.Is(err, fs.ErrPermission)` | Go stdlib handles both EACCES and EPERM. Covers traditional permissions and TCC blocks. |
| Risk level color mapping | Manual ANSI codes per risk level | `fatih/color` with `color.New(color.FgRed)`, `color.New(color.FgYellow)`, `color.New(color.FgGreen)` | Already in the codebase. Handles NoColor, TTY detection, etc. |
| Category-to-risk lookup | if/else chains per category | `map[string]string` with function wrapper | Data-driven approach (matches existing `buildSkipSet` pattern). One-line additions for new categories. |

**Key insight:** Phase 7 adds metadata (risk levels) and error handling (permission reporting) to an already-working system. No new algorithms, no new external tools, no new dependencies. The complexity is in the plumbing: threading the new fields through scanners, display, and confirmation flow.

## Common Pitfalls

### Pitfall 1: os.IsPermission Does Not Catch All macOS TCC Denials

**What goes wrong:** On some macOS versions, TCC denials may return unexpected error codes that `os.IsPermission()` does not recognize.
**Why it happens:** macOS TCC errors typically return EPERM (Operation not permitted), which `os.IsPermission()` does catch. However, some edge cases (particularly with sandboxed apps or specific macOS versions) may return different error types.
**How to avoid:** Use `errors.Is(err, fs.ErrPermission)` as the primary check. As a fallback, also check for the string "operation not permitted" in the error message for robustness. Test on real macOS with TCC-blocked directories (Safari cache is the easiest test case). In practice, `os.IsPermission` handles the vast majority of cases correctly since Go maps both EACCES and EPERM to permission errors.
**Warning signs:** Safari cache permission errors are not reported as permission issues but instead silently swallowed.

### Pitfall 2: Circular Import Between safety and scan Packages

**What goes wrong:** Adding `RiskForCategory()` to `internal/safety` with risk level constants in `internal/scan` creates a circular dependency: `safety` imports `scan` for constants, `scan/helpers.go` already imports `safety` for `IsPathBlocked`.
**Why it happens:** The risk level constants (`RiskSafe`, `RiskModerate`, `RiskRisky`) naturally belong in `scan` (they are part of the scan type system), but the risk mapping function naturally belongs in `safety` (it is a safety classification).
**How to avoid:** Two options:
  1. **Put constants and mapping both in `safety` package.** The `scan.ScanEntry.RiskLevel` field is just a string; the constants can live in `safety` and scanners reference `safety.RiskSafe`. This avoids the circular import entirely.
  2. **Put mapping in a new `internal/risk` package** that imports both `scan` (for constants) and is imported by scanners. Neither `scan` nor `safety` imports `risk`.
  **Recommendation: Option 1.** Keep risk constants in `safety` alongside the mapping. `ScanEntry.RiskLevel` is a plain `string`; it does not import anything. Scanners already import `safety` (via `scan/helpers.go` which calls `safety.IsPathBlocked`). Adding `safety.RiskSafe` is natural.
**Warning signs:** `go build` fails with "import cycle not allowed."

### Pitfall 3: Risky Item Warning Fatigue

**What goes wrong:** Every Xcode DerivedData subdirectory is tagged "risky," so a user with 20 Xcode projects sees 20 "[risky]" tags and a long warning section, causing them to ignore the warnings entirely.
**Why it happens:** Risk is per-entry, and Xcode DerivedData can have many entries.
**How to avoid:** In the confirmation prompt, group risky items by category: "3 risky items: 2 Xcode DerivedData entries, 1 iOS Backup" rather than listing each individually. The per-entry risk tag in the scan output is fine (users expect to see it), but the warning summary should be concise.
**Warning signs:** Users always type "yes" without reading the risky item warning.

### Pitfall 4: Permission Issues Duplicated Between Scanner and DirSize

**What goes wrong:** A scanner reports a permission issue for a directory, AND `DirSize` internally skips permission-denied files in that same directory, leading to confusing output (the directory appears in results with a partial size AND in the permission issues list).
**Why it happens:** Permission errors can occur at multiple levels: the top-level directory Stat, the ReadDir call, or individual files within a directory.
**How to avoid:** Permission issues should be collected at the scanner level (the `Stat` or `ReadDir` call), not inside `DirSize`. If the top-level directory can be accessed but some files inside cannot, `DirSize` already handles this correctly (skips inaccessible files, reports accessible size). No permission issue needs to be reported for partial access within a directory -- that is normal behavior on macOS. Only report permission issues when an entire directory cannot be accessed at all.
**Warning signs:** The same path appears in both scan results (with partial size) and permission issues.

### Pitfall 5: Breaking JSON Schema Backward Compatibility

**What goes wrong:** Adding `risk_level` and `permission_issues` to the JSON output breaks existing automation consumers that parse the JSON.
**Why it happens:** New fields added to existing types.
**How to avoid:** New fields are additive and optional. JSON parsers typically ignore unknown fields. The `risk_level` field is always present (non-null) on every entry. The `permission_issues` field on `ScanSummary` is present but may be an empty array. This is standard JSON evolution. No breaking change occurs because no existing field is removed or changed.
**Warning signs:** None expected. This is a non-issue with standard JSON parsing practices.

### Pitfall 6: Scanner Signature Change Cascading Through Tests

**What goes wrong:** Changing scanner `Scan()` return type to include `PermissionIssue` breaks all existing scanner tests.
**Why it happens:** Every test calls `Scan()` and checks the return values. Adding a new return value means every test needs updating.
**How to avoid:** If using the `PermissionIssues` field on `CategoryResult` approach (recommended), no signature changes are needed. Each `CategoryResult` optionally carries its permission issues. Existing tests that create `CategoryResult` values without permission issues continue to work (the field defaults to `nil`). Only new tests specifically for permission error collection need the field.
**Warning signs:** Dozens of test compilation errors after adding a return value.

## Code Examples

### Example 1: Updated ScanEntry and PermissionIssue Types

```go
// internal/scan/types.go
package scan

// ScanEntry represents a single scannable item on the filesystem.
type ScanEntry struct {
    Path        string `json:"path"`
    Description string `json:"description"`
    Size        int64  `json:"size"`
    RiskLevel   string `json:"risk_level"`
}

// PermissionIssue records a path that could not be accessed during scanning.
type PermissionIssue struct {
    Path        string `json:"path"`
    Description string `json:"description"`
}

// CategoryResult groups scan entries under a named category.
type CategoryResult struct {
    Category         string            `json:"category"`
    Description      string            `json:"description"`
    Entries          []ScanEntry       `json:"entries"`
    TotalSize        int64             `json:"total_size"`
    PermissionIssues []PermissionIssue `json:"permission_issues,omitempty"`
}

// ScanSummary aggregates results from all scanned categories.
type ScanSummary struct {
    Categories       []CategoryResult  `json:"categories"`
    TotalSize        int64             `json:"total_size"`
    PermissionIssues []PermissionIssue `json:"permission_issues,omitempty"`
}
```

### Example 2: Risk Level Constants and Mapping

```go
// internal/safety/risk.go
package safety

// Risk level constants for scan entry categorization.
const (
    RiskSafe     = "safe"
    RiskModerate = "moderate"
    RiskRisky    = "risky"
)

var categoryRisk = map[string]string{
    "system-caches":      RiskSafe,
    "system-logs":        RiskSafe,
    "quicklook":          RiskSafe,
    "browser-safari":     RiskModerate,
    "browser-chrome":     RiskModerate,
    "browser-firefox":    RiskModerate,
    "dev-xcode":          RiskRisky,
    "dev-npm":            RiskModerate,
    "dev-yarn":           RiskModerate,
    "dev-homebrew":       RiskModerate,
    "dev-docker":         RiskRisky,
    "app-orphaned-prefs": RiskRisky,
    "app-ios-backups":    RiskRisky,
    "app-old-downloads":  RiskModerate,
}

// RiskForCategory returns the risk level for a given category ID.
// Unknown categories default to "moderate" for safety.
func RiskForCategory(categoryID string) string {
    if level, ok := categoryRisk[categoryID]; ok {
        return level
    }
    return RiskModerate
}
```

### Example 3: SetRiskLevels Helper on CategoryResult

```go
// internal/scan/types.go (add method)

// SetRiskLevels assigns the risk level for all entries in this category
// using the provided risk function (typically safety.RiskForCategory).
func (cr *CategoryResult) SetRiskLevels(riskFn func(string) string) {
    level := riskFn(cr.Category)
    for i := range cr.Entries {
        cr.Entries[i].RiskLevel = level
    }
}
```

### Example 4: Scanner Integration (system scanner)

```go
// pkg/system/scanner.go - Modified Scan function
func Scan() ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("cannot determine home directory: %w", err)
    }

    var results []scan.CategoryResult

    if cr, err := scan.ScanTopLevel(filepath.Join(home, "Library", "Caches"), "system-caches", "User App Caches"); err == nil && cr != nil {
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, *cr)
    }

    if cr, err := scan.ScanTopLevel(filepath.Join(home, "Library", "Logs"), "system-logs", "User Logs"); err == nil && cr != nil {
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, *cr)
    }

    // ... QuickLook ...

    return results, nil
}
```

### Example 5: Permission Error Collection in Browser Scanner

```go
// pkg/browser/scanner.go - Modified scanSafari
func scanSafari(home string) (*scan.CategoryResult, []scan.PermissionIssue) {
    safariDir := filepath.Join(home, "Library", "Caches", "com.apple.Safari")

    _, err := os.Stat(safariDir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        if os.IsPermission(err) {
            return nil, []scan.PermissionIssue{{
                Path:        safariDir,
                Description: "Safari cache (requires Full Disk Access to scan)",
            }}
        }
        return nil, nil
    }

    size, err := scan.DirSize(safariDir)
    if err != nil {
        if os.IsPermission(err) {
            return nil, []scan.PermissionIssue{{
                Path:        safariDir,
                Description: "Safari cache (requires Full Disk Access to scan)",
            }}
        }
        return nil, nil
    }

    if size == 0 {
        return nil, nil
    }

    cr := &scan.CategoryResult{
        Category:    "browser-safari",
        Description: "Safari Cache",
        Entries: []scan.ScanEntry{{
            Path:        safariDir,
            Description: "com.apple.Safari",
            Size:        size,
        }},
        TotalSize: size,
    }
    cr.SetRiskLevels(safety.RiskForCategory)
    return cr, nil
}
```

### Example 6: Risk-Aware Display in printResults

```go
// cmd/root.go - Modified entry display in printResults
red := color.New(color.FgRed)
yellow := color.New(color.FgYellow)

for _, entry := range cat.Entries {
    sizeStr := scan.FormatSize(entry.Size)
    riskTag := ""
    switch entry.RiskLevel {
    case safety.RiskRisky:
        riskTag = red.Sprint("  [risky]")
    case safety.RiskModerate:
        riskTag = yellow.Sprint("  [moderate]")
    // safe: no tag (clean output for majority of items)
    }
    fmt.Fprintf(w, "    %s%s\t  %s\t\n", entry.Description, riskTag, cyan.Sprint(sizeStr))
    if flagVerbose {
        path := shortenHome(entry.Path, home)
        fmt.Fprintf(w, "      %s\t\t\n", path)
    }
}
```

### Example 7: Permission Issues Display

```go
// cmd/root.go - New function
func printPermissionIssues(results []scan.CategoryResult) {
    var issues []scan.PermissionIssue
    for _, cat := range results {
        issues = append(issues, cat.PermissionIssues...)
    }
    if len(issues) == 0 {
        return
    }

    home, _ := os.UserHomeDir()
    yellow := color.New(color.FgYellow)

    fmt.Fprintln(os.Stderr)
    yellow.Fprintf(os.Stderr, "Note: %d path(s) could not be accessed (permission denied):\n", len(issues))
    for _, issue := range issues {
        path := shortenHome(issue.Path, home)
        fmt.Fprintf(os.Stderr, "  %s — %s\n", path, issue.Description)
    }
}
```

### Example 8: Risky Item Warning in Confirmation

```go
// internal/confirm/confirm.go - Add risky item warning
func hasRiskyItems(results []scan.CategoryResult) bool {
    for _, cat := range results {
        for _, entry := range cat.Entries {
            if entry.RiskLevel == safety.RiskRisky {
                return true
            }
        }
    }
    return false
}

// In PromptConfirmation, before "Type 'yes' to proceed":
if hasRiskyItems(results) {
    red := color.New(color.FgRed, color.Bold)
    red.Fprintln(out, "\nWARNING: Selection includes risky items that may be difficult to recover.")
}
```

## macOS Permission Model Reference

Understanding how macOS permission denials manifest in Go:

| Protection Layer | Error | `os.IsPermission` | Typical Paths |
|-----------------|-------|-------------------|---------------|
| BSD file permissions (EACCES) | "permission denied" | true | Files owned by other users, restricted mode bits |
| TCC / Full Disk Access (EPERM) | "operation not permitted" | true | `~/Library/Safari`, `~/Library/Mail`, `~/Library/Messages`, `~/Library/Cookies` |
| SIP (EPERM) | "operation not permitted" | true | `/System`, `/usr/bin`, `/bin`, `/sbin` (already handled by IsPathBlocked) |
| Sandbox (EPERM) | "operation not permitted" | true | App-specific sandbox restrictions |

**Key fact:** Go's `os.IsPermission()` returns `true` for both EACCES and EPERM on all Unix platforms including macOS. This means the existing `os.IsPermission(err)` check already used in `scanSafari` is sufficient for detecting TCC denials. No special macOS-specific error handling is needed.

**TCC-protected directories relevant to mac-cleaner:**
- `~/Library/Safari` (browser-safari scanner may encounter this)
- `~/Library/Cookies` (not scanned by current tool)
- `~/Library/Mail` (not scanned by current tool)
- `~/Library/Messages` (not scanned by current tool)

Of these, only Safari is currently scanned. The existing `scanSafari` function already handles this case. The general pattern should be extracted for consistency.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Silent skip on permission errors | Collect and report permission issues | Phase 7 (this phase) | Users know what was skipped and why |
| No risk categorization | Per-entry risk levels | Phase 7 (this phase) | Users make informed decisions about what to delete |
| Ad-hoc permission messages per scanner | Centralized PermissionIssue collection | Phase 7 (this phase) | Consistent UX, testable, included in JSON |

**Deprecated/outdated:**
- `os.IsPermission()` is functional but deprecated in favor of `errors.Is(err, fs.ErrPermission)` per Go issue #41122. Both work identically for syscall errors. New code should prefer `errors.Is`.

## Open Questions

1. **Should "risky" items require a separate confirmation from "safe"/"moderate" items?**
   - What we know: The success criteria says "Risky items highlighted in output and require explicit confirmation." The tool already has a confirmation prompt for all items.
   - What's unclear: Does "require explicit confirmation" mean the existing confirmation is sufficient (since it already requires "yes"), or does it mean risky items need an additional/separate confirmation step?
   - Recommendation: The existing confirmation prompt is sufficient. Add a WARNING line before the prompt when risky items are present. Do NOT add a second confirmation step -- that would be annoying UX. The interactive walkthrough already provides per-item control where users can choose to keep risky items.

2. **Should the interactive walkthrough default to "keep" for risky items?**
   - What we know: Currently EOF defaults to "keep" (safe default). The prompt is the same for all items.
   - What's unclear: Whether risky items should have a different default or prompt text.
   - Recommendation: Keep the same prompt for all items. The risk tag `[risky]` next to the item provides sufficient information. Changing the default behavior based on risk would be confusing.

3. **Where should PermissionIssues live -- on CategoryResult or as a separate return value?**
   - What we know: The cleanest API would return permission issues alongside results. The simplest implementation puts them on CategoryResult.
   - What's unclear: Whether permission issues should be per-category or aggregated at the top level.
   - Recommendation: Put `PermissionIssues []PermissionIssue` on `CategoryResult` with `omitempty` JSON tag. Aggregate them into a top-level `PermissionIssues` field on `ScanSummary` for JSON output. This preserves the existing scanner signatures while providing both per-category and aggregated views.

4. **How to handle scanners that currently swallow errors silently (not just permission errors)?**
   - What we know: Many scanners silently return nil on any error (e.g., `if _, err := os.Stat(dir); err != nil { return nil }`).
   - What's unclear: Whether non-permission errors should also be collected and reported.
   - Recommendation: For Phase 7, only collect permission errors. Other errors (nonexistent directories, timeout errors, etc.) represent expected conditions (tool not installed, directory not present) and should continue to be silently skipped. Collecting all errors would add noise.

## Sources

### Primary (HIGH confidence)
- **Codebase analysis** -- Direct reading of all source files in `/Users/gregor/projects/mac-clarner/`. Every scanner behavior, error handling pattern, type definition, and test pattern verified from source code.
- **Go os package documentation** -- `os.IsPermission()` checks both EACCES and EPERM. `errors.Is(err, fs.ErrPermission)` is the modern equivalent. Source: [Go os package](https://pkg.go.dev/os), [Go issue #41122](https://github.com/golang/go/issues/41122).
- **Go issue #22999** -- Confirms `os.IsPermission` returns true for EPERM ("operation not permitted") on macOS, which is the error code TCC returns. Source: [Mac OS operation not permitted](https://github.com/golang/go/issues/22999).
- **Prior phase research** -- Phases 1-6 research and summaries establish all architectural patterns, type definitions, and conventions used in recommendations above.

### Secondary (MEDIUM confidence)
- **Apple Developer Forums thread on file system permissions** -- Confirms EACCES for BSD permissions, EPERM for TCC/sandbox blocks on macOS. Source: [On File System Permissions](https://developer.apple.com/forums/thread/678819).
- **Eclectic Light Company (Howard Oakley)** -- Detailed analysis of macOS SIP vs TCC distinction, confirms TCC protects user data directories (Safari, Mail, Messages). Source: [Permissions, SIP and TCC](https://eclecticlight.co/2023/02/11/permissions-sip-and-tcc-whos-controlling-access/).
- **Fileside blog on Full Disk Access** -- Lists TCC-protected categories: Mail, Messages, Safari, Home, Time Machine. Source: [Full Disk Access - what is it](https://www.fileside.app/blog/2024-05-31_full-disk-access/).
- **Apple Community discussions** -- Confirms `ls ~/Library/Safari` returns "operation not permitted" without Full Disk Access. Source: [Apple Community thread](https://discussions.apple.com/thread/8637915).

### Tertiary (LOW confidence)
- **Risk level assignment for specific categories** -- The safe/moderate/risky classification is based on domain knowledge about what each cache type does and how it regenerates. There is no authoritative "risk level database" for macOS cleanup categories. The mapping is reasonable and matches how CleanMyMac and similar tools categorize items (based on web search), but should be reviewed by the project owner.

## Metadata

**Confidence breakdown:**
- Architecture (types, patterns): HIGH -- Direct extension of established codebase patterns. No new concepts.
- Risk level mapping: HIGH for the mechanism, MEDIUM for specific category assignments -- The mapping mechanism is straightforward. Individual risk assignments are based on domain knowledge and could be debated (e.g., is npm cache "moderate" or "safe"?). The key ones (DerivedData=risky, iOS backups=risky, system caches=safe) are clear.
- Permission handling: HIGH -- `os.IsPermission` behavior on macOS verified through Go issue tracker and Apple developer documentation. The existing Safari scanner already demonstrates the pattern.
- Circular import avoidance: HIGH -- The recommended approach (constants in `safety` package) is verified to avoid import cycles based on current dependency graph: scanners import `safety` (via `scan/helpers.go`), `safety` does not import `scan`.
- Pitfalls: HIGH -- Based on analysis of actual codebase structure and known Go/macOS behaviors.

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, no fast-moving dependencies)
