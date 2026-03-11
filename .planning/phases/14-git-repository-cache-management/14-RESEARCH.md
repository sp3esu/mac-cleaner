# Phase 14: Git Repository Cache Management - Research

**Researched:** 2026-03-11
**Domain:** Git repository discovery, `git count-objects`, `git gc`, `git lfs prune`, cleanup package extension
**Confidence:** HIGH

## Summary

This phase adds scanning for `.git` directories under the home directory, reporting their sizes and estimated reclaimable space via `git count-objects -v`, and offering `git gc --aggressive` (and optionally `git lfs prune`) as cleanup actions. Unlike all previous scanners where cleanup = `os.RemoveAll(path)`, git cleanup runs external commands. This requires extending `internal/cleanup/cleanup.go` to handle a new `git-gc:` pseudo-path prefix, following the pattern established by Docker's `docker:` prefix but adding actual command execution rather than skipping.

The scanner itself follows the established `pkg/` scanner pattern exactly, reusing the `internal/walk` package from Phase 12 to discover `.git` directories. The key architectural novelty is the cleanup extension: the existing `cleanup.Execute` dispatches on path format, and this phase adds a `git-gc:` prefix handler that runs `git gc --aggressive` (and `git lfs prune` when `.git/lfs` exists) via `exec.CommandContext` with timeout. The `CmdRunner` dependency injection pattern from `pkg/developer` (used for Docker) is reused for testability.

**Primary recommendation:** Create `pkg/gitrepos/scanner.go` using the walk utility to find `.git` directories, run `git count-objects -v` for reclaimable space estimation, and emit entries with `git-gc:` pseudo-path prefix. Extend `cleanup.Execute` to dispatch `git-gc:` prefixed paths to a git gc/lfs prune handler with 120-second timeout per repo.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| GIT-01 | Scan for large .git directories with size reporting | Walk utility discovers `.git` directories; verify `.git` is a directory (not file/submodule); size via `scan.DirSize()`; only report `.git` dirs >= configurable threshold |
| GIT-02 | Show estimated reclaimable space per repo via git count-objects | Run `git -C <repo> count-objects -v`; parse `size` (loose objects, in KiB) + `size-garbage` fields; sum as estimated reclaimable; display in Description |
| GIT-03 | Run git gc --aggressive as cleanup action | Entries use `git-gc:/path/to/repo` pseudo-path; cleanup package extended to handle prefix; runs `git -C <repo> gc --aggressive` via `exec.CommandContext` with timeout |
| GIT-04 | Run git lfs prune for repos with .git/lfs | During scan, check for `.git/lfs` directory; if present, cleanup runs both `git gc --aggressive` AND `git lfs prune` sequentially; only if `git-lfs` binary is available |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `filepath.WalkDir` (via `internal/walk`) | stdlib | Recursive directory traversal to find `.git` dirs | Reuse existing walk utility from Phase 12 |
| `os/exec` | stdlib | Run `git count-objects`, `git gc`, `git lfs prune` | Project convention; used by developer scanner for Docker |
| `os` | stdlib | File stat, check `.git` is directory vs file | Already used everywhere |
| `path/filepath` | stdlib | Path manipulation | Project convention |
| `context` | stdlib | Timeout for external commands | Used by developer scanner |
| `strings` / `strconv` | stdlib | Parse `git count-objects -v` output | Simple key-value parsing |

### Supporting (already in project)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/scan` | project | ScanEntry, CategoryResult, DirSize, FormatSize | All scan results use these types |
| `internal/safety` | project | RiskForCategory | Risk level mapping for new categories |
| `internal/engine` | project | Scanner interface, NewScanner adapter | Scanner registration |
| `internal/walk` | project | Walk with depth limits and exclusions | Find `.git` directories under home |
| `internal/cleanup` | project | Execute (extended) | Add `git-gc:` pseudo-path handler |
| `github.com/spf13/cobra` | existing dep | CLI flags | `--git-repos`, `--skip-git-repos` flags |

### No New Dependencies
This phase requires zero new external dependencies.

## Architecture Patterns

### Recommended Project Structure
```
pkg/
  gitrepos/
    scanner.go       # Scan() function, git count-objects parsing, entry creation
    scanner_test.go  # Tests with t.TempDir(), mock CmdRunner
internal/
  cleanup/
    cleanup.go       # Extended: add git-gc: pseudo-path handler
    cleanup_test.go  # Extended: add tests for git-gc: dispatch
```

### Pattern 1: New Scanner Package (Not Extension of Developer)

**What:** Create a new `pkg/gitrepos/` package rather than extending `pkg/developer/scanner.go`.
**Why:** The developer scanner is already large (13 scan functions, 620 lines). Adding git repo discovery with walk integration and `count-objects` parsing would make it unwieldy. A separate package is cleaner, follows the Phase 12/13 pattern of new `pkg/` packages, and gets its own scanner group with dedicated flags.
**Registration:** New scanner group `git-repos` in `scanGroups`, registered via `engine.RegisterDefaults`.

### Pattern 2: Walk-Based `.git` Directory Discovery

**What:** Use the existing `internal/walk` package to find `.git` directories under home.
**When to use:** During scanning phase.
**Key details:**
- Walk home directory with MaxDepth 5 (same as projects/homedir)
- Look for directories named `.git` via OnDir callback
- The walk utility skips dotfile directories at depth 1 -- but `.git` at depth > 1 (inside projects) is what we want
- When `.git` is found, the repo root is `filepath.Dir(gitDirPath)`
- Verify `.git` is a **directory** (not a file) to skip submodules and worktrees

```go
OnDir: func(path string, d fs.DirEntry, depth int) bool {
    if d.Name() == ".git" && d.IsDir() {
        // Found a real .git directory (not submodule/worktree)
        repoRoot := filepath.Dir(path)
        // Process repo...
        return false // don't descend into .git
    }
    return true
}
```

**Important:** The walk utility already skips dotfile dirs at depth 1 (direct children of home). This means `~/.git` (if it existed) would be skipped, which is correct. But `.git` inside `~/projects/myrepo/.git` at depth 2+ will be reached because the walk only excludes dotfiles at depth 1.

### Pattern 3: `git count-objects -v` Output Parsing

**What:** Parse the output of `git -C <repo> count-objects -v` to estimate reclaimable space.
**Output format** (verified on this machine):
```
count: 1251        # number of loose objects
size: 5492         # size of loose objects in KiB
in-pack: 0         # objects in pack files
packs: 0           # number of pack files
size-pack: 0       # size of pack files in KiB
prune-packable: 0  # loose objects also in packs (reclaimable)
garbage: 0         # garbage files
size-garbage: 0    # size of garbage in KiB
```

**Reclaimable space estimation:** `size` (loose object KiB) + `size-garbage` (garbage KiB). These are the bytes `git gc` will reclaim or compact. Note: `size` and `size-garbage` are in **KiB** (not bytes), so multiply by 1024.

**Parsing approach:**
```go
func parseCountObjects(output string) (reclaimableBytes int64) {
    for _, line := range strings.Split(output, "\n") {
        parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
        if len(parts) != 2 {
            continue
        }
        key := strings.TrimSpace(parts[0])
        val := strings.TrimSpace(parts[1])
        switch key {
        case "size", "size-garbage":
            n, _ := strconv.ParseInt(val, 10, 64)
            reclaimableBytes += n * 1024 // KiB -> bytes
        }
    }
    return
}
```

### Pattern 4: Pseudo-Path Cleanup Extension

**What:** Extend `cleanup.Execute` to handle `git-gc:` prefixed paths by running git commands.
**Current behavior:** Paths starting with `/` are cleaned with `os.RemoveAll`. Non-`/` paths (pseudo-paths like `docker:BuildCache`) are skipped with an error.
**New behavior:** Paths with `git-gc:` prefix extract the repo path and run `git gc --aggressive`, then `git lfs prune` if applicable.

```go
// In cleanup.Execute, before the isPseudoPath check:
if strings.HasPrefix(entry.Path, "git-gc:") {
    repoPath := strings.TrimPrefix(entry.Path, "git-gc:")
    err := runGitCleanup(repoPath, entry.Description)
    if err != nil {
        res.Failed++
        res.Errors = append(res.Errors, fmt.Errorf("git cleanup %s: %w", repoPath, err))
    } else {
        res.Removed++
        res.BytesFreed += entry.Size
    }
    continue
}
```

**CmdRunner injection:** The cleanup package currently does not run external commands. To maintain testability, pass a `CmdRunner` function or use a package-level variable (similar to developer scanner pattern). The simplest approach: add a `RunGitCleanup` field to a new options struct, or handle git cleanup in a dedicated function callable from the cleanup package.

### Pattern 5: CmdRunner Dependency Injection

**What:** Reuse the `CmdRunner` pattern from `pkg/developer` for both scanning (git count-objects) and cleanup (git gc, git lfs prune).
**Design:** The scanner needs `CmdRunner` for `git count-objects` during scan. The cleanup handler needs it for `git gc` and `git lfs prune`. Use the same `defaultRunner` pattern with `exec.CommandContext`.

```go
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, name, args...) // #nosec G204
    return cmd.CombinedOutput()
}
```

### Pattern 6: Two Categories -- Informational and Actionable

**What:** The scanner emits one category: `git-repos` with `git-gc:` pseudo-path entries.
**Rationale:** The requirements ask for scan + cleanup as a unified flow. Each entry shows `.git` size and reclaimable estimate in the description, and uses `git-gc:/path/to/repo` as the path for cleanup dispatch.

If the reclaimable estimate from `count-objects` is zero or negligible, still show the entry (users care about `.git` dir size itself), but mark the description to indicate nothing to reclaim.

### Anti-Patterns to Avoid

- **Never delete `.git` directories with `os.RemoveAll`:** This permanently destroys the repository. Cleanup must ONLY run `git gc` commands.
- **Never use `--prune=now`:** This removes the 2-week safety window and can corrupt repos with in-progress operations. Use `git gc --aggressive` which uses safe defaults.
- **Never run gc on submodules/worktrees:** Check that `.git` is a directory, not a file. If `.git` is a file, it is a submodule or worktree -- skip it.
- **Never run gc without checking for lock files:** If `.git/index.lock` exists, another git operation is in progress. Skip with a note.
- **Don't walk inside `.git` directories:** Stop descent immediately when `.git` is found. The `.git` directory can contain thousands of pack files and objects.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Directory walking | Custom recursive walk | `internal/walk.Walk()` | Already built and tested for Phase 12/13 |
| Directory sizing | Custom size calculator | `scan.DirSize()` | Handles permission errors, used everywhere |
| Size formatting | Custom formatter | `scan.FormatSize()` | Existing SI-unit formatter |
| Risk mapping | Inline risk strings | `safety.RiskForCategory()` | Central registry |
| Scanner registration | Custom engine hookup | `engine.NewScanner()` | Standard adapter pattern |
| CLI flag plumbing | Manual cobra setup | `scanGroups` registry | Auto-generates flags |
| External command execution | Raw `exec.Command` | `exec.CommandContext` with `CmdRunner` pattern | Timeout safety + testability |
| count-objects output parsing | Complex regex | Simple `strings.SplitN` key:value parser | Output is stable, line-based key-value |

## Common Pitfalls

### Pitfall 1: Running `git gc` on Submodules or Worktrees
**What goes wrong:** `.git` can be a file (not directory) in submodules and worktrees. Running `git gc` there fails or corrupts the parent repository.
**Why it happens:** Developers check for `.git` existence without verifying it is a directory.
**How to avoid:** Use `d.IsDir()` from the walk callback. Only process entries where `.git` is a real directory. If `.git` is a file, it contains `gitdir: ...` and should be skipped entirely.
**Warning signs:** Errors like "fatal: not a git repository" during gc.

### Pitfall 2: Concurrent Git Operations Cause Corruption
**What goes wrong:** Running `git gc` while another process (IDE, background fetch, user terminal) is using the repo can corrupt the object database.
**Why it happens:** Git gc deletes loose objects that may be referenced by in-progress operations.
**How to avoid:** Check for `.git/index.lock` before running gc. If the lock file exists, skip that repo with a message "skipped: operation in progress". Use `git gc --aggressive` (NOT `--prune=now`) which respects the 2-week grace period for loose objects.
**Warning signs:** `.git/index.lock` exists.

### Pitfall 3: git gc --aggressive Takes Very Long
**What goes wrong:** `git gc --aggressive` on a large monorepo (linux kernel, chromium) can take 10+ minutes. The tool appears hung.
**Why it happens:** `--aggressive` does deeper compression, which is CPU and I/O intensive.
**How to avoid:** Use `exec.CommandContext` with a 120-second timeout per repo. If it times out, report "cleanup timed out" rather than hanging forever. Consider whether `git gc` (without `--aggressive`) is sufficient -- the requirements specify `--aggressive` but this should be documented as a trade-off.
**Warning signs:** Cleanup taking more than 2 minutes per repo.

### Pitfall 4: git lfs prune Fails Silently
**What goes wrong:** `git lfs prune` requires `git-lfs` binary to be installed. If not installed, the command fails.
**Why it happens:** Not all users have git-lfs installed, even if their repos have `.git/lfs`.
**How to avoid:** Check `exec.LookPath("git-lfs")` before attempting lfs prune. If not available, skip lfs prune silently (still run git gc). Log which repos had lfs skipped.
**Warning signs:** LFS prune errors in cleanup output.

### Pitfall 5: Walk Utility Skips `.git` at Depth 1
**What goes wrong:** The walk utility skips ALL dotfile directories at depth 1 (direct children of home). This means `.git` at `~/.git` would be skipped -- which is correct behavior (no one has a meaningful repo at `~/`). However, if the walk's exclusion of `.git` is set via `Exclude: []string{".git"}`, then `.git` at ALL depths would be skipped, preventing discovery entirely.
**Why it happens:** Phase 12 used `Exclude: []string{".git"}` to avoid descending into .git dirs during project cache scanning.
**How to avoid:** Do NOT add `.git` to the Exclude list. Instead, handle `.git` in the OnDir callback: when found, process it and return `false` (don't descend). The walk's depth-1 dotfile skip handles `~/.git` automatically.
**Warning signs:** No .git directories found despite having many repos.

### Pitfall 6: BytesFreed Inaccuracy After git gc
**What goes wrong:** The `entry.Size` is set to estimated reclaimable (from count-objects) during scan, but actual bytes freed by `git gc --aggressive` may differ significantly (could be more or less).
**Why it happens:** `count-objects` reports loose objects and garbage, but `gc --aggressive` also repacks objects, which can increase or decrease total size unpredictably.
**How to avoid:** Acknowledge this is an estimate. Display it as "estimated reclaimable" in the description. Accept that `BytesFreed` in `CleanupResult` is approximate for git repos.
**Warning signs:** Users questioning why freed space doesn't match reported size.

## Code Examples

### Scanner: Git Repository Discovery
```go
// pkg/gitrepos/scanner.go
package gitrepos

func Scan() ([]scan.CategoryResult, error) {
    return scanWithRunner(defaultRunner)
}

func scanWithRunner(runner CmdRunner) ([]scan.CategoryResult, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("gitrepos: resolve home dir: %w", err)
    }

    if _, err := exec.LookPath("git"); err != nil {
        return nil, nil // git not installed, skip silently
    }

    return scanFrom(home, runner)
}

func scanFrom(root string, runner CmdRunner) ([]scan.CategoryResult, error) {
    var entries []scan.ScanEntry

    err := walk.Walk(walk.Options{
        Root:     root,
        MaxDepth: 5,
        // Do NOT exclude ".git" -- we want to find it
        Exclude: []string{"node_modules", "vendor"},
        ExcludeAbs: []string{
            filepath.Join(root, "Library"),
            filepath.Join(root, "Applications"),
        },
        OnDir: func(path string, d fs.DirEntry, depth int) bool {
            if d.Name() != ".git" {
                return true // continue descent
            }
            if !d.IsDir() {
                return false // submodule/worktree -- skip
            }

            // Check for lock file (concurrent operation)
            lockFile := filepath.Join(path, "index.lock")
            if _, err := os.Stat(lockFile); err == nil {
                return false // operation in progress, skip
            }

            repoRoot := filepath.Dir(path)
            gitDirSize, err := scan.DirSize(path)
            if err != nil || gitDirSize == 0 {
                return false
            }

            // Get reclaimable estimate
            reclaimable := countObjectsReclaimable(runner, repoRoot)

            // Check for LFS
            hasLFS := false
            lfsDir := filepath.Join(path, "lfs")
            if info, err := os.Stat(lfsDir); err == nil && info.IsDir() {
                hasLFS = true
            }

            // Build description
            desc := filepath.Base(repoRoot) + " (.git " + scan.FormatSize(gitDirSize) + ")"
            if reclaimable > 0 {
                desc += " ~" + scan.FormatSize(reclaimable) + " reclaimable"
            }
            if hasLFS {
                desc += " [LFS]"
            }

            entries = append(entries, scan.ScanEntry{
                Path:        "git-gc:" + repoRoot,
                Description: desc,
                Size:        reclaimable, // estimated reclaimable as cleanup size
            })

            return false // don't descend into .git
        },
    })
    if err != nil {
        return nil, fmt.Errorf("gitrepos: walk: %w", err)
    }

    if len(entries) == 0 {
        return nil, nil
    }

    // Sort by .git size descending
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Size > entries[j].Size
    })

    var totalSize int64
    for _, e := range entries {
        totalSize += e.Size
    }

    cr := scan.CategoryResult{
        Category:    "git-repos",
        Description: fmt.Sprintf("Git Repositories (%d repos, ~%s reclaimable)", len(entries), scan.FormatSize(totalSize)),
        Entries:     entries,
        TotalSize:   totalSize,
    }
    cr.SetRiskLevels(safety.RiskForCategory)

    return []scan.CategoryResult{cr}, nil
}
```

### Parsing `git count-objects -v`
```go
func countObjectsReclaimable(runner CmdRunner, repoPath string) int64 {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    out, err := runner(ctx, "git", "-C", repoPath, "count-objects", "-v")
    if err != nil {
        return 0
    }

    var reclaimable int64
    for _, line := range strings.Split(string(out), "\n") {
        parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
        if len(parts) != 2 {
            continue
        }
        key := strings.TrimSpace(parts[0])
        val := strings.TrimSpace(parts[1])
        // "size" = loose objects in KiB, "size-garbage" = garbage in KiB
        if key == "size" || key == "size-garbage" {
            n, _ := strconv.ParseInt(val, 10, 64)
            reclaimable += n * 1024 // KiB to bytes
        }
    }
    return reclaimable
}
```

### Cleanup Package Extension
```go
// In internal/cleanup/cleanup.go, within Execute() loop:
// Add before isPseudoPath check:

if repoPath, ok := strings.CutPrefix(entry.Path, "git-gc:"); ok {
    gcErr := runGitCleanup(repoPath)
    if gcErr != nil {
        res.Failed++
        res.Errors = append(res.Errors, fmt.Errorf("git cleanup %s: %w", repoPath, gcErr))
    } else {
        res.Removed++
        res.BytesFreed += entry.Size
    }
    continue
}
```

```go
// runGitCleanup runs git gc --aggressive and optionally git lfs prune.
func runGitCleanup(repoPath string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
    defer cancel()

    // Run git gc --aggressive
    cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "gc", "--aggressive")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("git gc: %w", err)
    }

    // Check for LFS and run prune if available
    lfsDir := filepath.Join(repoPath, ".git", "lfs")
    if _, err := os.Stat(lfsDir); err == nil {
        if _, err := exec.LookPath("git-lfs"); err == nil {
            lfsCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "lfs", "prune")
            _ = lfsCmd.Run() // best-effort, don't fail cleanup on lfs error
        }
    }

    return nil
}
```

### Engine Registration
```go
// In internal/engine/registry.go, add to RegisterDefaults:
e.Register(NewScanner(ScannerInfo{
    ID:          "git-repos",
    Name:        "Git Repositories",
    Description: "Git repository caches (gc and LFS cleanup)",
    CategoryIDs: []string{"git-repos"},
    SkipFlag:    "git-repos",
    SubCategories: []SubCategoryInfo{
        {ID: "git-repos", Label: "Git Repositories", SkipFlag: "git-repos", RiskLevel: safety.RiskForCategory("git-repos")},
    },
}, gitrepos.Scan))
```

### CLI Flag Registration
```go
// cmd/categories.go -- add to scanGroups:
{
    FlagName:    "git-repos",
    ScannerID:   "git-repos",
    GroupName:   "Git Repositories",
    Description: "git repository cache cleanup (gc, lfs prune)",
    ScanFlag:    &flagGitRepos,
    SkipFlag:    &flagSkipGitRepos,
    Items: []categoryDef{
        {FlagName: "git-repos", CategoryID: "git-repos", Description: "git repository caches", SkipFlag: &flagSkipGitRepos, ScanFlag: &flagScanGitRepos},
    },
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `git gc` (default) | `git gc --aggressive` | Always available | Deeper compression, slower but more effective at reclaiming space |
| `git count-objects` (basic) | `git count-objects -v` | Git 1.7+ | Verbose mode provides size breakdown (loose, packed, garbage) |
| Manual `.git/objects` deletion | `git gc` + `git prune` | Always | Safe garbage collection vs. corruption risk |
| `git lfs prune --dry-run` | `git lfs prune` | git-lfs 2.x | Removes local LFS objects that have been pushed to remote |

**Key facts about `git count-objects -v` output:**
- `size` field: KiB (NOT bytes), loose objects only
- `size-garbage` field: KiB, garbage files in object store
- `size-pack`: KiB, total pack file size (NOT reclaimable, just informational)
- `-H` flag (human-readable) available but harder to parse; use raw `-v` instead

## Open Questions

1. **Size threshold for reporting**
   - What we know: Reporting every repo's `.git` dir (even tiny ones) creates noise. Most `.git` dirs are < 50MB.
   - What's unclear: What minimum `.git` size should trigger reporting? Previous research suggested 500MB but that may be too high.
   - Recommendation: No minimum threshold for display -- show all `.git` dirs found. The user should see all repos. Filter in the summary description ("X repos, Y reclaimable"). Users can always `--skip-git-repos` to exclude from scans.

2. **`git gc --aggressive` vs `git gc`**
   - What we know: Requirements specify `--aggressive`. It is much slower (re-does delta compression) but produces better compression. Standard `git gc` is fast but less thorough.
   - What's unclear: Whether the performance cost is acceptable for users with many repos.
   - Recommendation: Use `--aggressive` as specified in requirements, but with the 120-second timeout as a safety valve. If timeout is hit, the repo is still functional -- gc just didn't complete its optimization pass.

3. **Cleanup package architecture for CmdRunner**
   - What we know: `cleanup.Execute` currently has no dependency injection for commands. Adding `CmdRunner` requires changing its signature or using a package-level variable.
   - What's unclear: Best approach to inject command execution into cleanup without changing the existing interface contract.
   - Recommendation: Use a package-level `var gitCleanupFn` that defaults to the real implementation. Tests override it. This is minimal-impact and consistent with how `defaultRunner` works in `pkg/developer`. Alternatively, add the `git-gc:` handler directly in `Execute()` with the `exec.CommandContext` call inlined, and test via integration tests with real git repos in `t.TempDir()`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | None (Go convention) |
| Quick run command | `go test ./pkg/gitrepos/... ./internal/cleanup/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GIT-01 | Discover .git directories with size | unit | `go test ./pkg/gitrepos/... -run TestScanFindsGitDirs` | Wave 0 |
| GIT-01 | Skip .git files (submodules/worktrees) | unit | `go test ./pkg/gitrepos/... -run TestSkipSubmodule` | Wave 0 |
| GIT-01 | Skip repos with index.lock | unit | `go test ./pkg/gitrepos/... -run TestSkipLockedRepo` | Wave 0 |
| GIT-02 | Parse count-objects -v output | unit | `go test ./pkg/gitrepos/... -run TestParseCountObjects` | Wave 0 |
| GIT-02 | Reclaimable shown in description | unit | `go test ./pkg/gitrepos/... -run TestDescriptionFormat` | Wave 0 |
| GIT-03 | git-gc: pseudo-path dispatch | unit | `go test ./internal/cleanup/... -run TestGitGCPseudoPath` | Wave 0 |
| GIT-03 | git gc --aggressive called | unit | `go test ./pkg/gitrepos/... -run TestGitGCCommand` | Wave 0 |
| GIT-04 | LFS detection via .git/lfs | unit | `go test ./pkg/gitrepos/... -run TestLFSDetection` | Wave 0 |
| GIT-04 | git lfs prune called when LFS present | unit | `go test ./pkg/gitrepos/... -run TestLFSPruneCommand` | Wave 0 |
| -- | git not installed -> nil result | unit | `go test ./pkg/gitrepos/... -run TestGitNotInstalled` | Wave 0 |
| -- | Risk level mapped correctly | unit | `go test ./internal/safety/... -run TestRiskForCategory` | Existing (needs `git-repos` entry) |

### Sampling Rate
- **Per task commit:** `go test ./pkg/gitrepos/... ./internal/cleanup/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green + `go vet ./...` + `gosec ./...`

### Wave 0 Gaps
- [ ] `pkg/gitrepos/scanner.go` -- scanner implementation
- [ ] `pkg/gitrepos/scanner_test.go` -- all discovery, parsing, and edge case tests
- [ ] `internal/cleanup/cleanup.go` -- extended with `git-gc:` handler
- [ ] `internal/cleanup/cleanup_test.go` -- extended with `git-gc:` dispatch tests
- [ ] `internal/safety/risk.go` -- add `git-repos` category risk entry

## Sources

### Primary (HIGH confidence)
- Project source code: `internal/walk/walk.go` -- walk utility API, depth-1 dotfile exclusion behavior
- Project source code: `internal/cleanup/cleanup.go` -- pseudo-path handling, `isPseudoPath`, `Execute` flow
- Project source code: `pkg/developer/scanner.go` -- `CmdRunner` pattern, Docker pseudo-path precedent, `exec.CommandContext` usage
- Project source code: `pkg/projects/scanner.go` -- walk-based scanner pattern (Phase 12 reference)
- Project source code: `internal/engine/registry.go` -- scanner registration pattern
- Project source code: `cmd/categories.go` -- flag registration pattern
- Local verification: `git count-objects -v` output format (verified on this machine)
- Local verification: `git count-objects -vH` output format (verified on this machine)
- Project research: `.planning/research/ARCHITECTURE.md` -- pseudo-path cleanup design
- Project research: `.planning/research/PITFALLS.md` -- git gc corruption, submodule/worktree detection

### Secondary (MEDIUM confidence)
- `git gc` man page -- `--aggressive` flag behavior, prune defaults
- `git count-objects` documentation -- `-v` output fields, KiB unit for size fields

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, no new deps, well-established project patterns
- Architecture: HIGH -- follows exact same scanner pattern as Phase 12/13; cleanup extension follows existing pseudo-path precedent (Docker)
- Pitfalls: HIGH -- informed by project-specific research (PITFALLS.md) and verified git CLI behavior
- Cleanup extension: HIGH -- pseudo-path pattern already proven with Docker; git-gc prefix is a natural extension

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable domain, git CLI is very stable)
