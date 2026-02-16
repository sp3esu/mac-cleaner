# mac-cleaner

## What This Is

A macOS CLI tool that scans the system for junk data — caches, temporary files, app leftovers, developer artifacts, and browser data — and helps users reclaim disk space. Offers interactive walkthrough mode and scriptable flag-based mode with dry-run support, granular skip controls, JSON output for AI agents, and risk-aware safety enforcement.

## Core Value

Users can safely and confidently reclaim disk space without worrying about deleting something important.

## Requirements

### Validated

- ✓ Scan and report disk usage across multiple junk categories (system caches, app leftovers, developer caches, browser data) — v1.0
- ✓ `--dry-run` flag to preview what would be removed without deleting anything — v1.0
- ✓ `--all` flag to target all categories at once — v1.0
- ✓ Interactive mode (no args) that walks through each item and asks keep/remove — v1.0
- ✓ Flag-based mode for scripting (`--all`, `--system-caches`, `--browser-data`, etc.) — v1.0
- ✓ Skip by category (`--skip-system-caches`) and by specific item (`--skip-derived-data`) — v1.0
- ✓ Summary output by default, `--verbose` for detailed file listing — v1.0
- ✓ `--json` flag for structured output (AI agent use case) — v1.0
- ✓ Confirmation prompt before any destructive action — v1.0
- ✓ Post-cleanup summary showing what was removed and space reclaimed — v1.0
- ✓ SIP/swap path protection with path traversal and symlink attack prevention — v1.0
- ✓ Risk categorization (safe/moderate/risky) with color-coded display — v1.0
- ✓ Permission error reporting without failing entire scan — v1.0

### Active (v1.1 — Swift Integration / Server Mode)

- ENG-01: Scanning orchestration decoupled from cobra into reusable engine package
- ENG-02: Engine supports per-scanner progress callbacks
- ENG-03: Engine supports category filtering (skip set)
- ENG-04: CLI refactored to use engine (no behavior change)
- PROTO-01: NDJSON request/response protocol with request IDs
- PROTO-02: Methods: scan, cleanup, categories, ping, shutdown
- PROTO-03: Scan method streams per-scanner progress events, then final result
- PROTO-04: Cleanup method streams per-entry progress events, then final result
- PROTO-05: Categories method returns available scanners with metadata
- SRV-01: Unix domain socket listener with graceful shutdown
- SRV-02: `serve` cobra subcommand with `--socket` flag
- SRV-03: Single-connection handling (reject concurrent operations)
- SRV-04: Socket file cleanup on shutdown and stale socket detection on startup
- HARD-01: Client disconnect during scan/cleanup handled gracefully
- HARD-02: Connection timeout and keep-alive
- HARD-03: Cleanup requests validated against prior scan results (replay protection)

### Out of Scope

- Windows/Linux support — macOS only
- GUI interface — native Swift macOS app connects via UDS server, not built into this project
- Real-time file monitoring / scheduled cleaning — manual invocation only
- Trash/undo support — files are permanently deleted (confirmation mitigates risk)
- Cloud storage cleanup — too destructive (Dropbox/iCloud deletion has remote consequences)
- Kernel extension cleanup — too risky, can break system
- Automatic scheduled cleaning — can surprise users with data loss
- Time Machine snapshot deletion — macOS manages automatically
- Malware detection — different problem domain

## Current Milestone: v1.1 — Swift Integration (Server Mode)

**Goal:** Add a Unix domain socket server mode (`mac-cleaner serve`) so a native Swift macOS app can control scanning and cleanup with real-time streaming progress, while keeping the CLI fully standalone.

**Architecture:** Single binary, two modes — CLI (unchanged) and IPC server via UDS with NDJSON protocol. Engine layer decouples scan/cleanup orchestration from cobra. Proven pattern used by Tailscale in production.

**Phases:** 8-11 (Engine Extraction → Protocol & Server Core → Scan & Cleanup Handlers → Hardening & Documentation)

## Context

Shipped v1.0 with 4,442 LOC Go.
Tech stack: Go, Cobra CLI, fatih/color for terminal output, tabwriter for formatting.
Binary name: `mac-cleaner` (project directory is `mac-cleaner`).
All 27 v1 requirements shipped across 7 phases and 13 plans in ~4.3 hours.

**Cleaning categories:**
- System caches: ~/Library/Caches, ~/Library/Logs, QuickLook thumbnails
- Browser data: Safari, Chrome (multi-profile), Firefox
- Developer caches: Xcode DerivedData, npm, yarn, Homebrew, Docker
- App leftovers: Orphaned preferences, iOS backups, old Downloads (90-day threshold)

## Constraints

- **Platform:** macOS only — uses macOS-specific paths and APIs
- **Language:** Go — compiled, single binary, no runtime dependencies
- **Safety:** Never deletes without explicit user confirmation (or `--force` flag for automation)
- **Permissions:** Handles permission errors gracefully (reports what it can't access)
- **SIP protection:** Hardcoded blocklist — no config can override

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| macOS only | Focused scope, can leverage macOS-specific paths and APIs | ✓ Good |
| Go with Cobra CLI | Performance, single binary distribution, no runtime deps | ✓ Good |
| Interactive by default, flags for scripting | Approachable for general users, powerful for automation | ✓ Good |
| Confirmation before deletion | Safety for general users, `--force` for AI agents | ✓ Good |
| SI units (base 1000) for sizes | Matches macOS Finder convention | ✓ Good |
| Hardcoded safety protections | No config can override SIP/swap blocklist | ✓ Good |
| Exact "yes" for confirmation | Prevents accidental deletion | ✓ Good |
| --force bypasses confirmation only | Safety layer always enforced regardless of flags | ✓ Good |
| Risk constants in safety package | Avoids circular imports between scan and safety | ✓ Good |
| Permission issues to stderr | Doesn't contaminate pipeable stdout output | ✓ Good |
| 90-day hardcoded download age | Simple for v1, configurability deferred | ⚠️ Revisit |
| UDS over XPC for Swift integration | Go XPC is dead ecosystem, GCD/goroutine conflicts; UDS+JSON proven by Tailscale | ✓ Good |
| NDJSON protocol | Simple, streamable, debuggable with standard tools (socat/netcat) | ✓ Good |
| Engine layer between CLI and scanners | Decouples orchestration for reuse by both CLI and server | ✓ Good |
| Single-connection server | Simplifies state management; macOS app is sole client | ✓ Good |

---
*Last updated: 2026-02-16 after v1.1 milestone setup*
