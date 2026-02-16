# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-16)

**Core value:** Users can safely and confidently reclaim disk space without worrying about deleting something important
**Current focus:** Phase 4 in progress -- app leftover scanning

## Current Position

Phase: 4 of 7 (App Leftover Scanning)
Plan: 1 of 2 completed in current phase
Status: In progress
Last activity: 2026-02-16 - Completed 04-01-PLAN.md (App leftovers scanner)

Progress: [█████░░░░░] ~50%

## Performance Metrics

**Velocity:**
- Total plans completed: 7
- Average duration: 2.7 min
- Total execution time: 0.32 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-project-setup-safety-foundation | 2/2 | 3 min | 1.5 min |
| 02-system-cache-scanning | 2/2 | 5 min | 2.5 min |
| 03-browser-developer-caches | 2/2 | 8 min | 4 min |
| 04-app-leftover-scanning | 1/2 | 3 min | 3 min |

**Recent Trend:**
- Last 5 plans: 02-02 (3 min), 03-01 (4 min), 03-02 (4 min), 04-01 (3 min)
- Trend: Consistent

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Version output is bare (no prefix) via SetVersionTemplate
- Root command runs Help() as default action (interactive mode deferred to Phase 5)
- Errors printed to stderr with os.Exit(1) on failure
- Core safety protections are hardcoded -- no config can override them
- Swap/VM prefixes checked before SIP prefixes (simpler, no exceptions)
- filepath.EvalSymlinks failure on existing path blocks for safety
- Non-existent path checked against literal cleaned path
- SI units (base 1000) for FormatSize to match macOS Finder convention
- os.Lstat pre-check before WalkDir to distinguish nonexistent root from permission-denied
- Category field is plain string (not enum) for extensibility
- QuickLook scanner searches all com.apple.quicklook.* entries, not just ThumbnailsAgent
- Zero-byte entries excluded from results to reduce noise
- Entries sorted by size descending within each category
- tabwriter with AlignRight for size column alignment
- Safari uses DirSize on single directory (not ScanTopLevel) since it is one cache entry
- Chrome scans all subdirectories as profiles (Default, Profile 1, etc.)
- Firefox uses shared ScanTopLevel since its cache follows directory-of-subdirectories pattern
- printResults generalized with title parameter instead of separate print functions per scan type
- Multiple scan flags supported via ran boolean tracker (not early return)
- npm cache scanned at ~/.npm/ (not ~/Library/Caches/) per npm documentation
- Yarn cache treated as single blob with DirSize rather than ScanTopLevel
- Docker size parsing uses ordered suffix slice to prevent map iteration ambiguity
- Docker entries use docker:Type pseudo-paths for non-filesystem entries
- exec.LookPath guard before Docker CLI calls
- scanOrphanedPrefs takes plistBuddyPath parameter for testability (no PATH manipulation needed)
- Prefix matching for bundle IDs: domain == id OR HasPrefix(domain, id+".") catches sub-preferences
- 90-day maxAge hardcoded in Scan(), configurability deferred to Phase 6
- entry.Info() used for Downloads (ReadDir provides Lstat semantics)

### Patterns Established

- Cobra command pattern: root command in cmd/root.go with Execute() export
- Version injection via ldflags: -X github.com/gregor/mac-cleaner/cmd.version=X.Y.Z
- Terse help text: no personality, no exclamation marks, factual descriptions only
- Safety-first: normalize path (Clean + EvalSymlinks) before any blocklist check
- Boundary-safe prefix matching: path == prefix OR HasPrefix(path, prefix + /)
- Table-driven tests for exhaustive edge case coverage
- Stderr-only warnings via WarnBlocked
- Scan types: ScanEntry/CategoryResult/ScanSummary as shared result types for all scanners
- DirSize pattern: WalkDir with error-skipping for resilient directory traversal
- Shared scan helper: scan.ScanTopLevel for directory-of-subdirectories pattern
- Browser scanner pattern: private helpers take home string for testability with temp dirs
- Generalized printResults(results, dryRun, title) for all scan categories
- Multi-flag CLI pattern: ran boolean tracker allowing combined flags
- CLI flag wiring: package-level bool vars, init() registration, Run func dispatch
- Output pattern: fatih/color bold headers, cyan sizes, green+bold total line
- Home path shortening: replace home prefix with ~ for display
- CmdRunner dependency injection for external CLI testability (Docker)
- External CLI integration: LookPath guard -> context timeout -> JSON parsing -> graceful nil on failure
- fakeDockerPath test helper for PATH manipulation in tests
- PlistBuddy path injection: pass path as parameter instead of LookPath with PATH manipulation
- Age-based filtering: time.Since(modTime) > maxAge with configurable duration parameter
- Bundle ID prefix matching for orphaned preference detection

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-02-16
Stopped at: Completed 04-01-PLAN.md (app leftovers scanner), ready for 04-02
Resume file: .planning/phases/04-app-leftover-scanning/04-02-PLAN.md

---
*State initialized: 2026-02-16*
*Last updated: 2026-02-16 (04-01 complete)*
