---
phase: 12
slug: project-cache-scanning
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` |
| **Config file** | None (Go convention) |
| **Quick run command** | `go test ./internal/walk/... ./pkg/projects/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/walk/... ./pkg/projects/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | PROJ-01 | unit | `go test ./pkg/projects/... -run TestScanNodeModules` | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | PROJ-02 | unit | `go test ./pkg/projects/... -run TestScanSwiftBuild` | ❌ W0 | ⬜ pending |
| 12-01-03 | 01 | 1 | PROJ-03 | unit | `go test ./pkg/projects/... -run TestScanRustTarget` | ❌ W0 | ⬜ pending |
| 12-01-04 | 01 | 1 | PROJ-04 | unit | `go test ./pkg/projects/... -run TestScanPythonVenv` | ❌ W0 | ⬜ pending |
| 12-01-05 | 01 | 1 | PROJ-05 | unit | `go test ./pkg/projects/... -run TestScanJavaBuild` | ❌ W0 | ⬜ pending |
| 12-01-06 | 01 | 1 | PROJ-06 | integration | `go test ./pkg/projects/... -run TestScanSinglePass` | ❌ W0 | ⬜ pending |
| 12-01-07 | 01 | 1 | PROJ-07 | unit | `go test ./pkg/projects/... -run TestStalenessMarker` | ❌ W0 | ⬜ pending |
| 12-01-08 | 01 | 1 | PROJ-08 | unit | `go test ./pkg/projects/... -run TestAggregateSummary` | ❌ W0 | ⬜ pending |
| 12-xx-01 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestMaxDepth` | ❌ W0 | ⬜ pending |
| 12-xx-02 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestExclusions` | ❌ W0 | ⬜ pending |
| 12-xx-03 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestCacheDirSkip` | ❌ W0 | ⬜ pending |
| 12-xx-04 | -- | 1 | -- | unit | `go test ./pkg/projects/... -run TestNoFalsePositive` | ❌ W0 | ⬜ pending |
| 12-xx-05 | -- | 1 | -- | unit | `go test ./pkg/projects/... -run TestProjectName` | ❌ W0 | ⬜ pending |
| 12-xx-06 | -- | 1 | -- | unit | `go test ./internal/safety/... -run TestRiskForCategory` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/walk/walk_test.go` — walker depth, exclusion, callback tests
- [ ] `pkg/projects/scanner_test.go` — all project type detection, staleness, aggregation tests

*Existing infrastructure covers safety/risk tests.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual staleness distinction in output | PROJ-07 | Formatting is visual | Run `mac-cleaner --project-caches` on a directory with old and new projects; verify stale ones show age annotation |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
