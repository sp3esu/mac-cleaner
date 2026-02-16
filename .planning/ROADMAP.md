# Roadmap — v1.1 Swift Integration (Server Mode)

## Phase 8: Engine Extraction
**Goal:** Extract scan/cleanup orchestration from `cmd/root.go` into `internal/engine/`

**Requirements:** ENG-01, ENG-02, ENG-03, ENG-04

**Plans:** 2 plans

Plans:
- [x] 08-01-PLAN.md — Build engine package: Scanner interface, Engine struct, channel-based ScanAll/Cleanup, token store, custom errors, tests
- [x] 08-02-PLAN.md — Wire CLI and server to use Engine struct, remove direct pkg/* imports, verify zero behavior change

**Key deliverables:**
- `internal/engine/engine.go` — Engine struct, ScanAll(), Run(), Cleanup()
- `internal/engine/scanner.go` — Scanner interface, ScannerInfo, adapter
- `internal/engine/registry.go` — Register(), RegisterDefaults(), Categories()
- `internal/engine/token.go` — ScanToken, replay protection
- `internal/engine/errors.go` — ScanError, CancelledError, TokenError
- `internal/engine/engine_test.go` — 24+ tests with mock scanners
- `cmd/root.go` modified to delegate to engine (no behavior change)
- `internal/server/` updated to use Engine struct instance

**Success criteria:**
- `go test ./...` passes
- CLI behavior identical: `mac-cleaner --all --dry-run` output unchanged
- Engine package is independently usable (no cobra dependency)

---

## Phase 9: Protocol & Server Core
**Goal:** Unix domain socket server with NDJSON protocol and `ping` handler

**Requirements:** PROTO-01, SRV-01, SRV-02, SRV-04

**Plans:** 1 plan

Plans:
- [ ] 09-01-PLAN.md — Verify and close Phase 9 requirements (all proactively implemented in Phase 8), add supplemental test coverage, update REQUIREMENTS.md

**Key deliverables:**
- `internal/server/protocol.go` — Request/Response types, NDJSON encoding
- `internal/server/server.go` — Socket listener, connection handler, dispatch, graceful shutdown
- `cmd/serve.go` — `serve` subcommand with `--socket` flag
- Tests for protocol encoding and server lifecycle

**Success criteria:**
- `mac-cleaner serve --socket /tmp/test.sock` starts and listens
- `ping` method responds via socat/netcat
- Server shuts down cleanly on SIGINT/SIGTERM
- Stale socket files detected and cleaned up on startup

---

## Phase 10: Scan & Cleanup Handlers
**Goal:** Wire scan and cleanup methods with streaming progress

**Requirements:** PROTO-02, PROTO-03, PROTO-04, PROTO-05, SRV-03

**Key deliverables:**
- `internal/server/handler_scan.go` — scan + categories methods
- `internal/server/handler_cleanup.go` — cleanup method with streaming
- Tests for all handlers

**Success criteria:**
- Full scan via socket with streaming per-scanner progress events
- Cleanup via socket with streaming per-entry progress events
- Categories method returns scanner metadata
- Concurrent operation requests rejected

---

## Phase 11: Hardening & Documentation
**Goal:** Production readiness, error handling, Swift integration reference

**Requirements:** HARD-01, HARD-02, HARD-03

**Key deliverables:**
- `internal/server/server.go` — timeouts, disconnect handling
- `docs/swift-integration.md` — Swift Codable types, NWConnection patterns
- Tests for edge cases

**Success criteria:**
- Client disconnect during scan/cleanup handled gracefully (no goroutine leaks)
- Connection timeout and keep-alive working
- Cleanup validated against prior scan results
- Swift integration documented with working code examples

---
*Created: 2026-02-16*
