# Domain Pitfalls: v1.7.0 Extended Scanning

**Domain:** Home directory file scanning, extended project caches, git repository cache management
**Researched:** 2026-03-11
**Confidence:** HIGH (codebase analysis + official docs + multiple web sources)

## Critical Pitfalls

Mistakes that cause data loss, repository corruption, or require rewrites.

### Pitfall 1: Home Directory Scan Deletes User Files Via Overly Broad Matching

**What goes wrong:** A "large/old files" scanner in `~/` flags files the user actually wants -- project archives, media libraries, VM images, encrypted vaults. User confirms deletion of a category and loses irreplaceable data.

**Why it happens:** The existing scanner pattern (`ScanTopLevel` + `DirSize`) works well for known cache directories where everything is deletable. Home directory scanning is fundamentally different: every file is potentially important. Reusing the same "scan, list, delete" pattern without understanding that the risk model is inverted leads to data loss.

**Consequences:**
- Permanent loss of user documents, media, projects
- Loss of trust -- one bad deletion means the tool is never used again
- Unlike cache deletion (recoverable via rebuild), home directory file deletion is irrecoverable without Time Machine

**Prevention:**
- Home directory scanner must be **read-only/advisory only** -- report large files but NEVER offer direct deletion through the existing cleanup flow
- If deletion is supported, it must be per-file confirmation (not per-category batch)
- Add a new risk level or flag: `"informational"` for items that should never enter the `cleanup.Execute` path
- Minimum size threshold (e.g., 100MB) to avoid noise
- Exclude well-known directories that have their own scanners: `Library/`, `.npm/`, `.gradle/`, etc.
- Exclude dotfiles and dotdirectories by default (configuration, not junk)

**Detection:** If the home directory scanner produces `ScanEntry` items with paths that could flow into `cleanup.Execute` unchanged, the design is wrong.

### Pitfall 2: Git GC/Prune Corrupts Repository During Concurrent Operations

**What goes wrong:** Running `git gc` or `git prune` on a repository while the user (or IDE background processes) is actively using it causes object database corruption. Loose objects referenced by an in-progress operation get deleted, leading to a broken repository.

**Why it happens:** Git's garbage collection deletes unreferenced loose objects. If another process has created an object but hasn't yet written the reference (e.g., during a rebase, merge, or commit), gc can delete that object. The default 2-week grace period on `--prune` mitigates this, but using `--prune=now` (which is tempting for "maximum space savings") eliminates the safety window entirely.

**Consequences:**
- Corrupted git repository (missing objects)
- Lost commits, broken history
- Recovery requires `git fsck` + manual repair or clone from remote
- If the user has unpushed work, it may be permanently lost

**Prevention:**
- NEVER use `--prune=now` -- always use the default 2-week grace period
- Check for `.git/index.lock` before running gc -- if present, another operation is in progress
- Check for active worktrees (`git worktree list`) -- gc in the main repo affects all worktrees
- Use `git gc` (which wraps prune safely) rather than calling `git prune` directly
- Run with `--dry-run` equivalent first: report what would be reclaimed without doing it
- Consider only offering `git gc` as an action, not as part of the automated cleanup flow
- Set the category risk to `RiskRisky`

**Detection:** If the implementation calls `git prune --prune=now` or `git gc --prune=now`, it is unsafe. If it runs git commands without checking for lock files, it risks concurrent operation conflicts.

### Pitfall 3: Project Cache Scanner Deletes Actively-Used Build Caches

**What goes wrong:** Scanner finds `node_modules/`, `.build/`, `target/`, `.gradle/` directories and offers to delete them. User accepts. The project they were actively working on now requires a full dependency reinstall and rebuild, which may take 10-30 minutes and require network access.

**Why it happens:** Project caches are indistinguishable from "junk" by size and age alone. A 2GB `node_modules/` in a project last modified yesterday is actively needed. The same directory in a project untouched for 2 years is safe to clean. The existing scanner pattern doesn't consider project activity.

**Consequences:**
- Developer loses build state mid-work
- `npm install` / `gradle build` required before resuming work (slow, needs network)
- If offline or on metered connection, developer is blocked
- If lock files have drifted from what's in the cache, dependency resolution may produce different results

**Prevention:**
- Use project root modification time (or git status) to determine activity -- never offer to clean caches from recently-active projects
- Define "recently active" threshold (e.g., 30 days since last modification to any source file)
- Show project path and last-active date in scan results so users can make informed decisions
- Default to showing project caches as informational, require explicit opt-in for deletion
- Set risk level to `RiskRisky` for all project build caches

**Detection:** If the scanner reports `node_modules/` from a project modified today as a deletion candidate without warning, the design lacks activity awareness.

---

## Moderate Pitfalls

### Pitfall 4: Home Directory Scan Performance Causes Tool to Appear Hung

**What goes wrong:** Scanning `~/` recursively with `filepath.WalkDir` on a home directory with 500K+ files takes minutes. The tool appears frozen. Users force-quit or assume it is broken.

**Why it happens:** `DirSize` uses `filepath.WalkDir` which stats every file. Home directories can contain millions of files (Photos library alone can have hundreds of thousands). The existing scanners target known paths with bounded depth, but a home directory scan is unbounded.

**Prevention:**
- Scan only top-level items in `~/` (depth 1) for the overview
- Use `os.ReadDir` + `entry.Info()` for top-level sizing, NOT recursive `DirSize` for every subdirectory
- For subdirectories the user drills into, size lazily on demand
- Set a timeout per top-level entry (e.g., 5 seconds) and report "size unknown" if exceeded
- Skip known-huge directories during initial scan: `Library/`, `Photos Library.photoslibrary/`, `.Trash/`
- Report progress via the existing `ScanEvent` mechanism so the UI shows activity

### Pitfall 5: iCloud Desktop & Documents Symlinks Cause Double-Counting or Deletion of Cloud Files

**What goes wrong:** When "Desktop & Documents Folders" is enabled in iCloud, `~/Desktop` and `~/Documents` may be symlinks to `~/Library/Mobile Documents/com~apple~CloudDocs/`. Scanning both `~/` top-level and following symlinks counts the same files twice. Worse: deleting files from the local path deletes them from iCloud Drive for all devices.

**Why it happens:** macOS transparently redirects Desktop/Documents to iCloud storage. The tool follows the symlink and treats the target as a separate location, or deletes "local" files that are actually cloud-synced.

**Prevention:**
- Use `os.Lstat` (not `os.Stat`) when scanning `~/` top-level to detect symlinks
- If a top-level entry is a symlink, resolve it and check if it points into `~/Library/Mobile Documents/`
- Flag iCloud-synced directories with a warning: "This directory is synced to iCloud -- deleting files here removes them from all devices"
- Never auto-delete files in iCloud-synced directories
- The existing `safety.IsPathBlocked` resolves symlinks, which is correct, but the scanner itself must be aware of iCloud semantics

### Pitfall 6: Git Scanner Finds Bare Repos, Submodules, and Worktrees

**What goes wrong:** The git cache scanner assumes every `.git` directory is a standalone repository. It misidentifies:
- Bare repositories (servers, mirrors) where the object database IS the repository
- Git submodules (`.git` is a file pointing to parent's `.git/modules/`)
- Git worktrees (share the object database with the main repo)

Running `git gc` on a submodule's `.git` directory or a worktree fails or corrupts the parent.

**Why it happens:** Developers check for `.git` directory existence as a repository indicator without checking its type.

**Prevention:**
- Check if `.git` is a file (submodule/worktree) vs directory (real repo) before offering gc
- For submodules: `.git` is a file containing `gitdir: ../..` -- skip these
- For worktrees: `.git` is a file containing `gitdir: /path/to/main/.git/worktrees/name` -- skip these
- For bare repos: no `.git` directory, the repo root IS the git database -- skip these (check for `HEAD` file + `objects/` dir without a parent `.git/`)
- Only offer gc on actual standalone repository roots

### Pitfall 7: Scanning Finds node_modules in Non-JS Projects (False Positive Pattern Matching)

**What goes wrong:** Scanner looks for known directory names (`node_modules`, `.build`, `target`, `.gradle`, `.cache`) but these names can appear in non-standard contexts:
- `target/` in a Rust project vs `target/` as a user-created directory
- `.build/` in a Swift Package Manager project vs `.build/` in a custom build system
- `.cache/` in a project vs `~/.cache/` (XDG standard)

**Why it happens:** Pattern matching on directory names without verifying project type.

**Prevention:**
- Verify project type before classifying caches:
  - `node_modules/` requires `package.json` in parent
  - `target/` as Rust requires `Cargo.toml` in parent
  - `.build/` as Swift requires `Package.swift` in parent
  - `.gradle/` build cache requires `build.gradle` or `build.gradle.kts` in parent
- Only scan for project caches when a matching project manifest file exists
- Category descriptions should include project context: "node_modules in ~/projects/my-app" not just "node_modules"

### Pitfall 8: Cleanup of Git Repos Requires exec.Command -- Different from File Deletion

**What goes wrong:** The existing `cleanup.Execute` uses `os.RemoveAll` for all entries. Git cache management requires running `git gc`, `git clean`, etc. -- external commands, not file deletion. Trying to shoehorn git operations into the existing cleanup path either breaks the abstraction or requires a parallel code path that bypasses safety checks.

**Why it happens:** The current architecture assumes cleanup = delete files. Git repo maintenance is a fundamentally different operation (run a command inside the repo).

**Prevention:**
- Extend `ScanEntry` or create a new type to distinguish "delete this path" from "run this command in this directory"
- Consider pseudo-paths (like `docker:BuildCache`) as precedent -- the codebase already handles non-filesystem entries
- Git operations should use the existing `CmdRunner` pattern (dependency injection for testing) from the developer scanner
- Safety checks for git operations: verify `.git` integrity before gc, check for lock files, verify no in-progress operations
- The cleanup progress reporting must handle the fact that `git gc` is a single long operation, not per-file deletion

---

## Minor Pitfalls

### Pitfall 9: Hardlinks in Git Object Database Cause Incorrect Size Reporting

**What goes wrong:** Git's object database uses hardlinks in some configurations (especially after `git clone --local`). `DirSize` counts each hardlink as a separate file, inflating the reported size. User sees "5 GB reclaimable" but only 500 MB is actually freed.

**Prevention:**
- For git repos, use `git count-objects -vH` to get accurate size of loose objects and packs
- Don't use `DirSize` on `.git/objects/` -- it will double-count hardlinked objects

### Pitfall 10: Extended Attributes and Quarantine Flags

**What goes wrong:** macOS attaches extended attributes (xattrs) to downloaded files, including quarantine flags (`com.apple.quarantine`). These are invisible to `DirSize` but affect file behavior. Deleting and recreating files (e.g., after `npm install`) may lose quarantine information, changing Gatekeeper behavior.

**Prevention:**
- This is informational only -- no action needed for the scanning phase
- If the tool ever implements "move to trash" instead of "delete," xattrs must be preserved

### Pitfall 11: Locale-Dependent Directory Names

**What goes wrong:** Some macOS directories have localized display names (e.g., "Schreibtisch" for "Desktop" in German Finder). The actual filesystem paths are always English, but users may be confused if the tool shows `~/Desktop` when their Finder shows "Schreibtisch."

**Prevention:**
- Always use filesystem paths (English) internally
- Consider displaying Finder-style localized names in descriptions where possible
- This is a UX concern, not a safety concern -- filesystem paths are always consistent

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Home directory scanning | Pitfall 1 (deleting user files), Pitfall 4 (performance), Pitfall 5 (iCloud) | Advisory-only mode, depth-1 scan, iCloud detection |
| Project cache detection | Pitfall 3 (active projects), Pitfall 7 (false positives) | Activity-based filtering, project manifest verification |
| Git repo management | Pitfall 2 (gc corruption), Pitfall 6 (submodules/worktrees), Pitfall 8 (exec vs delete) | Lock file checks, repo type detection, new cleanup abstraction |
| Integration with existing engine | Pitfall 8 (cleanup abstraction mismatch) | Extend ScanEntry or use pseudo-path pattern for non-delete operations |
| Risk level assignment | All new categories need correct risk mapping | Home files = risky, project caches = risky, git gc = risky |

## Integration Pitfalls (Specific to This Codebase)

### The cleanup.Execute Assumption

`cleanup.Execute` calls `os.RemoveAll(entry.Path)` for every entry. The three new features challenge this:

1. **Home directory files:** Should NOT enter `cleanup.Execute` at all (advisory only), or need per-file confirmation
2. **Project caches:** Can use `os.RemoveAll` but must be filtered by activity first
3. **Git repos:** Need `exec.Command("git", "gc")` instead of `os.RemoveAll`

**Resolution:** Use the existing pseudo-path pattern (`docker:BuildCache` precedent) for git operations. The `isPseudoPath` check in cleanup already skips non-`/` paths. Git entries could use `git-gc:/path/to/repo` as the path, with a separate handler. For home directory files, consider a new `ScanEntry.Action` field or separate the advisory scanner from the actionable scanner list.

### The Risk Level Gap

Current risk levels are `safe`, `moderate`, `risky`. Home directory files need a level above `risky` -- something like `"informational"` or `"advisory"` that signals "show but don't offer deletion." Without this, the UI will present home directory files alongside caches with the same "delete?" prompt.

**Resolution:** Add a fourth risk level or an `Actionable bool` field on `ScanEntry`. The existing `RiskForCategory` function and `categoryRisk` map in `safety/risk.go` accommodate this easily -- the question is whether the CLI and server UIs handle it correctly.

### Scanner Registration Order

New scanners must be registered in `engine.DefaultScanners()`. The home directory scanner will be slow (even depth-1 on a large home dir). If registered first, it delays all subsequent scanner results.

**Resolution:** Register the home directory scanner last, or implement concurrent scanner execution (noted as a future option in PROJECT.md: "concurrent can be added without API change").

## Sources

- [Git GC Documentation](https://git-scm.com/docs/git-gc) -- concurrent operation risks, prune grace period
- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree) -- shared object database
- [Git Prune - Atlassian](https://www.atlassian.com/git/tutorials/git-prune) -- prune safety practices
- [GitHub Engineering: Scaling Git's GC](https://github.blog/engineering/architecture-optimization/scaling-gits-garbage-collection/) -- gc at scale
- [Go filepath.Walk Performance Issue #16399](https://github.com/golang/go/issues/16399) -- Walk vs WalkDir performance
- [Go filepath.Walk Memory Leak Issue #36197](https://github.com/golang/go/issues/36197) -- memory issues with large trees
- [Apple Community: iCloud Desktop & Documents Symlinks](https://discussions.apple.com/thread/250723356) -- symlink behavior
- [DevToolbox: Git index.lock Fix Guide](https://devtoolbox.dedyn.io/blog/git-index-lock-file-exists-fix-guide) -- lock file detection

---
*Pitfalls research for: mac-cleaner v1.7.0 Extended Scanning*
*Researched: 2026-03-11*
