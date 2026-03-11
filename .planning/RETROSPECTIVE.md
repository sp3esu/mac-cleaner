# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.7.0 — Extended Scanning

**Shipped:** 2026-03-11
**Phases:** 3 | **Plans:** 6

### What Was Built
- Reusable walk utility (internal/walk) with OnDir/OnFile callbacks, depth limits, exclusions
- Project cache scanner: 5 project types (node_modules, .build, target/, venv, build/) with manifest validation
- Home directory file scanner: large files (≥100MB) and old files (≥180 days) with smart exclusions
- Git repository scanner: .git discovery, count-objects estimation, git gc --aggressive, git lfs prune
- Full CLI/IPC integration for all 3 new scanner groups with 6-language README updates

### What Worked
- Walk utility design paid off: built once in Phase 12, reused cleanly by Phases 13 and 14
- Consistent integration pattern from Phase 12-02 made subsequent CLI integrations fast and predictable
- TDD approach in Phase 13-01 caught edge cases early (OnFile callback nil-safety)
- All 3 phases completed in a single day with zero rework

### What Was Inefficient
- Missing VERIFICATION.md for all 3 phases — formal verification was skipped despite audit catching it
- Audit found 0/15 verified requirements despite all being functionally complete — process gap

### Patterns Established
- Walk callback pattern (OnDir + OnFile) as standard for new file-discovery scanners
- Manifest-validated detection pattern prevents false positives for common directory names
- git-gc pseudo-path dispatch pattern extends cleanup without new interfaces
- Single-item scanner group pattern (like unused-apps) avoids cobra flag collision

### Key Lessons
1. Shared utilities (walk) across phases should be designed with extension points from the start — OnFile was added in Phase 13 but could have been planned in Phase 12
2. Formal verification should be part of phase execution, not a separate step that gets skipped
3. Consistent integration patterns (flag registration, engine registry, README updates) dramatically reduce per-phase overhead

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 | 7 | 13 | Initial build, established scanner pattern |
| v1.1 | 4 | 5 | Engine extraction, IPC server mode |
| v1.7.0 | 3 | 6 | Shared walk utility, consistent integration pattern |

### Top Lessons (Verified Across Milestones)

1. Consistent patterns across scanners (registration, flags, tests) compound in speed with each new scanner added
2. Safety-first architecture (SIP blocking, risk levels, confirmation) has never caused a regression
