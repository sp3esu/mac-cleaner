---
phase: 13-home-directory-file-scanning
plan: 01
subsystem: scan
tags: [walk, filesystem, homedir, large-files, old-files]

# Dependency graph
requires:
  - phase: 12-project-cache-scanning
    provides: "walk utility with OnDir callback"
provides:
  - "walk.Options.OnFile callback for regular file processing"
  - "pkg/homedir scanner with large-files and old-files categories"
  - "Risk entries for home-large-files and home-old-files"
affects: [13-02-cli-integration]

# Tech tracking
tech-stack:
  added: []
  patterns: ["OnFile callback in walk.Options mirrors OnDir pattern"]

key-files:
  created:
    - "pkg/homedir/scanner.go"
    - "pkg/homedir/scanner_test.go"
  modified:
    - "internal/walk/walk.go"
    - "internal/walk/walk_test.go"
    - "internal/safety/risk.go"
    - "internal/safety/risk_test.go"

key-decisions:
  - "OnFile callback added to walk.Options struct, nil-safe for backward compatibility"
  - "formatAge duplicated from pkg/projects to avoid import cycle"

patterns-established:
  - "OnFile callback: same depth/exclusion semantics as OnDir for files"

requirements-completed: [HOME-01, HOME-02, HOME-03]

# Metrics
duration: 3min
completed: 2026-03-11
---

# Phase 13 Plan 01: Walk OnFile Extension and Home Directory File Scanner Summary

**Walk utility extended with OnFile callback; homedir scanner detects large files (>=100MB) and old files (>=180d, >=10MB) with exclusions and symlink skipping**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-11T19:44:49Z
- **Completed:** 2026-03-11T19:48:03Z
- **Tasks:** 1 (TDD: RED/GREEN/REFACTOR)
- **Files modified:** 6

## Accomplishments
- Extended walk.Walk with OnFile callback that respects MaxDepth, exclusions, and is nil-safe
- Built pkg/homedir scanner producing two categories: home-large-files and home-old-files
- Excludes ~/Library, ~/Applications, ~/.Trash, ~/Downloads, dotfile dirs, and project cache dirs
- Skips symlinks, sorts results by size descending, includes age annotation for old files
- Added risk entries (risky) for both new categories

## Task Commits

Each task was committed atomically (TDD flow):

1. **RED: Failing tests** - `8c1c990` (test)
2. **GREEN: Implementation** - `a335b11` (feat)
3. **REFACTOR: Doc updates** - `9a97500` (refactor)

## Files Created/Modified
- `pkg/homedir/scanner.go` - Home directory file scanner with large/old file detection
- `pkg/homedir/scanner_test.go` - 8 test cases covering all scanner behaviors
- `internal/walk/walk.go` - OnFile callback added to Options and Walk function
- `internal/walk/walk_test.go` - 4 OnFile tests (callback, max depth, exclusions, nil compat)
- `internal/safety/risk.go` - Risk entries for home-large-files and home-old-files
- `internal/safety/risk_test.go` - Test cases for new risk categories

## Decisions Made
- OnFile callback uses same depth calculation as OnDir, placed before directory exclusion checks since files inherit parent exclusion via WalkDir
- formatAge helper duplicated from pkg/projects to avoid import cycle (trivial 15-line function)
- MaxDepth 5 for home directory scan to balance depth coverage with performance

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Scanner core complete, ready for CLI integration in plan 02
- pkg/homedir.Scan() follows same pattern as pkg/projects.Scan() for engine registration

---
*Phase: 13-home-directory-file-scanning*
*Completed: 2026-03-11*
