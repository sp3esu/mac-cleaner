---
phase: 13
slug: home-directory-file-scanning
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` |
| **Config file** | None (Go convention) |
| **Quick run command** | `go test ./internal/walk/... ./pkg/homedir/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/walk/... ./pkg/homedir/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | -- | unit | `go test ./internal/walk/... -run TestOnFile` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | HOME-01 | unit | `go test ./pkg/homedir/... -run TestLargeFiles` | ❌ W0 | ⬜ pending |
| 13-01-03 | 01 | 1 | HOME-02 | unit | `go test ./pkg/homedir/... -run TestOldFiles` | ❌ W0 | ⬜ pending |
| 13-01-04 | 01 | 1 | HOME-03 | unit | `go test ./pkg/homedir/... -run TestExclusions` | ❌ W0 | ⬜ pending |
| 13-01-05 | 01 | 1 | -- | unit | `go test ./pkg/homedir/... -run TestSymlinkSkip` | ❌ W0 | ⬜ pending |
| 13-01-06 | 01 | 1 | -- | unit | `go test ./pkg/homedir/... -run TestDualCategory` | ❌ W0 | ⬜ pending |
| 13-01-07 | 01 | 1 | -- | unit | `go test ./internal/safety/... -run TestRiskForCategory` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/walk/walk_test.go` — add OnFile callback tests (file exists, needs new test functions)
- [ ] `pkg/homedir/scanner_test.go` — all file scanning tests (new file)
- [ ] `internal/safety/risk.go` — add `home-large-files` and `home-old-files` risk entries

*Existing infrastructure covers framework — Go stdlib `testing` is always available.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| iCloud placeholder files report on-disk size | -- | Requires iCloud setup | Enable iCloud, evict a file, run scan, verify reported size |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
