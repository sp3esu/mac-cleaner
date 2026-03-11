---
phase: 14
slug: git-repository-cache-management
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-11
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` |
| **Config file** | None (Go convention) |
| **Quick run command** | `go test ./pkg/gitrepos/... ./internal/cleanup/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/gitrepos/... ./internal/cleanup/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestScanFindsGitDirs` | ✅ | ✅ green |
| 14-01-02 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestSkipSubmodule` | ✅ | ✅ green |
| 14-01-03 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestSkipLockedRepo` | ✅ | ✅ green |
| 14-01-04 | 01 | 1 | GIT-02 | unit | `go test ./pkg/gitrepos/... -run TestParseCountObjects` | ✅ | ✅ green |
| 14-01-05 | 01 | 1 | GIT-02 | unit | `go test ./pkg/gitrepos/... -run TestDescriptionFormat` | ✅ | ✅ green |
| 14-02-01 | 02 | 1 | GIT-03 | unit | `go test ./internal/cleanup/... -run TestGitGCPseudoPath` | ✅ | ✅ green |
| 14-02-02 | 02 | 1 | GIT-03 | unit | `go test ./internal/cleanup/... -run TestGitGCPseudoPath_Failure` | ✅ | ✅ green |
| 14-02-03 | 02 | 1 | GIT-04 | unit | `go test ./pkg/gitrepos/... -run TestLFSDetection` | ✅ | ✅ green |
| 14-02-04 | 02 | 1 | GIT-04 | unit | `go test ./pkg/gitrepos/... -run TestDescriptionFormat_WithLFS` | ✅ | ✅ green |
| 14-02-05 | 02 | 1 | GIT-04 | unit | `go test ./internal/cleanup/... -run TestGitGCWithLFS` | ✅ | ✅ green |
| 14-xx-01 | -- | 1 | -- | unit | `go test ./pkg/gitrepos/... -run TestGitNotInstalled` | ✅ | ✅ green |
| 14-xx-02 | -- | 1 | -- | unit | `go test ./pkg/gitrepos/... -run TestRiskForCategory_GitRepos` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CLI `--git-repos` flag works end-to-end | GIT-01 | Full CLI integration | Run `go build -o mac-cleaner . && ./mac-cleaner --git-repos --dry-run` and verify git repos listed |
| `git gc --aggressive` actually frees space | GIT-03 | Requires real git repo with garbage | Create repo with loose objects, run cleanup, verify size decreased |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-03-11

---

## Validation Audit 2026-03-11

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 4 requirements (GIT-01 through GIT-04) have automated test coverage. All 11 scanner tests and 3 cleanup tests pass green.
