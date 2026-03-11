# Requirements: mac-cleaner

**Defined:** 2026-03-11
**Core Value:** Users can safely and confidently reclaim disk space without worrying about deleting something important

## v1.7.0 Requirements

Requirements for extended scanning milestone. Each maps to roadmap phases.

### Project Caches

- [ ] **PROJ-01**: User can scan for node_modules directories under home with size reporting
- [ ] **PROJ-02**: User can scan for Swift .build directories (Package.swift marker) with size reporting
- [ ] **PROJ-03**: User can scan for Rust target/ directories (Cargo.toml marker) with size reporting
- [ ] **PROJ-04**: User can scan for Python venv/.venv directories (pyvenv.cfg marker) with size reporting
- [ ] **PROJ-05**: User can scan for Java/Kotlin build/ directories (build.gradle/pom.xml marker) with size reporting
- [ ] **PROJ-06**: All project types detected in a single-pass recursive walk of home directory
- [ ] **PROJ-07**: Stale projects (not modified in 6+ months) are highlighted in scan results
- [ ] **PROJ-08**: Category descriptions show aggregate summaries (e.g. "47 node_modules totaling 28.3 GB")

### Git Repository Caches

- [ ] **GIT-01**: User can scan for large .git directories with size reporting
- [ ] **GIT-02**: Scan shows estimated reclaimable space per repo via git count-objects
- [ ] **GIT-03**: User can run git gc --aggressive as cleanup action for detected repos
- [ ] **GIT-04**: User can run git lfs prune for repos with .git/lfs present

### Home Directory Files

- [ ] **HOME-01**: User can scan for large files (default 100MB+) in home directory
- [ ] **HOME-02**: User can scan for old files (default 180+ days) in home directory
- [ ] **HOME-03**: Scanner excludes ~/Library, ~/Applications, ~/.Trash, ~/Downloads, and dotfile directories to avoid overlap

## Future Requirements

### Configuration

- **CONF-01**: User can configure large file threshold via --large-file-threshold flag
- **CONF-02**: User can configure old file age via --old-file-age flag
- **CONF-03**: User can configure scan depth via --scan-depth flag

### Deferred Features

- **DEFER-01**: Duplicate file detection across home directory
- **DEFER-02**: __pycache__ directory cleanup
- **DEFER-03**: Aggregate summary dashboard across all categories

## Out of Scope

| Feature | Reason |
|---------|--------|
| Scanning inside ~/Library | Already covered by existing system/browser/developer scanners |
| Scanning .photoslibrary bundles | Already covered by photos scanner; modifying can corrupt |
| Manual .git/objects deletion | Corrupts repos; use git gc/prune commands instead |
| Deep recursive scan without depth limits | Multi-minute scans, high I/O; max depth 5-6 sufficient |
| Scanning /Volumes or network mounts | Different lifecycle; accidental deletion catastrophic |
| Git history rewriting (BFG, filter-branch) | Destructive, irreversible, affects remote collaborators |
| Scanning node_modules inside node_modules | Managed by package manager; only top-level matters |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PROJ-01 | Phase 12 | Pending |
| PROJ-02 | Phase 12 | Pending |
| PROJ-03 | Phase 12 | Pending |
| PROJ-04 | Phase 12 | Pending |
| PROJ-05 | Phase 12 | Pending |
| PROJ-06 | Phase 12 | Pending |
| PROJ-07 | Phase 12 | Pending |
| PROJ-08 | Phase 12 | Pending |
| GIT-01 | Phase 14 | Pending |
| GIT-02 | Phase 14 | Pending |
| GIT-03 | Phase 14 | Pending |
| GIT-04 | Phase 14 | Pending |
| HOME-01 | Phase 13 | Pending |
| HOME-02 | Phase 13 | Pending |
| HOME-03 | Phase 13 | Pending |

**Coverage:**
- v1.7.0 requirements: 15 total
- Mapped to phases: 15
- Unmapped: 0

---
*Requirements defined: 2026-03-11*
*Last updated: 2026-03-11 (traceability updated during roadmap creation)*
