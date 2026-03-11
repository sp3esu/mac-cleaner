# Phase 13: Home Directory File Scanning - Research

**Researched:** 2026-03-11
**Domain:** Go filesystem walking, file size/age filtering, home directory scanning
**Confidence:** HIGH

## Summary

This phase adds a scanner that finds large files (100MB+) and old files (180+ days) in the user's home directory. The critical difference from Phase 12 (project caches) is that this scanner discovers **individual files**, not cache directories. The existing `internal/walk` utility only visits directories (files are skipped at line 53), so the scanner must either extend the walker or use `os.ReadDir` within walked directories to discover files.

The scanner produces two CategoryResult entries: `home-large-files` (files over 100MB) and `home-old-files` (files over 180 days old). These are independent criteria -- a file could appear in both categories. The exclusion list (~/Library, ~/Applications, ~/.Trash, ~/Downloads, dotfile directories) prevents overlap with existing scanners. The cleanup path uses the existing `os.RemoveAll` in `cleanup.Execute`, which works for individual files without modification.

The design decision flagged in STATE.md ("advisory-only vs. per-file confirmation") does not require new infrastructure. The existing cleanup flow already operates per-entry, and risk levels (`risky`) ensure the user sees appropriate warnings. The scanner is advisory by nature -- it reports findings, and the existing confirmation/interactive flows handle the rest.

**Primary recommendation:** Build `pkg/homedir/scanner.go` that uses the existing `internal/walk` package with an `OnFile` callback extension (or walks directories then reads files within each), producing two CategoryResult entries with `risky` risk level. Register as a single scanner group `home-files` following the exact pattern established by all other scanners.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `filepath.WalkDir` | stdlib | Recursive directory traversal (via `internal/walk`) | Already used by Phase 12; proven pattern |
| `os` | stdlib | File stat, mtime, size, home dir resolution | Used everywhere in project |
| `path/filepath` | stdlib | Path manipulation | Project convention |
| `time` | stdlib | Age calculation (180-day threshold) | Same pattern as `projects` scanner |

### Supporting (already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/walk` | project | Depth-limited directory walker with exclusions | Reuse for traversal; needs OnFile extension or ReadDir inside callback |
| `internal/scan` | project | ScanEntry, CategoryResult, FormatSize | All scan results use these types |
| `internal/safety` | project | IsPathBlocked, RiskForCategory | Path safety checks, risk level mapping |
| `internal/engine` | project | Scanner interface, NewScanner adapter | Scanner registration |
| `github.com/spf13/cobra` | existing dep | CLI flags | --home-files, --skip-home-files, etc. |

### No New Dependencies
This phase requires zero new external dependencies.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  walk/
    walk.go          # MODIFY: Add OnFile callback to Options
    walk_test.go     # ADD: Tests for OnFile callback
pkg/
  homedir/
    scanner.go       # NEW: Large file + old file scanning
    scanner_test.go  # NEW: Tests with t.TempDir() filesystem fixtures
```

### Pattern 1: Extend Walk Utility with OnFile Callback

**What:** Add an optional `OnFile` callback to `walk.Options` so the walker can report regular files in addition to directories. When `OnFile` is nil (existing behavior), files are still skipped. This keeps the walker generic and reusable.

**Why not ReadDir inside OnDir:** Using `os.ReadDir` inside the `OnDir` callback would double-read directory contents (once by `filepath.WalkDir`, once by `os.ReadDir`). Adding `OnFile` to the walker is a single-line change to the existing skip logic and avoids redundant I/O.

**Recommended change to `internal/walk/walk.go`:**
```go
type Options struct {
    Root       string
    MaxDepth   int
    Exclude    []string
    ExcludeAbs []string
    OnDir      func(path string, d fs.DirEntry, depth int) bool
    // OnFile is called for each regular file visited. If nil, files are skipped.
    OnFile     func(path string, d fs.DirEntry, depth int)
}

// In Walk(), replace:
//   if !d.IsDir() { return nil }
// with:
//   if !d.IsDir() {
//       if opts.OnFile != nil {
//           // Calculate depth for the file's parent directory + 1
//           opts.OnFile(path, d, depth)
//       }
//       return nil
//   }
```

**Key:** The depth calculation for files must use the same logic as directories. A file at `~/Documents/report.pdf` has depth 2 (Documents=1, report.pdf=2). The MaxDepth check should apply to files too -- if MaxDepth is 3, files at depth 4 should not be reported.

**Backward compatibility:** All existing callers (Phase 12 `pkg/projects`) leave OnFile as nil, so behavior is unchanged.

### Pattern 2: Scanner With Two Categories
**What:** `pkg/homedir/Scan()` walks the home directory with OnFile, and for each file checks two independent criteria: size > 100MB and age > 180 days. Files matching either criterion are collected into separate category buckets. A single file can appear in both categories.

**Implementation sketch:**
```go
package homedir

const (
    defaultSizeThreshold = 100 * 1024 * 1024  // 100 MB
    defaultAgeThreshold  = 180 * 24 * time.Hour // 180 days
)

func Scan() ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("homedir: resolve home dir: %w", err)
    }
    return scanFrom(home)
}

func scanFrom(root string) ([]scan.CategoryResult, error) {
    var largeFiles []scan.ScanEntry
    var oldFiles []scan.ScanEntry

    err := walk.Walk(walk.Options{
        Root:     root,
        MaxDepth: 5,
        Exclude:  []string{".git", "node_modules", "target", ".build", "build", "venv", ".venv"},
        ExcludeAbs: []string{
            filepath.Join(root, "Library"),
            filepath.Join(root, "Applications"),
            filepath.Join(root, ".Trash"),
            filepath.Join(root, "Downloads"),
        },
        OnFile: func(path string, d fs.DirEntry, depth int) {
            info, err := d.Info()
            if err != nil {
                return
            }

            size := info.Size()
            mtime := info.ModTime()

            if size >= defaultSizeThreshold {
                largeFiles = append(largeFiles, scan.ScanEntry{
                    Path:        path,
                    Description: filepath.Base(path),
                    Size:        size,
                })
            }

            if time.Since(mtime) >= defaultAgeThreshold {
                oldFiles = append(oldFiles, scan.ScanEntry{
                    Path:        path,
                    Description: filepath.Base(path) + "  " + formatAge(mtime),
                    Size:        size,
                })
            }
        },
    })
    if err != nil {
        return nil, fmt.Errorf("homedir: walk: %w", err)
    }

    var results []scan.CategoryResult
    if len(largeFiles) > 0 {
        // Sort by size descending
        sort.Slice(largeFiles, func(i, j int) bool {
            return largeFiles[i].Size > largeFiles[j].Size
        })
        var totalSize int64
        for _, e := range largeFiles {
            totalSize += e.Size
        }
        cr := scan.CategoryResult{
            Category:    "home-large-files",
            Description: fmt.Sprintf("Large Files >100 MB (%d files, %s)", len(largeFiles), scan.FormatSize(totalSize)),
            Entries:     largeFiles,
            TotalSize:   totalSize,
        }
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, cr)
    }
    if len(oldFiles) > 0 {
        // Sort by size descending (most impactful first)
        sort.Slice(oldFiles, func(i, j int) bool {
            return oldFiles[i].Size > oldFiles[j].Size
        })
        var totalSize int64
        for _, e := range oldFiles {
            totalSize += e.Size
        }
        cr := scan.CategoryResult{
            Category:    "home-old-files",
            Description: fmt.Sprintf("Old Files >180 days (%d files, %s)", len(oldFiles), scan.FormatSize(totalSize)),
            Entries:     oldFiles,
            TotalSize:   totalSize,
        }
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, cr)
    }

    return results, nil
}
```

### Pattern 3: Scanner Registration (same as all others)
```go
// In RegisterDefaults():
e.Register(NewScanner(ScannerInfo{
    ID:          "homedir",
    Name:        "Home Directory Files",
    Description: "Large and old files in home directory",
    CategoryIDs: []string{"home-large-files", "home-old-files"},
    SkipFlag:    "home-files",
    SubCategories: []SubCategoryInfo{
        {ID: "home-large-files", Label: "Large Files (>100 MB)", SkipFlag: "large-files", RiskLevel: safety.RiskForCategory("home-large-files")},
        {ID: "home-old-files", Label: "Old Files (>180 days)", SkipFlag: "old-files", RiskLevel: safety.RiskForCategory("home-old-files")},
    },
}, homedir.Scan))
```

### Pattern 4: CLI Flag Registration (same as all others)
```go
// In root.go flag variables:
var flagHomeFiles bool
var flagSkipHomeFiles bool
var flagSkipLargeFiles bool
var flagSkipOldFiles bool

// Targeted scan flags:
var flagScanLargeFiles bool
var flagScanOldFiles bool

// In scanGroups:
{
    FlagName:    "home-files",
    ScannerID:   "homedir",
    GroupName:   "Home Directory Files",
    Description: "large and old files in home directory",
    ScanFlag:    &flagHomeFiles,
    SkipFlag:    &flagSkipHomeFiles,
    Items: []categoryDef{
        {FlagName: "large-files", CategoryID: "home-large-files", Description: "large files (>100 MB)", SkipFlag: &flagSkipLargeFiles, ScanFlag: &flagScanLargeFiles},
        {FlagName: "old-files", CategoryID: "home-old-files", Description: "old files (>180 days)", SkipFlag: &flagSkipOldFiles, ScanFlag: &flagScanOldFiles},
    },
},

// In scannerMapping in rootCmd.Run:
{&flagHomeFiles, "homedir"},
```

### Anti-Patterns to Avoid
- **Scanning inside ~/Library, ~/Downloads, ~/.Trash:** These are already covered by existing scanners. Including them creates duplicate entries and confuses users.
- **Using os.ReadDir inside OnDir callback:** This double-reads directory contents since `filepath.WalkDir` already iterates entries. Extend the walker with OnFile instead.
- **Showing all old files regardless of size:** A 1KB file from 2020 is technically "old" but wastes display space. Consider a minimum size threshold for old files (e.g., 10MB) to keep results actionable. The FEATURES.md research suggested ">10MB" for old files.
- **Modifying ScanEntry type:** Do NOT add fields for file age or type. Use the Description field for annotations, matching the Phase 12 staleness pattern.
- **Making cleanup destructive by default:** These are personal files, not regenerable caches. Risk level MUST be `risky` to trigger the strongest warnings in the existing display/confirmation flow.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Directory walking | Custom recursive walk | `internal/walk.Walk()` (extended) | Proven depth/exclusion logic, reused across phases |
| Size formatting | Custom formatter | `scan.FormatSize()` | Already used everywhere |
| Path safety | Custom blocklist | `safety.IsPathBlocked()` | Resolves symlinks, checks SIP/swap |
| Risk mapping | Inline risk strings | `safety.RiskForCategory()` | Central registry, tested |
| Scanner registration | Custom engine hookup | `engine.NewScanner()` adapter | Standard pattern |
| CLI flag plumbing | Manual cobra setup | `scanGroups` registry | Auto-generates group/item/skip flags |
| File deletion | Custom removal | `cleanup.Execute()` with `os.RemoveAll()` | Already works for individual files |
| Age formatting | New formatter | Reuse `formatAge()` from `pkg/projects` | Same logic (or extract to shared package) |

**Key insight:** The `formatAge` and `isStale` functions in `pkg/projects/scanner.go` are unexported. For Phase 13, either duplicate them in `pkg/homedir/` (simple, avoids import cycles) or extract to a shared utility. Duplication is acceptable since the logic is trivial (10 lines total).

## Common Pitfalls

### Pitfall 1: Walking Into Excluded Directories Despite Exclusion
**What goes wrong:** The scanner walks ~/Library or ~/Downloads and reports files that are already covered by other scanners.
**Why it happens:** Incorrect exclusion configuration -- forgetting `.Trash` or using wrong path format.
**How to avoid:** Use `ExcludeAbs` with `filepath.Join(root, "Library")` etc. Add `.Trash` and `Downloads` to the absolute exclusion list. Test exclusions explicitly.
**Warning signs:** Duplicate entries in scan results; files from ~/Library appearing.

### Pitfall 2: Performance -- Scanning Too Many Files
**What goes wrong:** The scanner walks deep into project directories, scanning millions of files inside node_modules, .build, etc.
**Why it happens:** Not excluding project cache directory names from the walk.
**How to avoid:** Add project cache directory names (`node_modules`, `.build`, `target`, `build`, `venv`, `.venv`) to the `Exclude` list. The walk utility skips these by base name at any depth.
**Warning signs:** Scan takes >30 seconds on a developer machine.

### Pitfall 3: iCloud Drive Placeholder Files
**What goes wrong:** macOS iCloud Drive can show placeholder files that appear large (their "real" size) but are not actually on disk. Scanning these reports space that is not locally consumed.
**Why it happens:** iCloud evicted files have their original size in metadata but only occupy a few KB on disk.
**How to avoid:** For v1.7.0, document this as a known limitation. Full iCloud detection requires checking extended attributes (`com.apple.ubiquity.peercount` or file flags), which is complex. The file IS a real file that CAN be deleted. Users in iCloud-heavy setups may see inflated totals.
**Warning signs:** Reported total exceeds actual disk usage.

### Pitfall 4: Symlinks to External Volumes
**What goes wrong:** A symlink in the home directory points to a network mount or external drive. Walking it could scan unintended locations.
**Why it happens:** Users create convenience symlinks to NAS/external storage.
**How to avoid:** `filepath.WalkDir` does NOT follow symlinks by default. Regular files that are symlinks will be visited but `d.Info()` returns the symlink's own info, not the target. The `OnFile` callback should use `d.Type()` to check `fs.ModeSymlink` and skip symlinks, or use `os.Lstat` behavior (which is what `d.Info()` provides).
**Warning signs:** Files from external volumes appearing in results.

### Pitfall 5: Old Files Threshold Produces Too Many Results
**What goes wrong:** Without a minimum size filter for old files, the scanner returns thousands of small old files (configs, READMEs, etc.), making results unusable.
**Why it happens:** Most files in a home directory are "old" since they were created once and rarely modified.
**How to avoid:** Apply a minimum size threshold for old files. The FEATURES.md research suggested 10MB. This keeps results focused on files that actually matter for space reclamation.
**Warning signs:** Hundreds of entries in the old files category, most under 1MB.

### Pitfall 6: Duplicate Entries Across Categories
**What goes wrong:** A 500MB file from 2022 appears in both "large files" and "old files" categories. User deletes it from one category, then the other category shows a stale entry.
**Why it happens:** The two criteria are independent; overlap is inherent.
**How to avoid:** This is expected behavior and acceptable. The cleanup module handles `os.IsNotExist` gracefully (line 70 of cleanup.go). Document in the category description that files may appear in both categories. The existing interactive mode already handles this since each category is cleaned independently.
**Warning signs:** None -- this is by design.

## Code Examples

### Walk Extension for File Callbacks
```go
// Source: extending existing internal/walk/walk.go
// In the Walk function, replace:
if !d.IsDir() {
    return nil
}
// With:
if !d.IsDir() {
    if opts.OnFile != nil {
        depth := 0
        if len(path) > rootLen {
            rel := path[rootLen+1:]
            depth = strings.Count(rel, string(filepath.Separator)) + 1
        }
        if opts.MaxDepth == 0 || depth <= opts.MaxDepth {
            opts.OnFile(path, d, depth)
        }
    }
    return nil
}
```

### Age Formatting (duplicated from pkg/projects)
```go
func formatAge(mtime time.Time) string {
    d := time.Since(mtime)
    days := int(d.Hours() / 24)
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
```

### Risk Level Registration
```go
// Add to internal/safety/risk.go categoryRisk map:
"home-large-files": RiskRisky,
"home-old-files":   RiskRisky,
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.Walk` | `filepath.WalkDir` | Go 1.16 (Feb 2021) | WalkDir uses `fs.DirEntry` -- avoids `os.Stat` on every file, much faster for large dirs |
| Separate walks per criterion | Single walk with multi-criterion callback | Phase 13 design | One pass over filesystem instead of two |

## Open Questions

1. **Minimum size threshold for old files**
   - What we know: FEATURES.md research suggested ">10MB" for old files to keep results actionable
   - What's unclear: Whether to hardcode 10MB or use the same threshold as large files (100MB)
   - Recommendation: Use 10MB as minimum for old files. A 500KB old text file is not worth flagging for space reclamation. This is a hardcoded constant, not user-configurable (CONF-01/02 are deferred to future requirements).

2. **Advisory-only vs. per-file confirmation**
   - What we know: STATE.md flags this as a design decision. The existing cleanup flow already works per-entry with risk-based warnings. Setting risk to `risky` triggers the strongest confirmation prompts.
   - What's unclear: Nothing -- the existing flow handles this.
   - Recommendation: Use `risky` risk level. The existing interactive mode asks per-category with risk warnings displayed. No new confirmation flow needed. This resolves the research flag from STATE.md.

3. **Symlink files in home directory**
   - What we know: `filepath.WalkDir` reports symlinks but does not follow them. `d.Info()` returns the symlink metadata, not the target.
   - What's unclear: Whether to report symlinks or skip them.
   - Recommendation: Skip symlink files. They point elsewhere; deleting the symlink frees negligible space (the link itself, not the target). Check `d.Type()&fs.ModeSymlink != 0` in OnFile.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | None (Go convention) |
| Quick run command | `go test ./internal/walk/... ./pkg/homedir/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HOME-01 | Detect files >100MB | unit | `go test ./pkg/homedir/... -run TestLargeFiles` | Wave 0 |
| HOME-02 | Detect files >180 days old | unit | `go test ./pkg/homedir/... -run TestOldFiles` | Wave 0 |
| HOME-03 | Exclude Library, Applications, .Trash, Downloads, dotfiles | unit | `go test ./pkg/homedir/... -run TestExclusions` | Wave 0 |
| -- | Walk OnFile callback works | unit | `go test ./internal/walk/... -run TestOnFileCallback` | Wave 0 |
| -- | Walk OnFile respects depth limit | unit | `go test ./internal/walk/... -run TestOnFileMaxDepth` | Wave 0 |
| -- | Walk OnFile skips excluded dirs | unit | `go test ./internal/walk/... -run TestOnFileExclusions` | Wave 0 |
| -- | Symlink files skipped | unit | `go test ./pkg/homedir/... -run TestSymlinkSkip` | Wave 0 |
| -- | Old files minimum size threshold | unit | `go test ./pkg/homedir/... -run TestOldFileMinSize` | Wave 0 |
| -- | Files can appear in both categories | unit | `go test ./pkg/homedir/... -run TestDualCategory` | Wave 0 |
| -- | Risk levels correctly mapped | unit | `go test ./internal/safety/... -run TestRiskForCategory` | Existing (needs new entries) |

### Sampling Rate
- **Per task commit:** `go test ./internal/walk/... ./pkg/homedir/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green + `go vet ./...` + `gosec ./...`

### Wave 0 Gaps
- [ ] `internal/walk/walk_test.go` -- add OnFile callback tests (file exists, needs new test functions)
- [ ] `pkg/homedir/scanner_test.go` -- all file scanning tests
- [ ] `internal/safety/risk.go` -- add `home-large-files` and `home-old-files` entries

## Sources

### Primary (HIGH confidence)
- Project source code: `internal/walk/walk.go` -- current walker implementation, line 53 skips non-directories
- Project source code: `pkg/projects/scanner.go` -- reference scanner with walk integration, age formatting
- Project source code: `internal/engine/registry.go` -- scanner registration pattern (10 examples)
- Project source code: `cmd/categories.go` -- scanGroups registry pattern
- Project source code: `cmd/root.go` -- flag variables and scannerMapping pattern
- Project source code: `internal/cleanup/cleanup.go` -- `os.RemoveAll` works for files, line 69
- Project source code: `internal/safety/risk.go` -- risk level registry
- Project source code: `internal/scan/types.go` -- ScanEntry, CategoryResult types

### Secondary (MEDIUM confidence)
- `.planning/research/FEATURES.md` -- original feature research with category IDs and suggested thresholds
- `.planning/research/SUMMARY.md` -- phase ordering rationale and design decision flags

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, zero new deps, 10+ existing scanner examples to follow
- Architecture: HIGH -- extends proven walk utility with minimal change; scanner follows exact established pattern
- Pitfalls: HIGH -- informed by actual codebase, iCloud limitation documented honestly
- Walk extension: HIGH -- single-line change to existing skip logic, backward compatible

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable domain, no external dependencies)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| HOME-01 | User can scan for large files (default 100MB+) in home directory | Walk utility extended with OnFile callback; scanner checks `info.Size() >= 100MB`; results in `home-large-files` CategoryResult sorted by size descending |
| HOME-02 | User can scan for old files (default 180+ days) in home directory | Same walk pass; scanner checks `time.Since(info.ModTime()) >= 180 days`; minimum 10MB size filter; results in `home-old-files` CategoryResult with age annotation in Description |
| HOME-03 | Scanner excludes ~/Library, ~/Applications, ~/.Trash, ~/Downloads, and dotfile directories to avoid overlap | Walk `ExcludeAbs` for Library, Applications, .Trash, Downloads; walk already skips dotfile dirs at depth 1; project cache dirs excluded by name |
</phase_requirements>
