# Feature Landscape: macOS Disk Cleaning Tools

**Domain:** macOS Disk Cleaning & System Maintenance
**Researched:** 2026-02-16
**Confidence:** MEDIUM

## Table Stakes

Features users expect. Missing = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| System cache cleaning | Every Mac cleaner has this; users expect GB of space back | Low | ~/Library/Caches and /Library/Caches are standard targets |
| User cache cleaning | Users download apps expecting their caches to be found | Low | App-specific caches in ~/Library/Caches/[bundle.id] |
| Browser cache cleaning | Multi-browser support is table stakes (Safari, Chrome, Firefox) | Low | Well-documented paths per browser |
| Dry run mode | Users fear data loss; need to preview before delete | Low | Show what would be deleted without deleting |
| Interactive prompts | Users want control over what gets deleted | Low | Ask before deleting each category |
| Size reporting | Users need to see space savings to justify using tool | Low | Calculate and display sizes before/after |
| Safe deletion | Never delete system-critical files | Medium | Requires careful path filtering and validation |
| App leftover removal | Uninstalled apps leave preferences/caches behind | Medium | Search ~/Library/Preferences, ~/Library/Application Support |
| Developer cache cleaning | Xcode DerivedData is massive and commonly cleaned | Low | ~/Library/Developer/Xcode/DerivedData is well-known |
| Log file cleaning | System and app logs accumulate over time | Low | ~/Library/Logs and /Library/Logs |

## Differentiators

Features that set product apart. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| JSON output | Enables automation & AI agent consumption | Low | Machine-readable output for programmatic use |
| Granular skip flags | Power users want fine control (--skip-browser --skip-xcode) | Low | Flag per category, easy to implement |
| Path-specific reporting | Show exactly which paths were cleaned, not just totals | Low | Transparency builds trust |
| Multiple package manager support | Clean npm, yarn, pip, brew caches | Medium | Developer-focused, high value for target audience |
| Universal binary stripping | Remove unused architectures (Intel/ARM) from apps | High | Reduces app size 30-50%, requires binary manipulation |
| Language file pruning | Remove unused localizations from apps | High | Can save hundreds of MB, requires app bundle knowledge |
| Smart categorization | Group findings by risk level (safe/moderate/expert) | Medium | Helps users make informed decisions |
| iOS backup cleaning | Old iPhone/iPad backups consume massive space | Low | ~/Library/Application Support/MobileSync/Backup/ |
| Mail attachment cleanup | Mail.app can accumulate GB of attachments | Medium | ~/Library/Mail/V*/MailData/Envelope Index* |
| QuickLook cache cleaning | Thumbnail caches can grow large, privacy risk | Medium | /private/var/folders/*/C/com.apple.QuickLook.thumbnailcache/ |
| Homebrew integration | Clean brew cache, outdated downloads | Low | High value for developers using Homebrew |
| Duplicate file detection | Find and remove duplicate files | High | CPU intensive, requires file hashing |
| Download folder cleanup | Auto-clean old downloads by age | Low | Users often forget about ~/Downloads |
| Plugin/extension removal | Find orphaned plugins and services | Medium | ~/Library/Services, ~/Library/Plugins paths |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| System file cleaning | /System/Library/Caches deletion can break macOS | Only clean user-space files (~/ prefix) |
| Aggressive log deletion | Deleting system logs can break diagnostics | Only clean app logs, leave system logs alone |
| Automatic daily cleaning | Can surprise users with data loss | Require explicit user action, never auto-delete |
| /private/var/folders cleanup | Temp files here may be in use, can break apps | Skip this directory entirely despite size |
| Time Machine snapshot deletion | macOS manages these automatically, manual deletion risky | Report existence but don't offer to delete |
| No confirmation mode | Deleting without preview creates support burden | Always show what will be deleted first |
| GUI wrapper | Adds complexity for CLI tool, reduces scriptability | Stay CLI-focused, let others build GUI if needed |
| Deep system optimization | "Speed up Mac" claims are snake oil | Focus on disk space only, no performance claims |
| Cloud storage cleanup | Deleting from Dropbox/iCloud is destructive | Report size but don't delete cloud files |
| Kernel extension cleanup | Too risky, can break system | Document location but don't touch |

## Feature Dependencies

```
Interactive Mode
    └──requires──> Size Reporting
    └──requires──> Dry Run Mode

JSON Output
    └──conflicts──> Interactive Mode (mutually exclusive)

Granular Skip Flags
    └──requires──> Category Detection
    └──enhances──> Dry Run Mode

Universal Binary Stripping
    └──requires──> App Bundle Parsing
    └──requires──> Architecture Detection

Language File Pruning
    └──requires──> App Bundle Parsing
    └──enhances──> Universal Binary Stripping (same code paths)

Mail Attachment Cleanup
    └──requires──> Mail.app Version Detection

Homebrew Integration
    └──requires──> Homebrew Installation Detection
    └──enhances──> Developer Cache Cleaning

Safe Deletion
    └──requires──> Path Validation
    └──required-by──> ALL cleaning features
```

## MVP Recommendation

### Launch With (v1.0)

Prioritize:
1. System cache cleaning (~/Library/Caches, /Library/Caches)
2. User cache cleaning (app-specific)
3. Browser cache cleaning (Safari, Chrome, Firefox)
4. Developer cache cleaning (Xcode DerivedData, npm, yarn)
5. App leftover removal (preferences, application support)
6. Dry run mode (--dry-run)
7. Interactive mode (prompt per category)
8. JSON output (--json)
9. Granular skip flags (--skip-*)
10. Safe deletion (path validation)
11. Size reporting (before/after)

**Rationale:** These features cover the 4 categories mentioned in project context (system caches, app leftovers, developer caches, browser data) with appropriate safety mechanisms.

### Add After Validation (v1.x)

- Log file cleaning (easy win for space)
- iOS backup cleaning (high impact, simple implementation)
- Download folder cleanup (user convenience)
- Homebrew cache cleaning (developer audience value)
- Mail attachment cleanup (high space savings for some users)

**Trigger:** User feedback requesting these specific features.

### Future Consideration (v2+)

- Universal binary stripping (complex, high value)
- Language file pruning (complex, moderate value)
- QuickLook cache cleaning (privacy angle, moderate value)
- Plugin/extension removal (lower priority)
- Duplicate file detection (CPU intensive, separate feature class)

**Why defer:** These require significant engineering effort (binary manipulation, file hashing) and should be validated separately after core product is proven.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| System cache cleaning | HIGH | LOW | P1 |
| Browser cache cleaning | HIGH | LOW | P1 |
| Developer cache cleaning | HIGH | LOW | P1 |
| App leftover removal | HIGH | MEDIUM | P1 |
| Dry run mode | HIGH | LOW | P1 |
| Interactive mode | HIGH | LOW | P1 |
| JSON output | HIGH | LOW | P1 |
| Granular skip flags | HIGH | LOW | P1 |
| Safe deletion | HIGH | MEDIUM | P1 |
| Size reporting | HIGH | LOW | P1 |
| Log file cleaning | MEDIUM | LOW | P2 |
| iOS backup cleaning | HIGH | LOW | P2 |
| Download folder cleanup | MEDIUM | LOW | P2 |
| Homebrew cache cleaning | HIGH | LOW | P2 |
| Mail attachment cleanup | MEDIUM | MEDIUM | P2 |
| QuickLook cache cleaning | LOW | MEDIUM | P3 |
| Plugin/extension removal | LOW | MEDIUM | P3 |
| Universal binary stripping | MEDIUM | HIGH | P3 |
| Language file pruning | LOW | HIGH | P3 |
| Duplicate file detection | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (MVP)
- P2: Should have, add when possible (v1.x)
- P3: Nice to have, future consideration (v2+)

## macOS Paths Reference

### System Caches
```
~/Library/Caches/                          # User app caches
/Library/Caches/                           # System app caches
AVOID: /System/Library/Caches/             # System-critical caches
```

### Browser Data
```
# Safari
~/Library/Caches/com.apple.Safari/
~/Library/Safari/LocalStorage/
~/Library/Safari/Databases/

# Chrome
~/Library/Caches/Google/Chrome/
~/Library/Application Support/Google/Chrome/Default/Cache/

# Firefox
~/Library/Caches/Firefox/Profiles/
~/Library/Application Support/Firefox/Profiles/*/cache2/
```

### Developer Caches
```
# Xcode
~/Library/Developer/Xcode/DerivedData/
~/Library/Developer/Xcode/Archives/
~/Library/Developer/Xcode/iOS DeviceSupport/
~/Library/Caches/com.apple.dt.Xcode/

# Node/npm
~/.npm/
~/Library/Caches/npm/

# Yarn
~/Library/Caches/Yarn/

# pip
~/Library/Caches/pip/

# Homebrew
~/Library/Caches/Homebrew/
```

### App Leftovers
```
~/Library/Preferences/                     # .plist files
~/Library/Application Support/             # App data
~/Library/LaunchAgents/                    # User launch agents
/Library/LaunchDaemons/                    # System launch daemons
/Library/Receipts/                         # Installation receipts (.pkg)
```

### Logs
```
~/Library/Logs/                            # User app logs
/Library/Logs/                             # System app logs
AVOID: /var/log/                           # System logs (expert only)
```

### Other
```
# iOS Backups
~/Library/Application Support/MobileSync/Backup/

# Mail Attachments
~/Library/Mail/V*/MailData/

# QuickLook Thumbnails
/private/var/folders/*/C/com.apple.QuickLook.thumbnailcache/

# Downloads
~/Downloads/

# Temporary Files
/private/tmp/                              # Auto-cleared on restart
/private/var/tmp/                          # Auto-cleared on restart
AVOID: /private/var/folders/               # In-use temp files, can break apps
```

### Time Machine (Report Only)
```
/.MobileBackups/                           # Local snapshots (older macOS)
# APFS snapshots on newer macOS are managed differently
```

## Competitor Feature Analysis

| Feature | CleanMyMac X | OnyX | Pearcleaner | DaisyDisk | Our Approach |
|---------|--------------|------|-------------|-----------|--------------|
| System cache cleaning | Yes (automated) | Yes (manual) | No | No (visualization only) | Yes (interactive + flags) |
| Browser cache cleaning | Yes | Yes | No | No | Yes |
| Developer cache cleaning | Partial | Yes | Yes (Homebrew) | No | Yes (comprehensive) |
| App uninstall | Yes | No | Yes | No | Yes (leftovers only) |
| Dry run mode | No | Preview in UI | Yes | N/A | Yes (--dry-run) |
| CLI interface | No | No | Partial | No | Yes (primary interface) |
| JSON output | No | No | No | No | Yes (AI agent friendly) |
| Granular control | UI checkboxes | Expert mode | Limited | N/A | Flags (--skip-*) |
| Universal binary stripping | Yes | No | Yes (Lipo tool) | No | Future (v2+) |
| Language file pruning | Yes | No | Yes | No | Future (v2+) |
| Malware detection | Yes | No | No | No | No (out of scope) |
| Storage visualization | Yes | No | No | Yes (primary feature) | No (CLI text only) |
| Pricing | $39.95/year | Free | Free | $9.99 one-time | Free & open source |

### Competitive Positioning

**vs CleanMyMac X:** We're the CLI/scriptable alternative. No GUI bloat, no subscription, perfect for automation and AI agents.

**vs OnyX:** We're safer and more user-friendly. OnyX gives expert access but can break your Mac if misused. We focus on safe operations only.

**vs Pearcleaner:** We complement it. Pearcleaner excels at app uninstallation, we excel at cache cleaning. Users might use both.

**vs DaisyDisk:** Different problem space. DaisyDisk visualizes, we clean. They help you find what's big, we help you delete the junk.

**Unique value:** Only CLI-first tool with JSON output, granular flags, and developer cache focus. Perfect for automation, AI agents, and power users who script their Mac maintenance.

## Sources

### Primary Research
- [CleanMyMac X Features](https://cleanmymac.com/) - Commercial leader, feature baseline
- [Best Mac Cleaner Software 2026](https://thesweetbits.com/best-mac-cleaner-software/) - Feature comparison
- [OnyX Alternative Tools](https://iboysoft.com/howto/onyx-alternative.html) - Free tool landscape
- [DaisyDisk Alternatives](https://www.drbuho.com/review/daisydisk-alternative) - Market positioning

### macOS Paths Documentation
- [Library/Caches on Mac](https://iboysoft.com/wiki/library-caches-mac.html) - Cache locations
- [Browser Cache Paths](https://echoone.com/filejuicer/formats/cache) - Safari, Chrome, Firefox locations
- [Xcode Cache Cleaning](https://macpaw.com/how-to/clear-xcode-cache) - Developer cache paths
- [App Leftover Files](https://app-cleaner.com/blog/delete-apps-and-their-leftovers) - Uninstaller paths
- [System Logs Location](https://iboysoft.com/wiki/mac-system-log-files.html) - Log file paths
- [iPhone Backups on Mac](https://support.apple.com/en-us/108809) - iOS backup location
- [Mail Cache Cleanup](https://cleanmymac.com/blog/clear-mail-cache-mac) - Mail.app paths
- [QuickLook Cache](https://appleinsider.com/inside/macos/tips/how-to-stop-and-disable-quick-look-cache-in-macos) - Thumbnail cache location

### Open Source Alternatives
- [Pearcleaner GitHub](https://github.com/alienator88/Pearcleaner) - Feature reference
- [Open Source CCleaner Alternatives](https://openalternative.co/alternatives/ccleaner) - FOSS landscape
- [AppCleaner vs Pearcleaner](https://www.drbuho.com/review/appcleaner-vs-pearcleaner) - Feature comparison

### Safety & Best Practices
- [macOS Cleaning Risks](https://macpaw.com/how-to/delete-junk-files-on-mac) - What not to delete
- [Temporary Files Safety](https://iboysoft.com/wiki/tmp-folder-mac.html) - Safe cleanup practices
- [Language Files Cleanup](https://macpaw.com/how-to/delete-language-files-from-mac-osx) - Localization removal
- [Time Machine Snapshots](https://support.apple.com/en-us/102154) - Backup management

### CLI Tools Research
- [Terminal Cleaning Commands](https://www.ticktechtold.com/clean-mac-terminal-commands/) - Command-line patterns
- [MacCleanCLI](https://github.com/QDenka/MacCleanCLI) - CLI tool reference
- [mac-cleanup-sh](https://github.com/mac-cleanup/mac-cleanup-sh) - Shell script approach

---
*Feature research for: mac-cleaner*
*Researched: 2026-02-16*
*Confidence: MEDIUM (WebSearch-verified, multiple sources, current year)*
