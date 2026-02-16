# Roadmap: mac-cleaner

## Overview

This roadmap takes mac-cleaner from zero to a safe, production-ready macOS disk cleaning CLI tool. Starting with a safety foundation (SIP/swap exclusions, dry-run scanning), we expand through categories (system caches, browser data, developer caches, app leftovers), add interactive and automation modes, then polish with comprehensive safety enforcement. The journey prioritizes safety-first architecture (preventing kernel panics and system breakage) before expanding features, following the proven pattern that catastrophic failures happen when safety is an afterthought.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Project Setup & Safety Foundation** - Go project initialization, safety layer, core types
- [x] **Phase 2: System Cache Scanning** - First category implementation with dry-run architecture
- [x] **Phase 3: Browser & Developer Caches** - Multi-category expansion (browser data, dev caches)
- [ ] **Phase 4: App Leftovers & Cleanup Execution** - Third category plus actual deletion capability
- [ ] **Phase 5: Interactive Mode** - Walkthrough mode with item-by-item confirmation
- [ ] **Phase 6: CLI Polish & Automation** - Advanced flags (JSON, verbose, skip, force)
- [ ] **Phase 7: Safety Enforcement** - Risk categorization and permission handling

## Phase Details

### Phase 1: Project Setup & Safety Foundation
**Goal**: Project scaffolding exists and safety layer prevents catastrophic failures
**Depends on**: Nothing (first phase)
**Requirements**: SAFE-01, SAFE-02
**Success Criteria** (what must be TRUE):
  1. Go project compiles and produces binary
  2. Safety layer rejects any attempt to touch SIP-protected paths (/System, /usr, /bin, /sbin)
  3. Safety layer rejects any attempt to touch swap files (/private/var/vm/)
  4. Basic CLI accepts flags and prints version
**Plans**: 2 plans

Plans:
- [x] 01-01-PLAN.md — Go project scaffolding with Cobra CLI and --version flag
- [x] 01-02-PLAN.md — Safety layer with TDD (IsPathBlocked for SIP/swap protection)

### Phase 2: System Cache Scanning
**Goal**: User can scan system caches with dry-run preview showing space reclaimable
**Depends on**: Phase 1
**Requirements**: SYS-01, SYS-02, SYS-03, CLI-01, CLI-05
**Success Criteria** (what must be TRUE):
  1. User can run `mac-cleaner --system-caches --dry-run` and see list of cache directories with sizes
  2. User sees summary showing total space reclaimable from system caches
  3. Scan covers user app caches (~/Library/Caches), user logs (~/Library/Logs), and QuickLook thumbnails
  4. No files are deleted in dry-run mode (verified by file existence checks)
**Plans**: 2 plans

Plans:
- [x] 02-01-PLAN.md — Core scan types and size utilities (TDD)
- [x] 02-02-PLAN.md — System scanner, CLI wiring, and formatted output

### Phase 3: Browser & Developer Caches
**Goal**: User can scan browser data and developer caches with same dry-run architecture
**Depends on**: Phase 2
**Requirements**: BRWS-01, BRWS-02, BRWS-03, DEV-01, DEV-02, DEV-03, DEV-04
**Success Criteria** (what must be TRUE):
  1. User can scan Safari, Chrome, and Firefox caches with `--browser-data`
  2. User can scan Xcode DerivedData, npm/yarn cache, Homebrew cache, Docker artifacts with `--dev-caches`
  3. Scan gracefully handles missing browsers (Chrome not installed) or dev tools (Docker not running)
  4. Summary shows space breakdown per tool (e.g., "Xcode: 5.2 GB, npm: 1.8 GB")
**Plans**: 2 plans

Plans:
- [x] 03-01-PLAN.md — Browser scanner (Safari/Chrome/Firefox), shared ScanTopLevel extraction, printResults generalization
- [x] 03-02-PLAN.md — Developer scanner (Xcode/npm/yarn/Homebrew/Docker) with CLI integration

### Phase 4: App Leftovers & Cleanup Execution
**Goal**: User can scan app leftovers and execute actual cleanup with confirmation
**Depends on**: Phase 3
**Requirements**: APP-01, APP-02, APP-03, CLI-04
**Success Criteria** (what must be TRUE):
  1. User can scan orphaned app preferences, old iOS backups, and old Downloads files with `--app-leftovers`
  2. User sees confirmation prompt before deletion listing exactly what will be removed
  3. User can proceed with deletion and files are permanently removed
  4. User sees post-cleanup summary showing items removed and space freed
  5. Deletion never proceeds without explicit confirmation (must type "yes" or press enter)
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD
- [ ] 04-03: TBD

### Phase 5: Interactive Mode
**Goal**: User can run walkthrough mode that asks keep/remove for each cleaning item
**Depends on**: Phase 4
**Requirements**: CLI-03
**Success Criteria** (what must be TRUE):
  1. User runs `mac-cleaner` (no args) and enters interactive mode
  2. Tool scans all categories and presents each item one-by-one with size
  3. User can respond "keep" or "remove" for each item
  4. Tool summarizes all marked items and asks final confirmation before deletion
  5. Cleanup executes only after final confirmation and shows summary
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD

### Phase 6: CLI Polish & Automation
**Goal**: User can automate cleaning with flags and get structured output for AI agents
**Depends on**: Phase 5
**Requirements**: CLI-02, CLI-06, CLI-07, CLI-08, CLI-09, CLI-10
**Success Criteria** (what must be TRUE):
  1. User can target all categories with `--all` flag
  2. User can get JSON output with `--json` (structured data for automation)
  3. User can get detailed file listing with `--verbose` (shows individual files, not just summaries)
  4. User can skip categories with `--skip-browser-data` and specific items with `--skip-derived-data`
  5. User can bypass confirmation with `--force` for automation (combined with --dry-run for safety testing)
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD
- [ ] 06-03: TBD

### Phase 7: Safety Enforcement
**Goal**: User sees risk categorization for all items and tool respects permission boundaries
**Depends on**: Phase 6
**Requirements**: SAFE-03, SAFE-04
**Success Criteria** (what must be TRUE):
  1. Each cleaning item tagged with risk level (safe/moderate/risky)
  2. Risky items (like Xcode caches, app preferences) highlighted in output and require explicit confirmation
  3. Tool gracefully handles permission errors (reports what it can't access without failing entire scan)
  4. Tool never requests Full Disk Access (works with standard user permissions)
  5. Safe items (system caches, browser caches) can be cleaned without elevated permissions
**Plans**: TBD

Plans:
- [ ] 07-01: TBD
- [ ] 07-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Project Setup & Safety Foundation | 2/2 | ✓ Complete | 2026-02-16 |
| 2. System Cache Scanning | 2/2 | ✓ Complete | 2026-02-16 |
| 3. Browser & Developer Caches | 2/2 | ✓ Complete | 2026-02-16 |
| 4. App Leftovers & Cleanup Execution | 0/TBD | Not started | - |
| 5. Interactive Mode | 0/TBD | Not started | - |
| 6. CLI Polish & Automation | 0/TBD | Not started | - |
| 7. Safety Enforcement | 0/TBD | Not started | - |

---
*Roadmap created: 2026-02-16*
