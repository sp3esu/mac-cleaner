# Phase 8: Engine Extraction - Context

**Gathered:** 2026-02-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract scan/cleanup orchestration from `cmd/root.go` into a reusable `internal/engine/` package, then refactor the CLI to delegate to it — with zero behavior change. The engine becomes the shared foundation for both CLI and the server mode (Phase 9+).

</domain>

<decisions>
## Implementation Decisions

### Engine API surface
- Expose both ScanAll() and individual per-scanner Run() — ScanAll for convenience, per-scanner for granular server control
- Progress callbacks at two granularities: per-scanner start/finish AND per-entry (file/directory found)
- Channel-based streaming: ScanAll returns a channel, results arrive as each scanner completes
- Context-aware: ScanAll takes context.Context for cancellation support (essential for server disconnect handling)
- Custom error types: Engine-specific errors (ScanError, CancelledError) for typed error handling by the server
- Unified streaming pattern: Both scan and cleanup stream through the same callback/channel pattern

### Scanner registration
- Registry pattern with explicit Register() calls — extensible but no init() magic
- Central DefaultScanners() function that explicitly registers all scanners — clear, easy to audit
- Rich metadata per scanner: name, category ID, description, risk level — enables the server 'categories' method without extra mapping
- Skip filtering at scan time, not registration time — all scanners always registered, skip set checked when ScanAll runs

### Cleanup orchestration
- Engine owns both scan and cleanup — single package orchestrates the full workflow
- Engine enforces confirmation: requires a confirmation token or flag to prevent accidental cleanup calls
- Scan-result token: ScanAll returns a token/ID, Cleanup requires that token — replay protection built in
- Partial selection: Cleanup(token, categoryIDs) — clean only selected categories from a scan
- Cleanup package (internal/cleanup) stays separate — engine wraps it, cleanup stays focused on file deletion

### Migration strategy
- Two plans: Plan 1 creates engine package with tests (mock scanners), Plan 2 wires cmd/root.go to use engine and verifies golden output
- Golden output comparison: capture current CLI output before refactor, compare after — exact match = success
- Golden files stored in repo (testdata/) as committed regression tests for ongoing protection
- Scanner defined as Go interface type: `type Scanner interface { Scan() ([]CategoryResult, error); Info() ScannerInfo }` — enables rich metadata and mockability

### Claude's Discretion
- Scanner concurrency model (sequential vs. concurrent with limit) — pick based on performance vs. complexity
- Engine struct design (struct fields vs. per-call arguments for config) — pick what works best for both CLI and server
- Exact channel types and streaming API shape

</decisions>

<specifics>
## Specific Ideas

- Token-based cleanup validation: scan produces a token, cleanup consumes it — prevents replaying old scan results
- Golden output files committed to testdata/ serve double duty: migration verification now, regression protection forever
- Interface type for Scanner maps naturally to the server 'categories' endpoint via Info() method

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-engine-extraction*
*Context gathered: 2026-02-16*
