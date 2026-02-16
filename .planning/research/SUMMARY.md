# Project Research Summary

**Project:** mac-cleaner
**Domain:** macOS Disk Cleaning CLI Tool
**Researched:** 2026-02-16
**Confidence:** MEDIUM-HIGH

## Executive Summary

Building a macOS disk cleaning CLI tool requires understanding three critical layers: macOS security architecture (SIP, SSV, APFS), safe file classification, and user trust through transparency. The research reveals that Go is the optimal technology choice for this domain, offering single binary distribution, excellent file system operations, and mature CLI ecosystem through Cobra framework. The tool should focus on four cleanable categories (system caches, app leftovers, developer caches, browser data) while explicitly avoiding system-critical paths that could break macOS.

The key insight from competitive analysis is that CLI tools in this space fail when they either request excessive permissions (breaking user trust) or delete indiscriminately (breaking apps/system). Success requires a safety-first architecture with multiple validation layers: SIP protection checks, risk categorization, and transparent preview before any deletion. The recommended approach is layered components (Scanner → Reporter → Cleaner) with rule-based configuration rather than hardcoded paths.

Critical risks include deleting SIP-protected directories (system won't boot), misclassifying cache importance (Spotlight re-index, app breakage), and confusing purgeable APFS space with truly reclaimable space (user expectations mismatch). Mitigation requires hardcoded exclusion lists, conservative file classification biased toward NOT deleting, and clear communication separating purgeable vs freed space.

## Key Findings

### Recommended Stack

Go 1.26+ with Cobra CLI framework provides the best foundation for macOS disk cleaning tools. Go's single static binary distribution (CGO_ENABLED=0) eliminates runtime dependencies, while the stdlib provides robust file operations (io/fs.WalkDir for efficient scanning, path/filepath for cross-platform paths). GoReleaser automates multi-architecture builds and Homebrew tap distribution, essential for easy installation.

**Core technologies:**
- **Go 1.26+**: Single binary distribution, fast compilation, excellent stdlib for file operations — industry standard for CLI tools
- **Cobra v1.10.2+**: Command tree architecture for subcommands (scan, clean, list), automatic help generation, persistent flags — used by kubectl, docker, GitHub CLI
- **Viper**: Unified config management (files, env vars, flags) — pairs perfectly with Cobra
- **fatih/color v1.18.0**: Colored terminal output with automatic tty detection and NO_COLOR support — essential for interactive mode UX
- **GoReleaser**: Automated cross-platform builds, GitHub releases, Homebrew tap updates — critical for distribution

**Critical version requirements:**
- Go 1.26+ for latest performance and encoding/json/v2 improvements (Feb 2026)
- Cobra v1.10.2+ (Dec 2024 release) for latest stability
- Use io/fs.WalkDir (Go 1.16+) instead of filepath.Walk for better efficiency

**What NOT to use:**
- Python/Node.js (requires runtime, slower, harder single-binary distribution)
- Rust (steeper learning curve, slower compilation, comparable binary sizes)
- CGO_ENABLED=1 (creates dynamic binaries, breaks portability)

### Expected Features

The feature landscape is well-established through competitive analysis of CleanMyMac X, OnyX, Pearcleaner, and DaisyDisk. Users expect basic cache cleaning (system, browser, developer) but value CLI-specific differentiators like JSON output and granular flags.

**Must have (table stakes):**
- System cache cleaning (~/Library/Caches, /Library/Caches) — every Mac cleaner has this
- Browser cache cleaning (Safari, Chrome, Firefox) — multi-browser support expected
- Developer cache cleaning (Xcode DerivedData, npm, yarn, Homebrew) — target audience is developers
- App leftover removal (preferences, application support) — uninstalled apps leave debris
- Dry run mode (--dry-run) — users fear data loss, need preview
- Interactive prompts — users want control over deletions
- Size reporting — users need to see space savings to justify using tool
- Safe deletion — never delete system-critical files
- JSON output (--json) — enables automation and AI agent consumption
- Granular skip flags (--skip-browser --skip-xcode) — power users want fine control

**Should have (competitive differentiators):**
- Path-specific reporting — show exactly which paths cleaned, not just totals (transparency builds trust)
- Multiple package manager support — npm, yarn, pip, brew caches (high value for developer audience)
- Smart categorization — group by risk level (safe/moderate/expert) for informed decisions
- iOS backup cleaning — old iPhone backups consume massive space (~/Library/Application Support/MobileSync/Backup/)
- Homebrew integration — `brew cleanup` automation for developer workflows
- CLI-first with scriptability — only CLI tool with JSON output + granular flags

**Defer (v2+):**
- Universal binary stripping — remove unused architectures (30-50% size reduction, but complex binary manipulation)
- Language file pruning — remove unused localizations (hundreds of MB, but requires app bundle expertise)
- Duplicate file detection — CPU intensive file hashing, separate feature class
- QuickLook cache cleaning — moderate value, privacy angle secondary

**Anti-features (explicitly NOT build):**
- System file cleaning (/System/Library/Caches) — SIP-protected, breaks macOS
- Aggressive log deletion (system logs) — breaks diagnostics
- Automatic daily cleaning — surprise data loss
- /private/var/folders cleanup — temp files in use, breaks apps
- Time Machine snapshot deletion — macOS manages automatically, risky
- GUI wrapper — adds complexity, reduces scriptability

### Architecture Approach

The standard architecture for macOS disk cleaning CLI tools follows a layered, component-based design with clear separation between scanning, analysis, user interaction, and cleanup operations. This pattern is validated across mac-cleanup-go, MacCleanCLI, and mac-cleanup-py implementations.

**Major components:**
1. **CLI Entry Layer** — Parse flags (--all, --dry-run, --skip-*), detect mode (interactive vs flags), route to orchestrator
2. **Orchestration Controller** — Coordinate workflow (scan → report → clean → summary), handle dry-run logic, manage skip flags
3. **Scanner Layer** — Discover cleanable files with parallel scanning (Promise.all with p-limit), apply rule matching, perform safety checks (SIP validation, risk assessment)
4. **Rule Matcher** — Match files against cleaning categories using config-based rules (system-caches, dev-caches, browser-data, app-leftovers), apply exclusion filters
5. **Safety Checker** — Multi-layer validation: SIP protection check → Risk assessment → Permission check → Path existence
6. **Reporter Layer** — Calculate sizes (handle APFS clones/hardlinks), format output (human-readable vs JSON), generate summaries
7. **Confirmation Layer** — Interactive prompts (category selection → item selection) OR auto-confirm based on flags
8. **Cleaner Layer** — Execute deletions via Trash API (default), direct delete (--force), or external tools (brew cleanup, docker prune)
9. **Summary Layer** — Aggregate results (scanned/selected/cleaned counts), report errors, show space freed

**Key patterns:**
- **Rule-based scanning** — Define cleaning targets in config files (JSON/TypeScript), not hardcoded in scanner logic (easier to extend, testable)
- **Safety-first architecture** — Multiple validation layers before deletion: SIP → Risk → Permission → Existence → User confirmation
- **Parallel scanning** — Use controlled concurrency (p-limit with limit=8) for 3-10x speedup, essential for good UX
- **Mode detection** — Detect interactive vs flags early, use adapter pattern for different UI flows
- **Graceful degradation** — Detect external tool availability (brew, docker), skip gracefully if not installed

**Critical build order:**
Types (1) → Safety (2) → Rules (3) → Scanner (4) → Reporter (5) + Cleaner (6) + UI (7) → Orchestrator (8) → CLI Entry (9)

### Critical Pitfalls

Research identified 8 critical pitfalls that break systems or user trust:

1. **Deleting SIP-protected directories** — Attempting to delete /System, /bin, /sbin, /usr causes "operation not permitted" errors or system breakage. **Prevention:** Hardcode SIP exclusions in Phase 1, never suggest disabling SIP, check extended attribute com.apple.rootless
2. **Deleting active swap files** — Deleting /private/var/vm/swapfile* causes kernel panic and immediate crash. **Prevention:** Exclude entire /private/var/vm/ from day one
3. **Deleting caches without understanding impact** — Blindly deleting all caches causes Spotlight re-index (hours), app re-authentication, Xcode breakage. **Prevention:** Categorize caches (safe/risky/forbidden), never delete Spotlight index, auth tokens, keychains; require consent for Xcode/font caches
4. **Misunderstanding purgeable APFS space** — Tool reports "X GB freed" but users see no change because purgeable space (Time Machine snapshots) is auto-managed by macOS. **Prevention:** Use diskutil apfs list to separate purgeable vs deletable, report separately
5. **Ignoring Sealed System Volume (SSV)** — Attempting to modify System volume on macOS 11+ fails (signed/sealed, read-only). **Prevention:** Only operate on Data volume, never scan /System on Big Sur+
6. **Requesting Full Disk Access without justification** — Users distrust tool or grant excessive permissions creating security risk. **Prevention:** Minimize permissions, provide "basic mode" without FDA, document why each permission needed
7. **False positives (wrong classification)** — Misidentifying important files as junk (app preferences, project files, documents). **Prevention:** Conservative classification biased toward NOT deleting, whitelist approach for known-safe patterns, never delete based on extension/age alone
8. **Breaking apps via Application Support deletion** — Deleting ~/Library/Application Support/ loses licenses, settings, local databases. **Prevention:** Never delete without per-app knowledge, whitelist known-safe subdirectories, require explicit approval

## Implications for Roadmap

Based on research, suggested 4-phase structure prioritizing safety foundation before features:

### Phase 1: Safe Foundation & Core Scanning
**Rationale:** Safety validation must exist before any deletion capability. Research shows tools fail catastrophically when they skip safety checks (kernel panics from swap deletion, SIP violations). Building the safety layer first prevents entire classes of pitfalls.

**Delivers:** Scanning engine with hardcoded safety exclusions (SIP paths, /private/var/vm/), basic rule system, dry-run reporting

**Addresses:**
- System cache cleaning (must-have from FEATURES.md)
- Safe deletion (table stakes)
- Dry run mode (table stakes)
- Size reporting (table stakes)

**Avoids:**
- Pitfall 1: SIP-protected deletion (hardcoded exclusions)
- Pitfall 2: Swap file deletion (VM directory exclusion)
- Pitfall 5: SSV modification (Data volume only)

**Architecture components:**
- Types & Core Models (phase 1 in build order)
- Safety Layer (SIP checker, risk assessor, path validator)
- Rule System (category-based rules for system caches)
- Scanner (file system scanning, rule matching)
- Reporter (size calculation, output formatting)

**Research flags:** Standard patterns, skip research-phase. File system operations and safety checks well-documented in Go stdlib and macOS documentation.

---

### Phase 2: Multi-Category Cleaning
**Rationale:** Once safe scanning works, expand to remaining categories (browser, developer, app leftovers). These share the same scanning architecture but require category-specific rules and risk assessment.

**Delivers:** Browser cache rules, developer cache rules (Xcode, npm, yarn), app leftover detection, interactive mode with category selection

**Addresses:**
- Browser cache cleaning (must-have)
- Developer cache cleaning (must-have)
- App leftover removal (must-have)
- Interactive prompts (must-have)
- Homebrew integration (differentiator)
- Smart categorization by risk (differentiator)

**Avoids:**
- Pitfall 3: Cache misclassification (category-specific risk levels)
- Pitfall 7: False positives (conservative rules per category)

**Architecture components:**
- Additional Rule definitions (browser-data, dev-caches, app-leftovers)
- UI Components (prompts, progress indicators)
- Cleaner Layer (Trash integration, external tool adapters)
- Orchestrator (workflow coordination)

**Research flags:** Might need research-phase for browser cache paths (version-specific locations) and Xcode cache safety (which DerivedData is safe to delete). Otherwise standard patterns.

---

### Phase 3: CLI Polish & Automation
**Rationale:** With core cleaning working safely, add CLI-specific differentiators (JSON output, granular flags) that enable automation and AI agent consumption. This is competitive advantage vs GUI tools.

**Delivers:** JSON output format, granular skip flags (--skip-browser, --skip-xcode), flag-based mode (non-interactive), exit codes for scripting

**Addresses:**
- JSON output (key differentiator from FEATURES.md)
- Granular skip flags (differentiator)
- Path-specific reporting (differentiator)

**Uses:**
- encoding/json/v2 (from STACK.md, Feb 2026 release)
- go-isatty for terminal detection
- Cobra's persistent flags for --skip-* options

**Architecture components:**
- CLI Entry (argument parsing, mode detection)
- Mode handlers (interactive vs flags)
- JSON formatters (machine-readable output)

**Research flags:** Standard patterns, skip research-phase. CLI flag handling and JSON serialization well-established in Go ecosystem.

---

### Phase 4: Distribution & Trust
**Rationale:** Defer distribution complexity until core functionality proven. Code signing and Homebrew tap important for user trust but not required for initial validation.

**Delivers:** GoReleaser configuration, Homebrew tap, code signing with Apple Developer ID, notarization via xcrun notarytool, universal binary (arm64 + x86_64)

**Addresses:**
- Easy installation via Homebrew
- Distribution trust (no "unidentified developer" warnings)
- Apple Silicon + Intel support

**Uses:**
- GoReleaser v2 for multi-arch builds
- Apple Developer ID certificate ($99/year program)
- xcrun notarytool for notarization

**Architecture components:**
- Build configuration
- Release automation
- Distribution packaging

**Research flags:** Might need research-phase for code signing/notarization workflow (requires Apple Developer account, certificates, specific tool flags). Otherwise standard GoReleaser patterns.

---

### Phase Ordering Rationale

**Why Safety First (Phase 1):**
- Research shows catastrophic failures (kernel panic, system breakage) happen when safety is afterthought
- SIP exclusions, swap file protection, SSV awareness must be in foundation
- Cannot safely test deletion without safety checks in place

**Why Multi-Category Before CLI Polish (Phase 2 before 3):**
- Validates architecture scales across different rule types
- Interactive mode user testing informs automation needs
- Category-specific risks (browser vs developer caches) affect JSON output design

**Why Distribution Last (Phase 4):**
- Requires Apple Developer account ($99/year) — validate product first
- Code signing/notarization complex but not needed for local testing
- Homebrew tap creation straightforward once binary stable

**Dependency Chain:**
- Phase 1 → Phase 2: Safety layer required before expanding categories
- Phase 2 → Phase 3: Interactive mode tested before automation flags
- Phase 3 → Phase 4: Stable CLI required before distribution packaging

**Pitfall Avoidance:**
- Critical pitfalls (1, 2, 5) addressed in Phase 1 foundation
- Cache classification pitfalls (3, 7, 8) addressed as rules added in Phase 2
- Permission pitfall (6) addressed throughout (minimize FDA requests)
- APFS purgeable space pitfall (4) addressed in Phase 3 reporting

### Research Flags

**Phases likely needing deeper research during planning:**

- **Phase 2:** Browser cache paths may be version-specific (Safari 18 vs 17, Chrome updates). Need to verify current macOS 15 Sequoia locations. Xcode DerivedData safety (which subdirectories safe vs risky) needs developer-focused validation.

- **Phase 4:** Code signing workflow (Apple Developer ID certificate setup, codesign flags, notarization process with xcrun notarytool) is procedural but poorly documented for CLI tools. May need research-phase for distribution best practices.

**Phases with standard patterns (skip research-phase):**

- **Phase 1:** File system operations (Go stdlib io/fs well-documented), SIP paths (hardcoded list from official Apple docs), safety checks (established patterns).

- **Phase 3:** CLI flag handling (Cobra framework standard), JSON output (encoding/json/v2 official docs), terminal detection (go-isatty established).

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | Go for CLI tools is industry standard (kubectl, docker, hugo use Cobra). Verified through official Go docs, JetBrains ecosystem analysis 2025, multiple comparison sources. GoReleaser for distribution is established pattern. |
| Features | **MEDIUM** | Feature list validated through competitive analysis (CleanMyMac X, OnyX, Pearcleaner) and user expectations, but specific cache paths may vary by macOS version. Must-have vs differentiator categorization based on multiple cleaner tool comparisons. |
| Architecture | **MEDIUM-HIGH** | Layered architecture pattern verified across 3 real-world implementations (mac-cleanup-go, MacCleanCLI, mac-cleanup-py). Component boundaries and build order logical, but specific concurrency limits (p-limit=8) may need tuning. |
| Pitfalls | **MEDIUM** | SIP/SSV/swap file risks verified through official Apple documentation (high confidence). Cache classification risks based on community consensus and CleanMyMac documentation (medium confidence). APFS purgeable space issue documented in DaisyDisk guide. |

**Overall confidence:** MEDIUM-HIGH

Research is solid for Go stack choice (HIGH), core architecture patterns (MEDIUM-HIGH), and critical safety pitfalls (SIP, swap files). Lower confidence on cache classification nuances and browser path locations which may need validation during Phase 2 implementation.

### Gaps to Address

**Cache safety classification:** Research identifies general categories (system caches safe, authentication tokens unsafe) but lacks per-app specifics. For example:
- Which Xcode caches are safe to delete without breaking builds? (DerivedData vs user data)
- Which browser caches can be deleted while browser is running? (potential corruption)
- Which Application Support subdirectories contain licenses vs regenerable caches?

**How to handle:** Use conservative classification in Phase 1 (only delete obviously safe caches), gather user feedback in Phase 2 about what breaks, refine rules in iterations.

**APFS space reporting accuracy:** Research identifies purgeable space confusion but doesn't provide exact algorithm for separating purgeable vs truly reclaimable space.

**How to handle:** Use `diskutil apfs list` to query APFS metadata, cross-reference with scan results, report both numbers separately ("X GB will be freed, Y GB already purgeable by macOS").

**macOS version differences:** Research covers macOS 11+ (SSV), but cache paths and system behavior may vary across 11/12/13/14/15.

**How to handle:** Test on multiple macOS versions during Phase 2, document per-version differences if found, use runtime version detection (syscall.Uname) to handle version-specific logic.

**Full Disk Access minimization:** Research recommends minimizing FDA requests but doesn't specify which exact paths require it vs standard permissions.

**How to handle:** Test cleaning without FDA first, identify which specific operations fail, document minimum required permissions, provide graceful degradation (skip FDA-only paths if permission denied).

## Sources

### Primary (HIGH confidence)
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26) — Latest Go version features
- [Cobra GitHub](https://github.com/spf13/cobra) v1.10.2 — CLI framework documentation
- [Go Solutions: CLIs](https://go.dev/solutions/clis) — Official Go CLI guidance
- [Apple: System Integrity Protection](https://support.apple.com/en-us/102149) — SIP documentation
- [Apple: Signed system volume security](https://support.apple.com/guide/security/secd698747c9/web) — SSV architecture
- [path/filepath Go Package](https://pkg.go.dev/path/filepath) — Stdlib file operations
- [io/fs Go Package](https://pkg.go.dev/io/fs) — Modern filesystem interface

### Secondary (MEDIUM confidence)
- [mac-cleanup-go GitHub](https://github.com/2ykwang/mac-cleanup-go) — Real-world TUI cleaner implementation
- [MacCleanCLI GitHub](https://github.com/QDenka/MacCleanCLI) — Python tool architecture reference
- [mac-cleanup-py GitHub](https://github.com/mac-cleanup/mac-cleanup-py) — Modular plugin architecture
- [Best Mac Cleaner Software 2026](https://thesweetbits.com/best-mac-cleaner-software/) — Competitive feature analysis
- [CleanMyMac X Features](https://cleanmymac.com/) — Commercial leader baseline
- [Pearcleaner GitHub](https://github.com/alienator88/Pearcleaner) — FOSS alternative features
- [Library/Caches on Mac](https://iboysoft.com/wiki/library-caches-mac.html) — Cache location documentation
- [Browser Cache Paths](https://echoone.com/filejuicer/formats/cache) — Safari/Chrome/Firefox locations
- [Xcode Cache Cleaning](https://macpaw.com/how-to/clear-xcode-cache) — Developer cache paths
- [DaisyDisk: Local APFS snapshots](https://daisydiskapp.com/guide/4/en/Snapshots/) — Purgeable space explanation
- [macOS SIP - HackTricks](https://book.hacktricks.wiki/en/macos-hardening/macos-security-and-privilege-escalation/macos-security-protections/macos-sip.html) — SIP protection paths
- [GoReleaser Documentation](https://goreleaser.com/) — Distribution tool documentation
- [Command Line Interface Guidelines](https://clig.dev/) — CLI design patterns

### Tertiary (LOW confidence, needs validation)
- [Homebrew cleanup documentation](https://docs.brew.sh/Manpage) — External tool integration
- [Full Disk Access explained](https://macpaw.com/how-to/full-disk-access) — Permission requirements
- [Rebuild Spotlight index](https://support.apple.com/en-us/102321) — Cache deletion consequences

---
*Research completed: 2026-02-16*
*Ready for roadmap: yes*
