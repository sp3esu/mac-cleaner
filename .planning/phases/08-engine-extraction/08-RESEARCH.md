# Phase 8: Engine Extraction - Research

**Researched:** 2026-02-16
**Domain:** Go refactoring -- extracting orchestration from CLI into reusable engine package
**Confidence:** HIGH

## Summary

Phase 8 is a structural refactoring within an existing Go codebase. The `internal/engine/` package already exists with basic `Scanner` struct, `ScanAll()`, and `FilterSkipped()` functions. The current `cmd/root.go` partially delegates to the engine for interactive mode but still directly calls individual `pkg/*/Scan()` functions for flag-based scanning (lines 102-125). The phase must evolve the engine to support: a `Scanner` interface (replacing the current struct), rich metadata via `ScannerInfo`, context-aware cancellation, channel-based streaming, cleanup orchestration with token-based replay protection, and per-scanner `Run()` for granular server control. Then `cmd/root.go` must be refactored to delegate all scanning/cleanup through the engine.

The codebase is a standard Go project (go1.25.7, cobra CLI, stdlib testing) with 6 scanner packages under `pkg/`, each exporting `Scan() ([]scan.CategoryResult, error)`. The `internal/server/` package already consumes the engine for IPC scan/cleanup operations with context cancellation and busy-flag concurrency control. The refactoring must preserve both the CLI's exact output and the server's existing integration patterns.

**Primary recommendation:** Evolve the existing `internal/engine/` package incrementally -- first add the `Scanner` interface and `ScannerInfo` metadata alongside the current struct, migrate `DefaultScanners()` to return interface implementations, add `context.Context` support and channel-based streaming, then add cleanup orchestration with token validation. Wire `cmd/root.go` last, verifying with golden output comparison.

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Engine API surface:**
- Expose both ScanAll() and individual per-scanner Run() -- ScanAll for convenience, per-scanner for granular server control
- Progress callbacks at two granularities: per-scanner start/finish AND per-entry (file/directory found)
- Channel-based streaming: ScanAll returns a channel, results arrive as each scanner completes
- Context-aware: ScanAll takes context.Context for cancellation support (essential for server disconnect handling)
- Custom error types: Engine-specific errors (ScanError, CancelledError) for typed error handling by the server
- Unified streaming pattern: Both scan and cleanup stream through the same callback/channel pattern

**Scanner registration:**
- Registry pattern with explicit Register() calls -- extensible but no init() magic
- Central DefaultScanners() function that explicitly registers all scanners -- clear, easy to audit
- Rich metadata per scanner: name, category ID, description, risk level -- enables the server 'categories' method without extra mapping
- Skip filtering at scan time, not registration time -- all scanners always registered, skip set checked when ScanAll runs

**Cleanup orchestration:**
- Engine owns both scan and cleanup -- single package orchestrates the full workflow
- Engine enforces confirmation: requires a confirmation token or flag to prevent accidental cleanup calls
- Scan-result token: ScanAll returns a token/ID, Cleanup requires that token -- replay protection built in
- Partial selection: Cleanup(token, categoryIDs) -- clean only selected categories from a scan
- Cleanup package (internal/cleanup) stays separate -- engine wraps it, cleanup stays focused on file deletion

**Migration strategy:**
- Two plans: Plan 1 creates engine package with tests (mock scanners), Plan 2 wires cmd/root.go to use engine and verifies golden output
- Golden output comparison: capture current CLI output before refactor, compare after -- exact match = success
- Golden files stored in repo (testdata/) as committed regression tests for ongoing protection
- Scanner defined as Go interface type: `type Scanner interface { Scan() ([]CategoryResult, error); Info() ScannerInfo }` -- enables rich metadata and mockability

### Claude's Discretion

- Scanner concurrency model (sequential vs. concurrent with limit) -- pick based on performance vs. complexity
- Engine struct design (struct fields vs. per-call arguments for config) -- pick what works best for both CLI and server
- Exact channel types and streaming API shape

### Deferred Ideas (OUT OF SCOPE)

None -- discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.25.7 | context, errors, sync, channels, testing | Already in use; all engine features use stdlib primitives |
| `internal/scan` | (internal) | `CategoryResult`, `ScanEntry`, `ScanSummary`, `FormatSize` | Existing shared types -- engine produces these |
| `internal/cleanup` | (internal) | `Execute()`, `CleanupResult`, `ProgressFunc` | Existing deletion engine -- engine wraps it |
| `internal/safety` | (internal) | `RiskForCategory`, risk constants | Existing risk classification -- `ScannerInfo` references these |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/rand` | stdlib | Generate scan tokens | Token generation for replay protection |
| `encoding/hex` | stdlib | Encode tokens to strings | Human-readable token format |
| `errors` | stdlib | Custom error types with `errors.Is`/`errors.As` | `ScanError`, `CancelledError` type checking |
| `sync` | stdlib | `sync.Mutex` for token store | Protect concurrent token access |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `crypto/rand` for tokens | UUID library | Unnecessary dependency; 16 random bytes as hex is sufficient |
| Channel-based streaming | Callback-only API | Channels compose better with `select` for cancellation; user decided channels |
| Custom error types | Error string matching | Types enable `errors.As()` which is idiomatic Go and type-safe |

**Installation:** No new dependencies needed. All requirements are met by Go stdlib and existing internal packages.

## Architecture Patterns

### Current State (What Exists)

```
internal/engine/
  engine.go          -- Scanner struct (not interface), ScanAll(), FilterSkipped()
  engine_test.go     -- 14 tests with mock scanners

cmd/root.go          -- Directly calls pkg/*/Scan() for flag-based mode
                     -- Uses engine.ScanAll() only for interactive (no-flags) mode
                     -- Owns all cleanup orchestration
                     -- 706 lines, tightly coupled to cobra flags

internal/server/
  handler_scan.go    -- Calls engine.ScanAll() + engine.DefaultScanners()
  handler_cleanup.go -- Calls cleanup.Execute() directly, manages lastScan state
```

### Target State (After Phase 8)

```
internal/engine/
  engine.go          -- Engine struct, ScanAll(), Cleanup(), Run()
  scanner.go         -- Scanner interface, ScannerInfo, scannerAdapter
  registry.go        -- Register(), DefaultScanners(), Categories()
  token.go           -- ScanToken generation and validation
  errors.go          -- ScanError, CancelledError types
  engine_test.go     -- Comprehensive tests with mock scanners

cmd/root.go          -- Delegates ALL scanning to engine
                     -- Delegates ALL cleanup to engine
                     -- Only owns: flag parsing, UI rendering, user interaction
```

### Recommended Project Structure

```
internal/engine/
  engine.go       # Engine struct with ScanAll(), Run(), Cleanup() methods
  scanner.go      # Scanner interface, ScannerInfo struct, adapter for pkg/* functions
  registry.go     # Register(), DefaultScanners(), Categories()
  token.go        # ScanToken type, generation, validation store
  errors.go       # ScanError, CancelledError, custom error types
  engine_test.go  # All engine tests (existing + new)
```

### Pattern 1: Scanner Interface with Adapter

**What:** Define a Scanner interface and adapt existing `pkg/*/Scan()` functions to it.
**When to use:** Every scanner registration.
**Example:**
```go
// scanner.go

// ScannerInfo holds metadata about a scanner.
type ScannerInfo struct {
    ID          string // e.g. "system", "browser"
    Name        string // e.g. "System Caches"
    Description string // e.g. "User caches, logs, and QuickLook thumbnails"
    CategoryIDs []string // e.g. ["system-caches", "system-logs", "quicklook"]
    RiskLevel   string // dominant risk level for the group
}

// Scanner is the interface all scanners implement.
type Scanner interface {
    Scan() ([]scan.CategoryResult, error)
    Info() ScannerInfo
}

// scannerAdapter wraps a bare Scan function into the Scanner interface.
type scannerAdapter struct {
    info   ScannerInfo
    scanFn func() ([]scan.CategoryResult, error)
}

func (a *scannerAdapter) Scan() ([]scan.CategoryResult, error) { return a.scanFn() }
func (a *scannerAdapter) Info() ScannerInfo                     { return a.info }

// NewScanner creates a Scanner from a function and metadata.
func NewScanner(info ScannerInfo, fn func() ([]scan.CategoryResult, error)) Scanner {
    return &scannerAdapter{info: info, scanFn: fn}
}
```

### Pattern 2: Channel-Based Streaming with Context

**What:** `ScanAll` returns a channel of `ScanEvent` values and a final results channel.
**When to use:** When callers need to process results as they arrive and support cancellation.
**Example:**
```go
// ScanResult holds the final aggregated output of ScanAll.
type ScanResult struct {
    Results []scan.CategoryResult
    Token   ScanToken
}

// ScanAll runs all registered scanners, streaming events through the returned channel.
// The channel is closed when all scanners complete or the context is cancelled.
// Returns a result channel that receives exactly one ScanResult when complete.
func (e *Engine) ScanAll(ctx context.Context, skip map[string]bool) (<-chan ScanEvent, <-chan ScanResult) {
    events := make(chan ScanEvent)
    done := make(chan ScanResult, 1)

    go func() {
        defer close(events)
        defer close(done)

        var all []scan.CategoryResult
        for _, s := range e.scanners {
            if ctx.Err() != nil {
                return // cancelled
            }

            info := s.Info()
            select {
            case events <- ScanEvent{Type: EventScannerStart, ScannerID: info.ID, Label: info.Name}:
            case <-ctx.Done():
                return
            }

            results, err := s.Scan()
            if err != nil {
                select {
                case events <- ScanEvent{Type: EventScannerError, ScannerID: info.ID, Label: info.Name, Err: err}:
                case <-ctx.Done():
                    return
                }
                continue
            }

            select {
            case events <- ScanEvent{Type: EventScannerDone, ScannerID: info.ID, Label: info.Name, Results: results}:
            case <-ctx.Done():
                return
            }
            all = append(all, results...)
        }

        filtered := FilterSkipped(all, skip)
        token := e.storeResults(filtered)
        done <- ScanResult{Results: filtered, Token: token}
    }()

    return events, done
}
```

### Pattern 3: Token-Based Cleanup Validation

**What:** Scan produces a token; Cleanup requires it as proof the caller owns valid scan results.
**When to use:** Every cleanup call, preventing replays.
**Example:**
```go
// token.go

// ScanToken is an opaque identifier linking a cleanup to a prior scan.
type ScanToken string

// tokenEntry stores scan results keyed by token.
type tokenEntry struct {
    results []scan.CategoryResult
    created time.Time
}

// storeResults saves results and returns a new token.
func (e *Engine) storeResults(results []scan.CategoryResult) ScanToken {
    b := make([]byte, 16)
    _, _ = rand.Read(b) // crypto/rand never errors on 16 bytes in practice
    token := ScanToken(hex.EncodeToString(b))

    e.mu.Lock()
    e.tokens[token] = tokenEntry{results: results, created: time.Now()}
    e.mu.Unlock()
    return token
}

// Cleanup removes files for the given categories from a prior scan.
// The token must match a prior ScanAll call. It is consumed (one-time use).
func (e *Engine) Cleanup(ctx context.Context, token ScanToken, categoryIDs []string) (<-chan CleanupEvent, <-chan cleanup.CleanupResult) {
    // ... validate token, filter categories, delegate to cleanup.Execute()
}
```

### Pattern 4: Engine Struct with Configuration

**Recommendation (Claude's Discretion):** Use an `Engine` struct rather than package-level functions. The struct holds the scanner registry, token store, and configuration. This works naturally for both CLI (create once, use during command execution) and server (create once in `server.New()`, reuse across requests).

```go
// engine.go

type Engine struct {
    scanners []Scanner
    mu       sync.Mutex
    tokens   map[ScanToken]tokenEntry
}

func New() *Engine {
    return &Engine{
        tokens: make(map[ScanToken]tokenEntry),
    }
}

func (e *Engine) Register(s Scanner) {
    e.scanners = append(e.scanners, s)
}

func (e *Engine) Categories() []ScannerInfo {
    infos := make([]ScannerInfo, len(e.scanners))
    for i, s := range e.scanners {
        infos[i] = s.Info()
    }
    return infos
}
```

### Pattern 5: Custom Error Types

**What:** Engine-specific errors for typed handling.
**Example:**
```go
// errors.go

// ScanError wraps a scanner-level error with the scanner ID.
type ScanError struct {
    ScannerID string
    Err       error
}

func (e *ScanError) Error() string { return fmt.Sprintf("scanner %s: %v", e.ScannerID, e.Err) }
func (e *ScanError) Unwrap() error { return e.Err }

// CancelledError indicates the operation was cancelled via context.
type CancelledError struct {
    Operation string // "scan" or "cleanup"
}

func (e *CancelledError) Error() string { return fmt.Sprintf("%s cancelled", e.Operation) }

// TokenError indicates an invalid or expired scan token.
type TokenError struct {
    Token ScanToken
    Reason string
}

func (e *TokenError) Error() string { return fmt.Sprintf("invalid token %s: %s", e.Token, e.Reason) }
```

### Pattern 6: Concurrency Model (Claude's Discretion)

**Recommendation: Sequential execution** for Phase 8. Reasons:

1. Current behavior is sequential -- zero behavior change is a requirement
2. Scanners hit the local filesystem; concurrent I/O on HDD gives no speedup, and even on SSD the benefit is marginal for 6 scanners
3. The server already has `busy` atomic to prevent concurrent operations
4. Sequential is simpler to debug, test, and reason about
5. Concurrency can be added later as an option (`Engine.SetConcurrency(n)`) without API changes

The channel-based API already supports concurrent consumption -- a future goroutine pool just changes the internal implementation, not the external API.

### Anti-Patterns to Avoid

- **Breaking the Scanner function signature:** Each `pkg/*/Scan()` returns `([]scan.CategoryResult, error)`. Do NOT modify these. The adapter pattern wraps them.
- **Engine importing cobra:** The engine package must have zero cobra dependency. All flag parsing stays in `cmd/`.
- **Engine doing UI:** No `fmt.Print*` to stdout/stderr in engine code. All output goes through events/channels/callbacks.
- **Modifying cleanup.Execute signature:** The `internal/cleanup/` package stays unchanged. Engine wraps it.
- **init() magic for scanner registration:** User explicitly decided against this. Use `DefaultScanners()` with explicit `Register()` calls.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token generation | Custom ID scheme | `crypto/rand` + `hex.EncodeToString` | Cryptographically random, no collisions |
| Error type checking | String matching on `err.Error()` | `errors.As()` with custom types | Idiomatic Go, compile-time safe |
| Concurrent channel management | Raw goroutine + channel logic | Structured pattern with `defer close()` | Prevents goroutine leaks |
| Result filtering | Re-implementing FilterSkipped | Reuse existing `engine.FilterSkipped()` | Already tested with 5 test cases |

**Key insight:** This phase is a refactoring, not new functionality. Maximize reuse of existing tested code (FilterSkipped, cleanup.Execute, safety.RiskForCategory). The engine is a new composition layer, not new business logic.

## Common Pitfalls

### Pitfall 1: Channel Send Blocking on Cancelled Context
**What goes wrong:** `events <- ScanEvent{...}` blocks forever if nobody reads the channel after context cancellation.
**Why it happens:** When ctx is cancelled, the consumer stops reading, but the producer still tries to send.
**How to avoid:** Always use `select` with `ctx.Done()` on every channel send:
```go
select {
case events <- event:
case <-ctx.Done():
    return
}
```
**Warning signs:** Test hangs, goroutine leak in `go test -race`.

### Pitfall 2: Golden Output Includes Color Codes
**What goes wrong:** Golden file comparison fails because ANSI escape codes differ between environments.
**Why it happens:** The `fatih/color` package emits ANSI codes unless `color.NoColor = true`.
**How to avoid:** Set `color.NoColor = true` before capturing golden output. The existing test suite already does this pattern consistently.
**Warning signs:** Tests pass locally but fail in CI, or vice versa.

### Pitfall 3: Token Store Memory Leak
**What goes wrong:** Tokens accumulate if scans happen without cleanup (e.g., repeated scan-only calls from the server).
**Why it happens:** Tokens are stored but never expired.
**How to avoid:** Implement a simple expiry (e.g., 1 hour) or a max-tokens limit. On new scan, clear previous tokens. For the CLI use case (single scan-cleanup cycle), this is not a concern, but the server can scan repeatedly.
**Warning signs:** Server process memory grows over time.

### Pitfall 4: Breaking cmd/root.go Flag-Based Scan Flow
**What goes wrong:** Flag-based scanning (`--system-caches`, `--browser-data`, etc.) behaves differently after refactoring.
**Why it happens:** The current flag-based flow calls individual scanners directly (lines 102-125 of root.go) and prints results between scanner calls. The engine's `ScanAll()` runs all scanners together.
**How to avoid:** For flag-based mode, use the engine's per-scanner `Run()` method to maintain the one-at-a-time flow with interleaved printing. Or use `ScanAll()` with a skip set that excludes unselected scanner groups. Golden output tests catch any divergence.
**Warning signs:** Output order changes, missing intermediate headers.

### Pitfall 5: Server Handler Coupling to Engine Internals
**What goes wrong:** `internal/server/handler_scan.go` currently calls `engine.ScanAll()` and `engine.DefaultScanners()` as package-level functions. After refactoring to an `Engine` struct, the server needs an engine instance.
**Why it happens:** The server currently uses the engine as a bag of functions, not as a struct.
**How to avoid:** Pass an `*Engine` instance to `server.New()` or `Handler`. This is a natural extension point -- the server already takes a `*Server` struct.
**Warning signs:** Compile errors in `internal/server/` after engine changes.

### Pitfall 6: Race Condition on Token Store
**What goes wrong:** Concurrent server requests (scan + cleanup) race on the token map.
**Why it happens:** The server's `busy` atomic prevents concurrent operations today, but the engine should be independently safe.
**How to avoid:** Use `sync.Mutex` around all token map operations. The engine must be safe for concurrent use even if callers currently serialize.
**Warning signs:** `-race` detector flags.

## Code Examples

### Existing Pattern: How cmd/root.go Currently Uses Engine

```go
// Source: cmd/root.go lines 448-464
func scanAll(sp *spinner.Spinner) []scan.CategoryResult {
    return engine.ScanAll(engine.DefaultScanners(), nil, func(e engine.ScanEvent) {
        switch e.Type {
        case engine.EventScannerStart:
            sp.UpdateMessage("Scanning " + strings.ToLower(e.Label) + "...")
            sp.Start()
        case engine.EventScannerDone:
            sp.Stop()
            if len(e.Results) > 0 {
                printResults(e.Results, true, e.Label)
            }
        case engine.EventScannerError:
            sp.Stop()
            fmt.Fprintf(os.Stderr, "Warning: %v\n", e.Err)
        }
    })
}
```

### Existing Pattern: How Server Uses Engine

```go
// Source: internal/server/handler_scan.go lines 62-84
results := engine.ScanAll(engine.DefaultScanners(), skip, func(e engine.ScanEvent) {
    if ctx.Err() != nil {
        return
    }
    // ... stream progress to client
})
```

### Existing Pattern: How Server Manages Cleanup Validation

```go
// Source: internal/server/handler_cleanup.go lines 50-74
lastResults := h.server.lastScan.results.Load()
if lastResults == nil {
    _ = w.WriteErrorMsg(req.ID, "no prior scan results; run scan first")
    return
}
// ... filter by category, call cleanup.Execute()
// After cleanup:
h.server.lastScan.results.Store(nil) // consume token (prevent replay)
```

### Target Pattern: Engine Construction and Usage (CLI)

```go
// In cmd/root.go (after refactoring)
eng := engine.New()
engine.RegisterDefaults(eng) // registers all 6 scanner groups

// Interactive mode:
events, done := eng.ScanAll(context.Background(), buildSkipSet())
for event := range events {
    // drive spinner, print results
}
result := <-done

// Cleanup:
cleanEvents, cleanDone := eng.Cleanup(context.Background(), result.Token, selectedCategoryIDs)
for event := range cleanEvents {
    // drive spinner/progress
}
cleanResult := <-cleanDone
```

### Target Pattern: Engine Construction and Usage (Server)

```go
// In internal/server/server.go
func New(socketPath, version string) *Server {
    eng := engine.New()
    engine.RegisterDefaults(eng)
    s := &Server{
        socketPath: socketPath,
        version:    version,
        engine:     eng,
        done:       make(chan struct{}),
    }
    s.handler = NewHandler(s)
    return s
}
```

### Golden File Test Pattern

```go
// Source: Go testing conventions for golden files

var update = flag.Bool("update", false, "update golden files")

func TestCLI_DryRunAll_GoldenOutput(t *testing.T) {
    color.NoColor = true
    defer func() { color.NoColor = false }()

    // Run the CLI command and capture output
    actual := captureOutput(t, func() {
        // ... execute scan with mock/deterministic data
    })

    golden := filepath.Join("testdata", t.Name()+".golden")
    if *update {
        os.MkdirAll("testdata", 0o755)
        os.WriteFile(golden, []byte(actual), 0o644)
        return
    }

    expected, err := os.ReadFile(golden)
    if err != nil {
        t.Fatalf("read golden file: %v (run with -update to create)", err)
    }
    if actual != string(expected) {
        t.Errorf("output mismatch\n--- golden ---\n%s\n--- actual ---\n%s", expected, actual)
    }
}
```

## State of the Art

| Old Approach (Current) | New Approach (Phase 8) | Impact |
|-------------------------|------------------------|--------|
| `Scanner` is a struct with `ScanFn` field | `Scanner` is an interface with `Scan()` + `Info()` | Enables mocking, richer metadata |
| `ScanAll()` is a package-level function | `Engine.ScanAll()` is a method on `*Engine` struct | Holds state (registry, tokens) |
| Progress via callback `ScanProgressFunc` | Progress via `<-chan ScanEvent` | Composes with `select` for cancellation |
| Server manages scan results as `atomic.Pointer` | Engine manages tokens with validation | Replay protection built into engine |
| `cmd/root.go` calls `pkg/*/Scan()` directly for flag-based mode | `cmd/root.go` uses `engine.Run()` for all scanning | Engine is sole orchestrator |
| No context support in `ScanAll()` | `context.Context` as first parameter | Essential for server disconnect handling |

**Backward compatibility:** The existing `engine.Scanner` struct, `engine.ScanAll()` package-level function, and `engine.FilterSkipped()` must be removed and replaced. This is an internal package, so no external consumers exist. The `internal/server/` package must be updated to use the new `*Engine` struct API.

## Open Questions

1. **Channel API for cleanup: one channel or two?**
   - What we know: Scan uses events+done pattern (two channels). Cleanup also needs progress streaming.
   - What's unclear: Whether cleanup needs a separate events channel or if a callback is sufficient given that cleanup.Execute already uses a ProgressFunc internally.
   - Recommendation: Use the same two-channel pattern for consistency. The decision says "unified streaming pattern."

2. **Should Run() (per-scanner) also return channels?**
   - What we know: Run() executes a single scanner. It is simpler than ScanAll().
   - What's unclear: Whether a channel is overkill for a single scanner call that returns one result.
   - Recommendation: `Run()` returns `([]scan.CategoryResult, error)` directly (synchronous). Channels add complexity without benefit for single-scanner execution. The per-entry progress callback can be added later if needed.

3. **Token expiry policy**
   - What we know: Server can call scan many times without cleanup.
   - What's unclear: Exact TTL value, max stored tokens.
   - Recommendation: Keep it simple -- store at most 1 token (new scan invalidates previous). This matches the server's current behavior (single `lastScan` atomic pointer). Add TTL later if needed.

## Sources

### Primary (HIGH confidence)
- **Codebase inspection** -- Read all source files in cmd/, internal/engine/, internal/server/, internal/cleanup/, internal/scan/, internal/safety/, pkg/system/, pkg/browser/
- **Go official docs** -- context package patterns, errors.As/Is, channel idioms, sync.Mutex usage
- **Existing tests** -- 14 engine tests + 30+ cmd/root_test.go tests confirm current behavior

### Secondary (MEDIUM confidence)
- [Go Concurrency Patterns: Context](https://go.dev/blog/context) -- Official Go blog on context patterns
- [Go Interfaces: Design Patterns & Best Practices](https://blog.marcnuri.com/go-interfaces-design-patterns-and-best-practices) -- Interface design guidance
- [Testing with Golden Files in Go](https://medium.com/soon-london/testing-with-golden-files-in-go-7fccc71c43d3) -- Golden test pattern reference
- [File-driven testing in Go](https://eli.thegreenplace.net/2022/file-driven-testing-in-go/) -- Eli Bendersky's golden file patterns

### Tertiary (LOW confidence)
- None -- all findings verified against codebase and official Go documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies, all stdlib + existing internal packages
- Architecture: HIGH -- Based on direct reading of all affected source files (engine.go, root.go, server handlers, cleanup, scan types)
- Pitfalls: HIGH -- Identified from actual code patterns (e.g., channel blocking, color codes in golden files, token memory) and Go concurrency best practices
- Migration strategy: HIGH -- User decisions are clear; existing test coverage provides safety net

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable -- internal refactoring of existing Go code, no external dependency changes)
