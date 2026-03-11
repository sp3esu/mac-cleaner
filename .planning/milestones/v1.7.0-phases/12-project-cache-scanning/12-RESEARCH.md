# Phase 12: Project Cache Scanning - Research

**Researched:** 2026-03-11
**Domain:** Go filesystem walking, project cache detection, manifest parsing
**Confidence:** HIGH

## Summary

This phase adds a new scanner that walks the user's home directory once to discover project build caches (node_modules, .build, target/, venv/.venv, build/) and report their sizes. The core challenge is building an efficient recursive walker with depth limits and directory exclusions, then layering manifest-based validation on top to eliminate false positives. All infrastructure (types, engine registration, safety, display, cleanup) already exists and is well-established.

The walk utility (`internal/walk`) is the only genuinely new component. Everything else -- scanner registration, category results, risk levels, skip flags, display formatting -- follows patterns repeated 10 times across existing scanners. The staleness feature (annotating entries with age from manifest mtime) is a minor extension to the Description field on ScanEntry.

**Primary recommendation:** Build `internal/walk` as a callback-based directory walker with depth limit and exclusion set, then build `pkg/projects/scanner.go` that uses it to emit one CategoryResult per project type following existing patterns exactly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- One category per project type: proj-node, proj-swift, proj-rust, proj-python, proj-java
- Each type gets its own skip flag (--skip-node-modules, --skip-swift-build, etc.)
- Single scanner group "project-caches" with group-level --project-caches / --skip-project-caches flags
- Aggregate summary embedded in category description line: "node_modules (47 dirs, 28.3 GB)"
- Parse project name from manifest file; fall back to parent directory name
- Entry path is the cache directory; description is the project name
- Staleness from manifest file mtime; 6-month threshold hardcoded; stale entries marked with warning symbol and age in months
- Entries sorted by size descending
- Risk levels: node_modules/safe, .build/safe, target/safe, build/safe, venv-venv/moderate
- Delete entire cache directory via existing cleanup package
- New `internal/walk` package with max depth 5
- Walk exclusions: ~/Library, dotfile directories, ~/Applications, found cache directories
- New `pkg/projects/` scanner package
- Registered as single scanner emitting separate CategoryResult per type

### Claude's Discretion
- Walk utility API design (callback vs channel vs slice return)
- Exact manifest parsing implementation details
- Age formatting logic (days, months, years)
- How to handle symlinked project directories during walk
- Performance optimizations (parallel directory reading, etc.)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROJ-01 | Scan node_modules with size reporting | Walk utility discovers `node_modules` dirs; validate via sibling `package.json`; size via `scan.DirSize()` |
| PROJ-02 | Scan .build (Swift) with size reporting | Walk discovers `.build` dirs; validate via sibling `Package.swift` |
| PROJ-03 | Scan target/ (Rust) with size reporting | Walk discovers `target` dirs; validate via sibling `Cargo.toml` |
| PROJ-04 | Scan venv/.venv (Python) with size reporting | Walk discovers `venv`/`.venv` dirs; validate via `pyvenv.cfg` inside the dir |
| PROJ-05 | Scan build/ (Java/Kotlin) with size reporting | Walk discovers `build` dirs; validate via sibling `build.gradle`, `build.gradle.kts`, or `pom.xml` |
| PROJ-06 | Single-pass recursive walk | `internal/walk` package walks home once; scanner checks all cache dir names in a single pass |
| PROJ-07 | Stale projects highlighted (6+ months) | `os.Stat` manifest mtime; format age; embed warning symbol in Description |
| PROJ-08 | Aggregate summaries per type | Set CategoryResult.Description to "node_modules (47 dirs, 28.3 GB)" after collecting all entries |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `filepath.WalkDir` | stdlib | Recursive directory traversal | Standard Go approach; uses `fs.DirEntry` for lazy stat |
| `os` | stdlib | File stat, mtime, home dir | Already used everywhere in project |
| `path/filepath` | stdlib | Path manipulation | Project convention |
| `time` | stdlib | Staleness calculation | Duration arithmetic for 6-month threshold |

### Supporting (already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/scan` | project | ScanEntry, CategoryResult, DirSize, FormatSize | All scan results use these types |
| `internal/safety` | project | IsPathBlocked, RiskForCategory | Path safety checks, risk level mapping |
| `internal/engine` | project | Scanner interface, NewScanner adapter, RegisterDefaults | Scanner registration |
| `github.com/fatih/color` | existing dep | Colored terminal output | Warning symbol colorization |
| `github.com/spf13/cobra` | existing dep | CLI flags | --project-caches, --skip-* flags |

### No New Dependencies
This phase requires zero new external dependencies. Everything uses stdlib and existing project packages.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  walk/
    walk.go          # Walker type, Walk() function, Options, exclusion logic
    walk_test.go     # Tests with t.TempDir() filesystem fixtures
pkg/
  projects/
    scanner.go       # Scan() function, project type definitions, manifest parsing
    scanner_test.go  # Tests with t.TempDir() filesystem fixtures
```

### Pattern 1: Callback-Based Walker
**What:** The walk utility accepts a callback function invoked for each directory entry, with a depth limit and exclusion set. This is the simplest API that avoids allocating a large slice of all directories.
**When to use:** The scanner needs to process directories as they are found, not wait for a complete list.
**Recommended API:**
```go
// internal/walk/walk.go
package walk

// Options configures the directory walker.
type Options struct {
    Root      string       // Starting directory (e.g., home dir)
    MaxDepth  int          // Maximum recursion depth (0 = root only)
    Exclude   []string     // Directory base names to skip entirely
    ExcludeAbs []string    // Absolute path prefixes to skip
    OnDir     func(path string, d fs.DirEntry, depth int) (descend bool) // Called for each directory
}

// Walk performs a recursive directory walk respecting depth and exclusion rules.
func Walk(opts Options) error { ... }
```

**Key design points:**
- The `OnDir` callback returns `bool` -- `false` means "do not descend into this directory" (used for cache dirs like node_modules).
- `Exclude` matches base names (e.g., `.git`, `node_modules`); `ExcludeAbs` matches absolute path prefixes (e.g., `~/Library`).
- The walker itself does NOT know about project types. It is a generic utility reusable by Phases 13 and 14.

### Pattern 2: Scanner With Walk Integration
**What:** `pkg/projects/Scan()` creates a Walker, walks home dir, and in the callback checks each directory name against a map of known cache directory names. When a match is found, it validates via manifest marker, sizes the directory, and appends to the appropriate category's entries.
**Implementation sketch:**
```go
// pkg/projects/scanner.go
package projects

// projectType defines a project cache type with its detection rules.
type projectType struct {
    CategoryID   string   // e.g. "proj-node"
    CacheDirName string   // e.g. "node_modules"
    Markers      []string // Sibling manifest files: e.g. {"package.json"}
    MarkerInside bool     // If true, look for marker inside cache dir (pyvenv.cfg)
    Label        string   // e.g. "node_modules"
}

var projectTypes = []projectType{
    {CategoryID: "proj-node", CacheDirName: "node_modules", Markers: []string{"package.json"}, Label: "node_modules"},
    {CategoryID: "proj-swift", CacheDirName: ".build", Markers: []string{"Package.swift"}, Label: ".build (Swift)"},
    {CategoryID: "proj-rust", CacheDirName: "target", Markers: []string{"Cargo.toml"}, Label: "target (Rust)"},
    {CategoryID: "proj-python", CacheDirName: "venv", Markers: []string{"pyvenv.cfg"}, MarkerInside: true, Label: "venv (Python)"},
    // .venv handled as second entry with same CategoryID
    {CategoryID: "proj-python", CacheDirName: ".venv", Markers: []string{"pyvenv.cfg"}, MarkerInside: true, Label: "venv (Python)"},
    {CategoryID: "proj-java", CacheDirName: "build", Markers: []string{"build.gradle", "build.gradle.kts", "pom.xml"}, Label: "build (Java/Kotlin)"},
}

func Scan() ([]scan.CategoryResult, error) { ... }
```

### Pattern 3: Existing Scanner Registration
**What:** Follow exact same pattern as all other scanners in `registry.go`.
```go
// In RegisterDefaults():
e.Register(NewScanner(ScannerInfo{
    ID:          "projects",
    Name:        "Project Caches",
    Description: "node_modules, .build, target, venv, and build directories",
    CategoryIDs: []string{"proj-node", "proj-swift", "proj-rust", "proj-python", "proj-java"},
    SkipFlag:    "project-caches",
    SubCategories: []SubCategoryInfo{
        {ID: "proj-node", Label: "node_modules", SkipFlag: "node-modules", RiskLevel: safety.RiskForCategory("proj-node")},
        {ID: "proj-swift", Label: ".build (Swift)", SkipFlag: "swift-build", RiskLevel: safety.RiskForCategory("proj-swift")},
        {ID: "proj-rust", Label: "target (Rust)", SkipFlag: "rust-target", RiskLevel: safety.RiskForCategory("proj-rust")},
        {ID: "proj-python", Label: "venv (Python)", SkipFlag: "python-venv", RiskLevel: safety.RiskForCategory("proj-python")},
        {ID: "proj-java", Label: "build (Java/Kotlin)", SkipFlag: "java-build", RiskLevel: safety.RiskForCategory("proj-java")},
    },
}, projects.Scan))
```

### Pattern 4: Staleness Annotation in Description
**What:** Embed age information in the ScanEntry.Description field, since that is what `printResults` renders. The description becomes e.g., `"my-app  \u26a0 14mo"` for stale or `"dashboard  2d"` for recent.
**Why:** No changes needed to `cmd/root.go` display code -- it already prints Description as-is. The age annotation is part of the description string.
**Age formatting:** `<7d` -> "Xd", `<12mo` -> "Xmo", `>=12mo` -> "Xy"

### Anti-Patterns to Avoid
- **Walking inside cache directories:** Never descend into node_modules, target/, .build, etc. These contain millions of files. The walker must stop descent when it finds a cache dir.
- **One walk per project type:** Do NOT walk the home directory 5 times (once per type). Use a single walk with a lookup map.
- **Modifying ScanEntry type:** Do NOT add new fields to ScanEntry for staleness. Use the Description field. The type is shared across all scanners and serialized as JSON for the IPC server.
- **Walking ~/Library:** Already covered by existing scanners. Descending into it would be extremely slow and produce duplicates.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Directory sizing | Custom size calculator | `scan.DirSize()` | Already handles permission errors, skips unreadable files |
| Size formatting | Custom formatter | `scan.FormatSize()` | Existing SI-unit formatter used everywhere |
| Path safety | Custom blocklist | `safety.IsPathBlocked()` | Resolves symlinks, checks SIP, swap, home containment |
| Risk mapping | Inline risk strings | `safety.RiskForCategory()` | Central registry, tested, consistent |
| Scanner registration | Custom engine hookup | `engine.NewScanner()` adapter | Wraps `Scan()` functions into Scanner interface |
| CLI flag plumbing | Manual cobra flag setup | `scanGroups` registry + existing init() pattern | Auto-generates group/item/skip flags |

## Common Pitfalls

### Pitfall 1: Walking Into Cache Directories
**What goes wrong:** If the walker descends into `node_modules`, it walks millions of nested directories, making the scan take minutes instead of seconds.
**Why it happens:** `filepath.WalkDir` descends by default.
**How to avoid:** The `OnDir` callback must return `false` (skip descent) for recognized cache directory names. Additionally, the walk exclusion list must include these names.
**Warning signs:** Scan takes >10 seconds on a typical developer machine.

### Pitfall 2: False Positives on Generic Directory Names
**What goes wrong:** `build/` and `target/` are generic names. Without manifest validation, the scanner reports non-project directories (e.g., a `build/` inside a documentation site).
**Why it happens:** Only checking the directory name without verifying a project manifest exists.
**How to avoid:** Always require a manifest marker file (build.gradle, pom.xml, Cargo.toml) as a sibling. No marker = skip.
**Warning signs:** Reported directories that aren't actually build caches.

### Pitfall 3: Python venv Marker Location
**What goes wrong:** For Python virtualenvs, the marker file (`pyvenv.cfg`) is INSIDE the venv directory, not alongside it like other project types.
**Why it happens:** venv is a self-contained environment, not a build output directory.
**How to avoid:** Use the `MarkerInside` flag to check inside the directory (`venv/pyvenv.cfg`) rather than looking for a sibling file.
**Warning signs:** No Python venvs detected despite having many.

### Pitfall 4: Symlink Loops in Walk
**What goes wrong:** A symlink pointing to a parent directory causes infinite recursion.
**Why it happens:** Some developers create symlinks for convenience.
**How to avoid:** `filepath.WalkDir` does NOT follow symlinks by default -- it reports them as entries but does not descend. This is the correct behavior. Do NOT use `filepath.Walk` (deprecated) or manually resolve symlinks during the walk. If a cache directory IS a symlink, `os.Stat` (not `os.Lstat`) should be used for sizing to get the real directory.
**Warning signs:** Walk hangs or panics with stack overflow.

### Pitfall 5: DirSize on Large Cache Directories
**What goes wrong:** `scan.DirSize()` walks the entire directory tree to sum file sizes. For a large `node_modules` (100k+ files), this can take several seconds per directory.
**Why it happens:** No shortcut for directory size on macOS without walking.
**How to avoid:** This is unavoidable but acceptable -- the scan is I/O-bound and users expect it to take some time. The spinner already covers this. Do NOT try to parallelize DirSize calls without careful goroutine management (filesystem I/O doesn't scale linearly with parallelism).
**Warning signs:** None -- this is expected behavior. Just ensure the spinner message updates per-type.

### Pitfall 6: Dotfile Directories at Home Level
**What goes wrong:** Walking into `~/.local`, `~/.cache`, `~/.config` etc. wastes time and could produce false positives.
**Why it happens:** These are immediate children of the home directory.
**How to avoid:** Exclude any directory whose base name starts with `.` at the walk level. This covers `.config`, `.local`, `.cache`, `.git`, `.npm`, etc. Note: `.build` (Swift) is found INSIDE project directories, not at the home level, so this exclusion is safe.

## Code Examples

### Walk Utility Core Logic
```go
// Source: project pattern based on filepath.WalkDir
func Walk(opts Options) error {
    excludeNames := make(map[string]bool, len(opts.Exclude))
    for _, name := range opts.Exclude {
        excludeNames[name] = true
    }

    return filepath.WalkDir(opts.Root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // Skip permission errors
        }
        if !d.IsDir() {
            return nil // Only interested in directories
        }

        // Calculate depth relative to root.
        rel, _ := filepath.Rel(opts.Root, path)
        depth := 0
        if rel != "." {
            depth = strings.Count(rel, string(filepath.Separator)) + 1
        }

        // Check depth limit.
        if depth > opts.MaxDepth {
            return fs.SkipDir
        }

        name := d.Name()

        // Skip dotfile directories (except root).
        if depth > 0 && strings.HasPrefix(name, ".") && name != ".build" {
            // .build needs special handling -- skip only at home level
            return fs.SkipDir
        }

        // Check absolute path exclusions.
        for _, prefix := range opts.ExcludeAbs {
            if path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator)) {
                return fs.SkipDir
            }
        }

        // Check name exclusions.
        if excludeNames[name] {
            return fs.SkipDir
        }

        // Invoke callback.
        if !opts.OnDir(path, d, depth) {
            return fs.SkipDir
        }

        return nil
    })
}
```

### Manifest Validation
```go
// Source: project-specific logic
func hasManifest(projectDir string, markers []string) (string, bool) {
    for _, marker := range markers {
        path := filepath.Join(projectDir, marker)
        if _, err := os.Stat(path); err == nil {
            return path, true
        }
    }
    return "", false
}

// For most types: check parent directory of cache dir
// projectDir := filepath.Dir(cacheDirPath)
// _, ok := hasManifest(projectDir, pt.Markers)

// For Python venv: check inside cache dir
// _, ok := hasManifest(cacheDirPath, pt.Markers)
```

### Project Name From Manifest
```go
// Source: project-specific logic
func projectNameFromManifest(manifestPath string) string {
    base := filepath.Base(manifestPath)
    switch base {
    case "package.json":
        data, err := os.ReadFile(manifestPath)
        if err != nil {
            return ""
        }
        var pkg struct{ Name string `json:"name"` }
        if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
            return pkg.Name
        }
    case "Cargo.toml":
        // Simple line-by-line parse for name = "..."
        // under [package] section
    case "Package.swift":
        // Parse: name: "PackageName" from first Package() call
        // Regex: `name:\s*"([^"]+)"`
    }
    return ""
}
```

### Age Formatting
```go
// Source: project-specific logic
func formatAge(mtime time.Time) string {
    age := time.Since(mtime)
    days := int(age.Hours() / 24)
    if days < 1 {
        return "<1d"
    }
    if days < 30 {
        return fmt.Sprintf("%dd", days)
    }
    months := days / 30
    if months < 12 {
        return fmt.Sprintf("%dmo", months)
    }
    years := months / 12
    return fmt.Sprintf("%dy", years)
}

func isStale(mtime time.Time) bool {
    return time.Since(mtime) > 180*24*time.Hour // ~6 months
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.Walk` | `filepath.WalkDir` | Go 1.16 (Feb 2021) | WalkDir uses `fs.DirEntry` -- avoids `os.Stat` on every entry, much faster |
| Manual os.ReadDir loop | `filepath.WalkDir` | Go 1.16 | Handles recursion, errors, depth automatically |

**Note:** `filepath.WalkDir` does NOT follow symlinks. This is correct for our use case -- it prevents infinite loops and we don't want to scan symlinked projects twice.

## Open Questions

1. **Dotfile handling at non-root depth**
   - What we know: Dotfile dirs at home level (~/.config, ~/.local) should be skipped. But `.build` (Swift) appears inside project dirs at deeper levels.
   - What's unclear: Whether to skip ALL dotfile dirs at all depths, or only at depth 1.
   - Recommendation: Skip dotfile dirs ONLY at depth 1 (immediate children of home). At deeper levels, allow `.build` and `.venv` to be found by the cache-dir-name matching. Other dotfiles at depth >1 (like `.git`) should be in the walker's name exclusion list explicitly.

2. **Aggregate summary timing**
   - What we know: Description must show "node_modules (47 dirs, 28.3 GB)" but CategoryResult.Description is set when creating the result.
   - What's unclear: Nothing -- this is straightforward.
   - Recommendation: Collect all entries per type during walk, then build CategoryResult with computed description after walk completes.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | None (Go convention) |
| Quick run command | `go test ./internal/walk/... ./pkg/projects/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROJ-01 | Detect node_modules with package.json sibling | unit | `go test ./pkg/projects/... -run TestScanNodeModules -x` | Wave 0 |
| PROJ-02 | Detect .build with Package.swift sibling | unit | `go test ./pkg/projects/... -run TestScanSwiftBuild -x` | Wave 0 |
| PROJ-03 | Detect target/ with Cargo.toml sibling | unit | `go test ./pkg/projects/... -run TestScanRustTarget -x` | Wave 0 |
| PROJ-04 | Detect venv/.venv with pyvenv.cfg inside | unit | `go test ./pkg/projects/... -run TestScanPythonVenv -x` | Wave 0 |
| PROJ-05 | Detect build/ with build.gradle/pom.xml sibling | unit | `go test ./pkg/projects/... -run TestScanJavaBuild -x` | Wave 0 |
| PROJ-06 | Single-pass walk detects all types | integration | `go test ./pkg/projects/... -run TestScanSinglePass -x` | Wave 0 |
| PROJ-07 | Stale projects annotated with age | unit | `go test ./pkg/projects/... -run TestStalenessMarker -x` | Wave 0 |
| PROJ-08 | Aggregate summaries in description | unit | `go test ./pkg/projects/... -run TestAggregateSummary -x` | Wave 0 |
| -- | Walk depth limit respected | unit | `go test ./internal/walk/... -run TestMaxDepth -x` | Wave 0 |
| -- | Walk excludes ~/Library, dotfiles | unit | `go test ./internal/walk/... -run TestExclusions -x` | Wave 0 |
| -- | Walk does not descend into cache dirs | unit | `go test ./internal/walk/... -run TestCacheDirSkip -x` | Wave 0 |
| -- | No false positives without manifest | unit | `go test ./pkg/projects/... -run TestNoFalsePositive -x` | Wave 0 |
| -- | Project name parsed from manifest | unit | `go test ./pkg/projects/... -run TestProjectName -x` | Wave 0 |
| -- | Risk levels correctly mapped | unit | `go test ./internal/safety/... -run TestRiskForCategory -x` | Existing (needs new entries) |

### Sampling Rate
- **Per task commit:** `go test ./internal/walk/... ./pkg/projects/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green + `go vet ./...` + `gosec ./...`

### Wave 0 Gaps
- [ ] `internal/walk/walk_test.go` -- walker depth, exclusion, callback tests
- [ ] `pkg/projects/scanner_test.go` -- all project type detection, staleness, aggregation tests

## Sources

### Primary (HIGH confidence)
- Project source code: `internal/engine/registry.go`, `internal/scan/types.go`, `internal/scan/helpers.go`, `internal/safety/risk.go`, `cmd/categories.go`, `cmd/scan.go`, `cmd/root.go` -- established patterns for scanner registration, result types, display formatting, flag plumbing
- Project source code: `pkg/developer/scanner.go` -- reference scanner implementation showing Scan() pattern
- Project source code: `internal/safety/safety.go` -- path blocking logic and constraints

### Secondary (MEDIUM confidence)
- Go stdlib `filepath.WalkDir` documentation -- symlink behavior, fs.SkipDir semantics, DirEntry lazy stat

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, no new deps, well-established project patterns
- Architecture: HIGH -- follows exact same pattern as 10 existing scanners; walk utility is straightforward filepath.WalkDir wrapper
- Pitfalls: HIGH -- informed by actual codebase patterns and known filesystem behaviors
- Staleness/display: HIGH -- uses existing Description field, no type changes needed

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable domain, no external dependencies)
