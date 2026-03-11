# Architecture Patterns

**Domain:** Extended scanning for macOS disk cleaner -- home directory files, project caches, git repo management
**Researched:** 2026-03-11
**Confidence:** HIGH (based on direct codebase analysis of existing architecture)

## Recommended Architecture

The three new features map cleanly onto the existing scanner architecture with **two new scanner packages** and **one extension to an existing package**. No changes to the engine, scan types, or safety core are required -- the existing interfaces and patterns accommodate all three features.

### New vs Modified Components

| Component | Status | What Changes |
|-----------|--------|-------------|
| `pkg/homedir/` | **NEW** | Home directory large/old file scanner |
| `pkg/projectcache/` | **NEW** | Project build artifact scanner |
| `pkg/developer/scanner.go` | **MODIFIED** | Add git repo cache scanning functions |
| `internal/scan/walk.go` | **NEW** | Depth-limited directory walk helper |
| `internal/engine/registry.go` | **MODIFIED** | Register two new scanners, extend developer scanner info |
| `internal/safety/risk.go` | **MODIFIED** | Add risk mappings for new category IDs |
| `internal/cleanup/cleanup.go` | **MODIFIED** | Add `git-gc:` pseudo-path handler |
| `cmd/categories.go` | **MODIFIED** | Add new group/category definitions and flag variables |
| `cmd/scan.go` | **MODIFIED** | Wire new flags |
| `cmd/root.go` | **MODIFIED** | Wire new skip/group flags |
| `internal/scan/types.go` | **NO CHANGE** | Existing types sufficient |
| `internal/scan/helpers.go` | **NO CHANGE** | Existing helpers sufficient |
| `internal/engine/engine.go` | **NO CHANGE** | Existing ScanAll/Cleanup flow works |
| `internal/engine/scanner.go` | **NO CHANGE** | Scanner interface unchanged |

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `pkg/homedir/` | Walks ~/ finding large (>100MB) and old (>1yr) files, excludes known app dirs | `internal/scan`, `internal/safety` |
| `pkg/projectcache/` | Finds `node_modules/`, `target/`, `.build/`, `.gradle/build/` etc. under ~/ | `internal/scan`, `internal/safety` |
| `pkg/developer/` (extended) | Adds git repo detection: `.git/` dirs, measures repo size, estimates reclaimable space | `internal/scan`, `internal/safety`, `os/exec` for `git` CLI |
| `internal/scan/walk.go` | Shared depth-limited directory walk utility | Used by `homedir` and `projectcache` |
| `internal/cleanup/cleanup.go` | Extended to handle `git-gc:` pseudo-paths via git command execution | `os/exec` |

## Feature 1: Home Directory File Scanning

### Architecture Decision: Separate Package (`pkg/homedir/`)

Create a new scanner package rather than extending `appleftovers` because:
- Different scanning strategy: recursive file-level walk vs. directory-level inspection
- Different risk profile: individual user files vs. app artifacts
- Different category grouping in the CLI (own flag group)

### Category Design

| Category ID | Description | Risk | Rationale |
|-------------|-------------|------|-----------|
| `home-large-files` | Files >100MB in ~/ | `risky` | Could be important media, projects |
| `home-old-files` | Files not modified in 1+ year, >10MB | `moderate` | Age suggests low value, but not certain |

### Scan Strategy

```
Walk ~/ with max depth limit (3-4 levels)
  Skip: ~/Library, ~/Applications, ~/.Trash, hidden directories
  Skip: directories already covered by other scanners
  For each regular file:
    - Large file check: size > 100MB threshold
    - Old file check: mtime > 365 days AND size > 10MB (avoid noise)
  Group into two CategoryResult sets
```

### Key Design Decisions

1. **Depth-limited walk, not full recursive**: Scanning all of ~/ recursively is slow and noisy. Limit to top 3-4 levels to catch the common case (large files in Documents, Desktop, Downloads).
2. **Exclude known directories**: Skip `~/Library/`, `~/Applications/`, hidden directories, and directories already scanned by other scanners. Use a hardcoded exclude list.
3. **Use `os.Lstat` not `os.Stat`**: Do not follow symlinks -- could lead outside ~/ and violate safety constraints.
4. **Use mtime, not atime**: macOS APFS does not update `atime` by default (mounted with `noatime` since High Sierra). Use `mtime` (modification time) as the primary age signal. Document this limitation.
5. **Minimum size for old files**: Require >10MB for "old files" to avoid noise from config files, dotfiles, etc. that are legitimately untouched.

### Scanner Signature

```go
// pkg/homedir/scanner.go
package homedir

func Scan() ([]scan.CategoryResult, error)
```

Follows the exact same pattern as every other scanner. Returns 0-2 CategoryResult entries (large files, old files). Each ScanEntry is a single file path with its size.

### Performance Concern

Walking ~/ is the most expensive scan in the tool. Mitigations:
- Depth limit (3-4 levels)
- Directory exclusion list (skip Library, .git, node_modules, etc.)
- Skip hidden directories by default (dotfiles rarely contain large individual files worth surfacing)
- File count cap: stop adding entries after first 100 per category to avoid overwhelming output

## Feature 2: Extended Project Caches

### Architecture Decision: New Package (`pkg/projectcache/`)

Separate from `pkg/developer/` because:
- `developer/` scans well-known tool cache directories at fixed paths (e.g., `~/.npm`, `~/.gradle/caches`)
- Project caches require *discovering* project roots under ~/ at variable paths
- Different scan strategy: walk ~/ looking for marker files, then check for sibling build directories

### Category Design

| Category ID | Description | Risk | Rationale |
|-------------|-------------|------|-----------|
| `project-node-modules` | `node_modules/` directories in projects | `safe` | Fully reproducible from package.json + lock file |
| `project-build-dirs` | `.build/`, `build/`, `dist/`, `out/` in projects | `moderate` | Usually reproducible, but custom artifacts possible |
| `project-cargo-target` | `target/` in Rust projects | `safe` | Fully reproducible from Cargo.toml |
| `project-venv` | `.venv/`, `venv/`, `.tox/` in Python projects | `moderate` | Reproducible but may contain local config |

### Scan Strategy

```
Walk ~/ looking for project root markers (depth 1-4):
  package.json        -> check for node_modules/ beside it
  Cargo.toml          -> check for target/ beside it
  Package.swift       -> check for .build/ beside it
  setup.py/pyproject.toml -> check for .venv/, venv/, .tox/ beside it

For each discovered project with matching build dir:
  Size it with scan.DirSize()
  Create ScanEntry with path = build dir, description = "project-name (type)"
```

### Key Design Decisions

1. **Walk for markers, not for cache dirs directly**: Looking for `node_modules/` directly hits every nested `node_modules/` inside other `node_modules/`. Instead, walk looking for `package.json` at depth 1-4, then check if `node_modules/` exists beside it.
2. **Do not recurse into build directories**: When a marker is found, check the sibling build dir but do not recurse further into that project. This avoids counting nested monorepo node_modules separately.
3. **Each build-type produces one CategoryResult**: All `node_modules/` discoveries go into one `project-node-modules` CategoryResult with multiple entries. This mirrors how existing scanners work (e.g., `dev-xcode` has multiple DerivedData entries).
4. **Exclude paths already covered by `developer` scanner**: Never report `~/.npm`, `~/.gradle/caches`, `~/Library/Caches/Homebrew`. Only report directories *inside project directories*.

### Important: Avoid Double-Counting

The existing `developer` scanner covers global caches (`~/.npm`, `~/.gradle/caches`, etc.). The project cache scanner covers *project-local* build artifacts (`node_modules/`, `target/`, `.build/`). These are complementary, not overlapping. Category descriptions make this explicit.

## Feature 3: Git Repository Cache Management

### Architecture Decision: Extend `pkg/developer/`

Git caches are developer tooling. Adding to the existing developer scanner keeps related concerns together. The developer scanner already handles tools with external CLI dependencies (Docker via `docker system df`).

### Category Design

| Category ID | Description | Risk | Rationale |
|-------------|-------------|------|-----------|
| `dev-git-repos` | Large git repositories (>.5GB .git dir) | `moderate` | Informational only -- reports size, does not delete |
| `dev-git-cleanup` | Reclaimable git space via gc/prune | `safe` | git gc is always safe, reclaims loose objects and old packs |

### Two-Phase Design

**Phase A: Discovery (scan)**
Walk ~/ looking for `.git/` directories (depth-limited, reuse walk helper). For each:
- Measure `.git/` directory size with `scan.DirSize()`
- Only report repos where `.git/` > 500MB
- Run `git -C <repo> count-objects -vH` to estimate reclaimable space (loose objects + garbage)
- Create two kinds of entries: informational (dev-git-repos) and actionable (dev-git-cleanup)

**Phase B: Cleanup**
Git cleanup cannot use `os.RemoveAll` -- it requires running `git gc`. This introduces a new cleanup pattern using pseudo-paths.

### Cleanup Architecture Extension

The existing cleanup package uses `os.RemoveAll` for all filesystem paths and skips pseudo-paths (like `docker:BuildCache`). Git cleanup needs a similar pseudo-path approach:

```go
// ScanEntry for git cleanup:
ScanEntry{
    Path:        "git-gc:/Users/user/projects/my-repo",
    Description: "my-repo (git gc)",
    Size:        estimatedReclaimable,
}
```

The cleanup package needs a new handler for `git-gc:` prefixed paths:

```go
// In internal/cleanup/cleanup.go, extend Execute():
if strings.HasPrefix(entry.Path, "git-gc:") {
    repoPath := strings.TrimPrefix(entry.Path, "git-gc:")
    err := runGitGC(ctx, repoPath)
    // handle result
    continue
}
```

This follows the existing Docker pseudo-path pattern (`docker:BuildCache`) but adds command execution instead of skipping. Use `exec.CommandContext` with timeout (60s per repo) for safety.

### Key Design Decisions

1. **Pseudo-path pattern for git gc**: Consistent with how Docker entries already work. The cleanup package already handles pseudo-paths -- this adds a new actionable prefix.
2. **Separate "discovery" from "cleanup" categories**: `dev-git-repos` is informational (shows large repos, helps users understand disk usage). `dev-git-cleanup` is actionable (runs git gc). Users may want to see large repos without cleaning them.
3. **Size threshold for reporting**: Only report repos with `.git/` > 500MB to avoid noise. Most repos are small and git gc would save negligible space.
4. **Timeout on git gc**: Git gc on a large monorepo can take minutes. Use `exec.CommandContext` with a 60-second timeout per repo.
5. **Never offer to delete repos**: The `dev-git-repos` category uses real paths but is marked informational. The `dev-git-cleanup` category uses `git-gc:` pseudo-paths. Neither triggers `os.RemoveAll` on the repo itself.
6. **CmdRunner injection**: Reuse the existing `CmdRunner` pattern from developer scanner (used for Docker) to make git commands testable.

### Git CLI Dependency

Same pattern as Docker: check if `git` is available via `exec.LookPath("git")`, skip silently if not installed. Use `exec.CommandContext` with separate args (never shell invocation per security rules in CLAUDE.md).

## Data Flow

### Existing Flow (unchanged)

```
CLI/Server
    |
    v
Engine.ScanAll()
    |
    v (sequential, per registered scanner)
Scanner.Scan() -> []CategoryResult
    |
    v
FilterSkipped() -> storeResults() -> ScanToken
    |
    v
Engine.Cleanup(token) -> cleanup.Execute()
    |
    v
os.RemoveAll(path) per entry  |  skip pseudo-paths (docker:*)
```

### Extended Flow (additions marked with +)

```
CLI/Server
    |
    v
Engine.ScanAll()
    |
    v (sequential, per registered scanner)
Scanner.Scan() -> []CategoryResult
    + homedir.Scan()       -> home-large-files, home-old-files
    + projectcache.Scan()  -> project-node-modules, project-build-dirs, ...
    + developer.Scan()     -> ... + dev-git-repos, dev-git-cleanup
    |
    v
FilterSkipped() -> storeResults() -> ScanToken
    |
    v
Engine.Cleanup(token) -> cleanup.Execute()
    |
    v
+ if git-gc: prefix   -> runGitGC(repoPath)
  if docker: prefix    -> skip (existing)
  else                 -> os.RemoveAll(path)
```

## Patterns to Follow

### Pattern 1: Scanner Adapter (existing, reuse exactly)

Every scanner exports a `Scan() ([]scan.CategoryResult, error)` function. The engine wraps it via `NewScanner(info, fn)`. New scanners follow this exactly.

```go
// pkg/homedir/scanner.go
func Scan() ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("cannot determine home directory: %w", err)
    }
    var results []scan.CategoryResult
    if cr := scanLargeFiles(home); cr != nil {
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, *cr)
    }
    if cr := scanOldFiles(home); cr != nil {
        cr.SetRiskLevels(safety.RiskForCategory)
        results = append(results, *cr)
    }
    return results, nil
}
```

### Pattern 2: Pseudo-Path Cleanup (existing, extend)

Docker already uses `docker:Type` pseudo-paths that are skipped by cleanup. Git uses `git-gc:/path/to/repo` pseudo-paths that trigger `git gc` execution. The cleanup package dispatches based on prefix.

### Pattern 3: External CLI with Timeout (existing in developer scanner, reuse)

Developer scanner already uses `exec.CommandContext` with timeout for Docker. Git operations follow the same pattern with `CmdRunner` dependency injection for testability.

```go
func runGitGC(runner CmdRunner, repoPath string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    _, err := runner(ctx, "git", "-C", repoPath, "gc", "--prune=now")
    return err
}
```

### Pattern 4: Depth-Limited Walk (new shared utility)

Neither `scan.ScanTopLevel` (single level) nor `scan.DirSize` (full recursive) supports depth-limited walking with exclusion. A new helper is needed:

```go
// internal/scan/walk.go (new file)
// WalkDepthLimited walks a directory tree up to maxDepth levels deep,
// calling fn for each entry. Depth 0 means root entries only.
// The exclude set skips entire directory subtrees by base name.
func WalkDepthLimited(root string, maxDepth int, exclude map[string]bool,
    fn func(path string, d fs.DirEntry, depth int) error) error
```

Both `homedir` and `projectcache` scanners need depth-limited walks. This belongs in `internal/scan/` as a shared utility, consistent with `DirSize` and `ScanTopLevel` living there.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Full Recursive Walk of ~/

**What:** Walking every file under ~/ with no depth limit or exclusions.
**Why bad:** Can take 10+ minutes on a 500GB home directory. Walks through Library, .git object stores, nested node_modules -- all waste.
**Instead:** Depth-limited walk (3-4 levels) with directory exclusion list.

### Anti-Pattern 2: Overlapping with Existing Scanners

**What:** Project cache scanner also reports `~/.npm`, `~/.gradle/caches`, etc.
**Why bad:** Double-counting inflates reported savings. Users see duplicate entries and get confused.
**Instead:** Project caches = project-local build artifacts only. Global caches stay in developer scanner. Explicitly exclude known global cache paths.

### Anti-Pattern 3: Offering to Delete Git Repos

**What:** `dev-git-repos` category entries have real filesystem paths, allowing `os.RemoveAll` to delete the entire repo.
**Why bad:** Catastrophic data loss. Git repos contain irreplaceable local work (uncommitted changes, local branches, stashes).
**Instead:** `dev-git-repos` is informational only (pseudo-path like `git-info:/path` or not included in cleanup). Only `dev-git-cleanup` is actionable, using `git-gc:` pseudo-paths that run `git gc`, not deletion.

### Anti-Pattern 4: Relying on atime for "Old Files"

**What:** Using file access time to determine if a file is "old."
**Why bad:** APFS on macOS does not update atime by default (`noatime` mount since High Sierra). Files appear "never accessed" even if used daily.
**Instead:** Use mtime (modification time). Not perfect (files can be read without modification) but reliable on APFS.

### Anti-Pattern 5: Creating New Engine/Scanner Interfaces

**What:** Introducing new interfaces or registration patterns for the new scanners.
**Why bad:** The existing `Scanner` interface, `NewScanner` adapter, and `RegisterDefaults` flow work perfectly. Adding complexity creates inconsistency.
**Instead:** Follow the exact same registration pattern. New scanners are just more calls to `e.Register(NewScanner(...))` in `RegisterDefaults`.

## Integration Checklist (Registration Points)

Each new feature requires changes at exactly these points. This is the complete list -- no other files need modification:

### For Each New Scanner Package (`homedir`, `projectcache`)

1. **`pkg/<name>/scanner.go`** -- Implement `Scan() ([]scan.CategoryResult, error)`
2. **`internal/safety/risk.go`** -- Add category IDs to `categoryRisk` map
3. **`internal/engine/registry.go`** -- Add import and `e.Register(NewScanner(...))` call in `RegisterDefaults`
4. **`cmd/categories.go`** -- Add flag variables, `groupDef` entry to `scanGroups` with `categoryDef` items
5. **`cmd/scan.go`** -- Register cobra flags for new targeted scan flags
6. **`cmd/root.go`** -- Register group-level `--<name>` and `--skip-<name>` flags
7. **READMEs** -- Update all 6 README files (English + 5 translations)

### For Git Scanner Extension (within `developer`)

Same as above except:
- Step 1 is modification of `pkg/developer/scanner.go` (not a new package)
- Step 3 is modification of existing developer scanner info in `RegisterDefaults` (add new category IDs and subcategories)
- Additional step: **`internal/cleanup/cleanup.go`** -- Add `git-gc:` pseudo-path handler with `exec.CommandContext`

## Suggested Build Order

Build in this order because each phase builds on the previous and can be tested independently.

### Phase 1: Depth-Limited Walk Helper

**What:** Add `WalkDepthLimited()` to `internal/scan/walk.go`
**Why first:** Both Feature 1 (homedir) and Feature 2 (projectcache) depend on this. Small, well-defined, fully testable, no CLI changes needed.
**Tests:** Unit tests with `t.TempDir()` -- nested dirs at various depths, exclusion behavior, edge cases
**Touches:** `internal/scan/walk.go` (new file only)
**Estimated complexity:** Low

### Phase 2: Extended Project Caches (`pkg/projectcache/`)

**What:** New scanner that finds project build artifacts (node_modules, target/, .build/, venv/)
**Why before homedir:** Simpler scan logic (look for marker files, check sibling dirs). Establishes the depth-limited walk usage pattern. Results are highly actionable (always safe to delete reproducible build artifacts). Good validation that the walk helper works in practice.
**Tests:** Scanner unit tests with mock project directory trees in `t.TempDir()`
**Touches:** All 7 registration points listed above
**Estimated complexity:** Medium

### Phase 3: Home Directory File Scanning (`pkg/homedir/`)

**What:** New scanner for large/old files in ~/
**Why after project caches:** Reuses depth-limited walk (proven in Phase 2). Needs careful exclusion list that references project cache scanner's targets to avoid noise. More subjective thresholds (size, age) require more testing.
**Tests:** Scanner unit tests, threshold edge cases, exclusion list coverage, mtime-based age tests
**Touches:** All 7 registration points listed above
**Estimated complexity:** Medium-High (exclusion logic, threshold tuning)

### Phase 4: Git Repository Cache Management

**What:** Extend developer scanner with git repo discovery + extend cleanup package with `git-gc:` handler
**Why last:** Most architecturally distinct -- it is the only feature that modifies the cleanup package. Benefits from all prior patterns being established. The pseudo-path cleanup extension has the widest blast radius of all changes.
**Tests:** Scanner tests (mock `.git/` dirs with known sizes), git gc handler tests (mock `CmdRunner`), integration tests verifying pseudo-path dispatch
**Touches:** All 7 registration points + cleanup.go extension
**Estimated complexity:** Medium-High (external CLI, cleanup extension, two related categories)

### Phase Ordering Rationale

- Phase 1 is a pure utility with no UI/CLI surface -- safe, isolated foundation
- Phase 2 is the most straightforward new scanner -- validates the full registration pattern end-to-end
- Phase 3 is more complex but uses the same skeleton as Phase 2, so learnings carry over
- Phase 4 is architecturally distinct (cleanup extension) and should be last to minimize blast radius; all other scanners are proven working before touching cleanup

### Dependency Graph

```
Phase 1: internal/scan/walk.go
    |
    +---> Phase 2: pkg/projectcache/ + registration
    |         |
    +---> Phase 3: pkg/homedir/ + registration
              |
              +---> Phase 4: pkg/developer/ extension + cleanup extension
```

Phases 2 and 3 are independent of each other (could theoretically be parallel), but sequential is safer for the registration-point changes which overlap in the same files.

## Scalability Considerations

| Concern | Current (10 scanners) | After v1.7 (12 scanners) | Notes |
|---------|----------------------|-------------------------|-------|
| Scan time | ~5-10s typical | ~15-30s (homedir walk is expensive) | Consider running homedir last so fast scanners report first |
| Category count | ~40 categories | ~48 categories | CLI flag list getting long; may need flag grouping in help text |
| Cleanup pseudo-paths | 1 prefix (docker:) | 2 prefixes (docker:, git-gc:) | Consider a dispatcher map instead of if/else chain if more are added |

## Sources

- Codebase analysis: `internal/engine/`, `internal/scan/`, `internal/safety/`, `internal/cleanup/`, `pkg/developer/`, `cmd/categories.go`, `cmd/scan.go` (HIGH confidence -- direct code reading)
- Architecture patterns derived from existing scanner implementations in `pkg/*/scanner.go` (HIGH confidence)
- APFS atime behavior: macOS APFS mounts with `noatime` by default since High Sierra (MEDIUM confidence -- well-known macOS behavior, not verified against current Apple docs)
- git count-objects and git gc behavior: standard git documentation (HIGH confidence -- stable git features)
