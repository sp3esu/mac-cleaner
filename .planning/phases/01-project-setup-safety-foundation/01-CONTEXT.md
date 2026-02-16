# Phase 1: Project Setup & Safety Foundation - Context

**Gathered:** 2026-02-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Go project scaffolding with a safety layer that prevents catastrophic macOS system damage. Produces a compilable binary with version flag and hardcoded protections for SIP paths and swap files. Scanning, cleaning, and interactive modes are separate phases.

</domain>

<decisions>
## Implementation Decisions

### CLI identity & invocation
- Binary name: `mac-cleaner`
- `mac-cleaner --version` outputs just the version number (e.g., `0.1.0`)
- Tone: terse and technical — "system-caches: 5.2G", not "Looks like you have 5.2 GB!"
- Help text follows the same terse style — factual, no personality

### Safety layer behavior
- Blocked paths reported as warning lines to stderr: `SKIP: /System (SIP-protected)`
- Core protections hardcoded (SIP paths, swap files) — no config can override
- User can add extra protected paths via config (allowlist for additional safety)
- Logging to stderr only — no separate log file
- Symlink/traversal handling: Claude's discretion (see below)

### Project structure conventions
- Go module path: `github.com/gregor/mac-cleaner`
- Go 1.22+ minimum
- One package per cleaning category: `pkg/system/`, `pkg/browser/`, `pkg/developer/`
- Tests alongside code (Go standard): `safety_test.go` next to `safety.go`

### Output style & formatting
- Colors with TTY auto-detection — colors when terminal, plain when piped
- File sizes: human-readable (5.2 GB) — like `du -h`
- Unicode symbols for status indicators (checkmarks, arrows, warning triangles)
- Scan results in table format with aligned columns

### Claude's Discretion
- CLI structure (flags-only vs subcommands) — pick what fits the roadmap best
- Symlink safety approach (resolve vs skip) — pick the safest option
- Exact table formatting and column layout
- Color palette and specific Unicode symbols used

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-project-setup-safety-foundation*
*Context gathered: 2026-02-16*
