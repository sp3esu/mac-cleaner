---
phase: 14-git-repository-cache-management
plan: 01
subsystem: scanning
tags: [git, gc, lfs, walk, cleanup, pseudo-path]

requires:
  - phase: 12-project-cache-scanning
    provides: internal/walk package for directory traversal
provides:
  - pkg/gitrepos scanner discovering .git directories with count-objects reclaimable estimation
  - cleanup git-gc: pseudo-path handler running git gc --aggressive and git lfs prune
  - git-repos risk category (moderate) in safety package
affects: [14-02-PLAN, engine-registration, cli-flags]

tech-stack:
  added: []
  patterns: [git-gc pseudo-path cleanup dispatch, CmdRunner DI for git CLI, count-objects KiB parsing]

key-files:
  created:
    - pkg/gitrepos/scanner.go
    - pkg/gitrepos/scanner_test.go
  modified:
    - internal/cleanup/cleanup.go
    - internal/cleanup/cleanup_test.go
    - internal/safety/risk.go
    - internal/safety/risk_test.go

key-decisions:
  - "git-gc: pseudo-path prefix dispatches cleanup to git gc --aggressive instead of os.RemoveAll"
  - "LFS detection via .git/lfs directory during scan, flagged in description as [LFS] for cleanup dispatch"
  - "Package-level var runGitCleanup for test injection in cleanup package"

patterns-established:
  - "git-gc: pseudo-path pattern for non-destructive cleanup via external commands"
  - "CmdRunner + lookPathFn dependency injection for testable external command execution"

requirements-completed: [GIT-01, GIT-02, GIT-03, GIT-04]

duration: 5min
completed: 2026-03-11
---

# Phase 14 Plan 01: Git Repository Scanner and Cleanup Summary

**Walk-based .git directory scanner with count-objects reclaimable estimation and git gc --aggressive cleanup via git-gc: pseudo-path dispatch**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-11T20:30:01Z
- **Completed:** 2026-03-11T20:35:50Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Created pkg/gitrepos scanner that discovers .git directories under home using walk utility, parses git count-objects -v for reclaimable space estimation, detects LFS repos, skips submodules/worktrees and locked repos
- Extended cleanup.Execute with git-gc: pseudo-path handler that runs git gc --aggressive (120s timeout) and optionally git lfs prune
- Added git-repos category as moderate risk in safety package
- 16 new tests covering all scanner edge cases and cleanup dispatch paths

## Task Commits

Each task was committed atomically:

1. **Task 1: Create git repository scanner package with tests** - `86efa68` (feat)
2. **Task 2: Extend cleanup package with git-gc: pseudo-path handler** - `f4a7969` (feat)

_Both tasks followed TDD: tests written first (RED), then implementation (GREEN)._

## Files Created/Modified
- `pkg/gitrepos/scanner.go` - Git repo scanner with walk-based discovery, count-objects parsing, LFS detection
- `pkg/gitrepos/scanner_test.go` - 11 tests: discovery, submodule skip, lock skip, parsing, description format, LFS, path prefix
- `internal/cleanup/cleanup.go` - Extended with git-gc: pseudo-path dispatch and runGitCleanup function
- `internal/cleanup/cleanup_test.go` - 5 new tests: success, failure, LFS, regular paths, docker regression
- `internal/safety/risk.go` - Added git-repos: moderate risk entry
- `internal/safety/risk_test.go` - Added git-repos test case

## Decisions Made
- Used git-gc: pseudo-path prefix for cleanup dispatch (consistent with docker: pattern but executes commands instead of skipping)
- LFS presence detected via .git/lfs directory during scan, communicated to cleanup via [LFS] marker in description string
- Package-level var runGitCleanup for test injection (minimal-impact, avoids changing Execute signature)
- lookPathFn variable for testing git-not-installed scenario without modifying PATH

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Scanner package ready for engine registration and CLI flag integration (14-02)
- All interfaces match expected contracts from research
- No blockers

---
*Phase: 14-git-repository-cache-management*
*Completed: 2026-03-11*
