# Milestones: mac-cleaner

## v1.7.0 Extended Scanning (Shipped: 2026-03-11)

**Delivered:** Three new scanning categories — project build caches, home directory file analysis, and git repository cache management — with a shared walk utility, full CLI/IPC integration, and 6-language documentation.

**Phases completed:** 12-14 (6 plans total)

**Key accomplishments:**

- Reusable walk utility (internal/walk) with depth limits, exclusions, OnDir/OnFile callbacks — shared across all 3 phases
- Project cache scanner detecting 5 types (node_modules, .build, target/, venv, build/) via manifest validation in single-pass walk
- Home directory file scanner finding large (≥100MB) and old (≥180 days) files with smart exclusions (Library, Apps, Trash, Downloads, dotfiles)
- Git repository scanner with count-objects reclaimable estimation and git gc --aggressive / git lfs prune cleanup
- Full CLI integration with scan/skip flags for all new categories, IPC server support, and 6-language README updates

**Stats:**

- 42 files created/modified
- 16,970 lines of Go (total project, up from 10,572 at v1.1)
- 3 phases, 6 plans
- 1 day (2026-03-11)

**Git range:** `feat(12-01)` → `docs(phase-14)`

### Known Gaps

Missing formal VERIFICATION.md for phases 12, 13, 14. All requirements claimed complete by SUMMARY frontmatter, checked in REQUIREMENTS.md, all integration wiring and E2E flows verified by audit. Gap is procedural (missing verification artifact), not functional.

**What's next:** TBD — next milestone defined via `/gsd:new-milestone`

---

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
