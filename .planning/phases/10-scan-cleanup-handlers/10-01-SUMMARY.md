---
phase: 10-scan-cleanup-handlers
plan: 01
subsystem: testing, api
tags: [integration-tests, ndjson, unix-socket, streaming, mock-engine]

# Dependency graph
requires:
  - phase: 08-engine-extraction
    provides: "Engine with ScanAll/Cleanup streaming channels, scanner registry"
  - phase: 09-protocol-server-core
    provides: "NDJSON protocol, server with socket lifecycle, handler dispatch"
provides:
  - "Integration tests verifying scan streaming, cleanup flow, concurrent rejection, skip filtering through socket"
  - "Corrected Swift integration docs matching actual wire format"
  - "Phase 10 requirements marked complete in REQUIREMENTS.md"
affects: [11-hardening-documentation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Mock engine with deterministic fake scanners for socket-level tests"
    - "Line-based NDJSON response reader (bufio.Scanner) for streaming test assertions"
    - "Direct handler Dispatch for testing busy flag concurrency rejection"

key-files:
  created: []
  modified:
    - "internal/server/server_test.go"
    - "docs/swift-integration.md"
    - ".planning/REQUIREMENTS.md"

key-decisions:
  - "Used direct Dispatch call for concurrent rejection test (server architecture serializes socket requests)"
  - "Mock paths /tmp/mock-test/* intentionally non-existent to test handler plumbing without filesystem"

patterns-established:
  - "newMockTestEngine(): deterministic mock engine with 2 scanners for server integration tests"
  - "readAllResponses(): line-based streaming NDJSON reader avoiding json.Decoder buffering issues"
  - "isTimeout(): net.Error timeout check helper for deadline-aware test assertions"

# Metrics
duration: 4min
completed: 2026-02-17
---

# Phase 10 Plan 01: Scan & Cleanup Handlers Summary

**4 socket-level integration tests for scan streaming, cleanup flow, concurrent rejection, and skip filtering; Swift docs corrected to match wire format**

## Performance

- **Duration:** 4 min
- **Started:** 2026-02-16T23:33:47Z
- **Completed:** 2026-02-16T23:37:44Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added TestServer_ScanStreaming: verifies scanner_start/scanner_done progress events and final result with categories, total_size, token
- Added TestServer_ScanThenCleanup: verifies full scan-then-cleanup token round-trip through Unix socket
- Added TestServer_ConcurrentScanRejected: verifies busy flag rejects overlapping scan operations
- Added TestServer_ScanWithSkipParam: verifies category skip filtering through the socket protocol
- Fixed Swift integration docs to use correct cleanup event names (cleanup_category_start, cleanup_entry)
- Marked all Phase 10 requirements complete (PROTO-02 through PROTO-05, SRV-03)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add integration tests for streaming scan, cleanup, concurrent rejection, and skip params** - `6ed0d79` (test)
2. **Task 2: Fix Swift doc cleanup event names and update REQUIREMENTS.md** - `0d26ff0` (docs)

## Files Created/Modified
- `internal/server/server_test.go` - Added 4 integration tests, mock engine helper, streaming response reader, timeout helper
- `docs/swift-integration.md` - Fixed cleanup progress event names to match Go wire format
- `.planning/REQUIREMENTS.md` - Marked PROTO-02, PROTO-03, PROTO-04, PROTO-05, SRV-03 as complete

## Decisions Made
- Used direct handler Dispatch for concurrent rejection test because the server architecture processes requests sequentially per connection (busy flag is unreachable through socket-level concurrent requests on a single connection)
- Mock scanner paths are intentionally non-existent to test handler plumbing without filesystem side effects

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted concurrent scan rejection test approach**
- **Found during:** Task 1 (TestServer_ConcurrentScanRejected)
- **Issue:** Plan specified testing concurrent scan rejection by sending two scan requests on the same connection. The server processes requests sequentially in handleConnection (NDJSONReader.Read -> Dispatch -> repeat), so the busy flag is always cleared before the second request is read. True socket-level concurrent scans are architecturally impossible with the current single-connection sequential design.
- **Fix:** Test uses direct handler.Dispatch() call while first scan is running to verify the busy flag mechanism. This correctly exercises the concurrent rejection code path even though the socket layer serializes requests.
- **Files modified:** internal/server/server_test.go
- **Verification:** TestServer_ConcurrentScanRejected passes, confirms "another operation is in progress" error
- **Committed in:** 6ed0d79

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Test correctly verifies the busy flag mechanism. No scope change.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All Phase 10 requirements complete (13/16 total)
- Remaining 3 requirements (HARD-01, HARD-02, HARD-03) are Phase 11 hardening items
- Server test suite now covers all handler flows: ping, shutdown, categories, scan streaming, cleanup flow, concurrent rejection, skip params, invalid token, missing token, unknown method, client disconnect, context cancellation, stale socket, active server, non-socket file

## Self-Check: PASSED

All files verified present:
- internal/server/server_test.go
- docs/swift-integration.md
- .planning/REQUIREMENTS.md
- .planning/phases/10-scan-cleanup-handlers/10-01-SUMMARY.md

All commits verified:
- 6ed0d79: test(10-01) - integration tests
- 0d26ff0: docs(10-01) - Swift docs + requirements

---
*Phase: 10-scan-cleanup-handlers*
*Completed: 2026-02-17*
