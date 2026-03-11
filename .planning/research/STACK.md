# Stack Research

**Domain:** macOS disk cleaner - home directory scanning, project cache detection, git cache management
**Researched:** 2026-03-11
**Confidence:** HIGH

## Executive Recommendation

**No new dependencies needed.** All three v1.7.0 features (home directory file scanning, extended project cache detection, git repository cache management) build entirely on Go stdlib and existing internal packages. The codebase already has every pattern required: recursive directory walking, external command execution with dependency injection, safety checks, and pseudo-path conventions for non-filesystem cleanup.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `filepath.WalkDir` | Go 1.25 stdlib | Recursive directory traversal for home dir scanning and project cache discovery | Stdlib since Go 1.16. More efficient than `filepath.Walk` (avoids `os.Lstat` per entry). Already used implicitly by `scan.DirSize`. No reason to add a third-party walker. |
| `os/exec` | Go 1.25 stdlib | Running `git` commands for repo cache management (gc, prune) | Already established pattern in codebase (`developer.CmdRunner`, `appleftovers.CmdRunner`). Reuse the `CmdRunner` injection pattern for testability. |
| `os.FileInfo` / `fs.DirEntry` | Go 1.25 stdlib | File metadata (size, modification time) for large/old file detection | Standard approach. `DirEntry.Info()` gives size and mod time. No need for syscall-level stat. |
| `time` | Go 1.25 stdlib | Age threshold calculations for old file detection | Already used in `appleftovers` for 90-day download threshold. Same pattern applies. |

### Supporting Libraries (Existing Internal Packages)

| Library | Purpose | How It's Used |
|---------|---------|---------------|
| `internal/scan` | `ScanTopLevel`, `DirSize`, `ScanEntry`, `CategoryResult` types | Use for all new scanner output. Existing helpers handle permission errors, safety checks, and size computation. |
| `internal/safety` | `IsPathBlocked`, `RiskForCategory` | Must be called on every discovered path. Extend `RiskForCategory` with new category IDs. |
| `internal/engine` | `Scanner` interface, `NewScanner`, `Register` | Register new scanner groups in `RegisterDefaults`. Follow existing adapter pattern. |
| `internal/cleanup` | `Execute` for file deletion | Standard cleanup works for home dir files and project caches. Git cache cleanup needs pseudo-path convention (see below). |

### Development Tools

No new tools needed. Existing toolchain covers everything.

| Tool | Purpose | Notes |
|------|---------|-------|
| `gosec` (existing) | Security analysis | Will flag any unsafe `exec.Command` usage in git scanner |
| `go vet` (existing) | Static analysis | Standard checks apply |
| `govulncheck` (existing) | Dependency vuln check | No new deps means no new attack surface |

## Installation

```bash
# No new dependencies to install.
# All features use Go stdlib + existing internal packages.
```

## Key Integration Points

### 1. Home Directory Scanner (`pkg/homedir/scanner.go`)

**Pattern:** New scanner package following `pkg/*/scanner.go` convention.

**Needs from stdlib:**
- `filepath.WalkDir` to traverse `~/` recursively
- `fs.DirEntry.Info()` for size and mod time
- Size threshold (e.g., 100MB) and age threshold (e.g., 180 days) as constants
- Skip list for directories already covered by other scanners (Library, .npm, .gradle, etc.)

**Integration with existing code:**
- Output: `[]scan.CategoryResult` with category IDs like `"home-large-files"`, `"home-old-files"`
- Risk: `risky` -- users may not understand which large/old files are important
- Register via `engine.NewScanner` in `RegisterDefaults`

### 2. Extended Project Caches Scanner (`pkg/projects/scanner.go`)

**Pattern:** New `pkg/projects/` package. Separate from `pkg/developer/` because developer scanner handles tool-level caches at known fixed paths (npm global, Homebrew). Project caches are per-project build artifacts scattered across `~/` requiring recursive discovery.

**Needs from stdlib:**
- `filepath.WalkDir` to find directories matching target names
- `scan.DirSize` to measure each found directory
- `fs.SkipDir` to avoid descending into already-matched directories

**Target directories to detect:**

| Directory | Ecosystem | Typical Size |
|-----------|-----------|-------------|
| `node_modules/` | Node.js | 100MB-1GB per project |
| `.build/` | Swift Package Manager | 100MB-500MB |
| `target/` | Rust (Cargo) / Java (Maven) | 500MB-5GB |
| `build/` | Gradle/CMake/generic | 50MB-2GB |
| `.gradle/` (project-level) | Gradle wrapper caches | 50MB-500MB |
| `__pycache__/` | Python | 1-50MB |
| `.tox/` | Python tox | 100MB-1GB |
| `.pytest_cache/` | Python pytest | <10MB |
| `dist/` | Python/JS build output | 10-500MB |
| `.next/` | Next.js | 50-500MB |
| `.nuxt/` | Nuxt.js | 50-200MB |
| `Pods/` | CocoaPods (project-level) | 100MB-1GB |
| `.angular/` | Angular cache | 50-200MB |
| `.turbo/` | Turborepo cache | 50-500MB |
| `vendor/` | Go modules (when vendored) | 50-500MB |

### 3. Git Repository Cache Scanner (`pkg/git/scanner.go`)

**Pattern:** New scanner package. Uses `CmdRunner` pattern from `developer` and `appleftovers` packages.

**Needs from stdlib:**
- `filepath.WalkDir` to find `.git/` directories under `~/`
- `os/exec` to run `git` commands: `count-objects -vH`
- `exec.LookPath("git")` to verify git is available (same pattern as Docker scanner)
- Context-based timeout (60s for git gc, longer than Docker's 10s because gc on large repos is slow)

**Git commands needed:**

| Command | Purpose | When |
|---------|---------|------|
| `git -C <repo> count-objects -vH` | Measure loose objects and pack size | During scan |
| `git -C <repo> gc --prune=now` | Clean up and compress repo | During cleanup |
| `git -C <repo> remote prune origin` | Remove stale remote-tracking branches | During cleanup |

**Cleanup is non-standard:** Unlike other scanners where cleanup = `os.RemoveAll(path)`, git cleanup runs commands. Use the **pseudo-path convention** (like `docker:BuildCache`) so `cleanup.Execute` skips them via existing `isPseudoPath`. Then handle git cleanup in a dedicated function. This matches the Docker pattern exactly.

Pseudo-path format: `"git-gc:/path/to/repo"`

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `filepath.WalkDir` (stdlib) | `github.com/karrick/godirwalk` | Never for this project. godirwalk was valuable pre-Go 1.16. `WalkDir` eliminated the overhead. Adding a dependency for marginal gain is not worth it. |
| `os/exec` for git commands | `github.com/go-git/go-git` (pure Go git) | Never for this project. go-git is 15+ transitive dependencies, large binary size increase, and we only need 3-4 git CLI commands. The CLI is simpler, well-tested, and matches the existing Docker pattern. |
| Constants for thresholds | Config file / flags | Later (not for v1.7). Hardcoded thresholds match the 90-day download pattern. Add configurability when users request it. |
| New `pkg/projects/` package | Extend `pkg/developer/` | If the project cache list is very short (<5 types). With 15+ patterns, a separate package is cleaner. |
| Pseudo-path cleanup pattern | New cleanup interface with per-scanner hooks | If more scanners need command-based cleanup in the future. For now, only git and Docker need it, and pseudo-paths are proven. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/go-git/go-git` | 15+ transitive deps, large binary bloat, API complexity for simple operations. We need `count-objects` and `gc`, not programmatic git access. | `os/exec` with `git` CLI via existing `CmdRunner` pattern |
| `github.com/karrick/godirwalk` | Unnecessary since Go 1.16's `filepath.WalkDir`. Adds a dependency for no measurable benefit on macOS with APFS. | `filepath.WalkDir` (stdlib) |
| `github.com/djherbis/times` | For accessing birth time (creation time) on macOS. Not needed -- modification time (`ModTime()`) is sufficient for "old file" detection. | `os.FileInfo.ModTime()` |
| `github.com/shirou/gopsutil` | Disk usage info. Overkill -- we compute sizes by walking directories. | `scan.DirSize` (existing) |
| Concurrent directory walking | Premature optimization. Parallel `WalkDir` adds goroutine management, channel coordination, error aggregation complexity. Sequential walk is fast enough for home directory on APFS. Profile first if slow. | Sequential `filepath.WalkDir` |
| `github.com/spf13/viper` | Config management. Not needed for v1.7 -- thresholds are hardcoded constants following existing patterns. | Constants in scanner packages |

## Changes to Existing Code

### New category IDs in `internal/safety/risk.go`

| Category ID | Risk Level | Rationale |
|-------------|-----------|-----------|
| `home-large-files` | `risky` | Users may not realize which large files are important |
| `home-old-files` | `risky` | Old files may still be needed |
| `project-node-modules` | `safe` | Fully reproducible from package-lock.json |
| `project-build-artifacts` | `safe` | Fully reproducible from source |
| `project-python-cache` | `safe` | Bytecode cache, auto-regenerated |
| `git-repo-caches` | `moderate` | gc is safe but user should understand what it does |

### New scanner registrations in `internal/engine/registry.go`

Three new `e.Register(NewScanner(...))` calls following the existing pattern.

### New packages

| Package | Purpose |
|---------|---------|
| `pkg/homedir/` | Home directory large/old file scanner |
| `pkg/projects/` | Project build cache discovery scanner |
| `pkg/git/` | Git repository cache management scanner |

### Cleanup consideration

The `cleanup.Execute` function uses `os.RemoveAll` for all non-pseudo-paths and skips pseudo-paths via `isPseudoPath`. For git cleanup:
- Git scanner emits `"git-gc:/path/to/repo"` entries (skipped by `isPseudoPath`)
- A separate git-specific cleanup function in `pkg/git/` handles running `git gc`
- The engine or server layer calls this function for git categories before/after standard cleanup

This avoids modifying the shared `cleanup` package or `scan.ScanEntry` type.

## Version Compatibility

| Component | Compatible With | Notes |
|-----------|-----------------|-------|
| `filepath.WalkDir` | Go 1.16+ | Project uses Go 1.25, well above minimum |
| `fs.DirEntry` | Go 1.16+ | Used by WalkDir callback signature |
| `exec.CommandContext` | Go 1.7+ | Already used in project for Docker/PlistBuddy |
| Existing `scan` helpers | All new scanners | `ScanTopLevel` for known-path scanning, `DirSize` for measuring directories |

## Sources

- Go stdlib documentation (`filepath.WalkDir`, `fs.DirEntry`, `os/exec`) -- HIGH confidence, official stdlib
- Existing codebase patterns (`pkg/developer/scanner.go`, `pkg/appleftovers/scanner.go`, `internal/cleanup/cleanup.go`) -- HIGH confidence, direct code review
- `go-git/go-git` dependency assessment -- MEDIUM confidence, based on known dependency tree size
- `karrick/godirwalk` obsolescence -- HIGH confidence, well-documented that `filepath.WalkDir` (Go 1.16) replaced its primary use case

---
*Stack research for: macOS disk cleaner v1.7.0 extended scanning*
*Researched: 2026-03-11*
