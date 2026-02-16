---
phase: 07-safety-enforcement
plan: 02
subsystem: cli
tags: [risk-display, permissions, color-output, warnings, safety]

# Dependency graph
requires:
  - phase: 07-safety-enforcement
    provides: "RiskLevel on ScanEntry, PermissionIssue type, RiskForCategory mapping, SetRiskLevels method"
  - phase: 06-cli-polish-automation
    provides: "printResults, printJSON, flagJSON, flagVerbose, tabwriter formatting"
  - phase: 05-interactive-mode
    provides: "RunWalkthrough, PromptConfirmation with io.Reader/io.Writer injection"
provides:
  - "Risk-colored [risky]/[moderate] tags in scan output, confirmation, and interactive walkthrough"
  - "Bold red WARNING line in confirmation prompt when risky items are present"
  - "Permission issue reporting to stderr via printPermissionIssues"
  - "Aggregated permission_issues in JSON output ScanSummary"
  - "Empty category filtering in printResults (permission-only categories hidden)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: ["Risk tag coloring with fatih/color red/yellow instances outside loops", "Permission issue stderr reporting separate from JSON output path"]

key-files:
  created: []
  modified:
    - "cmd/root.go"
    - "internal/confirm/confirm.go"
    - "internal/interactive/interactive.go"

key-decisions:
  - "Risk tags appended after Description in tabwriter format (not as separate column)"
  - "printPermissionIssues writes to stderr (not stdout) to avoid contaminating pipeable output"
  - "Permission issues suppressed from stderr in JSON mode (included in JSON payload instead)"
  - "Empty categories (zero entries, only permission issues) skipped in printResults display"
  - "WARNING line placed after total size but before Type yes prompt in confirmation"

patterns-established:
  - "Risk tag display: switch on entry.RiskLevel with color.FgRed for risky, color.FgYellow for moderate, empty for safe"
  - "Permission stderr reporting: collect issues from all CategoryResults, print summary to os.Stderr"
  - "hasRiskyItems helper: scan all entries across all categories for any RiskRisky entry"

# Metrics
duration: 3min
completed: 2026-02-16
---

# Phase 7 Plan 2: Risk-Aware Display and Permission Reporting Summary

**Colored risk tags ([risky]/[moderate]) in all output paths with bold WARNING in confirmation and stderr permission issue reporting**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-16T13:22:22Z
- **Completed:** 2026-02-16T13:25:08Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Risk-colored tags appear in scan output, confirmation listing, and interactive walkthrough
- Confirmation prompt shows bold red WARNING when risky items are in the removal set
- Permission issues reported to stderr after scanning (suppressed in JSON mode)
- JSON output aggregates permission issues from all categories into ScanSummary
- Empty categories (permission-only, no entries) are hidden from human-readable display

## Task Commits

Each task was committed atomically:

1. **Task 1: Risk-colored display and permission reporting in CLI** - `190da75` (feat)
2. **Task 2: Risk warnings in confirmation and interactive walkthrough** - `c8cc3b9` (feat)

## Files Created/Modified
- `cmd/root.go` - Risk tags in printResults, printPermissionIssues function, permission aggregation in printJSON, empty category filtering
- `internal/confirm/confirm.go` - hasRiskyItems helper, WARNING line in confirmation, risk tags on per-entry display
- `internal/interactive/interactive.go` - Risk tags in walkthrough item display with color instances outside loops

## Decisions Made
- Risk tags are appended inline after entry.Description in the tabwriter format string, not as a separate column
- printPermissionIssues writes to stderr so it does not contaminate stdout for piping
- Permission issues are not printed to stderr when in JSON mode -- they are included in the JSON payload via ScanSummary.PermissionIssues
- Empty categories (zero entries but may have PermissionIssues) are skipped in printResults via `continue` guard
- WARNING line is placed after the "Total: X will be permanently deleted" line but before the "Type 'yes' to proceed:" prompt

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 7 complete: all safety enforcement features are in place
- Risk levels tagged on every scan entry and displayed in all output paths
- Permission issues collected and reported through structured channels
- All existing and new tests pass
- Project is feature-complete per the roadmap

## Self-Check: PASSED

All 3 modified files verified present. Both task commits (190da75, c8cc3b9) verified in git log. Build and all tests pass.

---
*Phase: 07-safety-enforcement*
*Completed: 2026-02-16*
