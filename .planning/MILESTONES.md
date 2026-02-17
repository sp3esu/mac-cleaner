# Milestones: mac-cleaner

## v1.1 Swift Integration (Shipped: 2026-02-17)

**Delivered:** Unix domain socket IPC server mode with NDJSON streaming protocol, enabling a native Swift macOS app to control scanning and cleanup with real-time progress.

**Phases completed:** 8-11 (5 plans total)

**Key accomplishments:**

- Built reusable Engine package decoupling scan/cleanup orchestration from CLI with Scanner interface and channel-based streaming API
- Unix domain socket IPC server with NDJSON protocol for Swift app integration
- Full streaming progress for scan and cleanup operations through socket protocol
- Production hardening: disconnect resilience, configurable idle timeout, token-based cleanup validation
- 44+ tests with race detection including socket-level integration tests
- Complete Swift integration documentation with working code examples

**Stats:**

- 33 files created/modified
- 10,572 lines of Go (total project)
- 4 phases, 5 plans, 12 tasks
- 2 days (2026-02-16 to 2026-02-17)

**Git range:** `docs(08-engine-extraction)` -> `docs: create v1.1 milestone audit`

**What's next:** TBD — next milestone defined via `/gsd:new-milestone`

---

## v1.0 MVP (Shipped: 2026-02-16)

**Delivered:** A complete macOS disk cleaning CLI tool with safe scanning across 4 categories, interactive and scripted modes, granular skip controls, JSON output for AI agents, and risk-aware safety enforcement.

**Phases completed:** 1-7 (13 plans total)

**Key accomplishments:**

- Safety-first architecture with SIP/swap path protection, path traversal prevention, and symlink resolution
- Four-category scanning engine covering system caches, browser data, developer caches, and app leftovers
- Interactive walkthrough mode with per-item keep/remove decisions and final confirmation
- Full CLI automation suite: --all, --json, --verbose, --force, 16 skip flags for granular control
- Risk categorization (safe/moderate/risky) with color-coded display and permission error reporting
- Confirmation-based deletion with safety re-checks at delete time and post-cleanup summaries

**Stats:**

- 71 files created/modified
- 4,442 lines of Go
- 7 phases, 13 plans
- 1 day (2026-02-16, ~4.3 hours execution)

**Git range:** `feat(01-01)` -> `feat(07-02)`

**What's next:** v1.1 — Swift Integration (Server Mode)

---
