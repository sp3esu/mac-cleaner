---
phase: 07-safety-enforcement
plan: 01
subsystem: safety
tags: [risk-levels, permissions, scanning, safety]

# Dependency graph
requires:
  - phase: 01-project-setup-safety-foundation
    provides: "safety.IsPathBlocked, scan types (ScanEntry, CategoryResult, ScanSummary)"
  - phase: 02-system-cache-scanning
    provides: "pkg/system scanner"
  - phase: 03-browser-developer-caches
    provides: "pkg/browser and pkg/developer scanners"
  - phase: 04-app-leftover-scanning
    provides: "pkg/appleftovers scanner"
provides:
  - "RiskLevel field on every ScanEntry produced by all scanners"
  - "PermissionIssue type and collection on CategoryResult and ScanSummary"
  - "RiskForCategory mapping function for all 14 category IDs"
  - "SetRiskLevels method on CategoryResult"
  - "Structured permission error reporting replacing ad-hoc stderr prints"
affects: [07-02-PLAN, display-layer, interactive-mode]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Risk-level tagging via SetRiskLevels callback pattern", "PermissionIssue collection instead of silent error swallowing"]

key-files:
  created:
    - "internal/safety/risk.go"
    - "internal/safety/risk_test.go"
  modified:
    - "internal/scan/types.go"
    - "internal/scan/helpers.go"
    - "pkg/system/scanner.go"
    - "pkg/browser/scanner.go"
    - "pkg/developer/scanner.go"
    - "pkg/appleftovers/scanner.go"

key-decisions:
  - "Risk constants live in safety package (not scan) to avoid circular imports"
  - "SetRiskLevels uses callback pattern: cr.SetRiskLevels(safety.RiskForCategory)"
  - "Safari TCC permission error returns structured PermissionIssue instead of fmt.Fprintf to stderr"
  - "Permission-only results (zero entries, non-zero PermissionIssues) propagate through nil guards"
  - "Unknown category IDs default to moderate risk"

patterns-established:
  - "Risk tagging: every scanner calls cr.SetRiskLevels(safety.RiskForCategory) before returning"
  - "Permission collection: os.IsPermission checks at Stat, ReadDir, and entry-level errors"
  - "Permission-only CategoryResult: zero entries + PermissionIssues still included in results"

# Metrics
duration: 4min
completed: 2026-02-16
---

# Phase 7 Plan 1: Risk Levels and Permission Collection Summary

**Risk level tagging (safe/moderate/risky) on all scan entries with structured permission error collection replacing ad-hoc stderr prints**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-16T13:15:15Z
- **Completed:** 2026-02-16T13:20:09Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Every ScanEntry produced by every scanner now has a non-empty RiskLevel field (safe, moderate, or risky)
- Risk mapping covers all 14 category IDs with sensible defaults (unknown defaults to moderate)
- Permission errors are collected as structured PermissionIssue structs instead of being silently swallowed
- Safari TCC permission denial now returns a PermissionIssue instead of printing to stderr
- All existing tests pass without modification (new fields are additive)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add risk/permission types and mapping function** - `fa4e7af` (feat)
2. **Task 2: Integrate risk levels and permission collection into all scanners** - `02f3493` (feat)

## Files Created/Modified
- `internal/safety/risk.go` - Risk level constants (safe/moderate/risky) and RiskForCategory mapping for 14 category IDs
- `internal/safety/risk_test.go` - Table-driven tests for all 14 IDs plus unknown and empty string
- `internal/scan/types.go` - RiskLevel on ScanEntry, PermissionIssue type, PermissionIssues on CategoryResult/ScanSummary, SetRiskLevels method
- `internal/scan/helpers.go` - Permission error detection in ScanTopLevel (ReadDir and entry-level)
- `pkg/system/scanner.go` - SetRiskLevels calls, permission handling in scanQuickLook
- `pkg/browser/scanner.go` - Safari TCC PermissionIssue, Chrome/Firefox permission handling, SetRiskLevels
- `pkg/developer/scanner.go` - Permission handling in all 5 helpers, SetRiskLevels calls
- `pkg/appleftovers/scanner.go` - Permission handling in all 3 helpers, SetRiskLevels calls

## Decisions Made
- Risk constants placed in `internal/safety` package (not `internal/scan`) to avoid circular imports since scan/helpers.go already imports safety
- SetRiskLevels uses a callback pattern (`func(string) string`) so the scan package does not need to import safety
- Safari TCC error path now returns a CategoryResult with PermissionIssue instead of using fmt.Fprintf to stderr -- the display layer (Plan 02) will handle presentation
- Permission-only CategoryResults (zero entries, non-zero PermissionIssues) propagate through callers by adding `len(cr.PermissionIssues) > 0` checks alongside existing `len(cr.Entries) == 0` guards
- Unknown category IDs default to "moderate" risk (conservative middle ground)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All scanners now produce risk-tagged entries and collected PermissionIssues
- Ready for Plan 02 (display layer changes to show risk levels and permission warnings to users)
- No blockers

## Self-Check: PASSED

All 8 key files verified present. Both task commits (fa4e7af, 02f3493) verified in git log.

---
*Phase: 07-safety-enforcement*
*Completed: 2026-02-16*
