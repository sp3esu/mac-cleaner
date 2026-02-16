# Requirements: mac-cleaner

**Defined:** 2026-02-16
**Core Value:** Users can safely and confidently reclaim disk space without worrying about deleting something important

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### System Cleaning

- [ ] **SYS-01**: User can scan and remove user app caches from ~/Library/Caches
- [ ] **SYS-02**: User can scan and remove user app logs from ~/Library/Logs
- [ ] **SYS-03**: User can scan and remove QuickLook thumbnail cache

### Browser Data

- [ ] **BRWS-01**: User can scan and remove Safari cache
- [ ] **BRWS-02**: User can scan and remove Chrome cache
- [ ] **BRWS-03**: User can scan and remove Firefox cache

### Developer Caches

- [ ] **DEV-01**: User can scan and remove Xcode DerivedData
- [ ] **DEV-02**: User can scan and remove npm/yarn cache
- [ ] **DEV-03**: User can scan and remove Homebrew cache
- [ ] **DEV-04**: User can scan and remove Docker unused images/containers/volumes

### App Leftovers

- [ ] **APP-01**: User can scan and remove orphaned preferences from uninstalled apps
- [ ] **APP-02**: User can scan and remove old iOS device backups
- [ ] **APP-03**: User can scan and remove old files from ~/Downloads (by age)

### CLI Interface

- [ ] **CLI-01**: User can preview cleanup with `--dry-run` (no files deleted)
- [ ] **CLI-02**: User can target all categories with `--all`
- [ ] **CLI-03**: User can run interactive mode (no args) that walks through each item
- [ ] **CLI-04**: User sees confirmation prompt before any deletion
- [ ] **CLI-05**: User sees summary after cleanup (items removed, space freed)
- [ ] **CLI-06**: User can get structured JSON output with `--json`
- [ ] **CLI-07**: User can get detailed file listing with `--verbose`
- [ ] **CLI-08**: User can skip categories with `--skip-<category>` flags
- [ ] **CLI-09**: User can skip specific items with `--skip-<item>` flags
- [ ] **CLI-10**: User can bypass confirmation with `--force` for automation

### Safety

- [ ] **SAFE-01**: Tool never touches SIP-protected paths (/System, /usr, /bin, /sbin)
- [ ] **SAFE-02**: Tool never touches swap files or sleepimage
- [ ] **SAFE-03**: Tool reports what it can't access (permission issues) without failing
- [ ] **SAFE-04**: Tool categorizes items by risk level (safe/moderate/risky)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Extended Cleaning

- **EXT-01**: User can scan and remove language/localization files from apps
- **EXT-02**: User can strip unused architectures from universal binaries
- **EXT-03**: User can scan and remove Mail.app attachment cache
- **EXT-04**: User can scan and remove duplicate files
- **EXT-05**: User can scan and remove Brave/Edge/Opera browser caches

### Extended Developer

- **EDEV-01**: User can scan and remove pip cache
- **EDEV-02**: User can scan and remove .gradle cache
- **EDEV-03**: User can scan and remove CocoaPods cache
- **EDEV-04**: User can scan and remove Xcode Archives and iOS DeviceSupport

### Distribution

- **DIST-01**: Tool distributed via Homebrew tap
- **DIST-02**: Tool code-signed with Apple Developer ID
- **DIST-03**: Tool notarized for macOS Gatekeeper

## Out of Scope

| Feature | Reason |
|---------|--------|
| Windows/Linux support | macOS only — focused scope |
| GUI interface | CLI only — scriptable, automatable |
| Real-time monitoring | Manual invocation only |
| Trash/undo support | Files permanently deleted (confirmation mitigates risk) |
| System performance optimization | Focus on disk space only, no "speed up Mac" claims |
| Cloud storage cleanup | Too destructive — Dropbox/iCloud deletion has remote consequences |
| Kernel extension cleanup | Too risky, can break system |
| Automatic scheduled cleaning | Can surprise users with data loss |
| Time Machine snapshot deletion | macOS manages automatically |
| Malware detection | Different problem domain |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SYS-01 | TBD | Pending |
| SYS-02 | TBD | Pending |
| SYS-03 | TBD | Pending |
| BRWS-01 | TBD | Pending |
| BRWS-02 | TBD | Pending |
| BRWS-03 | TBD | Pending |
| DEV-01 | TBD | Pending |
| DEV-02 | TBD | Pending |
| DEV-03 | TBD | Pending |
| DEV-04 | TBD | Pending |
| APP-01 | TBD | Pending |
| APP-02 | TBD | Pending |
| APP-03 | TBD | Pending |
| CLI-01 | TBD | Pending |
| CLI-02 | TBD | Pending |
| CLI-03 | TBD | Pending |
| CLI-04 | TBD | Pending |
| CLI-05 | TBD | Pending |
| CLI-06 | TBD | Pending |
| CLI-07 | TBD | Pending |
| CLI-08 | TBD | Pending |
| CLI-09 | TBD | Pending |
| CLI-10 | TBD | Pending |
| SAFE-01 | TBD | Pending |
| SAFE-02 | TBD | Pending |
| SAFE-03 | TBD | Pending |
| SAFE-04 | TBD | Pending |

**Coverage:**
- v1 requirements: 27 total
- Mapped to phases: 0
- Unmapped: 27

---
*Requirements defined: 2026-02-16*
*Last updated: 2026-02-16 after initial definition*
