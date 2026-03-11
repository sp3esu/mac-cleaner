# Phase 12: Project Cache Scanning - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Detect and report project build caches (node_modules, .build, target/, venv/.venv, build/) across the home directory via recursive walk. Report sizes, flag stale projects, show aggregate summaries per type. Cleanup deletes entire cache directories. Shared walk utility built here is reused by Phases 13 and 14.

</domain>

<decisions>
## Implementation Decisions

### Scan result grouping
- One category per project type: proj-node, proj-swift, proj-rust, proj-python, proj-java
- Each type gets its own skip flag (--skip-node-modules, --skip-swift-build, etc.)
- Single scanner group "project-caches" with group-level --project-caches / --skip-project-caches flags
- Aggregate summary embedded in category description line: "node_modules (47 dirs, 28.3 GB)"

### Entry labeling
- Parse project name from manifest file (package.json "name", Cargo.toml [package] name, etc.)
- Fall back to parent directory name if manifest parsing fails
- Entry path is the cache directory (e.g. ~/code/myapp/node_modules), description is the project name

### Staleness display
- Inline age marker next to each entry: "my-app  512 MB  ⚠ 14mo" for stale, "dashboard  1.2 GB  2d" for recent
- Staleness determined by manifest file mtime (package.json, Cargo.toml, etc.) — single stat call per project
- 6-month threshold hardcoded (configurability deferred to CONF requirements)
- Stale entries marked with ⚠ symbol and age in months
- Entries sorted by size descending (largest first), consistent with all existing scanners

### Risk levels (per-type)
- node_modules → safe (fully reproduced by npm/yarn install from lockfile)
- .build (Swift) → safe (fully reproduced by swift build)
- target/ (Rust) → safe (fully reproduced by cargo build)
- build/ (Java/Kotlin) → safe (fully reproduced by gradle/maven build)
- venv/.venv (Python) → moderate (may contain locally-installed packages or editable installs not in requirements.txt)

### Cleanup behavior
- Delete entire cache directory (rm -rf node_modules, target/, .build, venv, build/)
- Standard cleanup path through existing cleanup package — no special handling needed
- Build tools recreate directories on next install/build

### Walk utility
- New `internal/walk` package — dedicated recursive walker reused by Phases 13 and 14
- Max depth: 5 (covers ~/code/org/project/subproject/cache — most structures are 2-4 levels deep)
- Exclusions — never descend into:
  - ~/Library (covered by existing scanners, huge directory)
  - Dotfile directories (~/.config, ~/.local, ~/.cache, ~/.git, etc.)
  - ~/Applications (covered by unused-apps scanner)
  - Found cache directories (node_modules, target/, .build, venv, build/) — avoids walking millions of dependency files
- Walk discovers directories; manifest marker validation happens in scanner logic

### Scanner package
- New `pkg/projects/` package — separate from pkg/developer/ which handles fixed-path tool caches
- Registered as a single scanner in engine registry, emitting separate CategoryResult per project type

### Claude's Discretion
- Walk utility API design (callback vs channel vs slice return)
- Exact manifest parsing implementation details
- Age formatting logic (days, months, years)
- How to handle symlinked project directories during walk
- Performance optimizations (parallel directory reading, etc.)

</decisions>

<specifics>
## Specific Ideas

- Output preview per category type:
  ```
  node_modules (47 dirs, 28.3 GB)       safe
    my-app                  512 MB   ⚠ 14mo
    dashboard-ui            1.2 GB      2d
    old-project             890 MB   ⚠ 23mo
  ```
- Manifest markers for validation (no false positives): package.json, Cargo.toml, Package.swift, pyvenv.cfg, build.gradle/pom.xml

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/scan/types.go`: ScanEntry, CategoryResult, PermissionIssue types — project scanner produces these
- `internal/scan/helpers.go`: DirSize() for sizing directories, ScanTopLevel() pattern (though walk-based approach is different)
- `internal/engine/scanner.go`: Scanner interface, NewScanner() adapter — project scanner registers through this
- `internal/engine/registry.go`: RegisterDefaults() — new scanner added here
- `internal/safety/risk.go`: RiskForCategory() — needs new entries for proj-* categories
- `cmd/categories.go`: scanGroups registry — needs new group for project-caches

### Established Patterns
- Scanner adapter: `NewScanner(ScannerInfo{...}, projects.Scan)` wraps pkg function
- Category IDs follow prefix convention: system-*, browser-*, dev-*, msg-* → new: proj-*
- Skip flags: group-level + per-item, registered in cmd/categories.go
- Permission errors collected as PermissionIssue, not fatal
- Safety checks via safety.IsPathBlocked() before any deletion

### Integration Points
- `internal/engine/registry.go`: Register new scanner in RegisterDefaults()
- `cmd/categories.go`: Add project-caches group to scanGroups
- `cmd/scan.go`: Add targeted scan flags for project cache types
- `cmd/root.go`: Add --project-caches and --skip-project-caches flags
- `internal/safety/risk.go`: Add risk mappings for proj-node, proj-swift, proj-rust, proj-python, proj-java

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-project-cache-scanning*
*Context gathered: 2026-03-11*
