---
phase: 03-browser-developer-caches
plan: 02
subsystem: scanning
tags: [developer, xcode, npm, yarn, homebrew, docker, cache, cli, os-exec]

# Dependency graph
requires:
  - phase: 03-browser-developer-caches
    provides: Shared ScanTopLevel helper, generalized printResults, multi-flag CLI pattern
  - phase: 02-system-cache-scanning
    provides: scan types (ScanEntry, CategoryResult), DirSize
provides:
  - Developer cache scanner for Xcode DerivedData, npm, yarn, Homebrew, Docker
  - --dev-caches CLI flag with per-tool breakdown
  - CmdRunner dependency injection pattern for external CLI testability
  - parseDockerSize utility for Docker human-readable size strings
affects: [04-app-leftover-scanning, 06-removal-dry-run]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CmdRunner type for external CLI dependency injection in tests"
    - "Docker CLI integration: exec.LookPath guard, 10s context timeout, JSON line parsing"
    - "Yarn cache as single blob via DirSize (not ScanTopLevel)"
    - "Docker entries use docker: pseudo-path prefix for non-filesystem entries"

key-files:
  created:
    - pkg/developer/scanner.go
    - pkg/developer/scanner_test.go
  modified:
    - cmd/root.go

key-decisions:
  - "npm cache scanned at ~/.npm/ (not ~/Library/Caches/) per npm documentation"
  - "yarn cache treated as single blob with DirSize rather than ScanTopLevel"
  - "Docker reclaimable sizes parsed with ordered suffix matching (longest first)"
  - "Docker entries use docker:Type pseudo-paths since they are not filesystem paths"
  - "exec.LookPath check before Docker CLI call to avoid calling missing binary"

patterns-established:
  - "CmdRunner dependency injection: type CmdRunner func(ctx, name, args) ([]byte, error)"
  - "External CLI integration: LookPath guard -> context timeout -> JSON parsing -> graceful nil on failure"
  - "fakeDockerPath test helper: temp dir with fake executable prepended to PATH"

# Metrics
duration: 4min
completed: 2026-02-16
---

# Phase 3 Plan 2: Developer Cache Scanner Summary

**Developer cache scanner for Xcode DerivedData, npm, yarn, Homebrew, and Docker with CmdRunner dependency injection for Docker CLI testability**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-16T11:19:18Z
- **Completed:** 2026-02-16T11:23:19Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Built developer cache scanner covering 5 tool categories: Xcode DerivedData, npm cache, yarn cache, Homebrew cache, and Docker artifacts
- Docker CLI integration with 10-second timeout, JSON output parsing, and graceful failure when Docker is not installed or daemon is stopped
- CmdRunner dependency injection pattern enables full Docker test coverage without requiring a running Docker daemon
- Wired --dev-caches flag; all three scan flags (--system-caches, --browser-data, --dev-caches) work independently and combined

## Task Commits

Each task was committed atomically:

1. **Task 1: Build developer cache scanner with Docker CLI integration** - `e9608f8` (feat)
2. **Task 2: Wire --dev-caches flag in CLI** - `6e61494` (feat)

## Files Created/Modified
- `pkg/developer/scanner.go` - Developer cache scanner with Xcode, npm, yarn, Homebrew, Docker support
- `pkg/developer/scanner_test.go` - 16 test functions including Docker CLI mocking and parseDockerSize table tests
- `cmd/root.go` - Added --dev-caches flag, runDevCachesScan function, developer package import

## Decisions Made
- npm cache scanned at ~/.npm/ (the actual npm cache location) rather than ~/Library/Caches/ which does not contain npm data
- Yarn cache treated as a single blob using scan.DirSize since it is one logical cache, unlike Xcode DerivedData which has per-project subdirectories
- Docker size string parsing uses ordered suffix slice (TB, GB, MB, kB, KB, B) to prevent "B" suffix from matching before "GB" in map iteration
- Docker entries use "docker:Type" pseudo-paths since reclaimable Docker space is not tied to a single filesystem path
- exec.LookPath("docker") checked before any CLI call to avoid attempting execution of a missing binary

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed parseDockerSize suffix matching order**
- **Found during:** Task 1 (Docker size parsing)
- **Issue:** Map iteration order is non-deterministic in Go; "2.3GB" could match "B" suffix before "GB"
- **Fix:** Replaced map with ordered slice of unit entries, checking longest suffixes first
- **Files modified:** pkg/developer/scanner.go
- **Verification:** TestParseDockerSize table-driven tests all pass including 2.3GB, 1.5kB, 500MB cases
- **Committed in:** e9608f8 (Task 1 commit)

**2. [Rule 1 - Bug] Added fakeDockerPath helper for Docker test reliability**
- **Found during:** Task 1 (Docker tests)
- **Issue:** exec.LookPath("docker") in scanDocker checks real PATH; tests fail when Docker is not installed on CI/test machine
- **Fix:** Created fakeDockerPath test helper that places a fake docker script in a temp dir and prepends to PATH via t.Setenv
- **Files modified:** pkg/developer/scanner_test.go
- **Verification:** Docker tests pass regardless of whether Docker is installed on the test machine
- **Committed in:** e9608f8 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes necessary for correctness. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 3 complete: both browser and developer cache scanners implemented and tested
- All three scan categories (system, browser, developer) scannable via CLI flags
- Ready for Phase 4 (app leftover scanning)
- No blockers or concerns

## Self-Check: PASSED

All 3 files verified present. Both task commits (e9608f8, 6e61494) verified in git log.

---
*Phase: 03-browser-developer-caches*
*Completed: 2026-02-16*
