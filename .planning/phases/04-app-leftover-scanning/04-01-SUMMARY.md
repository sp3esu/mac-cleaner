---
phase: 04-app-leftover-scanning
plan: 01
subsystem: scanning
tags: [plistbuddy, preferences, ios-backups, downloads, orphan-detection]

# Dependency graph
requires:
  - phase: 02-system-cache-scanning
    provides: "scan.ScanTopLevel, scan.DirSize, scan.CategoryResult types"
  - phase: 03-browser-developer-caches
    provides: "CmdRunner injection pattern, LookPath guard pattern"
provides:
  - "appleftovers.Scan() returning orphaned prefs, iOS backups, old Downloads"
  - "--app-leftovers CLI flag"
affects: [05-interactive-ui, 06-removal-engine]

# Tech tracking
tech-stack:
  added: []
  patterns: [plistbuddy-bundle-id-extraction, prefix-based-orphan-detection, age-based-file-filtering]

key-files:
  created:
    - pkg/appleftovers/scanner.go
    - pkg/appleftovers/scanner_test.go
  modified:
    - cmd/root.go

key-decisions:
  - "scanOrphanedPrefs takes plistBuddyPath parameter for testability without PATH manipulation"
  - "Prefix matching for bundle IDs: domain == id OR HasPrefix(domain, id+'.') catches sub-domains"
  - "90-day maxAge hardcoded in Scan(), configurability deferred to Phase 6"
  - "entry.Info() used for Downloads (Lstat semantics from ReadDir) instead of explicit os.Lstat"

patterns-established:
  - "PlistBuddy path injection: pass path as parameter instead of relying on LookPath with PATH manipulation in tests"
  - "Age-based filtering: time.Since(modTime) > maxAge with configurable duration parameter"
  - "Bundle ID prefix matching: domain == id || HasPrefix(domain, id+'.') for sub-preference detection"

# Metrics
duration: 3min
completed: 2026-02-16
---

# Phase 4 Plan 1: App Leftovers Scanner Summary

**Three sub-scanners for orphaned preferences (PlistBuddy bundle ID matching), iOS device backups (ScanTopLevel), and old Downloads (90-day age filter) with --app-leftovers CLI flag**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-16T11:52:31Z
- **Completed:** 2026-02-16T11:55:41Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Orphaned preference detection scanning 5 app directories for bundle IDs via PlistBuddy, with com.apple.* always excluded
- iOS device backup scanning using established ScanTopLevel pattern on MobileSync/Backup
- Old Downloads scanning with 90-day age threshold, DirSize for directories, ModTime filtering
- --app-leftovers flag wired into CLI, works standalone and combined with all existing flags
- 13 tests covering all three sub-scanners, edge cases (missing dirs, zero-byte, no PlistBuddy), and integration

## Task Commits

Each task was committed atomically:

1. **Task 1: Build app leftovers scanner with three sub-scanners** - `a37a5ff` (feat)
2. **Task 2: Wire --app-leftovers flag in CLI** - `207aefd` (feat)

## Files Created/Modified
- `pkg/appleftovers/scanner.go` - Three sub-scanners: orphaned prefs, iOS backups, old Downloads
- `pkg/appleftovers/scanner_test.go` - 13 tests for all sub-scanners and edge cases
- `cmd/root.go` - Added --app-leftovers flag and runAppLeftoversScan dispatch

## Decisions Made
- scanOrphanedPrefs accepts plistBuddyPath as parameter rather than using exec.LookPath with PATH, making tests simpler (no fake docker PATH pattern needed)
- Prefix matching for bundle IDs uses domain == id OR HasPrefix(domain, id+".") to catch sub-preferences like com.known.app.helper
- 90-day maxAge hardcoded in Scan() function, configurability deferred to Phase 6 per plan
- Used entry.Info() for Downloads entries (ReadDir provides Lstat semantics) rather than separate os.Lstat calls

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- App leftovers scanner complete with all three sub-categories
- Ready for Phase 4 Plan 2 (whatever follows in the phase)
- All four scan categories (system, browser, developer, app-leftovers) now wired and tested
- Combined flag usage verified: --system-caches --browser-data --dev-caches --app-leftovers --dry-run

## Self-Check: PASSED

All files exist, all commits verified.

---
*Phase: 04-app-leftover-scanning*
*Completed: 2026-02-16*
