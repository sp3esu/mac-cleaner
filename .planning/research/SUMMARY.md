# Project Research Summary

**Project:** mac-cleaner v1.7.0 Extended Scanning
**Domain:** macOS disk cleaner — home directory scanning, project cache detection, git repository cache management
**Researched:** 2026-03-11
**Confidence:** HIGH

## Executive Summary

mac-cleaner v1.7.0 adds three scanning capabilities to an already well-structured Go/Cobra CLI: home directory large/old file detection, project build cache discovery across multiple languages, and git repository cache management. All four research streams converge on the same conclusion: the existing architecture is sound and accommodates all three features with no new external dependencies and no changes to the core engine or scanner interfaces. The implementation path is well-defined — three new packages, one shared internal utility, and one targeted cleanup module extension.

The recommended build sequence follows a clear dependency chain. A shared depth-limited walk utility (`internal/scan/walk.go`) comes first, then project cache scanning (`pkg/projectcache/`), then home directory scanning (`pkg/homedir/`), then git cache management as an extension of the existing developer scanner with a new cleanup dispatch path. This ordering minimizes risk: each phase is independently shippable, and the architecturally widest change (the cleanup module extension for git gc) is deferred until all other patterns are proven.

The primary risks in this milestone are data integrity risks, not technical risks. Home directory scanning can delete irreplaceable user files if the risk model from cache scanning is applied incorrectly. Git gc can corrupt a repository if run without checking for lock files, active worktrees, or submodule type. Project caches can disrupt active development if the scanner does not account for project activity. All three risks have clear, documented mitigations — but they must be addressed as Phase 1 design decisions, not retrofitted later.

## Key Findings

### Recommended Stack

The stack requires zero new external dependencies. All three features build on Go stdlib (`filepath.WalkDir`, `os/exec`, `os.FileInfo`) and existing internal packages (`internal/scan`, `internal/safety`, `internal/engine`, `internal/cleanup`). The codebase already has every pattern required: recursive directory walking, external command execution via `CmdRunner` injection, pseudo-path conventions for non-filesystem cleanup operations, and safety checks via `safety.IsPathBlocked`.

**Core technologies:**
- `filepath.WalkDir` (Go 1.25 stdlib): depth-limited home directory traversal — preferred over third-party walkers; `WalkDir` replaced `godirwalk` and similar libraries since Go 1.16
- `os/exec` with `CmdRunner` injection: git CLI operations for `count-objects -vH` and `gc` — mirrors the established Docker scanner pattern, keeps commands testable
- `internal/scan.WalkDepthLimited` (new shared utility): shared walk with configurable depth and exclusion list — required by both homedir and projectcache scanners; belongs in `internal/scan/walk.go`
- Pseudo-path convention (`git-gc:/path`): command-based cleanup dispatch — consistent with existing `docker:BuildCache` pattern; cleanup module dispatches on prefix before `os.RemoveAll`

### Expected Features

**Must have (table stakes):**
- Large file detection (>100MB) in home directory — every disk analyzer competitor provides this (DaisyDisk, CleanMyMac, GrandPerspective)
- Old file detection (>180 days, >10MB) in home directory — primary user expectation for this scanning category
- node_modules detection across all projects — npkill's entire market exists for this use case; users commonly accumulate 20-100 instances totaling 50-200GB
- Swift .build, Rust target/, Python venv/.venv, Java/Kotlin build/ detection — expected coverage for a macOS-native developer tool
- .git directory size reporting — users expect visibility before any action
- git gc as cleanup action — the standard mechanism to reclaim git repo space; `git count-objects -vH` shows reclaimable space during scan

**Should have (competitive differentiators):**
- Single-pass multi-language project detection — walk once, detect all project types via marker map simultaneously (kondo's approach)
- Stale project highlighting — surface last-modified date per project so users prioritize deletions rationally
- Dry-run git gc estimation via `git count-objects -v` during scan — shows potential savings without touching anything; builds trust
- .git size vs working tree size ratio — identifies repos most likely to benefit from gc

**Defer (v2+):**
- Configurable size/age thresholds via CLI flags — hardcoded constants follow existing patterns; add configurability when users request it
- Duplicate file detection — full-home hashing takes hours, better served by dedicated tools (fdupes, rdfind)
- `__pycache__` detection — too granular; noise-to-signal ratio is poor at individual directory level
- Aggregate summary views (e.g., "47 node_modules totaling 28.3 GB") — purely presentational, no architectural changes needed to add later

### Architecture Approach

The architecture maps cleanly onto the existing scanner pattern with no interface changes needed. Two new scanner packages (`pkg/homedir/`, `pkg/projectcache/`), one extension to `pkg/developer/` for git scanning, one new internal utility (`internal/scan/walk.go`), and one targeted extension to `internal/cleanup/cleanup.go` for git-gc pseudo-path dispatch. The `Scanner` interface, `NewScanner` adapter, and `RegisterDefaults` flow are reused without modification. Each new scanner package touches exactly seven integration points (scanner.go, risk.go, registry.go, categories.go, scan.go, root.go, READMEs) — consistent with all existing scanners.

**Major components:**
1. `internal/scan/walk.go` (new) — shared `WalkDepthLimited(root, maxDepth, exclude, fn)` utility used by both homedir and projectcache scanners; depth and exclusion-list parameterized; fully testable in isolation
2. `pkg/projectcache/` (new) — walks `~/` looking for project manifest markers (package.json, Cargo.toml, Package.swift, setup.py/pyproject.toml, build.gradle/pom.xml) then sizes sibling build directories; emits safe/moderate category results
3. `pkg/homedir/` (new) — depth-limited walk of `~/` detecting files exceeding size/age thresholds; advisory-only or per-file confirmation to prevent batch deletion of personal files
4. `pkg/developer/` (extended) — adds .git directory discovery (depth-limited walk), `git count-objects -vH` reclaimable space estimation, and `dev-git-repos`/`dev-git-cleanup` categories with `git-gc:` pseudo-path entries
5. `internal/cleanup/cleanup.go` (extended) — adds `git-gc:` prefix handler that runs `git gc` via `exec.CommandContext` with 60s timeout, dispatching alongside existing `docker:` skip and `os.RemoveAll` paths

### Critical Pitfalls

1. **Home directory scanner deletes user files via batch cleanup** — the existing scan-list-delete-category flow is appropriate for regenerable caches but catastrophic for personal files. Home directory entries must either be advisory-only (no cleanup path) or require per-file confirmation. Never allow batch deletion of home directory results. Minimum size threshold (100MB) and explicit exclusion of known application directories are required.

2. **Git gc corrupts repository during concurrent operations** — never use `--prune=now`; always use git's default 2-week grace period. Check for `.git/index.lock` before running gc. Skip entries where `.git` is a file rather than a directory (submodules, worktrees). Assign `risky` risk level to ensure user confirmation at cleanup time.

3. **Project cache scanner deletes actively-used build caches** — `node_modules/` in a project modified yesterday is needed. Surface project path and last-active date so users make informed decisions rather than accepting batch deletion. Consider filtering out projects modified within a recency threshold (30 days) by default.

4. **Home directory scan performance causes tool to appear hung** — limit walk depth to 3-4 levels, not full recursion. Skip `~/Library`, `~/Photos Library.photoslibrary`, and `.Trash` immediately on entry. Register the home directory scanner last in the engine so fast scanners report first.

5. **iCloud Desktop and Documents symlinks cause double-counting or cloud deletions** — use `os.Lstat` not `os.Stat` when traversing `~/`. If top-level entries are symlinks into `~/Library/Mobile Documents/`, flag them as iCloud-synced and never offer deletion without an explicit warning.

## Implications for Roadmap

Based on research, the four suggested phases follow an internal dependency chain. Each phase is independently shippable and testable before the next begins. The cleanup module extension (highest blast radius) is deferred until all other patterns are proven.

### Phase 1: Shared Walk Utility
**Rationale:** Both the project cache scanner and the home directory scanner depend on a depth-limited, exclusion-aware directory walk. This utility has no CLI surface area, is fully testable in isolation with `t.TempDir()`, and unblocks Phases 2 and 3. Building it first prevents divergent walk implementations that would be difficult to tune consistently.
**Delivers:** `internal/scan/walk.go` implementing `WalkDepthLimited(root string, maxDepth int, exclude map[string]bool, fn func(path string, d fs.DirEntry, depth int) error) error` with full unit test coverage including depth limit enforcement, exclusion behavior, symlink handling, and edge cases.
**Avoids:** Divergent walk implementations in two scanner packages causing inconsistent depth/exclusion behavior.

### Phase 2: Extended Project Cache Scanning
**Rationale:** Highest ROI feature — node_modules alone commonly represents 20-100GB reclaimable for developers, and all project cache results are fully reproducible (safe to delete). This phase also establishes the end-to-end seven-point registration pattern for the first time in v1.7, validating the approach before more complex features follow. Uses the walk utility from Phase 1.
**Delivers:** `pkg/projectcache/` scanner with node_modules, Swift .build, Rust target/, Python venv/.venv, Java/Kotlin build/ detection. New CLI flags: `--project-caches`, `--skip-project-caches`, `--node-modules`, `--swift-build`, `--rust-target`, `--python-venv`, `--java-build`. New risk mappings and all six README updates.
**Addresses:** node_modules, .build, target/, venv, build/ table stakes features from FEATURES.md.
**Avoids:** Pitfall 7 (false positive pattern matching) by requiring project manifest marker files; Pitfall 3 (active project disruption) by surfacing project path and last-modified date.

### Phase 3: Home Directory File Scanning
**Rationale:** Reuses the depth-limited walk proven in Phase 2. Deferred past project caches because the risk model is fundamentally inverted — personal files are not regenerable — and the cleanup path design (advisory-only or per-file confirmation) requires a decision that does not exist in the current codebase. Building after Phase 2 means that decision can be made with a working registration example to reference and a proven walk utility.
**Delivers:** `pkg/homedir/` scanner with large file (>100MB) and old file (>180 days, >10MB) detection. New CLI flags: `--home-files`, `--skip-home-files`, `--large-files`, `--old-files`. Advisory-only presentation or per-file confirmation flow (design decision must be resolved before coding begins). All six README updates.
**Addresses:** Large file detection and old file detection table stakes from FEATURES.md.
**Avoids:** Pitfall 1 (batch deletion of user files), Pitfall 4 (performance), Pitfall 5 (iCloud symlinks).

### Phase 4: Git Repository Cache Management
**Rationale:** Architecturally the most distinct feature — the only v1.7 change that modifies the cleanup module. Extends the developer scanner (established in current codebase) with git-specific discovery logic, and adds a new dispatch branch in `cleanup.Execute` for `git-gc:` pseudo-paths. Deferred last so all other features are proven working before touching cleanup, which has the widest blast radius of any change in this milestone.
**Delivers:** `pkg/developer/` extension with .git directory discovery, `git count-objects -vH` reclaimable space estimation, and `dev-git-repos`/`dev-git-cleanup` categories. `internal/cleanup/cleanup.go` extended with `git-gc:` handler using `exec.CommandContext` with 60s timeout per repo. New CLI flags: `--git-repos`, `--skip-git-repos`, `--git-cache`, `--git-lfs`. All six README updates.
**Addresses:** .git size detection, git gc cleanup, git lfs prune table stakes from FEATURES.md.
**Avoids:** Pitfall 2 (gc corruption from concurrent operations via lock file check), Pitfall 6 (submodule/worktree misidentification via `.git` file vs directory check), Pitfall 8 (exec vs delete abstraction mismatch via pseudo-path pattern).

### Phase Ordering Rationale

- Phase 1 is a pure utility with no CLI surface — safe to build in complete isolation, proves the core shared mechanism before any consumer depends on it.
- Phase 2 before Phase 3 because project caches have a simpler and safer risk model (reproducible artifacts) and establish the seven-point registration pattern the team will repeat in Phase 3.
- Phase 3 after Phase 2 because it shares the walk utility, reuses the registration pattern, but requires an additional design decision (advisory-only cleanup) that does not exist yet in the codebase.
- Phase 4 last because it is the only change that modifies the cleanup module. The pseudo-path dispatch pattern is already proven (Docker), but adding actual command execution rather than skipping is the highest-risk change in the milestone.
- Phases 2 and 3 are technically independent but should be sequential because their registration-point changes overlap in the same files (categories.go, scan.go, root.go), making parallel development error-prone.

### Research Flags

Phases with standard patterns (skip research-phase):
- **Phase 1 (walk utility):** Pure Go stdlib, well-documented. Walk pattern is straightforward; depth and exclusion logic are simple to unit test.
- **Phase 2 (project caches):** Established patterns from kondo and npkill validate the marker-file approach. Scanner registration is fully documented in existing codebase.

Phases needing design decisions before implementation begins:
- **Phase 3 (home directory):** The advisory-only vs. per-file confirmation design for cleanup must be resolved before writing code. This requires deciding whether to add a new risk level above `risky`, add an `Actionable bool` field to `ScanEntry`, or maintain a separate advisory scanner list in the engine. This is a design gap, not a research gap — the right answer is not obvious from existing patterns.
- **Phase 4 (git management):** The git-gc cleanup dispatch interface design must be resolved before implementation. The pseudo-path string prefix approach (`git-gc:/path`) is recommended by research and consistent with Docker, but the alternative (`CleanupCmd []string` field on `ScanEntry`) is more explicit. Choosing the wrong approach here affects both scanner output and the cleanup module.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | No new dependencies; all patterns exist in codebase. Direct code analysis confirmed stdlib and internal package sufficiency. |
| Features | HIGH | Multiple validated tools in this space (npkill, kondo, DaisyDisk). Feature expectations are well-documented and cross-validated. |
| Architecture | HIGH | Based on direct codebase analysis of existing scanner, engine, cleanup, and safety packages. Component boundaries are explicit. |
| Pitfalls | HIGH | Critical pitfalls sourced from official git documentation, Apple platform docs, and Go stdlib issue tracker. One MEDIUM item noted below. |

**Overall confidence:** HIGH

### Gaps to Address

- **APFS atime behavior:** Research confirms macOS mounts APFS with `noatime` since High Sierra, meaning modification time (`mtime`) is the correct proxy for "old files." This is well-known community consensus but was not verified against current Apple developer documentation. Validate before finalizing the old-files threshold logic.
- **git lfs prune scope:** LFS prune behavior (which local cached objects are safe to prune vs. required for uncommitted operations) is documented but context-specific. Recommend either testing against a real LFS repository before finalizing this feature or deferring git lfs prune to v1.8.
- **Advisory-only cleanup design:** The existing codebase has no precedent for informational-only scan categories that never enter the cleanup path. The mechanism (new risk level, `Actionable bool` on `ScanEntry`, or separate advisory scanner list) must be decided before Phase 3 implementation begins.
- **Home directory depth vs. coverage tradeoff:** Research recommends 3-4 levels max depth. The correct default requires empirical testing against real home directories to confirm that common large files (DMGs in Documents, ZIPs on Desktop) are captured without excessive scan time. Threshold tuning should be done with real data before the feature ships.

## Sources

### Primary (HIGH confidence)
- Existing codebase (`internal/engine/`, `internal/scan/`, `internal/safety/`, `internal/cleanup/`, `pkg/developer/`, `cmd/categories.go`) — direct code analysis confirming all integration patterns
- Go stdlib documentation (`filepath.WalkDir`, `fs.DirEntry`, `os/exec`) — stable stdlib since Go 1.16
- Git gc documentation (https://git-scm.com/docs/git-gc) — concurrent operation risks, prune grace period, count-objects output
- Git worktree documentation (https://git-scm.com/docs/git-worktree) — shared object database behavior
- Go filepath.Walk issue tracker (#16399, #36197) — WalkDir performance and memory considerations

### Secondary (MEDIUM confidence)
- npkill (https://npkill.js.org/) — validates node_modules cleanup use case and scale of savings
- kondo (https://github.com/tbillington/kondo) — validates multi-language marker-based project cache detection, 20+ project types
- DaisyDisk (https://daisydiskapp.com/) — reference for large file discovery UX expectations
- Atlassian Git GC tutorial (https://www.atlassian.com/git/tutorials/git-gc) — gc/prune relationship and safety practices
- GitHub Engineering: Scaling Git's GC (https://github.blog/engineering/architecture-optimization/scaling-gits-garbage-collection/) — gc operational considerations
- DevToolbox: Git index.lock Fix Guide (https://devtoolbox.dedyn.io/blog/git-index-lock-file-exists-fix-guide) — lock file detection patterns
- Apple Community: iCloud Desktop and Documents (https://discussions.apple.com/thread/250723356) — symlink behavior under iCloud sync
- macOS storage cleanup for developers (https://flyingharley.dev/posts/storage-optimization-on-mac-os-for-developer/) — developer-specific cache path inventory

### Tertiary (LOW confidence, needs validation)
- go-git/go-git dependency assessment — based on known dependency tree size, not verified against current release; conclusion (avoid) is robust even if specific count is slightly off
- godirwalk obsolescence — well-established community consensus that WalkDir replaced it; LOW only because not cross-checked against current godirwalk changelog

---
*Research completed: 2026-03-11*
*Ready for roadmap: yes*
