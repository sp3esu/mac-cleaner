# Feature Landscape: v1.7.0 Extended Scanning

**Domain:** Home directory file scanning, project cache detection, git cache management
**Researched:** 2026-03-11
**Confidence:** HIGH (established patterns, well-documented tools in this space)

## Table Stakes

Features users expect when a disk cleaner claims to handle these categories. Missing = product feels incomplete.

### Home Directory File Scanning

| Feature | Why Expected | Complexity | Dependencies |
|---------|-------------|------------|--------------|
| Large file detection (configurable threshold, default 100MB+) | Every disk analyzer shows large files (DaisyDisk, CleanMyMac, GrandPerspective). Users expect to find forgotten ISOs, DMGs, ZIPs. | Medium | New scanner in `pkg/`, new category IDs, threshold flag |
| Old file detection (configurable age, default 180+ days) | Stale files are the primary reason home directories bloat. Users want to see what they forgot about. | Medium | `os.Stat` mod-time checks; mirrors existing `scanOldDownloads` pattern |
| Size-sorted results | Users want to address biggest wins first. All competitors sort by size descending. | Low | Already the standard pattern in existing scanners |
| Exclude known directories (~/Library, ~/.Trash, ~/Applications, dotfiles) | Scanning ~/Library duplicates existing scanners. Scanning .Trash is confusing. Users expect smart defaults. | Low | Hardcoded exclusion list in scanner |
| Exclude paths covered by other scanners | If a file is already reported by old-downloads or another scanner, it should not appear again. ~/Downloads should be excluded since old-downloads already covers it. | Low | Static exclusion list (~/Downloads, ~/Library, etc.) |
| File type context in description | "backup-2024.dmg (4.2 GB)" is more useful than just a path. | Low | Use `filepath.Ext` + filename in ScanEntry.Description |

### Extended Project Caches

| Feature | Why Expected | Complexity | Dependencies |
|---------|-------------|------------|--------------|
| node_modules detection (recursive under ~/) | node_modules is the single biggest space consumer for JS devs. npkill exists solely for this use case. Commonly 200MB-2GB per project, users accumulate 20-100+ of these. | High | Recursive directory walk with depth limits and exclusions |
| Swift .build directories | Swift Package Manager creates .build/ in every package root. Xcode/SwiftPM developers accumulate many. Identified by Package.swift marker file. | Medium | Shared walk infrastructure, Package.swift marker detection |
| Rust target/ directories | Rust target/ dirs commonly reach 1-10GB per project. Well-known pain point in Rust community. Identified by Cargo.toml marker. | Medium | Shared walk infrastructure, Cargo.toml marker detection |
| Python venv/.venv directories | Virtual environments are 200-500MB each, often forgotten in old projects. Identified by pyvenv.cfg marker file inside the venv dir. | Medium | Shared walk infrastructure, pyvenv.cfg marker detection |
| Java/Kotlin project build/ directories | Gradle/Maven build outputs in project dirs (distinct from ~/.gradle global cache already scanned by developer scanner). Identified by build.gradle/pom.xml marker. | Medium | Shared walk infrastructure, build.gradle/pom.xml marker detection |
| Project path in description | Users need project context: "~/code/old-project/node_modules (1.2 GB)". | Low | Already supported by ScanEntry.Path and Description fields |

### Git Repository Cache Management

| Feature | Why Expected | Complexity | Dependencies |
|---------|-------------|------------|--------------|
| Detect large .git directories | .git dirs can grow to multiple GB with history, LFS objects, stale refs. Users want visibility into this hidden space. | Medium | Walk for .git dirs, use DirSize |
| Report reclaimable space per repo | `git count-objects -v` reports size-pack and size-garbage. Users want to know potential savings before acting. | Medium | Shell out to git, parse output. Mirrors Docker scanner CmdRunner pattern. |
| git gc as cleanup action | The standard way to compact a git repo. Users expect this as the primary reclaim mechanism. | High | **Architectural change**: cleanup module uses os.RemoveAll, but git gc is a command. Needs command-based cleanup support. |
| git lfs prune (when LFS present) | LFS objects are often the biggest .git consumers. Pruning local cache reclaims significant space. Only relevant when .git/lfs/ exists. | Medium | LFS detection + command execution |

## Differentiators

Features that set this apart from basic tools. Not expected, but valued by power users.

| Feature | Value Proposition | Complexity | Dependencies |
|---------|-------------------|------------|--------------|
| Single-pass multi-language project detection | Walk once, detect node_modules AND Cargo.toml AND Package.swift simultaneously. Far faster than N separate walks. kondo does this. | Medium | Careful walk design with marker map |
| Stale project highlighting | Projects not modified in 6+ months with large build caches are prime cleanup targets. Show last-modified date per project. | Medium | Check most recent source file mod-time (exclude build dirs) |
| Aggregate project cache summary | "47 node_modules totaling 28.3 GB" as category description is more actionable than 47 individual lines. Per-type subtotals. | Low | Presentation-level enhancement in category description |
| Dry-run git gc estimation | Run `git count-objects -v` in scan phase to show estimated savings without touching anything. Builds user trust. | Medium | Read-only command execution during scan |
| .git size vs working tree ratio | A 500MB .git on a 10MB working tree signals a repo that would benefit from gc. Ratio helps users prioritize. | Low | Compute both sizes, include ratio in description |

## Anti-Features

Features to explicitly NOT build in this milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Automatic deletion of all node_modules | Too destructive. Active projects need their deps. One wrong delete breaks a developer's workflow. | List and let user select via interactive mode or targeted flags. |
| Scanning inside ~/Library | Already covered by existing system/browser/developer/creative/messaging/photos/systemdata scanners. Double-counting confuses users and inflates totals. | Explicitly exclude ~/Library from home directory scanner. |
| Scanning inside .photoslibrary bundles | Already covered by photos scanner. Modifying Photos library structure can corrupt it. | Exclude *.photoslibrary. |
| Scanning inside .git/objects directly | Users must NOT manually delete git objects. That corrupts repos. | Use git gc/prune commands. Report .git size as opaque blob. |
| Deep recursive scan without depth limits | Scanning every nested dir under ~/ without limits causes multi-minute scans and high I/O. node_modules inside node_modules is never useful. | Max depth 5-6 levels for project discovery. Skip nested build artifact dirs. |
| Scanning /Volumes or network mounts | External drives and NAS have different lifecycle. Accidental deletion of network files is catastrophic. | Restrict to home directory on boot volume only. |
| Git history rewriting (BFG, filter-branch) | Destructive, irreversible, affects remote collaborators. Beyond a cleanup tool's scope. | Only offer gc/prune (safe, local-only operations). |
| Duplicate file detection | Full-home hashing is extremely slow (hours). Not worth the scan time for a CLI tool. | Defer to dedicated tools (fdupes, rdfind). Out of scope for v1.7. |
| __pycache__ detection | Too small individually (usually < 1MB per dir). Noise-to-signal ratio is terrible. | Skip or only report when aggregated above a threshold. |
| Scanning node_modules inside node_modules | Nested deps are managed by the package manager. Only the top-level node_modules matters for deletion. | When a node_modules is found, do not recurse into it. |

## Feature Dependencies

```
Recursive Walk Infrastructure (shared utility)
  |
  +--> Project Cache Scanner
  |      +-- node_modules (marker: package.json or package-lock.json nearby)
  |      +-- .build (marker: Package.swift)
  |      +-- target/ (marker: Cargo.toml)
  |      +-- venv/.venv (marker: pyvenv.cfg inside)
  |      +-- build/ (marker: build.gradle or build.gradle.kts or pom.xml)
  |
  +--> Git Cache Scanner
  |      +-- .git size detection
  |      +-- git count-objects (CmdRunner pattern)
  |      +-- git gc cleanup --> REQUIRES command-based cleanup support
  |      +-- git lfs prune --> REQUIRES .git/lfs detection + command execution
  |
  +--> Home Dir File Scanner (does NOT need walk infra -- shallow scan of ~/ only)
         +-- Large file detection (stat files in ~/, skip dirs)
         +-- Old file detection (check mod-time, skip dirs handled by other scanners)

Command-Based Cleanup Extension
  +-- Needed by: git gc, git lfs prune
  +-- Pattern: add CleanupCmd field to ScanEntry, or extend cleanup.Execute
  +-- Existing precedent: Docker uses pseudo-paths, but Docker cleanup is skipped (not executed)
```

**Critical dependency chain:**
1. Recursive directory walk utility (shared by project cache + git scanners)
2. Command-based cleanup support in engine/cleanup (needed for git gc/prune -- currently only os.RemoveAll exists)
3. Individual scanner implementations
4. Category registration (risk.go, categories.go, engine scanners.go)

## MVP Recommendation

### Phase 1: Project Cache Scanner (highest ROI, builds shared infrastructure)

Build:
1. Shared recursive walk utility with depth limits, exclusion list, multi-marker detection
2. node_modules detection -- biggest space savings for most developers (npkill users report 50-200GB reclaimed)
3. Swift .build detection -- natural for macOS-focused tool
4. Rust target/ detection -- known huge space consumer (1-10GB each)
5. Python venv/.venv detection -- common and straightforward
6. Java/Kotlin build/ detection -- rounds out language coverage

**Rationale:** Project caches are the single largest source of reclaimable space for developers (commonly 20-100GB across projects). All entries are safe to delete (regenerated by build tools). The recursive walk infrastructure built here is reused by git scanning in Phase 2.

**Risk classification:** All project caches should be `safe` (regenerable) except Python venv which is `moderate` (may contain custom-installed packages not in requirements.txt).

### Phase 2: Git Repository Cache Management

Build:
1. .git directory size detection (reuses walk from Phase 1)
2. `git count-objects -v` for reclaimable space estimation (read-only, shown during scan)
3. Command-based cleanup extension to engine/cleanup module
4. git gc --aggressive as cleanup action
5. git lfs prune (only when .git/lfs exists)

**Rationale:** Requires walk infrastructure from Phase 1. Also requires architectural work for command-based cleanup. Lower ROI than project caches -- git gc typically saves less space than deleting node_modules. But provides unique value (no other CLI cleaner does this well).

**Risk classification:** `moderate` for git-cache (gc/prune are safe but irreversible -- loose objects are gone once packed/pruned).

### Phase 3: Home Directory File Scanning

Build:
1. Large file detection (configurable threshold, default 100MB)
2. Old file detection (configurable age, default 180 days)
3. Exclusion of ~/Library, ~/Applications, ~/.Trash, ~/Downloads, and all dotfile directories

**Rationale:** Deferred to last because (a) overlaps with existing old-downloads scanner, (b) results are highly personal and need careful user review, (c) risk classification is harder (every file is unique and potentially irreplaceable), (d) does not need the shared walk infrastructure.

**Risk classification:** `risky` for both large-files and old-files (user data, not regenerable).

### Defer Entirely

- Duplicate file detection: High complexity, slow, better served by dedicated tools
- Aggregate summary views: Purely presentational, can be added later without architectural changes
- __pycache__ cleanup: Too granular, noise exceeds signal

## Architectural Notes for Roadmap

### Recursive Walk Infrastructure
- **Single walk, multiple detectors:** Walk ~/[non-excluded dirs] once, checking each directory for marker files (package.json, Cargo.toml, Package.swift, build.gradle, pom.xml, .git). This is orders of magnitude faster than N separate walks.
- **Max depth:** 5-6 levels sufficient. Projects rarely nested deeper.
- **Exclusion list:** ~/Library, ~/Applications, ~/.Trash, ~/Pictures/*.photoslibrary, ~/Music/Music, .git (don't recurse into), node_modules (don't recurse into), target (don't recurse into), .build (don't recurse into), build (don't recurse into when it's a detected project build dir)
- **Walk strategy:** `os.ReadDir` (not `filepath.Walk`) for explicit depth control. Skip symlinks to avoid cycles.
- **Performance target:** Walk should complete in < 5 seconds for typical home directory.

### Command-Based Cleanup (for git gc/prune)
Current cleanup module (`internal/cleanup/cleanup.go`) only supports `os.RemoveAll`. Git gc/prune require running shell commands. Recommended approach:

Add an optional `CleanupCmd []string` field to `ScanEntry`. When present, `cleanup.Execute` runs the command (via `exec.CommandContext`) instead of `os.RemoveAll`. Example:
```go
ScanEntry{
    Path:        "/Users/me/code/project/.git",
    Description: "project (.git 500 MB, ~200 MB reclaimable)",
    Size:        200_000_000, // estimated reclaimable, not total .git size
    CleanupCmd:  []string{"git", "-C", "/Users/me/code/project", "gc", "--aggressive"},
}
```

This is minimally invasive, keeps the existing interface stable, and reuses the CmdRunner/exec.CommandContext patterns already established in the Docker and PlistBuddy scanners.

### New Category IDs and Risk Levels

| Category ID | Risk Level | Scanner Group | Description |
|-------------|-----------|---------------|-------------|
| project-node-modules | safe | project-caches | node_modules directories |
| project-swift-build | safe | project-caches | Swift .build directories |
| project-rust-target | safe | project-caches | Rust target directories |
| project-python-venv | moderate | project-caches | Python virtual environments |
| project-java-build | safe | project-caches | Java/Kotlin build directories |
| git-cache | moderate | git-repos | Git repository caches |
| git-lfs-cache | moderate | git-repos | Git LFS local caches |
| home-large-files | risky | home-files | Large files in home directory |
| home-old-files | risky | home-files | Old files in home directory |

### New CLI Flags Needed

Group-level:
- `--project-caches` / `--skip-project-caches`
- `--git-repos` / `--skip-git-repos`
- `--home-files` / `--skip-home-files`

Item-level (scan subcommand):
- `--node-modules`, `--swift-build`, `--rust-target`, `--python-venv`, `--java-build`
- `--git-cache`, `--git-lfs`
- `--large-files`, `--old-files`

Configuration:
- `--large-file-threshold` (default: 100MB)
- `--old-file-age` (default: 180 days)
- `--scan-depth` (default: 5)

## Sources

- [npkill -- node_modules cleaner](https://npkill.js.org/) -- established tool, validates the node_modules use case
- [kondo -- multi-language build artifact cleaner](https://github.com/tbillington/kondo) -- validates multi-language project cache detection approach, supports 20+ project types
- [Git gc documentation](https://git-scm.com/docs/git-gc) -- official gc behavior and options
- [Atlassian Git GC tutorial](https://www.atlassian.com/git/tutorials/git-gc) -- gc/prune relationship explained
- [Git LFS prune documentation](https://www.mankier.com/1/git-lfs-prune) -- LFS cache management
- [DaisyDisk](https://daisydiskapp.com/) -- reference for large file discovery UX
- [macOS storage cleanup guide 2025](https://osxhub.com/macos-storage-cleanup-guide-2025/) -- user expectations
- [Storage Optimization on MacOS for Developer](https://flyingharley.dev/posts/storage-optimization-on-mac-os-for-developer/) -- developer-specific cache paths
- [How to find large files on Mac](https://macpaw.com/how-to/find-large-files-mac) -- competitor feature reference

---
*Feature research for: mac-cleaner v1.7.0 Extended Scanning*
*Researched: 2026-03-11*
*Confidence: HIGH (well-established patterns, multiple validated tools in this space)*
