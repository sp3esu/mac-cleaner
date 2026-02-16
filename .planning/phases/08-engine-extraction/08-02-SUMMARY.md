---
phase: 08-engine-extraction
plan: 02
subsystem: engine-wiring
tags: [refactor, channel-api, token-validation, gosec, regression-tests]

# Dependency graph
requires:
  - "08-01 (Engine package with Scanner interface, ScanAll/Run/Cleanup methods)"
provides:
  - "CLI using Engine struct for all scanning (no direct pkg/* imports)"
  - "Server using Engine struct instance with token-based cleanup"
  - "Scan response includes token for cleanup validation"
  - "CleanupParams requires token field (protocol change)"
  - "8 pre-existing gosec G104 findings resolved in server.go"
affects:
  - "09-protocol-server (server API already updated)"
  - "10-scan-cleanup-handlers (handlers already using Engine channels)"
  - "11-hardening (docs already updated for token protocol)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Table-driven flag-to-scanner mapping (scannerMapping struct)"
    - "Channel draining for ScanAll events in CLI scanAll()"
    - "Engine.Run() for single-scanner flag-based scanning"
    - "Token round-trip: scan returns token, cleanup requires token"
    - "nosec annotations for best-effort Close/Remove patterns"

key-files:
  created: []
  modified:
    - "cmd/root.go"
    - "cmd/root_test.go"
    - "cmd/serve.go"
    - "internal/server/server.go"
    - "internal/server/handler_scan.go"
    - "internal/server/handler_cleanup.go"
    - "internal/server/protocol.go"
    - "internal/server/server_test.go"
    - "docs/swift-integration.md"

key-decisions:
  - "CLI cleanup stays in cmd/root.go (interactive confirmation is CLI-specific UI logic)"
  - "Engine initialized in PreRun (after flag expansion, before command execution)"
  - "Token included in scan result JSON for server clients"
  - "CleanupParams.Token required (empty token returns error, not token validation)"
  - "Pre-existing gosec findings fixed with nosec annotations and _ = discard pattern"

# Metrics
duration: 9min
completed: 2026-02-16
---

# Phase 8 Plan 02: CLI and Server Wiring Summary

**Wired cmd/root.go and internal/server/ to use Engine struct API, replacing 6 direct scanner calls with table-driven engine delegation and channel-based ScanAll, adding token round-trip for cleanup validation**

## Performance

- **Duration:** 9 min
- **Started:** 2026-02-16T22:50:57Z
- **Completed:** 2026-02-16T23:00:01Z
- **Tasks:** 3/3
- **Files modified:** 9

## Accomplishments

- Removed all 6 `pkg/*` scanner imports from `cmd/root.go` -- all scanning now routes through the Engine
- Replaced 6 `runXxxScan()` functions with single `runScannerByID()` using `engine.Run()`
- Replaced flag-based scanning block with table-driven `scannerMapping` loop
- Replaced callback-based `scanAll()` with channel-draining `engine.ScanAll()` API
- Updated `cmd/serve.go` to create Engine and pass to `server.New()`
- Updated Server struct to hold `*engine.Engine` field, removed `lastScan` atomic.Pointer
- Scan handler streams events via channel API, includes token in response
- Cleanup handler requires token parameter, delegates to `engine.Cleanup()`
- Categories handler uses `engine.Categories()` instead of package-level function
- Added 3 regression tests (flag mappings, scanner info lookup, engine categories)
- Fixed 8 pre-existing gosec G104 findings in server.go
- Updated `docs/swift-integration.md` with token protocol changes
- All tests pass with `-race`, `go vet` clean, `gosec` zero issues

## Task Commits

Each task was committed atomically:

1. **Task 1: Refactor cmd/root.go to use Engine struct** - `db2d2ba` (refactor)
2. **Task 2: Update server to use Engine struct with token-based cleanup** - `4d61c24` (refactor)
3. **Task 3: Verify zero behavior change and add regression tests** - `c51990e` (test)

## Files Modified

- `cmd/root.go` - Removed pkg/* imports, added scannerMapping table, runScannerByID(), channel-draining scanAll()
- `cmd/root_test.go` - Updated filterSkipped calls to engine.FilterSkipped, added 3 regression tests
- `cmd/serve.go` - Creates Engine instance, passes to server.New()
- `internal/server/server.go` - Added engine field, updated New() signature, removed lastScan atomic.Pointer, fixed gosec findings
- `internal/server/handler_scan.go` - Channel-based ScanAll, token in response, engine.Categories()
- `internal/server/handler_cleanup.go` - Token-required cleanup via engine.Cleanup()
- `internal/server/protocol.go` - Added Token field to CleanupParams
- `internal/server/server_test.go` - Updated all tests for new New() signature, added invalid token test
- `docs/swift-integration.md` - Token in scan result, token required in cleanup params, updated Swift types

## Decisions Made

- **CLI cleanup stays in cmd/root.go:** Interactive confirmation, walkthrough mode, and cleanup.Execute() calls remain in the CLI layer. The engine's Cleanup() with token is primarily for the server. The CLI handles its own UI-specific cleanup flow.
- **Engine initialization in PreRun:** Engine is created in PreRun (after --all flag expansion) rather than in init(), keeping it close to where flags are processed.
- **Token round-trip protocol:** Scan result now includes a `token` field. Cleanup requests require this token. Empty token returns a clear error message. This is a protocol change documented in swift-integration.md.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Fixed 8 pre-existing gosec G104 findings**
- **Found during:** Task 3 (gosec verification)
- **Issue:** `server.go` had 8 unhandled error returns on Close(), SetReadDeadline(), and os.Remove() calls (pre-existing from before Plan 08)
- **Fix:** Added `#nosec G104` annotations for best-effort close/shutdown patterns, used `_ =` discard for SetReadDeadline and os.Remove
- **Files modified:** `internal/server/server.go`
- **Commit:** `c51990e`

**2. [Rule 2 - Missing Critical] Updated swift-integration.md for token protocol**
- **Found during:** Task 2 (cleanup handler update)
- **Issue:** Documentation did not reflect the token requirement in cleanup params or token field in scan result
- **Fix:** Updated protocol examples, Swift Codable types, and error handling section
- **Files modified:** `docs/swift-integration.md`
- **Commit:** (included in metadata commit)

## Issues Encountered

None.

## User Setup Required

None.

## Next Phase Readiness

- Phase 8 (Engine Extraction) is COMPLETE
- All compilation errors from Plan 01 are resolved
- Engine is the sole orchestrator for both CLI and server
- `cmd/root.go` has zero direct `pkg/*` imports
- Server uses Engine struct with channel APIs and token validation
- Phase 9 (Protocol & Server Core) targets are already implemented (server, protocol, handlers exist)
- Phase 10 (Scan & Cleanup Handlers) targets are already implemented (channel-based handlers exist)
- Phases 9-10 may focus on refinement/hardening rather than net-new implementation

## Self-Check: PASSED

All 9 modified files verified present. All 3 task commits verified in git log.

---
*Phase: 08-engine-extraction*
*Completed: 2026-02-16*
