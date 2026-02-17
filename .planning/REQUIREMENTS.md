# Requirements — v1.1 Swift Integration (Server Mode)

## Engine (ENG)

| ID | Requirement | Phase | Status |
|----|-------------|-------|--------|
| ENG-01 | Scanning orchestration is decoupled from cobra into a reusable engine package | 8 | complete |
| ENG-02 | Engine supports per-scanner progress callbacks | 8 | complete |
| ENG-03 | Engine supports category filtering (skip set) | 8 | complete |
| ENG-04 | CLI refactored to use engine (no behavior change) | 8 | complete |

## Protocol (PROTO)

| ID | Requirement | Phase | Status |
|----|-------------|-------|--------|
| PROTO-01 | NDJSON request/response protocol with request IDs | 9 | complete |
| PROTO-02 | Methods: scan, cleanup, categories, ping, shutdown | 10 | complete |
| PROTO-03 | Scan method streams per-scanner progress events, then final result | 10 | complete |
| PROTO-04 | Cleanup method streams per-entry progress events, then final result | 10 | complete |
| PROTO-05 | Categories method returns available scanners with metadata | 10 | complete |

## Server (SRV)

| ID | Requirement | Phase | Status |
|----|-------------|-------|--------|
| SRV-01 | Unix domain socket listener with graceful shutdown | 9 | complete |
| SRV-02 | `serve` cobra subcommand with `--socket` flag | 9 | complete |
| SRV-03 | Single-connection handling (reject concurrent operations) | 10 | complete |
| SRV-04 | Socket file cleanup on shutdown and stale socket detection on startup | 9 | complete |

## Hardening (HARD)

| ID | Requirement | Phase | Status |
|----|-------------|-------|--------|
| HARD-01 | Client disconnect during scan/cleanup handled gracefully | 11 | complete |
| HARD-02 | Connection timeout and keep-alive | 11 | complete |
| HARD-03 | Cleanup requests validated against prior scan results (replay protection) | 11 | complete |

---
*Created: 2026-02-16*
