# Phase 9: Protocol & Server Core - Research

**Researched:** 2026-02-17
**Domain:** Go Unix domain socket server with NDJSON protocol
**Confidence:** HIGH

## Summary

Phase 9's deliverables are already fully implemented. During Phase 8 (Engine Extraction), Plans 08-01 and 08-02 built the engine package and wired both the CLI and server to use it. The 08-02 summary explicitly states: "Phase 9 (Protocol & Server Core) targets are already implemented (server, protocol, handlers exist)" and "Phases 9-10 may focus on refinement/hardening rather than net-new implementation."

A systematic audit of each Phase 9 requirement against the current codebase confirms this:

- **PROTO-01 (NDJSON protocol with request IDs):** Fully implemented in `internal/server/protocol.go` -- Request/Response types, NDJSONReader/NDJSONWriter, method constants, all with request ID echoing. 9 unit tests in `protocol_test.go` cover encoding, decoding, round-trips, EOF, invalid JSON, and multi-message streams.
- **SRV-01 (UDS listener with graceful shutdown):** Fully implemented in `internal/server/server.go` -- `Serve()` with context cancellation, `Shutdown()` with done-channel coordination, active connection cleanup, listener close. 4 integration tests cover shutdown-via-method, context cancellation, and socket cleanup.
- **SRV-02 (serve cobra subcommand with --socket flag):** Fully implemented in `cmd/serve.go` -- `serve` subcommand with `--socket` flag (default `/tmp/mac-cleaner.sock`), SIGINT/SIGTERM handling, engine creation with `RegisterDefaults`.
- **SRV-04 (Socket cleanup and stale detection):** Fully implemented in `server.go` -- `cleanStaleSocket()` checks file type, probes for active listener, removes stale sockets. `cleanup()` removes socket on shutdown. Tests cover stale socket cleanup and non-socket file blocking.

All 20 server tests pass. All 170+ total project tests pass. The server is production-ready for Phase 9 requirements.

**Primary recommendation:** Phase 9 should be a verification-and-gap-analysis pass, not new implementation. The planner should create a single plan focused on: (1) auditing the existing code against each requirement, (2) identifying any gaps or missing edge-case tests, (3) adding any missing tests, and (4) updating requirement status to "complete". No new files need to be created.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net` | 1.25.7 | Unix domain socket listener (`net.Listen("unix", path)`) | Already in use; standard Go networking |
| Go stdlib `encoding/json` | 1.25.7 | NDJSON encoding/decoding | Already in use; json.Encoder/Decoder with newline framing |
| Go stdlib `bufio` | 1.25.7 | Line-oriented NDJSON reading | Already in use; bufio.Scanner for line-by-line reads |
| Go stdlib `sync` | 1.25.7 | NDJSONWriter mutex, server state guards | Already in use |
| Go stdlib `context` | 1.25.7 | Per-connection context, cancellation propagation | Already in use |
| Go stdlib `sync/atomic` | 1.25.7 | Busy flag for concurrent operation rejection | Already in use |
| cobra `v1.10.2` | 1.10.2 | CLI subcommand framework | Already in use for `serve` subcommand |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go stdlib `os/signal` | 1.25.7 | SIGINT/SIGTERM handler | Already in use in `cmd/serve.go` |
| Go stdlib `time` | 1.25.7 | Idle timeout, read deadline management | Already in use for connection management |
| `internal/engine` | (internal) | Scan/cleanup orchestration | Already injected into `server.New()` |

### Alternatives Considered

None -- all stack decisions were made and implemented during Phase 8. No changes needed.

**Installation:** No new dependencies needed. Everything is already in `go.mod`.

## Architecture Patterns

### Current Implementation Structure

```
internal/server/
  protocol.go          # Request/Response types, NDJSONReader/Writer, method constants
  handler.go           # Handler struct, Dispatch(), handlePing()
  handler_scan.go      # handleScan() with channel-based streaming, handleCategories()
  handler_cleanup.go   # handleCleanup() with token validation and streaming
  server.go            # Server struct, Serve(), Shutdown(), handleConnection(), stale socket cleanup
  protocol_test.go     # 9 unit tests for NDJSON encoding/decoding
  server_test.go       # 11 integration tests for server lifecycle, handlers, edge cases

cmd/
  serve.go             # serve subcommand with --socket flag, SIGINT/SIGTERM handling
```

### Pattern 1: NDJSON Protocol (Already Implemented)

**What:** Newline-delimited JSON with request IDs echoed in all responses.
**Implementation:** `NDJSONReader` uses `bufio.Scanner` for line-oriented reading (up to 1MB per line). `NDJSONWriter` wraps `json.Encoder` with mutex for concurrent safety. `Request` uses `json.RawMessage` for method-specific params (late binding). `Response` uses `any` for result payloads (flexible serialization).

```go
// Source: internal/server/protocol.go
type Request struct {
    ID     string          `json:"id"`
    Method string          `json:"method"`
    Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    ID     string `json:"id"`
    Type   string `json:"type"`
    Result any    `json:"result,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

### Pattern 2: Single-Connection Server with Per-Connection Context (Already Implemented)

**What:** Server handles one connection at a time. Each connection gets a derived context that is cancelled on disconnect, allowing long-running handlers to abort cleanly.
**Implementation:** `handleConnection()` creates `context.WithCancel(parentCtx)`, stores the cancel function in `s.connCancel` for shutdown to invoke. The connection loop checks `connCtx.Done()` and `s.done` on each iteration.

### Pattern 3: Method Dispatch (Already Implemented)

**What:** `Handler.Dispatch()` routes requests to method-specific handlers via switch statement.
**Implementation:** The switch handles `ping`, `scan`, `cleanup`, `categories` as named methods. The `shutdown` method is handled directly in the connection loop (before dispatch) because it needs to call `s.Shutdown()`.

### Pattern 4: Stale Socket Detection (Already Implemented)

**What:** On startup, the server checks for leftover socket files from crashed instances.
**Implementation:** `cleanStaleSocket()` uses `os.Lstat` to check file type (socket vs. regular), then `net.Dial` to probe for an active listener. If the socket exists but no one is listening, it removes it. If another server is already listening, it returns an error.

### Anti-Patterns to Avoid

- **Multiple connections:** The server is deliberately single-connection. Do not add connection pooling.
- **Shutdown from inside a handler:** The `shutdown` method is handled in the connection loop, not in `Handler.Dispatch()`. This is correct -- shutdown needs to close the listener.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| NDJSON framing | Custom line parser | `bufio.Scanner` + `json.Unmarshal` | Already implemented correctly with 1MB buffer limit |
| Socket cleanup | Custom file watcher | `os.Lstat` + `net.Dial` probe | Already implemented with proper type checking |
| Concurrent write safety | Raw mutex around conn.Write | `NDJSONWriter` with internal mutex | Already implemented, all handlers share one writer |
| Graceful shutdown | Custom signal handling | `context.WithCancel` + `signal.Notify` | Already implemented in server.go and serve.go |

**Key insight:** Nothing needs to be hand-rolled. All Phase 9 infrastructure exists.

## Common Pitfalls

### Pitfall 1: Thinking This Phase Requires New Code

**What goes wrong:** Creating duplicate implementations of code that already exists.
**Why it happens:** The roadmap was written before Phase 8 was implemented. Phase 8 proactively built the server/protocol infrastructure as part of wiring the engine.
**How to avoid:** Audit existing code against requirements first. The Phase 8 summaries explicitly note that Phase 9 targets are already implemented.
**Warning signs:** Creating new files in `internal/server/` that duplicate existing functionality.

### Pitfall 2: Missing Test Coverage Gaps

**What goes wrong:** Declaring requirements "done" without verifying edge-case test coverage.
**Why it happens:** The implementation exists, but some edge cases might not be tested.
**How to avoid:** For each requirement, verify both implementation AND test coverage. Key areas to check:
- Malformed JSON requests (tested via `TestNDJSONReader_InvalidJSON`)
- Empty object requests (tested via `TestNDJSONReader_EmptyObject`)
- Unknown method handling (tested via `TestServer_UnknownMethod`)
- Client disconnect during idle (tested via `TestServer_ClientDisconnectHandledGracefully`)
- Non-socket file at socket path (tested via `TestServer_NonSocketFileBlocks`)
**Warning signs:** Requirements marked complete without corresponding test references.

### Pitfall 3: Stale Socket Test Platform Dependency

**What goes wrong:** `TestServer_StaleSocketCleanup` is SKIPped on macOS because `net.Listener.Close()` removes the socket file.
**Why it happens:** macOS (unlike Linux) cleans up UDS files when the listener closes, so you cannot create a "stale" socket in the test.
**How to avoid:** This is a known limitation. The stale socket code is still exercised by the `cleanStaleSocket()` logic -- it just cannot be integration-tested end-to-end on macOS. The `TestServer_NonSocketFileBlocks` test covers the "file exists but is not a socket" path. Consider adding a unit test for `cleanStaleSocket` with a mock file to cover the remaining paths.
**Warning signs:** CI on Linux passes the stale socket test but it is skipped on macOS.

### Pitfall 4: Read Deadline Handling Edge Cases

**What goes wrong:** Connection deadlines interact poorly with long-running operations.
**Why it happens:** The server sets `IdleTimeout` (5min) on read deadline before each read, then clears it after reading. If a long-running scan takes more than 5 minutes and the client sends no messages, the idle timeout would not trigger because the server is not reading during the operation.
**How to avoid:** This is actually correct behavior -- during a scan/cleanup, the server is streaming progress events (writes), not waiting for reads. The idle timeout only applies between request-response cycles.
**Warning signs:** Connection drops during long scans (this would indicate a bug, not a design issue).

## Code Examples

All code examples reference existing implemented code:

### Server Creation and Startup
```go
// Source: cmd/serve.go
eng := engine.New()
engine.RegisterDefaults(eng)
srv := server.New(flagSocket, version, eng)

go func() {
    <-sigCh
    fmt.Fprintln(os.Stderr, "\nShutting down...")
    srv.Shutdown()
    cancel()
}()

fmt.Fprintf(os.Stderr, "Listening on %s\n", flagSocket)
return srv.Serve(ctx)
```

### Ping Handler
```go
// Source: internal/server/handler.go
func (h *Handler) handlePing(req Request, w *NDJSONWriter) {
    _ = w.WriteResult(req.ID, PingResult{
        Status:  "ok",
        Version: h.server.version,
    })
}
```

### Manual Testing with socat
```bash
# Start server
mac-cleaner serve --socket /tmp/mc.sock &

# Connect and send ping
echo '{"id":"1","method":"ping"}' | socat - UNIX-CONNECT:/tmp/mc.sock
# Expected: {"id":"1","type":"result","result":{"status":"ok","version":"dev"}}

# Shutdown
echo '{"id":"2","method":"shutdown"}' | socat - UNIX-CONNECT:/tmp/mc.sock
```

## State of the Art

| Roadmap Expectation | Current State | Gap |
|---------------------|---------------|-----|
| `internal/server/protocol.go` with Request/Response types | EXISTS: 138 lines, Request, Response, NDJSONReader, NDJSONWriter, ScanParams, CleanupParams, PingResult | NONE |
| `internal/server/server.go` with socket listener, dispatch, shutdown | EXISTS: 218 lines, Serve(), Shutdown(), handleConnection(), cleanStaleSocket(), cleanup() | NONE |
| `cmd/serve.go` with --socket flag | EXISTS: 49 lines, serve subcommand, SIGINT/SIGTERM, engine creation | NONE |
| Tests for protocol encoding | EXISTS: 170 lines in protocol_test.go, 9 tests | NONE |
| Tests for server lifecycle | EXISTS: 480 lines in server_test.go, 11 tests | MINOR: stale socket test skipped on macOS |
| Ping method responds | EXISTS: handlePing in handler.go + TestServer_PingIntegration | NONE |
| SIGINT/SIGTERM graceful shutdown | EXISTS: signal handler in serve.go + srv.Shutdown() | NONE |
| Stale socket detection and cleanup | EXISTS: cleanStaleSocket() + cleanup() | NONE |

## Requirement Coverage Analysis

### PROTO-01: NDJSON request/response protocol with request IDs

| Aspect | Status | Evidence |
|--------|--------|----------|
| Request type with id/method/params | DONE | `protocol.go:24-31` |
| Response type with id/type/result/error | DONE | `protocol.go:34-43` |
| NDJSON encoding (newline-delimited) | DONE | `NDJSONWriter.Write()` uses `json.Encoder.Encode()` which appends newline |
| NDJSON reading (line-oriented) | DONE | `NDJSONReader.Read()` uses `bufio.Scanner` |
| Request ID echoed in responses | DONE | All handler methods pass `req.ID` to writer |
| Method constants defined | DONE | `protocol.go:15-21`: ping, shutdown, scan, cleanup, categories |
| Unit tests | DONE | 9 tests in `protocol_test.go` |

### SRV-01: Unix domain socket listener with graceful shutdown

| Aspect | Status | Evidence |
|--------|--------|----------|
| UDS listener | DONE | `server.go:74`: `net.Listen("unix", s.socketPath)` |
| Accept loop | DONE | `server.go:92-108`: for loop with `ln.Accept()` |
| Context cancellation stops server | DONE | `server.go:84-90`: goroutine closes listener on ctx.Done() |
| Shutdown() method | DONE | `server.go:111-129`: closes done channel, listener, active connection |
| Clean exit on done channel | DONE | `server.go:95-97`: accept error with done check returns nil |
| Test: context cancellation | DONE | `TestServer_ContextCancellation` |
| Test: shutdown via method | DONE | `TestServer_ShutdownViaMethod` |

### SRV-02: serve cobra subcommand with --socket flag

| Aspect | Status | Evidence |
|--------|--------|----------|
| serve subcommand | DONE | `cmd/serve.go:18-44`: `serveCmd` with `RunE` |
| --socket flag with default | DONE | `cmd/serve.go:47`: default `/tmp/mac-cleaner.sock` |
| SIGINT/SIGTERM handling | DONE | `cmd/serve.go:27-39`: signal.Notify, srv.Shutdown(), cancel() |
| Engine creation | DONE | `cmd/serve.go:30-32`: engine.New() + RegisterDefaults |
| Added to rootCmd | DONE | `cmd/serve.go:48`: `rootCmd.AddCommand(serveCmd)` |

### SRV-04: Socket file cleanup on shutdown and stale socket detection

| Aspect | Status | Evidence |
|--------|--------|----------|
| Socket removed on shutdown | DONE | `server.go:81,216-218`: `defer s.cleanup()` in Serve() |
| Stale socket detection | DONE | `server.go:187-213`: Lstat + type check + dial probe |
| Active server detection | DONE | `server.go:202-206`: dial succeeds = another server running |
| Non-socket file rejection | DONE | `server.go:197-199`: type check returns error |
| Test: stale socket | EXISTS (macOS-skipped) | `TestServer_StaleSocketCleanup` |
| Test: non-socket file | DONE | `TestServer_NonSocketFileBlocks` |
| Test: socket removed after shutdown | DONE | `TestServer_ShutdownViaMethod` checks `os.IsNotExist` |

## Open Questions

1. **Should the planner create tasks for this phase at all?**
   - What we know: All requirements are fully implemented and tested.
   - What's unclear: Whether the user expects Phase 9 to be a verification pass or if they want to skip it entirely.
   - Recommendation: Create a single lightweight plan with verification tasks that audit code against requirements, check for any missing edge-case tests, and update REQUIREMENTS.md status. This documents the work formally and ensures no gaps are overlooked.

2. **Should the macOS-skipped stale socket test be supplemented?**
   - What we know: `TestServer_StaleSocketCleanup` skips on macOS because the platform removes socket files on `Close()`. The `cleanStaleSocket()` function has 4 code paths: (a) no file exists, (b) file is not a socket, (c) another server is listening, (d) stale socket removed.
   - What's unclear: Whether paths (a), (c), (d) need dedicated unit tests beyond the integration test.
   - Recommendation: Path (a) is trivially covered (early return). Path (b) is covered by `TestServer_NonSocketFileBlocks`. Path (c) could be tested by starting a listener in the test, but the integration test already covers it via the "another server already listening" error path. Path (d) is the one that's macOS-skipped. A unit test that creates a socket file directly (via `os.MkdirTemp` + manual socket creation) could supplement this, but the value is marginal.

3. **Should Phase 9 update REQUIREMENTS.md status?**
   - What we know: REQUIREMENTS.md lists all PROTO-01, SRV-01, SRV-02, SRV-04 as "pending".
   - Recommendation: Yes, the verification plan should include updating requirement status.

## Sources

### Primary (HIGH confidence)

- **Codebase inspection** -- Direct reading of all files in `internal/server/` (7 files, 1257 lines), `cmd/serve.go` (49 lines), `internal/engine/` (6 files), `cmd/root.go`
- **Phase 8 summaries** -- `08-01-SUMMARY.md` and `08-02-SUMMARY.md` document what was built and their explicit notes about Phase 9 readiness
- **Test execution** -- All 20 server tests pass (`go test ./internal/server/... -v`), all 170+ project tests pass (`go test ./...`)
- **Go stdlib docs** -- `net` package UDS support, `encoding/json` Encoder/Decoder, `bufio.Scanner` for NDJSON

### Secondary (MEDIUM confidence)

- **docs/swift-integration.md** -- Protocol documentation matches implementation (verified by cross-referencing JSON examples against Response/Request types)

### Tertiary (LOW confidence)

- None -- all findings verified against source code

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies needed; everything exists
- Architecture: HIGH -- Direct source code reading of all 7 server files + serve.go
- Pitfalls: HIGH -- Based on existing test coverage analysis and known platform-specific behavior (macOS socket cleanup)
- Requirements coverage: HIGH -- Every requirement aspect verified against specific file:line evidence

**Research date:** 2026-02-17
**Valid until:** 2026-03-17 (stable -- no external dependencies, internal Go code)
