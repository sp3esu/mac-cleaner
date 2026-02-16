# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-16)

**Core value:** Users can safely and confidently reclaim disk space without worrying about deleting something important
**Current focus:** Phase 6 in progress -- CLI polish and automation flags

## Current Position

Phase: 6 of 7 (CLI Polish & Automation)
Plan: 1 of 2 completed in current phase
Status: In progress
Last activity: 2026-02-16 - Completed 06-01-PLAN.md (JSON/all/verbose flags)

Progress: [█████████░] ~71%

## Performance Metrics

**Velocity:**
- Total plans completed: 10
- Average duration: 2.8 min
- Total execution time: 0.47 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-project-setup-safety-foundation | 2/2 | 3 min | 1.5 min |
| 02-system-cache-scanning | 2/2 | 5 min | 2.5 min |
| 03-browser-developer-caches | 2/2 | 8 min | 4 min |
| 04-app-leftover-scanning | 2/2 | 6 min | 3 min |
| 05-interactive-mode | 1/1 | 3 min | 3 min |
| 06-cli-polish-automation | 1/2 | 3 min | 3 min |

**Recent Trend:**
- Last 5 plans: 04-01 (3 min), 04-02 (3 min), 05-01 (3 min), 06-01 (3 min)
- Trend: Consistent

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Version output is bare (no prefix) via SetVersionTemplate
- Root command runs interactive walkthrough as default no-args action
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
- PromptConfirmation uses io.Reader/io.Writer for full testability without stdin/stdout coupling
- Exact "yes" required for confirmation (case-sensitive, whitespace-trimmed)
- Pseudo-paths containing ":" skipped during cleanup (macOS paths never contain colons)
- os.RemoveAll for all entries (handles files and directories; nil on nonexistent = success)
- Combined scan flags produce single aggregated confirmation prompt
- Shared bufio.Reader between walkthrough and confirmation prevents buffered data loss
- EOF defaults remaining items to keep (safe default)
- scanAll always prints with dryRun=true since interactive mode handles deletion decisions
- Scanner errors in scanAll logged to stderr, partial results still returned
- --all uses PreRun hook to set all four category flags before Run
- --json sets color.NoColor=true in PreRun to prevent ANSI contamination
- --json without scan flags exits with error (requires --all or specific flag)
- --json suppresses per-category printResults calls; single printJSON at end
- --verbose adds path line below each entry in tabwriter (no effect in JSON mode)

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
- io.Reader/io.Writer injection for testable interactive prompts
- Safety re-check at deletion time (IsPathBlocked before each os.RemoveAll)
- Pseudo-path filtering: entries with ":" skipped in filesystem operations
- Scan result aggregation: runner functions return []CategoryResult for Root to aggregate
- Interactive walkthrough: io.Reader/io.Writer injection with readChoice re-prompt loop
- Shared bufio.Reader: single reader for multi-stage interactive flows
- PreRun hook pattern: flag expansion and output mode setup before Run
- JSON output mode: suppress human-readable output, emit structured JSON to stdout
- Flag guard pattern: !flagJSON check before printResults in each runner function

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-02-16
Stopped at: Completed 06-01-PLAN.md (JSON/all/verbose flags), Phase 6 plan 1 of 2 done
Resume file: .planning/phases/06-cli-polish-automation/06-02-PLAN.md

---
*State initialized: 2026-02-16*
*Last updated: 2026-02-16 (06-01 complete)*
