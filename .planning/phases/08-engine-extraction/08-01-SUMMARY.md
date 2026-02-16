---
phase: 08-engine-extraction
plan: 01
subsystem: engine
tags: [go-interface, channels, context, crypto-rand, sync-mutex, adapter-pattern]

# Dependency graph
requires: []
provides:
  - "Scanner interface with Scan() and Info() methods"
  - "Engine struct with ScanAll(), Run(), Cleanup() methods"
  - "Channel-based streaming API for scan and cleanup events"
  - "ScanToken with single-token store and replay protection"
  - "Custom error types: ScanError, CancelledError, TokenError (errors.As compatible)"
  - "RegisterDefaults() registering all 6 scanner groups with full metadata"
  - "FilterSkipped() preserved as package-level utility"
affects:
  - "08-02 (wire CLI and server to Engine struct)"
  - "09-protocol-server (server will consume Engine instance)"
  - "10-scan-cleanup-handlers (handlers use ScanAll/Cleanup channels)"

# Tech tracking
tech-stack:
  added: ["crypto/rand (token generation)", "encoding/hex (token encoding)"]
  patterns:
    - "Scanner interface with adapter wrapping pkg/*/Scan() functions"
    - "Channel-based streaming: ScanAll/Cleanup return (<-chan Event, <-chan Result)"
    - "Select-on-send with ctx.Done() for all channel operations"
    - "Single-token store with one-time-use replay protection"
    - "Engine struct holding scanner registry and token store"

key-files:
  created:
    - "internal/engine/scanner.go"
    - "internal/engine/errors.go"
    - "internal/engine/token.go"
    - "internal/engine/registry.go"
  modified:
    - "internal/engine/engine.go"
    - "internal/engine/engine_test.go"

key-decisions:
  - "Sequential scanner execution (matching current behavior, concurrent can be added later)"
  - "Single-token store instead of map (new scan invalidates previous, matching server lastScan pattern)"
  - "Run() returns synchronously (channels overkill for single-scanner execution)"
  - "CleanupDone struct wraps both Result and Err (clean channel API without separate error path)"

patterns-established:
  - "Adapter pattern: NewScanner(info, fn) wraps bare Scan functions into Scanner interface"
  - "Two-channel pattern: events channel for streaming + done channel for final result"
  - "Context-aware sends: every channel send uses select with ctx.Done()"
  - "Token lifecycle: storeResults on scan completion, validateToken+clear on cleanup"

# Metrics
duration: 6min
completed: 2026-02-16
---

# Phase 8 Plan 01: Engine Package Summary

**Scanner interface with channel-based Engine API, single-token store, custom error types, and 31 tests covering all code paths with race detection**

## Performance

- **Duration:** 6 min
- **Started:** 2026-02-16T22:41:38Z
- **Completed:** 2026-02-16T22:47:45Z
- **Tasks:** 3/3
- **Files created/modified:** 6

## Accomplishments

- Built complete `internal/engine/` package replacing old struct-based Scanner with interface-based API
- Implemented channel-based ScanAll() and Cleanup() with context cancellation support throughout
- Token-based cleanup validation with single-token store (one-time use, replay protection)
- 31 tests passing with `-race` flag covering interface, streaming, cancellation, errors, and token lifecycle
- RegisterDefaults() wraps all 6 scanner packages with rich ScannerInfo metadata (ID, Name, Description, CategoryIDs)
- Zero external dependencies added (stdlib only: crypto/rand, encoding/hex, sync, context)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Scanner interface, error types, and token store** - `1ae4018` (feat)
2. **Task 2: Rewrite Engine struct with channel-based ScanAll, Run, Cleanup, and registry** - `466a13b` (feat)
3. **Task 3: Write comprehensive engine tests with mock scanners** - `0aa2ec9` (test)

## Files Created/Modified

- `internal/engine/scanner.go` - Scanner interface, ScannerInfo struct, scannerAdapter, NewScanner() constructor
- `internal/engine/errors.go` - ScanError, CancelledError, TokenError custom error types with Unwrap()
- `internal/engine/token.go` - ScanToken type, storeResults(), validateToken() with mutex protection
- `internal/engine/registry.go` - Register(), Categories(), RegisterDefaults() for all 6 scanner groups
- `internal/engine/engine.go` - Engine struct, New(), ScanAll(), Run(), Cleanup(), FilterSkipped(), event types
- `internal/engine/engine_test.go` - 31 tests with mock scanners, context cancellation, token lifecycle, error types

## Decisions Made

- **Sequential execution**: Scanners run sequentially in ScanAll(), matching current CLI behavior. Concurrent execution can be added later via internal change without API modification.
- **Single-token store**: Engine stores at most one token (new scan invalidates previous). Matches the server's existing `lastScan` atomic pointer pattern and avoids memory leak concerns.
- **Synchronous Run()**: Run() returns `([]scan.CategoryResult, error)` directly rather than channels. For single-scanner execution, channels add complexity without benefit.
- **CleanupDone struct**: Cleanup done channel uses `CleanupDone{Result, Err}` to carry both the cleanup result and any token validation error through a single channel type.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Engine package is complete and independently usable (zero cobra dependency, zero fmt.Print)
- `cmd/...` and `internal/server/...` intentionally fail to compile (3 call sites reference removed `engine.ScanAll()` and `engine.DefaultScanners()`)
- Plan 02 will wire the CLI and server to use the new `*Engine` struct API
- The adapter pattern ensures zero changes needed in `pkg/*/scanner.go` files

## Self-Check: PASSED

All 6 files verified present. All 3 task commits verified in git log.

---
*Phase: 08-engine-extraction*
*Completed: 2026-02-16*
