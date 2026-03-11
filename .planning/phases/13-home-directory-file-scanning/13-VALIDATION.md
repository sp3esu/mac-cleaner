---
phase: 13
slug: home-directory-file-scanning
status: complete
nyquist_compliant: true
wave_0_complete: true
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
| 13-01-01 | 01 | 1 | -- | unit | `go test ./internal/walk/... -run TestOnFileCallback` | ✅ | ✅ green |
| 13-01-02 | 01 | 1 | HOME-01 | unit | `go test ./pkg/homedir/... -run TestLargeFiles` | ✅ | ✅ green |
| 13-01-03 | 01 | 1 | HOME-02 | unit | `go test ./pkg/homedir/... -run TestOldFiles` | ✅ | ✅ green |
| 13-01-04 | 01 | 1 | HOME-03 | unit | `go test ./pkg/homedir/... -run TestExclusions` | ✅ | ✅ green |
| 13-01-05 | 01 | 1 | -- | unit | `go test ./pkg/homedir/... -run TestSymlinkSkip` | ✅ | ✅ green |
| 13-01-06 | 01 | 1 | -- | unit | `go test ./pkg/homedir/... -run TestDualCategory` | ✅ | ✅ green |
| 13-01-07 | 01 | 1 | -- | unit | `go test ./internal/safety/... -run TestRiskForCategory` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| iCloud placeholder files report on-disk size | -- | Requires iCloud setup | Enable iCloud, evict a file, run scan, verify reported size |

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

All 3 requirements (HOME-01 through HOME-03) have automated test coverage. All 8 scanner tests and 4 OnFile walk tests pass green.
