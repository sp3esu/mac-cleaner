# mac-cleaner

## What This Is

A macOS CLI tool that scans the system for junk data — caches, temporary files, app leftovers, developer artifacts, and browser data — and helps users reclaim disk space. It offers both an interactive walkthrough mode and a scriptable flag-based mode, with `--dry-run` support and granular skip controls. Designed for general macOS users today, with structured output to support AI agent integration in the future.

## Core Value

Users can safely and confidently reclaim disk space without worrying about deleting something important.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Scan and report disk usage across multiple junk categories (system caches, app leftovers, developer caches, browser data)
- [ ] `--dry-run` flag to preview what would be removed without deleting anything
- [ ] `--all` flag to target all categories at once
- [ ] Interactive mode (no args) that walks through each item and asks keep/remove
- [ ] Flag-based mode for scripting (`--all`, `--system-caches`, `--browser-data`, etc.)
- [ ] Skip by category (`--skip-system-caches`) and by specific item (`--skip-derived-data`)
- [ ] Summary output by default, `--verbose` for detailed file listing
- [ ] `--json` flag for structured output (AI agent use case)
- [ ] Confirmation prompt before any destructive action
- [ ] Post-cleanup summary showing what was removed and space reclaimed

### Out of Scope

- Windows/Linux support — macOS only
- GUI interface — CLI only
- Real-time file monitoring / scheduled cleaning — manual invocation only
- Trash/undo support — files are permanently deleted (confirmation mitigates risk)

## Context

The tool name is "mac-cleaner" (the project directory is "mac-clarner" but the binary/tool name is "mac-cleaner").

**Usage patterns:**

1. **Quick audit:** `mac-cleaner --all --dry-run` — see what's there, decide what to skip
2. **Full clean:** `mac-cleaner --all` — clean everything with confirmation
3. **Full clean with skips:** `mac-cleaner --all --skip-derived-data` — clean everything except specific items
4. **Interactive:** `mac-cleaner` — walk through each item one by one, confirm at end, then execute
5. **AI agent:** `mac-cleaner --all --dry-run --json` — structured output for automated decision-making

**Cleaning categories:**
- **System caches:** ~/Library/Caches, /Library/Caches, system temp files
- **App leftovers:** Remnants from uninstalled apps (preferences, support files, containers)
- **Developer caches:** Xcode DerivedData, node_modules, .gradle, Docker images, Homebrew cache
- **Browser data:** Browser caches, cookies, download history

**Interactive mode flow:**
1. Scan all categories
2. Present each cleaning item one by one with size
3. User says keep or remove for each
4. Summarize all targets for removal
5. Ask final confirmation
6. Execute removal
7. Display summary of what was removed and space reclaimed

**Output:**
- Human-readable by default (summary per category with sizes)
- `--verbose` adds per-file detail
- `--json` switches to structured JSON output
- Output should be informative enough for AI agents to make decisions

## Constraints

- **Platform:** macOS only — can use macOS-specific APIs and paths
- **Language:** Needs research — must be compiled, stable, easy to use, cross-compilable (even though targeting macOS only, the language should be production-grade)
- **Safety:** Must never delete without explicit user confirmation (or `--force` flag for automation)
- **Permissions:** Some system paths require elevated permissions — tool should handle gracefully (report what it can't access)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| macOS only | Focused scope, can leverage macOS-specific paths and APIs | — Pending |
| Compiled language (Go-like) | Performance, single binary distribution, no runtime deps | — Pending (needs research) |
| Interactive by default, flags for scripting | Approachable for general users, powerful for automation | — Pending |
| Confirmation before deletion | Safety for general users, `--force` for AI agents | — Pending |

---
*Last updated: 2026-02-16 after initialization*
