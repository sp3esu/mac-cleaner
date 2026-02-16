# Phase 6: CLI Polish & Automation - Research

**Researched:** 2026-02-16
**Domain:** Cobra CLI flags, JSON serialization, output formatting, category/item filtering
**Confidence:** HIGH

## Summary

Phase 6 adds six CLI capabilities to the existing mac-cleaner tool: `--all` (target all categories), `--json` (structured output), `--verbose` (detailed file listing), `--skip-<category>` flags (skip entire scanner groups), `--skip-<item>` flags (skip specific scan categories), and `--force` (bypass confirmation for automation). All six features are flag additions to the existing root command in `cmd/root.go`, using the established pattern of package-level bool vars registered in `init()` and dispatched in the `Run` function.

The implementation requires no new dependencies. `encoding/json` from the Go stdlib handles JSON serialization. The existing `scan.CategoryResult` and `scan.ScanEntry` types already have all the fields needed for JSON output -- they just need `json:` struct tags added. The `--verbose` flag changes the display format of `printResults` to show individual file paths and sizes instead of just category summaries. The skip flags filter scan results by matching against the `Category` field already present on every `CategoryResult`. The `--force` flag bypasses the `confirm.PromptConfirmation` call in the deletion flow.

The key architectural decision is how `--all` interacts with the existing flag dispatch. Currently, when specific scan flags (`--system-caches`, `--browser-data`, etc.) are set, the `ran` boolean is set to true and the tool runs in "flag mode" (scan + confirm + delete). When no flags are set (`!ran`), it runs in "interactive mode" (walkthrough). The `--all` flag should set all four scan flags to true, making it equivalent to `--system-caches --browser-data --dev-caches --app-leftovers`. This preserves the existing dispatch logic without a new code path.

**Primary recommendation:** Implement as incremental additions to `cmd/root.go` with no new packages. Add `json` struct tags to `scan.ScanEntry` and `scan.CategoryResult`. Add a `--json` output path that serializes results via `encoding/json`. Add `--verbose` to `printResults`. Add skip flags that filter `[]scan.CategoryResult` by Category field. Add `--force` that skips confirmation. Wire `--all` to set all four scan flags in a `PreRun` hook.

## Standard Stack

### Core (no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| encoding/json (stdlib) | -- | JSON serialization for `--json` output | Standard Go JSON encoding. Already well-understood, zero dependency. `json.MarshalIndent` for pretty-printed output, `json.NewEncoder` for stream output. |
| os (stdlib) | -- | `os.Stdout` file descriptor for isatty checks | Already used throughout codebase |
| fatih/color v1.18.0 | already in go.mod | Color output, auto-disabled when piped | Already used. `color.NoColor` can be set programmatically when `--json` is active to prevent ANSI codes in JSON output. |
| mattn/go-isatty v0.0.20 | already in go.mod (transitive via fatih/color) | TTY detection for `--json` auto-behavior | Already in dependency tree. `isatty.IsTerminal(os.Stdout.Fd())` available without new imports. |
| spf13/cobra v1.10.2 | already in go.mod | `PersistentPreRun` hook for `--all` expansion, flag registration | Established CLI framework. |

### Not Needed

| Library | Why Not |
|---------|---------|
| tidwall/gjson | For JSON querying, not generation. We generate JSON, we don't query it. |
| json-iterator/go | Performance JSON encoding. Our output is small (kilobytes). stdlib encoding/json is sufficient. |
| olekukonenko/tablewriter | For ASCII table formatting. We already have `text/tabwriter` from stdlib which is used in `printResults`. |

**Installation:** No new dependencies needed. `encoding/json` is stdlib.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| encoding/json | protobuf or msgpack | Binary formats are inappropriate for CLI tool human-readable output. JSON is the universal choice for CLI `--json` flags. |
| json.MarshalIndent | json.NewEncoder with SetIndent | Encoder writes directly to a writer (slightly more efficient), MarshalIndent returns a byte slice (easier to test). Either works; MarshalIndent is simpler for one-shot output. |
| Manual flag dispatch for --all | Cobra flag groups | Cobra has `MarkFlagsOneRequired` and `MarkFlagsMutuallyExclusive` but no "set these flags together" built-in. Manual PreRun dispatch is cleaner. |

## Architecture Patterns

### Recommended Changes (Phase 6)

```
mac-cleaner/
├── cmd/
│   └── root.go                    # Add: --all, --json, --verbose, --skip-*, --force flags
│                                  #       JSON output path, verbose printResults variant
│                                  #       PreRun hook for --all expansion
│                                  #       Skip filtering logic
├── internal/
│   └── scan/
│       └── types.go               # Add: json struct tags to ScanEntry and CategoryResult
│                                  #       (ScanSummary already exists for top-level JSON)
```

No new packages. No new files. This phase is entirely modifications to existing files.

### Pattern 1: --all via PreRun Hook

**What:** The `--all` flag sets all four category scan flags to true before the main Run function executes.

**When to use:** When the user wants to scan all categories in non-interactive (flag-based) mode.

**Key design decisions:**

1. **Use `PersistentPreRun` or `PreRun`** -- Cobra supports pre-run hooks that execute before the main `Run` function. Setting the four scan booleans in `PreRun` means the existing dispatch logic in `Run` sees them as if the user typed `--system-caches --browser-data --dev-caches --app-leftovers`. No new code path needed.

2. **`--all` is mutually exclusive with interactive mode** -- When `--all` is set, the `ran` boolean will be true (because all four scan flags are true), so the interactive walkthrough path is never entered. This is the correct behavior.

3. **`--all` combines naturally with `--skip-*`** -- The user can type `mac-cleaner --all --skip-browser-data` to scan everything except browser data. The skip filtering runs after scanning.

**Example:**
```go
var flagAll bool

// In init():
rootCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")

// PreRun hook:
rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
    if flagAll {
        flagSystemCaches = true
        flagBrowserData = true
        flagDevCaches = true
        flagAppLeftovers = true
    }
}
```

### Pattern 2: --json Output with Structured Types

**What:** When `--json` is set, suppress all human-readable output and emit a single JSON document to stdout at the end.

**When to use:** For automation and AI agent consumption.

**Key design decisions:**

1. **Add `json` struct tags to existing types** -- `scan.ScanEntry` and `scan.CategoryResult` get tags like `json:"path"`, `json:"size"`, etc. The `ScanSummary` type already exists and can serve as the top-level JSON envelope.

2. **Disable color when `--json` is active** -- Set `color.NoColor = true` to prevent ANSI escape codes from contaminating JSON output. This is idempotent and safe.

3. **Collect results, then serialize once** -- Don't stream JSON line by line. Collect all `[]scan.CategoryResult` into a `ScanSummary`, then `json.MarshalIndent` the whole thing to stdout. This produces valid, parseable JSON.

4. **JSON output includes all data** -- Each entry gets path, description, size (bytes), and formatted size. Each category gets category ID, description, entries, and total size. The summary gets total size across all categories.

5. **--json is mutually exclusive with interactive mode** -- Using `--json` without scan flags should produce an error or scan all. Recommendation: require either `--all` or specific scan flags when `--json` is used. Or: allow `--json` alone to mean "scan all and output JSON" since JSON consumers always want structured output.

6. **--json with --dry-run** -- This is the primary AI agent use case: `mac-cleaner --all --dry-run --json`. It scans everything, outputs structured JSON, and deletes nothing.

**JSON schema:**
```json
{
  "categories": [
    {
      "category": "system-caches",
      "description": "User App Caches",
      "entries": [
        {
          "path": "/Users/gregor/Library/Caches/com.apple.Safari",
          "description": "com.apple.Safari",
          "size": 42100000
        }
      ],
      "total_size": 42100000
    }
  ],
  "total_size": 123456789
}
```

**Example:**
```go
// In scan/types.go, add json tags:
type ScanEntry struct {
    Path        string `json:"path"`
    Description string `json:"description"`
    Size        int64  `json:"size"`
}

type CategoryResult struct {
    Category    string      `json:"category"`
    Description string      `json:"description"`
    Entries     []ScanEntry `json:"entries"`
    TotalSize   int64       `json:"total_size"`
}

type ScanSummary struct {
    Categories []CategoryResult `json:"categories"`
    TotalSize  int64            `json:"total_size"`
}

// In cmd/root.go:
func printJSON(results []scan.CategoryResult) {
    var totalSize int64
    for _, cat := range results {
        totalSize += cat.TotalSize
    }
    summary := scan.ScanSummary{
        Categories: results,
        TotalSize:  totalSize,
    }
    data, err := json.MarshalIndent(summary, "", "  ")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Println(string(data))
}
```

### Pattern 3: --verbose Detailed Output

**What:** When `--verbose` is set, `printResults` shows individual file paths and sizes for each entry, instead of just entry descriptions.

**When to use:** When the user wants to see exactly which files/directories were found.

**Key design decisions:**

1. **Extend existing `printResults`** -- Don't create a separate function. Add a `verbose` parameter (or check the global flag) to show paths in addition to descriptions.

2. **Verbose adds path display** -- In the default mode, each entry shows `Description` and `Size`. In verbose mode, it also shows the shortened path (with `~` for home directory).

3. **Verbose is independent of --json** -- `--verbose` affects human-readable output only. If `--json` is also set, `--json` takes precedence (JSON always includes all data regardless of verbose).

**Default output:**
```
System Caches (dry run)

  User App Caches    ~/Library/Caches
    com.apple.Safari                42.1 MB
    com.spotify.client              18.3 MB
```

**Verbose output:**
```
System Caches (dry run)

  User App Caches    ~/Library/Caches
    com.apple.Safari                42.1 MB
      ~/Library/Caches/com.apple.Safari
    com.spotify.client              18.3 MB
      ~/Library/Caches/com.spotify.client
```

### Pattern 4: --skip-<category> Flags (CLI-08)

**What:** Skip flags that exclude entire scanner groups from running. These correspond to the existing scan flags.

**When to use:** Combined with `--all` to exclude specific categories.

**Key design decisions:**

1. **Four skip-category flags** -- One per scanner group, matching the existing scan flags:
   - `--skip-system-caches` (skips system.Scan)
   - `--skip-browser-data` (skips browser.Scan)
   - `--skip-dev-caches` (skips developer.Scan)
   - `--skip-app-leftovers` (skips appleftovers.Scan)

2. **Skip overrides scan** -- If both `--all` and `--skip-browser-data` are set, browser scanning is skipped. Implementation: after PreRun sets all flags from `--all`, apply skip overrides to turn specific flags back off.

3. **Skip flags have no effect in interactive mode** -- Interactive mode scans everything. Skip flags only apply in flag-based mode (when `--all` or specific scan flags are used). Alternatively, skip flags could filter results even in interactive mode (so skipped items never appear in the walkthrough). Recommendation: apply skip filtering in both modes for consistency.

**Example:**
```go
var (
    flagSkipSystemCaches bool
    flagSkipBrowserData  bool
    flagSkipDevCaches    bool
    flagSkipAppLeftovers bool
)

// In init():
rootCmd.Flags().BoolVar(&flagSkipSystemCaches, "skip-system-caches", false, "skip system cache scanning")
rootCmd.Flags().BoolVar(&flagSkipBrowserData, "skip-browser-data", false, "skip browser data scanning")
rootCmd.Flags().BoolVar(&flagSkipDevCaches, "skip-dev-caches", false, "skip developer cache scanning")
rootCmd.Flags().BoolVar(&flagSkipAppLeftovers, "skip-app-leftovers", false, "skip app leftover scanning")

// In PreRun, after --all expansion:
if flagSkipSystemCaches { flagSystemCaches = false }
if flagSkipBrowserData  { flagBrowserData = false }
if flagSkipDevCaches    { flagDevCaches = false }
if flagSkipAppLeftovers { flagAppLeftovers = false }
```

### Pattern 5: --skip-<item> Flags (CLI-09)

**What:** Skip flags that exclude specific scan categories (individual items within a scanner group).

**When to use:** When the user wants to keep most items in a scanner group but exclude specific ones.

**Key design decisions:**

1. **Item-level skip flags** -- These map to the `Category` field on `CategoryResult`. The existing category IDs are:
   - System group: `system-caches`, `system-logs`, `quicklook`
   - Browser group: `browser-safari`, `browser-chrome`, `browser-firefox`
   - Developer group: `dev-xcode`, `dev-npm`, `dev-yarn`, `dev-homebrew`, `dev-docker`
   - App leftovers group: `app-orphaned-prefs`, `app-ios-backups`, `app-old-downloads`

2. **Flag naming convention** -- Use the category ID with prefix `--skip-`: `--skip-derived-data` (matching success criteria), `--skip-npm`, `--skip-docker`, etc. The exact mapping from requirement "skip-derived-data" to category ID "dev-xcode" needs a human-friendly alias.

3. **Recommended skip-item flags** -- Based on the category IDs and the success criteria mentioning `--skip-derived-data`:
   - `--skip-derived-data` (filters out category "dev-xcode")
   - `--skip-npm` (filters out "dev-npm")
   - `--skip-yarn` (filters out "dev-yarn")
   - `--skip-homebrew` (filters out "dev-homebrew")
   - `--skip-docker` (filters out "dev-docker")
   - `--skip-safari` (filters out "browser-safari")
   - `--skip-chrome` (filters out "browser-chrome")
   - `--skip-firefox` (filters out "browser-firefox")
   - `--skip-quicklook` (filters out "quicklook")
   - `--skip-orphaned-prefs` (filters out "app-orphaned-prefs")
   - `--skip-ios-backups` (filters out "app-ios-backups")
   - `--skip-old-downloads` (filters out "app-old-downloads")

4. **Implementation: post-scan filtering** -- After scanning, filter `[]scan.CategoryResult` to exclude categories whose IDs match any active skip-item flags. This is simpler and more testable than modifying each scanner to accept skip parameters.

5. **Build a skip set** -- Use a `map[string]bool` to collect all skipped category IDs, then filter in one pass.

**Example:**
```go
// Build skip set from flags
skipCategories := map[string]bool{}
if flagSkipDerivedData  { skipCategories["dev-xcode"] = true }
if flagSkipNpm          { skipCategories["dev-npm"] = true }
// ... etc

// Filter results
func filterSkipped(results []scan.CategoryResult, skip map[string]bool) []scan.CategoryResult {
    if len(skip) == 0 {
        return results
    }
    var filtered []scan.CategoryResult
    for _, cat := range results {
        if !skip[cat.Category] {
            filtered = append(filtered, cat)
        }
    }
    return filtered
}
```

### Pattern 6: --force Flag (CLI-10)

**What:** Bypass the confirmation prompt and proceed directly to deletion.

**When to use:** For automation scripts and CI pipelines. Combined with `--dry-run` for safety testing.

**Key design decisions:**

1. **`--force` skips confirmation only** -- It does NOT skip the scan display. The user still sees what will be deleted (unless `--json` is used, in which case output is structured).

2. **`--force` does not bypass safety** -- The safety layer (`IsPathBlocked`) still runs at deletion time. `--force` only skips the interactive "Type 'yes' to proceed" confirmation.

3. **`--force` is meaningless with `--dry-run`** -- When `--dry-run` is set, nothing is deleted regardless. But `--force --dry-run` is a valid combination for automation: "show me what would be deleted, don't ask me anything, don't delete anything."

4. **`--force` in interactive mode** -- If the user runs `mac-cleaner --force` with no other flags, this enters interactive mode (walkthrough). `--force` could mean "skip final confirmation after walkthrough" or it could be an error. Recommendation: `--force` only applies when scan flags (or `--all`) are used. In interactive mode, force is ignored (the walkthrough IS the user's explicit selection).

5. **`--force` requires scan flags or `--all`** -- To prevent accidentally deleting everything with just `mac-cleaner --force`, require that at least one scan flag or `--all` is also set. This can be enforced with a validation check.

**Example:**
```go
var flagForce bool

// In init():
rootCmd.Flags().BoolVar(&flagForce, "force", false, "bypass confirmation prompt (for automation)")

// In the deletion flow (flag-based mode):
if !flagDryRun && len(allResults) > 0 {
    if !flagForce {
        if !confirm.PromptConfirmation(os.Stdin, os.Stdout, allResults) {
            fmt.Println("Aborted.")
            return
        }
    }
    result := cleanup.Execute(allResults)
    printCleanupSummary(result)
}
```

### Anti-Patterns to Avoid

- **Printing JSON mixed with human-readable text:** When `--json` is active, ALL non-JSON output must be suppressed or redirected to stderr. Color codes, progress messages, and headers must not appear on stdout. Only the JSON document goes to stdout.
- **Registering dozens of skip-item flags individually:** While each flag is a separate bool, use a data-driven approach (map from flag name to category ID) rather than 14 separate if-else blocks.
- **Making `--force` work without scan flags:** `mac-cleaner --force` with no other flags should NOT silently scan-all and delete everything. Require explicit category selection.
- **Modifying scanner packages for skip logic:** Skip filtering should happen in `cmd/root.go` after scanners return results. Don't change the scanner function signatures.
- **Making `--json` and `--verbose` mutually exclusive:** They can coexist. `--json` takes precedence for output format; `--verbose` has no additional effect when `--json` is active (JSON always includes all data).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON serialization | Custom JSON string building | `encoding/json.MarshalIndent` | Handles escaping, nesting, and formatting correctly. Custom JSON building is an inexhaustible source of bugs. |
| TTY detection | Manual `/dev/tty` checks | `mattn/go-isatty` (already in dep tree) | Cross-platform, handles edge cases (Cygwin, WSL, SSH). Already imported transitively via fatih/color. |
| Color suppression for JSON | Manual ANSI stripping | `color.NoColor = true` | fatih/color's built-in mechanism. Set it before any output when `--json` is active. |
| Flag validation | Custom arg parsing | Cobra's `MarkFlagsMutuallyExclusive` | Cobra handles the error message and help text automatically. |
| Category filtering | Per-scanner skip parameters | Post-scan `map[string]bool` filtering | Single function, works for all scanners, no scanner API changes needed. |

**Key insight:** Phase 6 adds flags and output formatting. All the scanning, safety, confirmation, and cleanup machinery already exists. The new code is entirely in `cmd/root.go` (flag wiring, output routing) and `internal/scan/types.go` (JSON tags). No new packages needed.

## Common Pitfalls

### Pitfall 1: ANSI Color Codes in JSON Output

**What goes wrong:** JSON output contains embedded ANSI escape codes like `\033[1m` from fatih/color, making it unparseable by JSON consumers.
**Why it happens:** fatih/color auto-detects TTY and enables colors. When the user pipes output to a file or another command, fatih/color usually disables itself. But with `--json`, the output might still go to a TTY (e.g., `mac-cleaner --all --json` typed directly in terminal), so fatih/color enables colors.
**How to avoid:** Set `color.NoColor = true` at the top of the Run function when `flagJSON` is true. Do this BEFORE any output functions are called.
**Warning signs:** JSON output looks correct in tests (no TTY) but fails when run in a terminal.

### Pitfall 2: --all + Interactive Mode Confusion

**What goes wrong:** `mac-cleaner --all` enters interactive walkthrough instead of flag-based scan mode.
**Why it happens:** If `--all` expansion happens too late (after the `ran` boolean check), the dispatch logic doesn't see any scan flags set and falls into interactive mode.
**How to avoid:** Use Cobra's `PreRun` hook to expand `--all` into the four scan flags BEFORE the main `Run` function checks them. The `ran` boolean will then correctly see the flags.
**Warning signs:** `mac-cleaner --all` starts asking "keep or remove?" instead of scanning and showing results.

### Pitfall 3: --force Without Safety

**What goes wrong:** `mac-cleaner --force` with no scan flags interprets as interactive mode, walks through items, then skips confirmation and deletes immediately.
**Why it happens:** `--force` only suppresses the confirmation prompt. In interactive mode, the walkthrough is the user's selection mechanism. Skipping confirmation after walkthrough removes the safety gate.
**How to avoid:** `--force` should only suppress the confirmation in flag-based (non-interactive) mode. In interactive mode, the walkthrough already provides explicit selection, but the final confirmation should still happen (or `--force` should skip it since the user already chose item-by-item). Clear behavior: `--force` applies to the automatic confirmation prompt in flag-based mode only. The interactive walkthrough's item-by-item selection is already explicit consent.
**Warning signs:** Automation scripts running `mac-cleaner --force` without `--all` enter an unexpected interactive mode.

### Pitfall 4: JSON Output Includes Formatted Sizes

**What goes wrong:** JSON output includes human-readable sizes like "42.1 MB" instead of raw byte counts, making programmatic size comparison impossible.
**Why it happens:** The developer adds `scan.FormatSize(entry.Size)` to the JSON output for convenience.
**How to avoid:** JSON output uses raw `int64` byte counts in the `size` field. Human-readable formatting is a display concern, not a data concern. JSON consumers can format as needed. Optionally add a `size_formatted` convenience field, but `size` must always be the raw number.
**Warning signs:** JSON consumers struggle to sort by size or calculate totals because the size field is a string.

### Pitfall 5: Skip Flag Proliferation Makes Help Text Unreadable

**What goes wrong:** With 4 category-skip flags and 12 item-skip flags, `mac-cleaner --help` becomes overwhelming.
**Why it happens:** Each flag is registered independently with its own help text.
**How to avoid:** Group flags logically in the help text. Cobra doesn't natively support flag groups in help output, but you can use annotations or organize flags with descriptive help text. Consider whether all 12 item-level skip flags are needed for v1, or whether category-level skips are sufficient and item-level skips can be deferred.
**Warning signs:** Users see a wall of `--skip-*` flags and don't understand which to use.

### Pitfall 6: printResults Writes to os.Stdout Directly

**What goes wrong:** `printResults` uses `os.Stdout` directly (via `fmt.Println`, `tabwriter.NewWriter(os.Stdout, ...)`, and `color` which targets stdout). When `--json` is active, these writes contaminate the JSON output.
**Why it happens:** The existing code was written before JSON output was a requirement.
**How to avoid:** When `--json` is active, either (a) suppress all `printResults` calls and only call `printJSON` at the end, or (b) redirect `printResults` to stderr. Recommendation: suppress `printResults` entirely when `--json` is active. The JSON output replaces the human-readable output, it doesn't supplement it.
**Warning signs:** JSON output has human-readable scan results interspersed with the JSON document.

### Pitfall 7: ScanSummary Type Exists But Is Unused

**What goes wrong:** The `scan.ScanSummary` type exists in `types.go` but was never used in prior phases. A developer might create a new type for JSON output instead of using the existing one.
**Why it happens:** `ScanSummary` was defined in Phase 2 as forward-looking but never wired up.
**How to avoid:** Use `ScanSummary` as the top-level JSON envelope. It already has `Categories []CategoryResult` and `TotalSize int64` -- exactly what's needed.
**Warning signs:** Duplicate type definitions for the same concept.

## Code Examples

### Example 1: JSON Struct Tags on Existing Types

```go
// internal/scan/types.go
type ScanEntry struct {
    Path        string `json:"path"`
    Description string `json:"description"`
    Size        int64  `json:"size"`
}

type CategoryResult struct {
    Category    string      `json:"category"`
    Description string      `json:"description"`
    Entries     []ScanEntry `json:"entries"`
    TotalSize   int64       `json:"total_size"`
}

type ScanSummary struct {
    Categories []CategoryResult `json:"categories"`
    TotalSize  int64            `json:"total_size"`
}
```

### Example 2: Flag Registration in init()

```go
var (
    flagAll             bool
    flagJSON            bool
    flagVerbose         bool
    flagForce           bool
    flagSkipSystemCaches bool
    flagSkipBrowserData  bool
    flagSkipDevCaches    bool
    flagSkipAppLeftovers bool
    // Item-level skip flags
    flagSkipDerivedData  bool
    flagSkipNpm          bool
    flagSkipDocker       bool
    // ... etc
)

func init() {
    // ... existing flag registrations ...
    rootCmd.Flags().BoolVar(&flagAll, "all", false, "scan all categories")
    rootCmd.Flags().BoolVar(&flagJSON, "json", false, "output results as JSON")
    rootCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "show detailed file listing")
    rootCmd.Flags().BoolVar(&flagForce, "force", false, "bypass confirmation prompt")

    // Category-level skip flags
    rootCmd.Flags().BoolVar(&flagSkipSystemCaches, "skip-system-caches", false, "skip system cache scanning")
    rootCmd.Flags().BoolVar(&flagSkipBrowserData, "skip-browser-data", false, "skip browser data scanning")
    rootCmd.Flags().BoolVar(&flagSkipDevCaches, "skip-dev-caches", false, "skip developer cache scanning")
    rootCmd.Flags().BoolVar(&flagSkipAppLeftovers, "skip-app-leftovers", false, "skip app leftover scanning")

    // Item-level skip flags
    rootCmd.Flags().BoolVar(&flagSkipDerivedData, "skip-derived-data", false, "skip Xcode DerivedData")
    // ... register all item-level skip flags ...
}
```

### Example 3: PreRun Hook for --all Expansion and Skip Application

```go
rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
    if flagAll {
        flagSystemCaches = true
        flagBrowserData = true
        flagDevCaches = true
        flagAppLeftovers = true
    }

    // Apply category-level skip overrides
    if flagSkipSystemCaches { flagSystemCaches = false }
    if flagSkipBrowserData  { flagBrowserData = false }
    if flagSkipDevCaches    { flagDevCaches = false }
    if flagSkipAppLeftovers { flagAppLeftovers = false }

    // Disable color for JSON mode
    if flagJSON {
        color.NoColor = true
    }
}
```

### Example 4: Item-Level Skip Filtering

```go
// buildSkipSet collects category IDs that should be excluded from results.
func buildSkipSet() map[string]bool {
    skip := map[string]bool{}
    type skipMapping struct {
        flag       *bool
        categoryID string
    }
    mappings := []skipMapping{
        {&flagSkipDerivedData, "dev-xcode"},
        {&flagSkipNpm, "dev-npm"},
        {&flagSkipDocker, "dev-docker"},
        {&flagSkipSafari, "browser-safari"},
        // ... all mappings
    }
    for _, m := range mappings {
        if *m.flag {
            skip[m.categoryID] = true
        }
    }
    return skip
}

// filterSkipped removes categories matching the skip set.
func filterSkipped(results []scan.CategoryResult, skip map[string]bool) []scan.CategoryResult {
    if len(skip) == 0 {
        return results
    }
    var filtered []scan.CategoryResult
    for _, cat := range results {
        if !skip[cat.Category] {
            filtered = append(filtered, cat)
        }
    }
    return filtered
}
```

### Example 5: JSON Output Function

```go
func printJSON(results []scan.CategoryResult) {
    var totalSize int64
    for _, cat := range results {
        totalSize += cat.TotalSize
    }
    summary := scan.ScanSummary{
        Categories: results,
        TotalSize:  totalSize,
    }
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    if err := enc.Encode(summary); err != nil {
        fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
        os.Exit(1)
    }
}
```

### Example 6: --force in Deletion Flow

```go
// Flag-based deletion flow (inside the `ran` branch):
if !flagDryRun && len(allResults) > 0 {
    if !flagForce {
        if !confirm.PromptConfirmation(os.Stdin, os.Stdout, allResults) {
            fmt.Println("Aborted.")
            return
        }
    }
    result := cleanup.Execute(allResults)
    if flagJSON {
        // JSON mode: include cleanup result in structured output
        // (or output was already printed, just exit)
    } else {
        printCleanupSummary(result)
    }
}
```

### Example 7: Verbose Print Extension

```go
func printResults(results []scan.CategoryResult, dryRun bool, title string) {
    // ... existing header and category logic ...

    for _, entry := range cat.Entries {
        sizeStr := scan.FormatSize(entry.Size)
        fmt.Fprintf(w, "    %s\t  %s\t\n", entry.Description, cyan.Sprint(sizeStr))

        if flagVerbose {
            path := shortenHome(entry.Path, home)
            fmt.Fprintf(w, "      %s\t\t\n", path)
        }
    }
    w.Flush()

    // ... rest of function ...
}
```

## Complete Category ID Reference

This is the authoritative mapping of scanner groups to category IDs, derived from source code analysis:

| Scanner Group | Flag | Category ID | Description | Source |
|---------------|------|-------------|-------------|--------|
| system | `--system-caches` | `system-caches` | User App Caches | `pkg/system/scanner.go` |
| system | `--system-caches` | `system-logs` | User Logs | `pkg/system/scanner.go` |
| system | `--system-caches` | `quicklook` | QuickLook Thumbnails | `pkg/system/scanner.go` |
| browser | `--browser-data` | `browser-safari` | Safari Cache | `pkg/browser/scanner.go` |
| browser | `--browser-data` | `browser-chrome` | Chrome Cache | `pkg/browser/scanner.go` |
| browser | `--browser-data` | `browser-firefox` | Firefox Cache | `pkg/browser/scanner.go` |
| developer | `--dev-caches` | `dev-xcode` | Xcode DerivedData | `pkg/developer/scanner.go` |
| developer | `--dev-caches` | `dev-npm` | npm Cache | `pkg/developer/scanner.go` |
| developer | `--dev-caches` | `dev-yarn` | Yarn Cache | `pkg/developer/scanner.go` |
| developer | `--dev-caches` | `dev-homebrew` | Homebrew Cache | `pkg/developer/scanner.go` |
| developer | `--dev-caches` | `dev-docker` | Docker Reclaimable | `pkg/developer/scanner.go` |
| appleftovers | `--app-leftovers` | `app-orphaned-prefs` | Orphaned Preferences | `pkg/appleftovers/scanner.go` |
| appleftovers | `--app-leftovers` | `app-ios-backups` | iOS Device Backups | `pkg/appleftovers/scanner.go` |
| appleftovers | `--app-leftovers` | `app-old-downloads` | Old Downloads (90+ days) | `pkg/appleftovers/scanner.go` |

**Total: 4 scanner groups, 14 category IDs.**

## Recommended Skip-Item Flag Names

Based on the category IDs and the success criteria (which specifically mention `--skip-derived-data`):

| Skip Flag | Maps to Category ID | Help Text |
|-----------|-------------------|-----------|
| `--skip-derived-data` | `dev-xcode` | skip Xcode DerivedData |
| `--skip-npm` | `dev-npm` | skip npm cache |
| `--skip-yarn` | `dev-yarn` | skip Yarn cache |
| `--skip-homebrew` | `dev-homebrew` | skip Homebrew cache |
| `--skip-docker` | `dev-docker` | skip Docker reclaimable space |
| `--skip-safari` | `browser-safari` | skip Safari cache |
| `--skip-chrome` | `browser-chrome` | skip Chrome cache |
| `--skip-firefox` | `browser-firefox` | skip Firefox cache |
| `--skip-quicklook` | `quicklook` | skip QuickLook thumbnails |
| `--skip-orphaned-prefs` | `app-orphaned-prefs` | skip orphaned preferences |
| `--skip-ios-backups` | `app-ios-backups` | skip iOS device backups |
| `--skip-old-downloads` | `app-old-downloads` | skip old Downloads |

**Note:** `system-caches` and `system-logs` don't have dedicated item-level skip flags because they are already covered by `--skip-system-caches` at the category level. If needed, `--skip-logs` and `--skip-caches` could be added but the success criteria don't require them.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom JSON formatting | `encoding/json` with struct tags | N/A (always use stdlib) | Struct tags are the standard Go pattern. Custom JSON building is an anti-pattern. |
| Separate output format functions | Single function with format parameter | N/A (design choice) | Reduces code duplication between human and JSON output paths |
| Color codes in piped output | `fatih/color` auto-detection + explicit `NoColor` | fatih/color v1.7+ | fatih/color checks `NO_COLOR` env var, `TERM=dumb`, and TTY status automatically |

**Deprecated/outdated:**
- None relevant. `encoding/json` and Cobra flag patterns are stable.

## Open Questions

1. **Should `--json` alone (without scan flags) scan all or error?**
   - What we know: The success criteria say `--json` provides "structured data for automation." The AI agent use case from PROJECT.md is `mac-cleaner --all --dry-run --json`.
   - What's unclear: Whether `mac-cleaner --json` alone should imply `--all` or require explicit scan selection.
   - Recommendation: Require at least `--all` or one scan flag when `--json` is used. Print error and exit if `--json` is used without scan flags. This prevents accidental interactive-mode-with-JSON which doesn't make sense (JSON output can't coexist with interactive prompts).

2. **Should JSON output include cleanup results (items removed, errors)?**
   - What we know: The success criteria focus on `--json` for scan output. The primary use case is `--dry-run --json`.
   - What's unclear: Whether `--json` without `--dry-run` should also output cleanup results in JSON format.
   - Recommendation: For v1, `--json` outputs scan results only. If `--force` is combined with `--json` (no dry-run), still output the scan results as JSON. Cleanup result (items removed, bytes freed, errors) can be added to the JSON schema later if needed.

3. **How many item-level skip flags to implement?**
   - What we know: The success criteria mention `--skip-derived-data` by name. The requirement says "skip specific items with `--skip-<item>` flags."
   - What's unclear: Whether all 12 possible item-level skip flags are needed, or whether a subset suffices.
   - Recommendation: Implement all 12 for completeness. They are trivial to add (one bool per flag, one map entry) and provide the granular control that differentiates this tool from competitors. The data-driven approach (flag-to-category mapping) makes each additional flag a one-line addition.

4. **Should `--force` be allowed in interactive mode?**
   - What we know: Interactive mode has its own confirmation mechanism (item-by-item keep/remove selection).
   - What's unclear: Whether `--force` should skip the final "Type 'yes' to proceed" prompt in interactive mode.
   - Recommendation: `--force` skips the final confirmation in both interactive and flag-based modes. In interactive mode, the user already made explicit per-item choices, so the final confirmation is redundant when `--force` is set. This is consistent and predictable.

## Sources

### Primary (HIGH confidence)
- **Codebase analysis** -- Direct reading of all 22 source files in `/Users/gregor/projects/mac-clarner/`. Every category ID, flag name, type definition, and test pattern verified from source code.
- [Go encoding/json docs](https://pkg.go.dev/encoding/json) -- `MarshalIndent`, `NewEncoder`, struct tag syntax. Verified via `go doc`.
- [Cobra Command docs](https://pkg.go.dev/github.com/spf13/cobra) -- `PreRun`, `MarkFlagsMutuallyExclusive`, `BoolVar` registration pattern. Verified via `go doc`.
- [fatih/color NoColor](https://pkg.go.dev/github.com/fatih/color) -- `color.NoColor` global variable for disabling color programmatically.
- [mattn/go-isatty](https://pkg.go.dev/github.com/mattn/go-isatty) -- `IsTerminal(fd uintptr) bool` for TTY detection. Already in dependency tree.

### Secondary (MEDIUM confidence)
- Prior phase research (Phases 1-5) -- Established patterns for flag registration, output formatting, io.Reader/io.Writer injection, and test patterns. All verified against current source code.

### Tertiary (LOW confidence)
- None -- all findings verified through codebase analysis or official Go/Cobra documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies. All stdlib patterns. `encoding/json` is the most battle-tested JSON library in Go.
- Architecture: HIGH -- All patterns are direct extensions of established codebase conventions. Flag registration, PreRun hooks, and result filtering are straightforward Cobra/Go patterns.
- Category ID mapping: HIGH -- Every category ID verified by reading all four scanner source files and their tests.
- Skip flag design: HIGH -- Direct mapping from existing category IDs to flag names. Data-driven filtering approach is well-understood.
- Pitfalls: HIGH -- ANSI-in-JSON, --all dispatch order, and --force safety are common CLI tool issues with clear mitigations.

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, stdlib APIs don't change)
