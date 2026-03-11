# Roadmap: mac-cleaner

## Milestones

- v1.0 MVP - Phases 1-7 (shipped 2026-02-16)
- v1.1 Swift Integration - Phases 8-11 (shipped 2026-02-17)
- **v1.7.0 Extended Scanning** - Phases 12-14 (in progress)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

### v1.7.0 Extended Scanning

- [x] **Phase 12: Project Cache Scanning** - Detect and report project build caches (node_modules, .build, target/, venv, build/) across home directory (completed 2026-03-11)
- [x] **Phase 13: Home Directory File Scanning** - Detect large and old files consuming space in user home directory (completed 2026-03-11)
- [ ] **Phase 14: Git Repository Cache Management** - Detect large git repos and offer git gc/lfs prune as cleanup actions

## Phase Details

### Phase 12: Project Cache Scanning
**Goal**: Users can discover how much disk space is consumed by project build caches across all their development projects
**Depends on**: Nothing (first phase of v1.7.0; builds on existing scanner infrastructure from v1.1)
**Requirements**: PROJ-01, PROJ-02, PROJ-03, PROJ-04, PROJ-05, PROJ-06, PROJ-07, PROJ-08
**Success Criteria** (what must be TRUE):
  1. User can run `mac-cleaner --project-caches` and see node_modules, .build, target/, venv/.venv, and build/ directories with their sizes
  2. All project types are detected in a single scan pass without re-walking the home directory per type
  3. Projects not modified in 6+ months are visually distinguished from recent projects in scan output
  4. Aggregate summaries show total count and combined size per project type (e.g., "47 node_modules totaling 28.3 GB")
  5. Only directories with matching project manifest markers (package.json, Cargo.toml, etc.) are reported -- no false positives from unrelated directories with the same name
**Plans:** 2/2 plans complete

Plans:
- [x] 12-01-PLAN.md — Walk utility and project cache scanner core (internal/walk + pkg/projects + safety risk entries)
- [x] 12-02-PLAN.md — CLI integration (engine registry + cmd flags + README updates)

### Phase 13: Home Directory File Scanning
**Goal**: Users can identify large and stale files in their home directory that are consuming space
**Depends on**: Phase 12 (reuses shared walk utility built in Phase 12)
**Requirements**: HOME-01, HOME-02, HOME-03
**Success Criteria** (what must be TRUE):
  1. User can run `mac-cleaner --home-files` and see files larger than 100MB in their home directory
  2. User can see files older than 180 days in their home directory
  3. Results exclude ~/Library, ~/Applications, ~/.Trash, ~/Downloads, and dotfile directories -- no overlap with existing scanners
**Plans:** 2/2 plans complete

Plans:
- [ ] 13-01-PLAN.md — Walk OnFile extension and home directory file scanner (TDD: internal/walk + pkg/homedir + safety risk entries)
- [ ] 13-02-PLAN.md — CLI integration (engine registry + cmd flags + README updates)

### Phase 14: Git Repository Cache Management
**Goal**: Users can reclaim space from git repository caches through git gc and lfs prune operations
**Depends on**: Phase 12 (reuses walk utility; extends existing developer scanner pattern)
**Requirements**: GIT-01, GIT-02, GIT-03, GIT-04
**Success Criteria** (what must be TRUE):
  1. User can run `mac-cleaner --git-repos` and see .git directories with their sizes
  2. Each detected repo shows estimated reclaimable space from `git count-objects`
  3. User can clean up git repos via `git gc --aggressive` as the cleanup action
  4. Repos with .git/lfs present offer `git lfs prune` as an additional cleanup action
**Plans:** 1/2 plans executed

Plans:
- [ ] 14-01-PLAN.md — Git repo scanner core + cleanup extension (pkg/gitrepos + internal/cleanup git-gc: handler + safety risk)
- [ ] 14-02-PLAN.md — CLI integration (engine registry + cmd flags + README updates)

## Progress

**Execution Order:**
Phases execute in numeric order: 12 -> 13 -> 14

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-7 | v1.0 MVP | 13/13 | Complete | 2026-02-16 |
| 8-11 | v1.1 Swift Integration | 5/5 | Complete | 2026-02-17 |
| 12. Project Cache Scanning | 2/2 | Complete    | 2026-03-11 | - |
| 13. Home Directory File Scanning | 2/2 | Complete    | 2026-03-11 | - |
| 14. Git Repository Cache Management | 1/2 | In Progress|  | - |
