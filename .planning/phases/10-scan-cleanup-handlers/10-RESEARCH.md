# Phase 10: Scan & Cleanup Handlers - Research

**Researched:** 2026-02-17
**Domain:** Go IPC server handlers for scan, cleanup, and categories with streaming progress
**Confidence:** HIGH

## Summary

Phase 10's deliverables are already fully implemented. During Phase 8 (Engine Extraction), Plan 08-02 wired the server handlers to use the new Engine struct with channel-based streaming, token-based cleanup validation, and the categories method. The 08-02 summary explicitly states: "Phase 10 (Scan & Cleanup Handlers) targets are already implemented (channel-based handlers exist)" and "Phases 9-10 may focus on refinement/hardening rather than net-new implementation."

A systematic audit of each Phase 10 requirement against the current codebase confirms this:

- **PROTO-02 (Methods: scan, cleanup, categories, ping, shutdown):** All five methods are implemented in the handler dispatch table (`handler.go:20-31`). `ping` and `shutdown` are handled directly. `scan`, `cleanup`, and `categories` route to dedicated handler functions in `handler_scan.go` and `handler_cleanup.go`.
- **PROTO-03 (Scan method streams per-scanner progress events, then final result):** Fully implemented in `handler_scan.go:41-109`. The handler calls `engine.ScanAll()` which returns an events channel and a done channel. Events are drained and streamed as NDJSON progress messages (`scanner_start`, `scanner_done`, `scanner_error`). The final result includes `categories`, `total_size`, and `token`.
- **PROTO-04 (Cleanup method streams per-entry progress events, then final result):** Fully implemented in `handler_cleanup.go:28-93`. The handler calls `engine.Cleanup()` which returns events and done channels. Per-entry progress events (`cleanup_category_start`, `cleanup_entry`) are streamed. The final result includes `removed`, `failed`, `bytes_freed`, and `errors`.
- **PROTO-05 (Categories method returns available scanners with metadata):** Fully implemented in `handler_scan.go:111-118`. Calls `engine.Categories()` and maps `ScannerInfo` to `CategoryInfo{ID, Label}` for the JSON response.
- **SRV-03 (Single-connection handling, reject concurrent operations):** Fully implemented via `atomic.Bool` busy flag. Both `handleScan` (`handler_scan.go:42-44`) and `handleCleanup` (`handler_cleanup.go:29-31`) use `CompareAndSwap(false, true)` to reject concurrent operations with an error message "another operation is in progress".

All 20 server tests pass, including integration tests for categories (`TestServer_CategoriesMethod`), cleanup without scan (`TestServer_CleanupWithoutScan`), cleanup with invalid token (`TestServer_CleanupWithInvalidToken`), and concurrent operation rejection (tested via the busy flag in handler code). All 170+ total project tests pass.

**Primary recommendation:** Phase 10 should be a verification-and-gap-analysis pass, not new implementation. The planner should create a single plan focused on: (1) auditing existing code against each requirement, (2) identifying any missing integration tests for scan/cleanup streaming, (3) adding targeted tests for currently untested scenarios, and (4) updating REQUIREMENTS.md status to "complete".

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `encoding/json` | 1.25.7 | NDJSON request params unmarshalling, response encoding | Already in use for all handler param parsing |
| Go stdlib `context` | 1.25.7 | Per-connection context for cancellation propagation | Already in use; handlers receive connCtx |
| Go stdlib `sync/atomic` | 1.25.7 | Busy flag for concurrent operation rejection | Already in use; `server.busy` atomic.Bool |
| `internal/engine` | (internal) | ScanAll(), Cleanup(), Categories() orchestration | Already injected into Server, handlers call it |
| `internal/cleanup` | (internal) | CleanupResult type for final response | Engine wraps this; handler reads result fields |
| `internal/scan` | (internal) | CategoryResult, ScanEntry types | Serialized as JSON in scan result response |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go stdlib `fmt` | 1.25.7 | Error message formatting | Already in use for param validation errors |

### Alternatives Considered

None -- all stack decisions were made and implemented during Phase 8. No changes needed.

**Installation:** No new dependencies needed. Everything is already in `go.mod`.

## Architecture Patterns

### Current Implementation Structure

```
internal/server/
  protocol.go          # Request/Response types, ScanParams, CleanupParams, PingResult
  handler.go           # Handler struct, Dispatch() switch, handlePing()
  handler_scan.go      # handleScan() with streaming, handleCategories(), ScanProgress, ScanResult types
  handler_cleanup.go   # handleCleanup() with streaming, CleanupProgress, CleanupResult types
  server.go            # Server struct with busy atomic.Bool, engine field, connection handling
  protocol_test.go     # 9 unit tests for NDJSON encoding/decoding
  server_test.go       # 12 integration tests for server lifecycle, handlers, edge cases
```

### Pattern 1: Channel-Based Streaming Handler (Already Implemented)

**What:** Handlers call Engine methods that return `(<-chan Event, <-chan Result)`. The handler drains the events channel, writing each as an NDJSON progress message, then reads the done channel for the final result.
**Implementation:**
```go
// Source: internal/server/handler_scan.go:66-108
events, done := h.server.engine.ScanAll(ctx, skip)

// Drain events channel, streaming progress to client.
for event := range events {
    if ctx.Err() != nil {
        break
    }
    progress := ScanProgress{ScannerID: event.ScannerID, Label: event.Label}
    switch event.Type {
    case engine.EventScannerStart:
        progress.Event = "scanner_start"
    case engine.EventScannerDone:
        progress.Event = "scanner_done"
    case engine.EventScannerError:
        progress.Event = "scanner_error"
        if event.Err != nil {
            progress.Error = event.Err.Error()
        }
    }
    _ = w.WriteProgress(req.ID, progress)
}

result := <-done
```

### Pattern 2: Busy Flag for Concurrent Operation Rejection (Already Implemented)

**What:** Both scan and cleanup handlers use `atomic.Bool` CompareAndSwap to reject concurrent operations.
**Implementation:**
```go
// Source: internal/server/handler_scan.go:42-44 (identical pattern in handler_cleanup.go:29-31)
if !h.server.busy.CompareAndSwap(false, true) {
    _ = w.WriteErrorMsg(req.ID, "another operation is in progress")
    return
}
defer h.server.busy.Store(false)
```

### Pattern 3: Token-Required Cleanup (Already Implemented)

**What:** Cleanup handler validates the token before proceeding. Empty token returns a clear error. Invalid token returns engine's TokenError.
**Implementation:**
```go
// Source: internal/server/handler_cleanup.go:49-52
if params.Token == "" {
    _ = w.WriteErrorMsg(req.ID, "token is required; run scan first")
    return
}
```

### Pattern 4: Context-Aware Client Disconnect (Already Implemented)

**What:** Handlers check `ctx.Err()` before starting, during event streaming (break on disconnect), and before writing the final result (skip if disconnected).
**Implementation:**
```go
// Source: internal/server/handler_scan.go:48-51, 70-72, 92-94
if ctx.Err() != nil {
    return // client disconnected before starting
}
// ... during streaming:
if ctx.Err() != nil {
    break // stop streaming, client gone
}
// ... before final result:
if ctx.Err() != nil {
    return // skip result, client gone
}
```

### Anti-Patterns to Avoid

- **Adding new handler dispatch methods:** The switch in `handler.go:20-31` already routes all five methods. Do not restructure the dispatch.
- **Changing the Engine API:** Handlers consume the Engine's existing channel-based API. Do not modify engine signatures.
- **Removing the busy flag:** The `atomic.Bool` approach is simple and correct for single-connection servers. Do not replace with mutexes or more complex concurrency control.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Progress streaming | Custom event loop | Engine's `<-chan ScanEvent` / `<-chan CleanupEvent` | Already implemented with context-aware sends |
| Concurrent op rejection | Mutex-based approach | `atomic.Bool.CompareAndSwap` | Already implemented, lock-free, simple |
| Token validation | Server-side token store | `engine.Cleanup()` with built-in token validation | Engine owns tokens; handler just passes them through |
| JSON serialization | Custom wire format | `NDJSONWriter.WriteProgress/WriteResult` | Already handles concurrent writes with mutex |

**Key insight:** Nothing needs to be hand-rolled. All Phase 10 handler logic exists.

## Common Pitfalls

### Pitfall 1: Thinking This Phase Requires New Code

**What goes wrong:** Creating duplicate implementations of handlers that already exist.
**Why it happens:** The roadmap was written before Phase 8 was implemented. Phase 8 proactively built the scan, cleanup, and categories handlers as part of wiring the engine.
**How to avoid:** Audit existing code against requirements first. The Phase 8-02 summary explicitly states handlers are already using Engine channels.
**Warning signs:** Creating new files or rewriting handler functions.

### Pitfall 2: Missing Integration Test Coverage for Streaming

**What goes wrong:** Declaring handler requirements "done" without verifying that streaming progress events are tested end-to-end through the socket.
**Why it happens:** Current tests cover categories, ping, shutdown, cleanup-without-token, and cleanup-with-invalid-token. However, there is no test that performs a full scan through the socket and verifies progress events are received. There is no test that performs a full scan-then-cleanup through the socket and verifies cleanup progress events.
**How to avoid:** Add integration tests that:
- Send a `scan` request and verify both progress events AND the final result (with token)
- Send a `scan` then `cleanup` with the returned token, verifying cleanup progress events AND the final result
- Send a concurrent `scan` while one is in progress and verify the "another operation is in progress" error
**Warning signs:** Requirements marked complete without end-to-end streaming test evidence.

### Pitfall 3: Scan Integration Tests Touch Real Filesystem

**What goes wrong:** Integration tests that call `scan` on a real server with `RegisterDefaults()` will scan the actual filesystem, making tests slow, non-deterministic, and environment-dependent.
**Why it happens:** The existing `newTestEngine()` helper uses `RegisterDefaults()` which registers all real scanners.
**How to avoid:** For handler-focused integration tests, create a test engine with mock scanners (using `engine.NewScanner()` with fake data). This gives deterministic results, fast execution, and known categories/entries for assertions.
**Warning signs:** Tests that take multiple seconds per scan, or tests that fail on different machines because file paths differ.

### Pitfall 4: JSON Field Name Mismatches in Assertions

**What goes wrong:** Progress event fields use snake_case in JSON (`scanner_id`, `entry_path`, `bytes_freed`) but Go struct fields use PascalCase. Test assertions on raw JSON must use the correct field names.
**Why it happens:** Go's JSON tags handle the mapping, but tests that unmarshal into `map[string]interface{}` or raw JSON need to know the wire format.
**How to avoid:** Use typed structs (ScanProgress, CleanupProgress, ScanResult, CleanupResult) for test assertions, not raw maps.
**Warning signs:** Test assertions that check `result["scannerID"]` instead of `result["scanner_id"]`.

### Pitfall 5: Done Channel Must Be Read After Events Channel Closes

**What goes wrong:** Reading the done channel before draining events can deadlock because the engine goroutine sends to events first, and events is unbuffered.
**Why it happens:** The engine's `ScanAll` goroutine defers `close(events)` before `close(done)`, and sends events on an unbuffered channel. If the reader blocks on `<-done` without draining events, the goroutine cannot proceed.
**How to avoid:** Always drain the events channel first (`for range events`), then read done. The handlers already do this correctly. Tests must follow the same pattern.
**Warning signs:** Test hangs, goroutine leak detected by `go test -race`.

## Code Examples

All code examples reference existing implemented code:

### Scan Handler (Complete Implementation)

```go
// Source: internal/server/handler_scan.go:41-109
func (h *Handler) handleScan(ctx context.Context, req Request, w *NDJSONWriter) {
    if !h.server.busy.CompareAndSwap(false, true) {
        _ = w.WriteErrorMsg(req.ID, "another operation is in progress")
        return
    }
    defer h.server.busy.Store(false)

    if ctx.Err() != nil {
        return
    }

    var params ScanParams
    if len(req.Params) > 0 {
        if err := json.Unmarshal(req.Params, &params); err != nil {
            _ = w.WriteErrorMsg(req.ID, fmt.Sprintf("invalid params: %v", err))
            return
        }
    }

    skip := make(map[string]bool, len(params.Skip))
    for _, id := range params.Skip {
        skip[id] = true
    }

    events, done := h.server.engine.ScanAll(ctx, skip)

    for event := range events {
        if ctx.Err() != nil {
            break
        }
        progress := ScanProgress{ScannerID: event.ScannerID, Label: event.Label}
        switch event.Type {
        case engine.EventScannerStart:
            progress.Event = "scanner_start"
        case engine.EventScannerDone:
            progress.Event = "scanner_done"
        case engine.EventScannerError:
            progress.Event = "scanner_error"
            if event.Err != nil {
                progress.Error = event.Err.Error()
            }
        }
        _ = w.WriteProgress(req.ID, progress)
    }

    result := <-done

    if ctx.Err() != nil {
        return
    }

    var totalSize int64
    for _, cat := range result.Results {
        totalSize += cat.TotalSize
    }

    _ = w.WriteResult(req.ID, struct {
        Categories interface{} `json:"categories"`
        TotalSize  int64       `json:"total_size"`
        Token      string      `json:"token"`
    }{
        Categories: result.Results,
        TotalSize:  totalSize,
        Token:      string(result.Token),
    })
}
```

### Cleanup Handler (Complete Implementation)

```go
// Source: internal/server/handler_cleanup.go:28-93
func (h *Handler) handleCleanup(ctx context.Context, req Request, w *NDJSONWriter) {
    if !h.server.busy.CompareAndSwap(false, true) {
        _ = w.WriteErrorMsg(req.ID, "another operation is in progress")
        return
    }
    defer h.server.busy.Store(false)

    if ctx.Err() != nil {
        return
    }

    var params CleanupParams
    if len(req.Params) > 0 {
        if err := json.Unmarshal(req.Params, &params); err != nil {
            _ = w.WriteErrorMsg(req.ID, fmt.Sprintf("invalid params: %v", err))
            return
        }
    }

    if params.Token == "" {
        _ = w.WriteErrorMsg(req.ID, "token is required; run scan first")
        return
    }

    events, done := h.server.engine.Cleanup(ctx, engine.ScanToken(params.Token), params.Categories)

    for event := range events {
        if ctx.Err() != nil {
            break
        }
        _ = w.WriteProgress(req.ID, CleanupProgress{
            Event:     event.Type,
            Category:  event.Category,
            EntryPath: event.EntryPath,
            Current:   event.Current,
            Total:     event.Total,
        })
    }

    result := <-done

    if ctx.Err() != nil {
        return
    }

    if result.Err != nil {
        _ = w.WriteErrorMsg(req.ID, result.Err.Error())
        return
    }

    var errs []string
    for _, e := range result.Result.Errors {
        errs = append(errs, e.Error())
    }

    _ = w.WriteResult(req.ID, CleanupResult{
        Removed:    result.Result.Removed,
        Failed:     result.Result.Failed,
        BytesFreed: result.Result.BytesFreed,
        Errors:     errs,
    })
}
```

### Categories Handler (Complete Implementation)

```go
// Source: internal/server/handler_scan.go:111-118
func (h *Handler) handleCategories(req Request, w *NDJSONWriter) {
    infos := h.server.engine.Categories()
    cats := make([]CategoryInfo, len(infos))
    for i, info := range infos {
        cats[i] = CategoryInfo{ID: info.ID, Label: info.Name}
    }
    _ = w.WriteResult(req.ID, CategoriesResult{Scanners: cats})
}
```

### Mock Scanner for Testing (Recommended Pattern)

```go
// Create deterministic test engine with mock scanners
func newMockTestEngine() *engine.Engine {
    eng := engine.New()
    eng.Register(engine.NewScanner(engine.ScannerInfo{
        ID:   "mock-sys",
        Name: "Mock System",
    }, func() ([]scan.CategoryResult, error) {
        return []scan.CategoryResult{
            {
                Category:    "mock-caches",
                Description: "Mock Caches",
                TotalSize:   1024,
                Entries: []scan.ScanEntry{
                    {Path: "/tmp/mock/cache1", Description: "Cache 1", Size: 512},
                    {Path: "/tmp/mock/cache2", Description: "Cache 2", Size: 512},
                },
            },
        }, nil
    }))
    return eng
}
```

## State of the Art

| Roadmap Expectation | Current State | Gap |
|---------------------|---------------|-----|
| `handler_scan.go` with scan + categories methods | EXISTS: 118 lines, handleScan() with streaming, handleCategories() | NONE |
| `handler_cleanup.go` with cleanup method + streaming | EXISTS: 93 lines, handleCleanup() with streaming and token validation | NONE |
| Tests for all handlers | PARTIAL: Categories, cleanup-without-token, cleanup-invalid-token tested. Missing: full scan streaming test, scan-then-cleanup flow test, concurrent operation rejection test | GAP: Integration tests for streaming flows |
| Streaming per-scanner progress events | EXISTS: ScanProgress struct with scanner_start/done/error events | NONE |
| Streaming per-entry progress events | EXISTS: CleanupProgress struct with cleanup_category_start/cleanup_entry events | NONE |
| Concurrent operation rejection | EXISTS: atomic.Bool busy flag with CompareAndSwap in both handlers | NONE: Missing dedicated test |

## Requirement Coverage Analysis

### PROTO-02: Methods: scan, cleanup, categories, ping, shutdown

| Aspect | Status | Evidence |
|--------|--------|----------|
| ping method | DONE | `handler.go:35-39`, `TestServer_PingIntegration` |
| shutdown method | DONE | `server.go:175-179`, `TestServer_ShutdownViaMethod` |
| scan method | DONE | `handler_scan.go:41-109` |
| cleanup method | DONE | `handler_cleanup.go:28-93` |
| categories method | DONE | `handler_scan.go:111-118`, `TestServer_CategoriesMethod` |
| Dispatch routing | DONE | `handler.go:19-32` handles all 5 methods + unknown |
| Unknown method error | DONE | `handler.go:30-31`, `TestServer_UnknownMethod` |

### PROTO-03: Scan method streams per-scanner progress events, then final result

| Aspect | Status | Evidence |
|--------|--------|----------|
| Param parsing (skip) | DONE | `handler_scan.go:53-58` |
| scanner_start events | DONE | `handler_scan.go:76-77` |
| scanner_done events | DONE | `handler_scan.go:78-79` |
| scanner_error events | DONE | `handler_scan.go:80-84` |
| Progress via WriteProgress | DONE | `handler_scan.go:85` |
| Final result with categories | DONE | `handler_scan.go:100-108` |
| Final result with total_size | DONE | `handler_scan.go:95-98` |
| Final result with token | DONE | `handler_scan.go:107` |
| Client disconnect handling | DONE | `handler_scan.go:48-51, 70-72, 92-94` |
| Integration test (streaming) | **MISSING** | No test sends scan request and verifies progress + result |

### PROTO-04: Cleanup method streams per-entry progress events, then final result

| Aspect | Status | Evidence |
|--------|--------|----------|
| Param parsing (token, categories) | DONE | `handler_cleanup.go:40-52` |
| Token required validation | DONE | `handler_cleanup.go:49-52`, `TestServer_CleanupWithoutScan` |
| Invalid token error | DONE | `handler_cleanup.go:77-80`, `TestServer_CleanupWithInvalidToken` |
| cleanup_category_start events | DONE | `handler_cleanup.go:61-66` (via engine event type) |
| cleanup_entry events | DONE | `handler_cleanup.go:61-66` (via engine event type) |
| Progress includes current/total | DONE | `handler_cleanup.go:64-65` |
| Final result with removed/failed/bytes_freed | DONE | `handler_cleanup.go:87-92` |
| Final result with errors | DONE | `handler_cleanup.go:82-86` |
| Client disconnect handling | DONE | `handler_cleanup.go:36-38, 58-60, 72-74` |
| Integration test (streaming) | **MISSING** | No test sends scan + cleanup and verifies cleanup progress + result |

### PROTO-05: Categories method returns available scanners with metadata

| Aspect | Status | Evidence |
|--------|--------|----------|
| Returns scanner IDs | DONE | `handler_scan.go:114` maps info.ID |
| Returns scanner labels | DONE | `handler_scan.go:114` maps info.Name to Label |
| Wrapped in CategoriesResult | DONE | `handler_scan.go:117` |
| Integration test | DONE | `TestServer_CategoriesMethod` verifies 6 scanners returned |

### SRV-03: Single-connection handling (reject concurrent operations)

| Aspect | Status | Evidence |
|--------|--------|----------|
| Scan rejects if busy | DONE | `handler_scan.go:42-44` |
| Cleanup rejects if busy | DONE | `handler_cleanup.go:29-31` |
| Error message clear | DONE | "another operation is in progress" |
| Busy flag released on completion | DONE | `defer h.server.busy.Store(false)` in both handlers |
| Single-connection accept loop | DONE | `server.go:106` handleConnection (not goroutine) |
| Integration test | **MISSING** | No test verifies concurrent rejection through the socket |

## Test Gap Analysis

The following integration tests are missing and should be added in the verification plan:

1. **TestServer_ScanStreaming** -- Send a `scan` request via socket, verify:
   - At least one `progress` response with `scanner_start` event
   - At least one `progress` response with `scanner_done` event
   - One final `result` response with `categories`, `total_size`, and `token`
   - Use mock engine with deterministic scanners for fast, predictable results

2. **TestServer_ScanThenCleanup** -- Full workflow test:
   - Send `scan`, collect token from result
   - Send `cleanup` with the token, verify:
     - Progress events received (cleanup_category_start, cleanup_entry)
     - Final result with removed/failed/bytes_freed

3. **TestServer_ConcurrentScanRejected** -- Verify SRV-03:
   - Use a slow/blocking mock scanner
   - Send first `scan` request
   - While first scan is in progress, send second `scan`
   - Verify second request gets error "another operation is in progress"

4. **TestServer_ScanWithSkipParam** -- Verify skip parameter works through socket:
   - Register two mock scanners
   - Send scan with skip param for one scanner
   - Verify only one scanner's results in the final output

## Open Questions

1. **Should the planner create a full-featured plan or a lightweight verification plan?**
   - What we know: All requirements are fully implemented. The only gap is integration test coverage for the streaming handler flows.
   - Recommendation: Create a single plan with two tasks: (1) Add 3-4 integration tests using mock engines for deterministic fast tests, (2) Update REQUIREMENTS.md to mark PROTO-02 through PROTO-05 and SRV-03 as complete.

2. **Should the cleanup progress event types match the swift-integration.md documentation exactly?**
   - What we know: The cleanup handler sends `event.Type` directly from the engine constants (`cleanup_category_start`, `cleanup_entry`). The Swift documentation shows `category_start` and `entry_progress` as event names.
   - What's unclear: Whether the Swift doc event names are the authoritative specification or if the Go implementation is canonical.
   - Recommendation: Flag this for verification. The handler sends `event.Type` which is `cleanup_category_start` and `cleanup_entry` (from `engine.EventCleanupCategoryStart` and `engine.EventCleanupEntry`). The Swift doc says `category_start` and `entry_progress`. **There is a naming discrepancy.** The planner should include a task to either update the Swift docs to match the implementation or update the implementation to match the Swift docs. The implementation should be considered canonical since it has tests.

## Sources

### Primary (HIGH confidence)

- **Codebase inspection** -- Direct reading of all files in `internal/server/` (7 files), `internal/engine/` (6 files), `internal/cleanup/` (2 files), `internal/scan/types.go`
- **Phase 8-02 summary** -- `08-02-SUMMARY.md` documents handler wiring and explicit statement about Phase 10 readiness
- **Phase 9-01 summary** -- `09-01-SUMMARY.md` confirms Phase 9 complete, ready for Phase 10
- **Test execution** -- All 20 server tests pass, all 170+ project tests pass (`go test ./...`)
- **REQUIREMENTS.md** -- Current status of all Phase 10 requirements (pending)

### Secondary (MEDIUM confidence)

- **docs/swift-integration.md** -- Protocol documentation, used to identify naming discrepancy in cleanup events

### Tertiary (LOW confidence)

- None -- all findings verified against source code

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies needed; everything exists and is tested
- Architecture: HIGH -- Direct source code reading of all handler files, engine API, and protocol types
- Pitfalls: HIGH -- Identified from actual code patterns, test gap analysis, and naming discrepancy discovery
- Requirements coverage: HIGH -- Every requirement aspect verified against specific file:line evidence
- Test gaps: HIGH -- Systematic comparison of requirement aspects vs existing test names

**Research date:** 2026-02-17
**Valid until:** 2026-03-17 (stable -- no external dependencies, internal Go code)
