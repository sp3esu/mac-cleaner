# Phase 11: Hardening & Documentation - Research

**Researched:** 2026-02-17
**Domain:** Go Unix domain socket server hardening, goroutine lifecycle management, Swift integration documentation
**Confidence:** HIGH

## Summary

Phase 11 covers three hardening requirements (HARD-01, HARD-02, HARD-03) and documentation updates. A systematic audit of the codebase reveals that much of the required functionality is already implemented, but with gaps in test coverage, one unused constant, and documentation that needs enrichment.

HARD-03 (replay protection) is fully complete -- the token lifecycle (`storeResults` / `validateToken` / one-time-use consumption) is implemented in `engine/token.go` and thoroughly tested at both the engine level (`TestCleanup_ValidToken`, `TestCleanup_InvalidToken`, `TestCleanup_TokenConsumed`) and server level (`TestServer_CleanupWithInvalidToken`, `TestServer_CleanupWithoutScan`, `TestServer_ScanThenCleanup`). No work needed.

HARD-01 (client disconnect) is partially implemented. The per-connection context (`connCtx`) in `handleConnection` is cancelled when the client disconnects. Both handlers check `ctx.Err()` at three points: before starting, during event streaming, and before writing the final result. The engine's `ScanAll` and `Cleanup` goroutines have context-aware channel sends. However, there is no test that disconnects a client during an active scan/cleanup and verifies the goroutine exits cleanly without leaks. Additionally, `cleanup.Execute` does not accept a context -- it runs to completion even after client disconnect, which is a defensible design choice (never abort file deletion mid-stream) but should be explicitly documented and verified as intentional.

HARD-02 (connection timeout and keep-alive) is partially implemented. The `IdleTimeout` constant (5 minutes) is applied via `conn.SetReadDeadline()` before each read in the connection loop, and reset after successful reads. However, the `ReadTimeout` constant (30 seconds) is defined but never applied anywhere -- it is dead code. There is no test verifying the idle timeout actually closes the connection. Keep-alive is not applicable for Unix domain sockets (no TCP stack), so the requirement's "keep-alive" aspect translates to the idle timeout mechanism already in place.

The Swift integration documentation (`docs/swift-integration.md`) is already comprehensive with Codable types, NWConnection examples, and lifecycle management guidance. Minor improvements could include adding an `NWProtocolFramer`-based approach for robust NDJSON framing and error handling guidance for mid-stream disconnects.

**Primary recommendation:** Create a single plan with two tasks: (1) add targeted hardening tests for disconnect-during-scan, disconnect-during-cleanup, and idle timeout; clean up the unused `ReadTimeout` constant; (2) verify HARD-03 as complete, update REQUIREMENTS.md, and optionally enhance Swift docs.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net` | 1.25.7 | Unix domain socket connections, SetReadDeadline | Already in use for server/client |
| Go stdlib `context` | 1.25.7 | Per-connection cancellation, goroutine lifecycle | Already in use throughout |
| Go stdlib `sync/atomic` | 1.25.7 | Busy flag for operation serialization | Already in use; `server.busy` |
| Go stdlib `time` | 1.25.7 | Timeout constants, deadline management | Already in use for IdleTimeout |
| Go stdlib `testing` | 1.25.7 | Test framework with `t.TempDir()` for socket paths | Project convention |
| Go stdlib `bufio` | 1.25.7 | NDJSON line-based reading | Already in use for NDJSONReader |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/engine` | (internal) | ScanAll/Cleanup with context-aware channels | Handlers delegate all operations |
| `internal/cleanup` | (internal) | File deletion execution (no context) | Called by engine's Cleanup goroutine |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual goroutine leak checks | `uber-go/goleak` | Adds external dependency; stdlib approach (timing + reconnect) sufficient for this project |
| Application-level keepalive | TCP keepalive | Unix domain sockets don't use TCP; idle timeout via `SetReadDeadline` is the correct approach |

**Installation:** No new dependencies needed. Everything is in Go stdlib.

## Architecture Patterns

### Current Server Connection Lifecycle

```
Client connects
  -> handleConnection(ctx, conn)
    -> connCtx, cancel := context.WithCancel(ctx)
    -> store conn and cancel in server fields
    -> read loop:
       -> SetReadDeadline(now + IdleTimeout)
       -> NDJSONReader.Read()
       -> reset deadline
       -> Dispatch(connCtx, req, writer)
    -> on error/disconnect: cancel(), close conn, clear fields
```

### Pattern 1: Context-Aware Disconnect Detection

**What:** The per-connection context (`connCtx`) is the primary mechanism for detecting client disconnects. When the connection is closed (by client or timeout), the NDJSONReader.Read() returns an error, which exits the read loop and triggers `defer cancel()`.
**Where implemented:** `server.go:134-183`
**How handlers use it:**
```go
// Source: internal/server/handler_scan.go:48-51, 70-72, 92-94
// Before starting:
if ctx.Err() != nil { return }
// During streaming:
if ctx.Err() != nil { break }
// Before final result:
if ctx.Err() != nil { return }
```

### Pattern 2: Idle Timeout via SetReadDeadline

**What:** Before each read, the connection deadline is set to `now + IdleTimeout`. If no message arrives within 5 minutes, the read returns a timeout error, which exits the connection loop and closes the connection.
**Where implemented:** `server.go:165-166`
**Gap:** The deadline is set per-read. During a long-running scan/cleanup, no reads happen, so the idle timeout doesn't apply (correctly -- the operation is active). After the operation completes, the next read applies the idle timeout again.

### Pattern 3: Write Error Swallowing

**What:** All `NDJSONWriter.Write*` calls have their errors discarded with `_ =`. When a client disconnects, subsequent writes to the closed connection return errors, which are silently ignored. The handler will naturally exit when it checks `ctx.Err()` or when the events channel closes.
**Where implemented:** All handler functions in `handler_scan.go` and `handler_cleanup.go`
**Why correct:** Writing to a disconnected client is not an error condition for the server. The context cancellation is the primary signal. Logging write errors would be noisy during normal disconnect scenarios.

### Anti-Patterns to Avoid

- **Adding TCP keepalive to Unix sockets:** Unix domain sockets are local IPC. TCP keepalive options (`KeepAlive`, `KeepAliveConfig`) are not applicable. The idle timeout via `SetReadDeadline` is the correct mechanism.
- **Making `cleanup.Execute` context-aware:** File deletion should NOT be interrupted mid-stream. Once deletion starts, it should complete. The progress channel may stop being consumed (ctx cancelled), but the `select` on `ctx.Done()` in the progress callback handles that gracefully.
- **Using `goleak` for goroutine leak tests:** The project uses only stdlib for testing. Goroutine leak verification can be done with `runtime.NumGoroutine()` before and after operations, combined with timing-based assertions.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Idle timeout | Custom timer goroutine | `conn.SetReadDeadline()` | Already implemented; net package handles deadline natively |
| Disconnect detection | Poll-based connection check | Per-connection `context.WithCancel` | Already implemented; read error triggers context cancellation |
| Goroutine leak detection | Custom goroutine counter | `runtime.NumGoroutine()` in tests | Simple, no external dependencies, sufficient for this use case |
| NDJSON framing (Swift) | Manual string buffering | `NWProtocolFramer` or simple buffer accumulation | Existing docs already show the buffer approach |
| Application keepalive | Custom heartbeat protocol | Idle timeout (server-side) + reconnect (client-side) | Simpler architecture; Unix sockets are reliable local IPC |

## Common Pitfalls

### Pitfall 1: Testing Disconnect During Active Operations

**What goes wrong:** Testing client disconnect during a scan/cleanup requires careful synchronization. The test must: (1) start a long-running operation, (2) verify it has started, (3) close the connection, (4) verify the server handled it gracefully.
**Why it happens:** The mock scanner executes instantly, so the client disconnects after the operation completes, not during it.
**How to avoid:** Use a blocking mock scanner (channel-based) that pauses mid-scan. The test closes the connection while the scanner is blocked, then releases the blocker. Verify the server accepts new connections afterward.
**Warning signs:** Test passes but doesn't actually test mid-operation disconnect because the scan completed before the connection was closed.

### Pitfall 2: Goroutine Leak From Engine ScanAll

**What goes wrong:** When `connCtx` is cancelled during a scan, the `ScanAll` goroutine may still be running (blocked in a `Scan()` call). The goroutine will eventually exit when `Scan()` returns and it attempts to send on the cancelled context, but there's a window where it's lingering.
**Why it happens:** `Scanner.Scan()` is not context-aware -- it scans the filesystem synchronously. The goroutine can only check `ctx.Done()` between scanners, not during a scan.
**How to avoid:** This is acceptable behavior. The goroutine WILL exit -- it just may take until the current scanner finishes. Tests should account for a brief delay (e.g., `time.Sleep` or polling) before asserting the goroutine count returned to baseline.
**Warning signs:** Flaky goroutine leak tests that pass on fast machines but fail on slow ones.

### Pitfall 3: ReadTimeout Constant Is Dead Code

**What goes wrong:** `ReadTimeout = 30 * time.Second` is defined in `server.go:23` but never referenced. It suggests an intended per-message read timeout that was never wired up.
**Why it happens:** The idle timeout was implemented but the per-read timeout during active messaging was deferred or forgotten.
**How to avoid:** Either remove the constant (if idle timeout is sufficient) or apply it appropriately. For an NDJSON protocol on Unix sockets, the idle timeout is the primary mechanism. Per-read timeouts during active messaging are not typically needed for local IPC.
**Recommendation:** Remove `ReadTimeout` to eliminate dead code. The idle timeout handles the primary case of detecting abandoned connections.

### Pitfall 4: Cleanup Continues After Client Disconnect

**What goes wrong:** When a client disconnects during cleanup, `cleanup.Execute()` continues deleting files because it doesn't accept a context. The engine's cleanup goroutine keeps running until all files are deleted.
**Why it happens:** `cleanup.Execute` was designed for safety -- never abort deletion mid-stream. The progress callback's `select` on `ctx.Done()` handles the case where progress events can't be sent.
**How to avoid:** This is correct behavior. Document it explicitly: "If the client disconnects during cleanup, file deletion continues to completion. This is by design to prevent partially-deleted state." The server goroutine will exit once cleanup finishes and it finds the context cancelled.
**Warning signs:** Treating this as a bug and adding context cancellation to file deletion loops.

### Pitfall 5: Unix Socket Path Length on macOS

**What goes wrong:** macOS has a 104-character limit for Unix socket paths. Using `t.TempDir()` (which generates long paths under `/var/folders/...`) can exceed this limit.
**Why it happens:** Go's `t.TempDir()` creates paths like `/var/folders/xx/xxxxxxxxxxxx/T/TestFoo123456789/001/test.sock`.
**How to avoid:** Use `os.TempDir()` (returns `/tmp` or equivalent short path) for socket paths in tests. This pattern is already established in the existing test suite.
**Warning signs:** Tests that work on Linux but fail on macOS with "invalid argument" or "name too long" errors.

## Code Examples

### Testing Client Disconnect During Active Scan

```go
// Pattern: blocking scanner + client disconnect + reconnect verification
func TestServer_DisconnectDuringScan(t *testing.T) {
    blocker := make(chan struct{})
    eng := engine.New()
    eng.Register(engine.NewScanner(engine.ScannerInfo{
        ID: "slow", Name: "Slow Scanner",
    }, func() ([]scan.CategoryResult, error) {
        <-blocker // block until released
        return []scan.CategoryResult{{Category: "slow-cat", TotalSize: 100}}, nil
    }))

    socketPath := filepath.Join(os.TempDir(), "mc-test-disconnect.sock")
    os.Remove(socketPath)
    defer os.Remove(socketPath)
    srv := New(socketPath, "test", eng)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    defer srv.Shutdown()

    go srv.Serve(ctx)
    waitForSocket(t, socketPath)

    // Connect and start a scan.
    conn, _ := net.Dial("unix", socketPath)
    sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})

    // Wait for scan to start (first progress event).
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    sc := bufio.NewScanner(conn)
    sc.Scan() // read scanner_start progress event

    // Disconnect while scan is in progress.
    conn.Close()

    // Release the blocker so the goroutine can finish.
    close(blocker)

    // Brief pause for server to process disconnect.
    time.Sleep(100 * time.Millisecond)

    // Server should still accept new connections.
    conn2, err := net.Dial("unix", socketPath)
    if err != nil {
        t.Fatalf("server not accepting connections after disconnect: %v", err)
    }
    defer conn2.Close()

    sendRequest(t, conn2, Request{ID: "p1", Method: MethodPing})
    resp := readResponse(t, conn2)
    if resp.Type != ResponseResult {
        t.Errorf("expected result after reconnect, got %q", resp.Type)
    }
}
```

### Testing Idle Timeout

```go
// Pattern: set short idle timeout for testing, verify connection closes
func TestServer_IdleTimeoutClosesConnection(t *testing.T) {
    // This would require the ability to configure IdleTimeout for testing.
    // Options:
    // 1. Make IdleTimeout configurable on the Server struct
    // 2. Use a functional option pattern: New(..., WithIdleTimeout(100*time.Millisecond))
    // 3. Export a test-only setter

    // After connecting and sending a ping, wait beyond the idle timeout,
    // then attempt to read -- should get an error (connection closed by server).
}
```

### Verifying Goroutine Cleanup (stdlib approach)

```go
// Pattern: count goroutines before and after to verify no leaks
func TestServer_NoGoroutineLeakOnDisconnect(t *testing.T) {
    // ... set up server with blocking scanner ...

    baseline := runtime.NumGoroutine()

    // Connect, start scan, disconnect, release blocker
    // ... (same as disconnect test) ...

    // Wait for goroutines to settle.
    deadline := time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        if runtime.NumGoroutine() <= baseline+1 {
            return // goroutines cleaned up
        }
        time.Sleep(50 * time.Millisecond)
    }
    t.Errorf("goroutine leak: started with %d, now %d", baseline, runtime.NumGoroutine())
}
```

## Requirement Status Analysis

### HARD-01: Client Disconnect During Scan/Cleanup Handled Gracefully

| Aspect | Status | Evidence |
|--------|--------|----------|
| Per-connection context creation | DONE | `server.go:135-136` `context.WithCancel(ctx)` |
| Context stored for external cancellation | DONE | `server.go:139-141` connCancel stored in server |
| Handlers check ctx before starting | DONE | `handler_scan.go:48-51`, `handler_cleanup.go:36-38` |
| Handlers check ctx during streaming | DONE | `handler_scan.go:70-72`, `handler_cleanup.go:58-60` |
| Handlers check ctx before final result | DONE | `handler_scan.go:92-94`, `handler_cleanup.go:72-74` |
| Engine ScanAll respects context | DONE | `engine.go:99, 104-108, 112-118, 120-125` |
| Engine Cleanup progress respects context | DONE | `engine.go:210-214` select on ctx.Done() |
| Connection cleanup on disconnect | DONE | `server.go:143-149` defer close + clear |
| Simple disconnect test | DONE | `TestServer_ClientDisconnectHandledGracefully` |
| **Disconnect during active scan test** | **MISSING** | No test with blocking scanner + mid-scan disconnect |
| **Disconnect during active cleanup test** | **MISSING** | No test with mid-cleanup disconnect |
| **Goroutine leak verification** | **MISSING** | No test verifies goroutines exit after disconnect |

### HARD-02: Connection Timeout and Keep-Alive

| Aspect | Status | Evidence |
|--------|--------|----------|
| IdleTimeout constant defined | DONE | `server.go:19` `5 * time.Minute` |
| IdleTimeout applied before each read | DONE | `server.go:165` `SetReadDeadline(now + IdleTimeout)` |
| Deadline reset after read | DONE | `server.go:173` `SetReadDeadline(time.Time{})` |
| ReadTimeout constant defined | DONE (unused) | `server.go:23` `30 * time.Second` -- dead code |
| Keep-alive mechanism | N/A | Unix domain sockets don't have TCP keepalive |
| **Idle timeout integration test** | **MISSING** | No test verifies timeout closes connection |
| **ReadTimeout cleanup** | **NEEDED** | Remove or apply the dead constant |

### HARD-03: Cleanup Validated Against Prior Scan Results (Replay Protection)

| Aspect | Status | Evidence |
|--------|--------|----------|
| Token generation on scan | DONE | `engine/token.go:22-36` storeResults |
| Token validation on cleanup | DONE | `engine/token.go:42-60` validateToken |
| Single-token store policy | DONE | New scan invalidates previous token |
| One-time-use consumption | DONE | Token cleared after successful validation |
| Token required by handler | DONE | `handler_cleanup.go:49-52` empty token check |
| Invalid token error | DONE | `handler_cleanup.go:77-80` engine TokenError |
| Engine-level tests | DONE | `TestCleanup_ValidToken`, `TestCleanup_InvalidToken`, `TestCleanup_TokenConsumed` |
| Server-level tests | DONE | `TestServer_CleanupWithInvalidToken`, `TestServer_CleanupWithoutScan`, `TestServer_ScanThenCleanup` |
| **Requirement status** | **COMPLETE** | All aspects implemented and tested |

## Test Gap Summary

Tests needed for Phase 11:

1. **TestServer_DisconnectDuringScan** -- Connect, start scan with blocking scanner, disconnect mid-scan, verify server recovers and accepts new connections
2. **TestServer_DisconnectDuringCleanup** -- Same pattern but for cleanup operation
3. **TestServer_IdleTimeout** -- Either make IdleTimeout configurable for tests, or use a very short timeout to verify connection closure after idle period
4. **TestServer_NoGoroutineLeakOnDisconnect** -- Verify goroutine count returns to baseline after client disconnect during active operation (optional; may be fragile)

## Design Decision: Idle Timeout Configurability

The current `IdleTimeout` is a package-level constant (5 minutes). To test it effectively, it needs to be configurable. Options:

1. **Server struct field with default** (recommended): Add `IdleTimeout time.Duration` to the Server struct, initialize to `DefaultIdleTimeout` in `New()`. Tests can override it. Clean, no new patterns needed.
2. **Functional options pattern**: `New(..., WithIdleTimeout(...))`. Over-engineering for a single option.
3. **Test-only build tag**: Complex, fragile.

Recommendation: Option 1 -- add `IdleTimeout` field to Server struct with a default value.

## Design Decision: ReadTimeout Disposition

`ReadTimeout = 30 * time.Second` is defined but never used. Options:

1. **Remove it** (recommended): The idle timeout handles abandoned connections. Per-message read timeouts are not needed for local IPC on Unix domain sockets.
2. **Apply it as a per-read timeout during handler execution**: Adds complexity for no benefit on local IPC.

Recommendation: Remove the dead constant.

## Swift Documentation Assessment

The current `docs/swift-integration.md` is comprehensive:
- Protocol format with request/response examples
- All five methods documented with JSON examples
- Swift Codable types for all request/response structures
- NWConnection example with basic NDJSON framing
- Lifecycle management guidance
- Error handling section
- Testing with socat examples

Potential enhancements (optional, not required by HARD requirements):
- **NDJSON buffering robustness:** The current Swift example uses `text.split(separator: "\n")` which doesn't handle partial messages correctly. A line-buffered approach (accumulate data, split on newlines, keep incomplete trailing data) would be more robust. See the Tim Weiss article pattern with buffer accumulation.
- **Reconnection guidance:** Document what happens if the server process exits unexpectedly and how the client should handle reconnection.
- **Timeout documentation:** Document the server's 5-minute idle timeout so Swift clients know to send periodic pings or handle reconnection after idle periods.

## Open Questions

1. **Should cleanup.Execute become context-aware?**
   - What we know: Currently continues file deletion to completion regardless of client disconnect. The engine goroutine exits cleanly after cleanup finishes.
   - Recommendation: Keep current behavior. Document it as intentional. File deletion should not be interrupted mid-stream to prevent partially-deleted state. The goroutine will exit naturally.

2. **How to test idle timeout without waiting 5 minutes?**
   - What we know: The timeout is a package-level constant.
   - Recommendation: Make `IdleTimeout` a configurable field on the Server struct with a default. Tests set it to 100ms or similar.

3. **Should the Swift docs be enhanced as part of Phase 11?**
   - What we know: The docs are already functional. The HARD requirements don't explicitly require doc changes, but the roadmap lists "Swift Codable types, NWConnection patterns" as a key deliverable.
   - Recommendation: Minor doc updates (add timeout/disconnect info, improve NDJSON buffering example) if time permits, but the existing docs are functional. Focus on hardening tests first.

## Sources

### Primary (HIGH confidence)
- **Codebase inspection** -- Direct reading of all files in `internal/server/` (7 files), `internal/engine/` (6 files), `internal/cleanup/cleanup.go`, `docs/swift-integration.md`
- **Test execution** -- All 16 packages pass (`go test ./...`)
- **REQUIREMENTS.md** -- Current status of all HARD requirements (pending)
- **STATE.md** -- Project state, accumulated context, patterns established

### Secondary (MEDIUM confidence)
- [Working with line-based sockets in Swift with Network.framework](https://timweiss.net/blog/2024-01-24-working-with-line-based-sockets-in-swift-with-network-framework/) -- NDJSON buffering pattern for Swift
- [Go net package docs](https://pkg.go.dev/net) -- SetReadDeadline, Unix socket behavior
- [uber-go/goleak](https://github.com/uber-go/goleak) -- Goroutine leak detection (considered but not recommended for this project)

### Tertiary (LOW confidence)
- [NWConnection Apple Developer Documentation](https://developer.apple.com/documentation/network/nwconnection) -- Could not extract content (JS-only page), relied on existing Swift docs in codebase

## Metadata

**Confidence breakdown:**
- HARD-01 analysis: HIGH -- Direct code reading of all handler paths, connection lifecycle, and engine context propagation
- HARD-02 analysis: HIGH -- Verified IdleTimeout is applied, ReadTimeout is dead code, Unix sockets don't support TCP keepalive
- HARD-03 analysis: HIGH -- Complete token lifecycle verified at both engine and server test levels
- Test gaps: HIGH -- Systematic comparison of requirement aspects vs existing test names
- Swift docs: MEDIUM -- Based on codebase reading + limited web research on NWConnection patterns

**Research date:** 2026-02-17
**Valid until:** 2026-03-17 (stable -- no external dependencies, internal Go code)
