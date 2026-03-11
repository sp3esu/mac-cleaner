---
phase: 12
slug: project-cache-scanning
status: complete
nyquist_compliant: true
wave_0_complete: true
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
| 12-01-01 | 01 | 1 | PROJ-01 | unit | `go test ./pkg/projects/... -run TestScanNodeModules` | ✅ | ✅ green |
| 12-01-02 | 01 | 1 | PROJ-02 | unit | `go test ./pkg/projects/... -run TestScanSwiftBuild` | ✅ | ✅ green |
| 12-01-03 | 01 | 1 | PROJ-03 | unit | `go test ./pkg/projects/... -run TestScanRustTarget` | ✅ | ✅ green |
| 12-01-04 | 01 | 1 | PROJ-04 | unit | `go test ./pkg/projects/... -run TestScanPythonVenv` | ✅ | ✅ green |
| 12-01-05 | 01 | 1 | PROJ-05 | unit | `go test ./pkg/projects/... -run TestScanJavaBuild` | ✅ | ✅ green |
| 12-01-06 | 01 | 1 | PROJ-06 | integration | `go test ./pkg/projects/... -run TestSinglePassWalk` | ✅ | ✅ green |
| 12-01-07 | 01 | 1 | PROJ-07 | unit | `go test ./pkg/projects/... -run TestStalenessMarker` | ✅ | ✅ green |
| 12-01-08 | 01 | 1 | PROJ-08 | unit | `go test ./pkg/projects/... -run TestAggregateSummary` | ✅ | ✅ green |
| 12-xx-01 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestMaxDepth` | ✅ | ✅ green |
| 12-xx-02 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestExcludeByName` | ✅ | ✅ green |
| 12-xx-03 | -- | 1 | -- | unit | `go test ./internal/walk/... -run TestDotfileDirs` | ✅ | ✅ green |
| 12-xx-04 | -- | 1 | -- | unit | `go test ./pkg/projects/... -run TestNoFalsePositive` | ✅ | ✅ green |
| 12-xx-05 | -- | 1 | -- | unit | `go test ./pkg/projects/... -run TestProjectNameFromPackageJson` | ✅ | ✅ green |
| 12-xx-06 | -- | 1 | -- | unit | `go test ./internal/safety/... -run TestRiskForCategory` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual staleness distinction in output | PROJ-07 | Formatting is visual | Run `mac-cleaner --project-caches` on a directory with old and new projects; verify stale ones show age annotation |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-03-11

---

## Validation Audit 2026-03-11

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 8 requirements (PROJ-01 through PROJ-08) have automated test coverage. All 14 scanner tests and 10 walk tests pass green.
