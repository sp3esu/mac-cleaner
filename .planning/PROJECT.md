# mac-cleaner

## What This Is

A macOS CLI tool that scans the system for junk data — caches, temporary files, app leftovers, developer artifacts, browser data, project build caches, large/old files, and git repository caches — and helps users reclaim disk space. Offers interactive walkthrough mode and scriptable flag-based mode with dry-run support, granular skip controls, JSON output for AI agents, and risk-aware safety enforcement. Includes a Unix domain socket IPC server mode for native Swift macOS app integration with real-time streaming progress.

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
- ✓ Scanning orchestration decoupled from cobra into reusable engine package — v1.1
- ✓ Engine supports per-scanner progress callbacks — v1.1
- ✓ Engine supports category filtering (skip set) — v1.1
- ✓ CLI refactored to use engine (no behavior change) — v1.1
- ✓ NDJSON request/response protocol with request IDs — v1.1
- ✓ Methods: scan, cleanup, categories, ping, shutdown — v1.1
- ✓ Scan method streams per-scanner progress events, then final result — v1.1
- ✓ Cleanup method streams per-entry progress events, then final result — v1.1
- ✓ Categories method returns available scanners with metadata — v1.1
- ✓ Unix domain socket listener with graceful shutdown — v1.1
- ✓ `serve` cobra subcommand with `--socket` flag — v1.1
- ✓ Single-connection handling (reject concurrent operations) — v1.1
- ✓ Socket file cleanup on shutdown and stale socket detection on startup — v1.1
- ✓ Client disconnect during scan/cleanup handled gracefully — v1.1
- ✓ Connection timeout and keep-alive — v1.1
- ✓ Cleanup requests validated against prior scan results (replay protection) — v1.1
- ✓ Project cache scanning for node_modules, .build, target/, venv, build/ with manifest validation — v1.7.0
- ✓ Single-pass recursive walk utility with depth limits and exclusions — v1.7.0
- ✓ Stale project highlighting (6+ months) and aggregate summaries per project type — v1.7.0
- ✓ Home directory large file (≥100MB) and old file (≥180 days) scanning with smart exclusions — v1.7.0
- ✓ Git repository scanning with count-objects reclaimable estimation — v1.7.0
- ✓ Git cleanup via git gc --aggressive and git lfs prune through pseudo-path dispatch — v1.7.0

### Active

(None — next milestone not yet defined)

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
- Scanning inside ~/Library — already covered by existing system/browser/developer scanners
- Manual .git/objects deletion — corrupts repos; use git gc/prune instead
- Git history rewriting (BFG, filter-branch) — destructive, irreversible, affects remote collaborators

## Context

Shipped v1.7.0 with 16,970 LOC Go (up from 10,572 at v1.1, 4,442 at v1.0).
Tech stack: Go, Cobra CLI, fatih/color for terminal output, tabwriter for formatting.
Binary name: `mac-cleaner` (project directory is `mac-cleaner`).

v1.0: 13 requirements shipped across 7 phases and 13 plans (~4.3 hours).
v1.1: 16 requirements shipped across 4 phases and 5 plans (2 days).
v1.7.0: 15 requirements shipped across 3 phases and 6 plans (1 day).

**Architecture:** Single binary, two modes — CLI (interactive/flag-based) and IPC server via Unix domain socket with NDJSON protocol. Engine layer decouples scan/cleanup orchestration for reuse by both CLI and server. Shared walk utility (internal/walk) provides recursive directory traversal with callbacks.

**Cleaning categories:**
- System caches: ~/Library/Caches, ~/Library/Logs, QuickLook thumbnails
- Browser data: Safari, Chrome (multi-profile), Firefox
- Developer caches: Xcode DerivedData, npm, yarn, Homebrew, Docker
- App leftovers: Orphaned preferences, iOS backups, old Downloads (90-day threshold)
- Project caches: node_modules, .build (Swift), target/ (Rust), venv (Python), build/ (Java/Kotlin)
- Home directory files: Large files (≥100MB), old files (≥180 days)
- Git repos: .git directory sizes, reclaimable space, gc/lfs cleanup

**Known tech debt:**
- Minor doc inaccuracy: swift-integration.md token lifecycle description (low severity)
- 90-day hardcoded download age threshold (revisit when configurability needed)
- formatAge duplicated between pkg/projects and pkg/homedir (avoids import cycle)

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
| Sequential scanner execution | Matches CLI behavior; concurrent can be added without API change | ✓ Good |
| Single-token store | New scan invalidates previous; avoids memory leak | ✓ Good |
| IdleTimeout as public struct field | Test override flexibility vs constructor param | ✓ Good |
| Cleanup continues on disconnect | Partially-deleted state worse than completing the operation | ✓ Good |
| Shared walk utility with callbacks | Single-pass traversal reused by project/homedir/git scanners; avoids re-walking | ✓ Good |
| Manifest-validated cache detection | Prevents false positives (e.g. random "build/" dirs without build.gradle) | ✓ Good |
| git-gc pseudo-path dispatch | Extends cleanup.Execute without new interfaces; git commands run in repo directory | ✓ Good |
| Single-item scanner group for git-repos | Follows unused-apps pattern; avoids cobra flag collision with per-item flags | ✓ Good |
| formatAge duplication across packages | Avoids import cycle between pkg/projects and pkg/homedir; simple helper | ✓ Good |

---
*Last updated: 2026-03-11 after v1.7.0 milestone*
