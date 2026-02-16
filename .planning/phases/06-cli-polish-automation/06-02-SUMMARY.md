---
phase: 06-cli-polish-automation
plan: 02
subsystem: cli
tags: [cobra, skip-flags, force, automation, filtering]

# Dependency graph
requires:
  - phase: 06-01
    provides: --all, --json, --verbose flags and PreRun hook pattern
provides:
  - 16 skip flags (4 category-level, 12 item-level) for granular scan exclusion
  - --force flag for automation-friendly confirmation bypass
  - buildSkipSet/filterSkipped data-driven filtering pattern
affects: [07-packaging-release]

# Tech tracking
tech-stack:
  added: []
  patterns: [data-driven skip mapping, category-level vs item-level flag separation, PreRun skip override after --all expansion]

key-files:
  created: []
  modified: [cmd/root.go]

key-decisions:
  - "Category-level skip flags negate scan booleans in PreRun (scanner never runs)"
  - "Item-level skip flags filter results after scanning via buildSkipSet/filterSkipped"
  - "--force only bypasses confirmation prompt, NOT the safety layer"
  - "Skip flags compose with --all: --all --skip-browser-data correctly excludes browser"
  - "Data-driven skipMapping pattern for maintainable flag-to-category-ID mapping"

patterns-established:
  - "Category-level skip: negate scan flag in PreRun hook (prevents scanner execution)"
  - "Item-level skip: data-driven buildSkipSet + filterSkipped post-scan filtering"
  - "Force bypass: nested !flagForce guard around confirmation prompt"

# Metrics
duration: 3min
completed: 2026-02-16
---

# Phase 6 Plan 2: Skip Flags and Force Bypass Summary

**16 --skip-* flags for granular scan exclusion and --force flag for automation-friendly confirmation bypass**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-16T12:52:00Z
- **Completed:** 2026-02-16T12:54:42Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- 4 category-level skip flags (--skip-system-caches, --skip-browser-data, --skip-dev-caches, --skip-app-leftovers) that prevent scanners from running entirely
- 12 item-level skip flags (--skip-derived-data, --skip-npm, --skip-yarn, --skip-homebrew, --skip-docker, --skip-safari, --skip-chrome, --skip-firefox, --skip-quicklook, --skip-orphaned-prefs, --skip-ios-backups, --skip-old-downloads) that filter specific categories from results
- --force flag bypasses confirmation prompt in both flag-based and interactive modes
- Skip flags compose correctly with --all (e.g. --all --skip-browser-data scans everything except browser data)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add skip flags, force flag, filtering, and bypass logic** - `e6cd8bf` (feat)

**Plan metadata:** pending (docs: complete plan)

## Files Created/Modified
- `cmd/root.go` - Added 17 new CLI flags (16 skip + 1 force), category-level skip overrides in PreRun, buildSkipSet/filterSkipped functions, force bypass in deletion flow

## Decisions Made
- Category-level skip flags negate scan booleans in PreRun (scanner never runs) -- most efficient approach since scanning is skipped entirely
- Item-level skip flags filter results after scanning via data-driven buildSkipSet/filterSkipped -- necessary because item-level categories are nested within scanner output
- --force only bypasses confirmation prompt, NOT the safety layer -- safety re-check at deletion time remains in place
- Data-driven skipMapping pattern maps flag pointers to category ID strings -- extensible for future categories

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 6 (CLI Polish & Automation) complete -- all planned CLI flags implemented
- CLI supports: --all, --json, --verbose, --dry-run, --force, 16 --skip-* flags
- Ready for Phase 7 (Packaging & Release)

## Self-Check: PASSED

- FOUND: cmd/root.go
- FOUND: commit e6cd8bf
- FOUND: 06-02-SUMMARY.md

---
*Phase: 06-cli-polish-automation*
*Completed: 2026-02-16*
