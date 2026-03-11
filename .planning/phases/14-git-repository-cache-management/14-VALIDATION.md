---
phase: 14
slug: git-repository-cache-management
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 14-01-01 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestScanFindsGitDirs` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestSkipSubmodule` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 1 | GIT-01 | unit | `go test ./pkg/gitrepos/... -run TestSkipLockedRepo` | ❌ W0 | ⬜ pending |
| 14-01-04 | 01 | 1 | GIT-02 | unit | `go test ./pkg/gitrepos/... -run TestParseCountObjects` | ❌ W0 | ⬜ pending |
| 14-01-05 | 01 | 1 | GIT-02 | unit | `go test ./pkg/gitrepos/... -run TestDescriptionFormat` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 1 | GIT-03 | unit | `go test ./internal/cleanup/... -run TestGitGCPseudoPath` | ❌ W0 | ⬜ pending |
| 14-02-02 | 02 | 1 | GIT-03 | unit | `go test ./pkg/gitrepos/... -run TestGitGCCommand` | ❌ W0 | ⬜ pending |
| 14-02-03 | 02 | 1 | GIT-04 | unit | `go test ./pkg/gitrepos/... -run TestLFSDetection` | ❌ W0 | ⬜ pending |
| 14-02-04 | 02 | 1 | GIT-04 | unit | `go test ./pkg/gitrepos/... -run TestLFSPruneCommand` | ❌ W0 | ⬜ pending |
| 14-xx-xx | -- | 1 | -- | unit | `go test ./pkg/gitrepos/... -run TestGitNotInstalled` | ❌ W0 | ⬜ pending |
| 14-xx-xx | -- | 1 | -- | unit | `go test ./internal/safety/... -run TestRiskForCategory` | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `pkg/gitrepos/scanner_test.go` — stubs for GIT-01, GIT-02, GIT-04 tests
- [ ] `internal/cleanup/cleanup_test.go` — extended with git-gc: dispatch tests for GIT-03
- [ ] `internal/safety/risk.go` — add `git-repos` category risk entry

*Existing infrastructure covers framework and fixture needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CLI `--git-repos` flag works end-to-end | GIT-01 | Full CLI integration | Run `go build -o mac-cleaner . && ./mac-cleaner --git-repos --dry-run` and verify git repos listed |
| `git gc --aggressive` actually frees space | GIT-03 | Requires real git repo with garbage | Create repo with loose objects, run cleanup, verify size decreased |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
