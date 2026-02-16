# Pitfalls Research

**Domain:** macOS Disk Cleaning CLI Tools
**Researched:** 2026-02-16
**Confidence:** MEDIUM (WebSearch-verified, multiple sources)

## Critical Pitfalls

### Pitfall 1: Deleting System Integrity Protection (SIP) Protected Directories

**What goes wrong:**
Tool attempts to delete files in `/System`, `/bin`, `/sbin`, `/usr` (except `/usr/local`), causing operation to fail with "operation not permitted" errors, or worse, breaking system boot if SIP is disabled during cleaning.

**Why it happens:**
Developers unfamiliar with macOS security model assume root/sudo access grants full filesystem control. SIP prevents modification of protected directories even with root privileges.

**How to avoid:**
- Hardcode SIP-protected paths into exclusion list: `/System/*`, `/bin/*`, `/sbin/*`, `/usr/*` (except `/usr/local`)
- Never suggest disabling SIP to users
- Test on SIP-enabled systems (default configuration)
- Check extended attribute `com.apple.rootless` before attempting deletion

**Warning signs:**
- "Operation not permitted" errors during cleaning
- User reports requiring Recovery Mode boot after cleaning
- Requests to disable SIP in documentation/issues

**Phase to address:**
Phase 1: Core Scanning - Build hardcoded SIP protection into path scanning logic from day one.

---

### Pitfall 2: Deleting Active Swap Files

**What goes wrong:**
Attempting to delete `/private/var/vm/swapfile*` while system is running causes kernel panic and immediate system crash. User loses all unsaved work.

**Why it happens:**
Swap files appear as large space consumers (can be GBs), making them tempting cleanup targets. Developers assume "if it's deletable, it's safe."

**How to avoid:**
- Never delete swap files (`/private/var/vm/swapfile*`)
- Never delete sleepimage while system is running (`/private/var/vm/sleepimage`)
- Exclude entire `/private/var/vm/` directory from cleaning operations
- Document: "Swap files are automatically managed by macOS and deleted on reboot"

**Warning signs:**
- Tool shows swap files as reclaimable space
- Kernel panics reported after cleaning
- Users report system crashes mid-cleanup

**Phase to address:**
Phase 1: Core Scanning - Hardcode `/private/var/vm/` exclusion in initial implementation.

---

### Pitfall 3: Deleting Caches Without Understanding Type/Impact

**What goes wrong:**
Blindly deleting all caches causes severe consequences:
- Spotlight re-index (hours of high CPU/disk usage)
- App re-authentication (users locked out of accounts)
- Developer tools breakage (Xcode won't build, can require macOS reinstall)
- Performance degradation until caches rebuild

**Why it happens:**
All caches look the same (just files in directories). Developers treat "cache" as universally safe to delete without understanding macOS cache hierarchy.

**How to avoid:**
**NEVER delete:**
- System caches: `/System/Library/Caches/*` (SIP-protected anyway)
- Spotlight index: `/private/var/db/Spotlight-V100/` or `.Spotlight-V100/`
- Authentication tokens: `~/Library/Application Support/*/Tokens/`
- Keychain caches: `~/Library/Keychains/`

**DELETE ONLY:**
- User app caches: `~/Library/Caches/[app-name]/*`
- Browser caches: `~/Library/Caches/com.apple.Safari/`
- Package manager caches: `~/Library/Caches/Homebrew/`, `~/.npm/`

**Require explicit user consent for:**
- Xcode caches: `~/Library/Developer/Xcode/DerivedData/` (can be 100GB+, but rebuilding is slow)
- Font caches: `~/Library/Caches/com.apple.FontRegistry/` (causes font rendering issues until rebuild)
- Photo library caches: Deleting causes re-generation of thumbnails (hours for large libraries)

**Warning signs:**
- Users report "Mac is slow after cleaning"
- Spotlight search not working after cleanup
- Apps requiring re-login after cleanup
- Developer reports Xcode build failures

**Phase to address:**
Phase 1: Core Scanning - Implement cache categorization (safe/risky/forbidden)
Phase 2: Smart Cleaning - Add risk warnings and consent UI for risky caches

---

### Pitfall 4: Misunderstanding Purgeable Space on APFS

**What goes wrong:**
Tool reports "X GB freed" but users see no change in available space. Users think tool is broken or scam. Root cause: purgeable space (local Time Machine snapshots, caches) is already automatically managed by macOS.

**Why it happens:**
APFS reports purgeable space separately. Developers count it as "reclaimable" without understanding macOS purges it automatically when needed.

**How to avoid:**
- Use `diskutil apfs list` to distinguish purgeable vs. truly deletable space
- Don't count local Time Machine snapshots (auto-deleted after 24h or when space needed)
- Report separately: "X GB freed + Y GB already purgeable by macOS"
- Document: "macOS automatically frees purgeable space when storage is low"
- Don't delete Time Machine local snapshots unless user explicitly requests (they enable quick restores)

**Warning signs:**
- Users report "cleaned X GB but Finder shows same available space"
- Confusion about why reported savings don't match reality
- Bad reviews claiming tool doesn't work

**Phase to address:**
Phase 2: Smart Cleaning - Implement proper APFS space reporting
Phase 3: Reporting - Clear communication about purgeable vs. freed space

---

### Pitfall 5: Ignoring Sealed System Volume (SSV) on Modern macOS

**What goes wrong:**
Tool attempts to modify System volume, fails silently or with cryptic errors. macOS Big Sur+ uses signed/sealed system volume that rejects all modifications.

**Why it happens:**
Developers test on older macOS versions or don't understand SSV architecture. System volume is read-only and cryptographically signed.

**How to avoid:**
- Understand APFS volume structure: System (sealed, read-only) vs. Data (writable)
- Never attempt to clean System volume
- All user-deletable content is on Data volume
- Use `diskutil apfs list` to identify volume types
- Only operate on Data volume and user directories

**Warning signs:**
- Errors when trying to delete from `/System/` on macOS 11+
- Tool works on Catalina but fails on Big Sur+
- Users report "permission denied" despite granting Full Disk Access

**Phase to address:**
Phase 1: Core Scanning - Implement SSV awareness, restrict operations to Data volume

---

### Pitfall 6: Requesting Full Disk Access Without Justification

**What goes wrong:**
Users distrust tool, refuse to grant permissions, or grant permissions without understanding risk. If tool is compromised (supply chain attack, malicious update), attacker has full system access.

**Why it happens:**
Developers request maximum permissions "just in case" rather than least-privilege approach.

**How to avoid:**
- Minimize permission requests - only ask for what's truly needed
- Most user caches don't require Full Disk Access
- Document exactly why each permission is needed
- Provide "basic mode" without Full Disk Access for cautious users
- Consider sandboxed distribution (Mac App Store) for trust

**Warning signs:**
- Tool requests Full Disk Access immediately on first run
- No explanation of why permissions are needed
- Users refuse to grant permissions
- Security-conscious users flag tool as suspicious

**Phase to address:**
Phase 1: Core Scanning - Design to minimize required permissions
Phase 3: Reporting - Transparent permission documentation

---

### Pitfall 7: Deleting Files with False Positives (Wrong File Classification)

**What goes wrong:**
Tool misidentifies important files as junk:
- Application preferences as "unused config"
- Project files as "temp files"
- User documents in uncommon locations as "junk"
- Plugin/extension files as "leftover files"

Result: User loses data, broken applications, requires restore from backup.

**Why it happens:**
Overly aggressive pattern matching ("anything in /tmp is safe", "*.log can be deleted", "if not accessed in 30 days, it's unused"). Edge cases not tested.

**How to avoid:**
- Conservative classification - bias toward NOT deleting
- Whitelist approach for known-safe patterns, not blacklist
- Never delete files based solely on:
  - Extension (.log, .tmp, .cache can be important)
  - Last access time (backup drives, seasonal projects)
  - Filename pattern (temp-* might be user's naming)
- Show preview before deletion with file paths visible
- Categorize by confidence: "Safe" vs "Review Recommended"
- Exclude common project directories: `*/node_modules/*/`, `*/.git/`, `*/build/` (user should clean these intentionally)

**Warning signs:**
- Users report broken apps after cleaning
- "I lost my files" support requests
- Need for "undo" functionality
- Requests for backup/restore features

**Phase to address:**
Phase 1: Core Scanning - Conservative classification rules
Phase 2: Smart Cleaning - Preview UI with clear categorization
Phase 4: Safety Features - Dry-run mode (already planned)

---

### Pitfall 8: Breaking App Functionality by Deleting "Application Support" Files

**What goes wrong:**
Deleting `~/Library/Application Support/[app]/` content breaks apps:
- Licenses/activation lost (must re-purchase or re-activate)
- Settings/preferences lost
- Plugin configurations lost
- Local databases corrupted

**Why it happens:**
"Application Support" sounds like it contains support files, not essential data. Directory can be large (GBs), making it tempting target.

**How to avoid:**
- Never delete `~/Library/Application Support/` without per-app knowledge
- Whitelist known-safe subdirectories (app-specific research required)
- Known risky areas:
  - `*/Adobe/` (licenses, activations)
  - `*/Steam/` (game installations)
  - Database files (*.sqlite, *.db)
- Only clean obvious caches: `*/Cache/`, `*/Caches/`, `*/HTTPCache/`
- Require explicit user approval for Application Support cleaning

**Warning signs:**
- Users report apps asking for license codes after cleaning
- "My app settings are gone" complaints
- Apps won't launch after cleanup

**Phase to address:**
Phase 2: Smart Cleaning - Application Support analysis with risk classification
Phase 4: Safety Features - Per-app cleaning with warnings

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Using simple file age for cleanup decisions | Easy to implement | False positives (deletes rarely-accessed but important files) | Never - too risky |
| Requesting Full Disk Access upfront | Access to all cleanable areas | User distrust, security risk | Only if truly needed and justified |
| Deleting entire cache directories | Maximum space savings | App breakage, performance issues | Only with explicit per-app knowledge |
| Using sudo/root permissions | Bypass permission checks | Security risk, can bypass SIP on older systems | Never - use proper user permissions |
| Hard-coding file paths | Fast implementation | Breaks on macOS updates, localization issues | Never - use system APIs |
| Skipping dry-run testing | Faster development | Production data loss | Never - dry-run is essential |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| APFS space calculation | Using `df` (shows inaccurate data with purgeable space) | Use `diskutil apfs list` for accurate breakdown |
| File permissions check | Using `access()` or stat (doesn't account for SIP/TCC) | Use `removefile()` with error handling, check extended attributes |
| Spotlight index location | Assuming single location | Check both `/private/var/db/Spotlight-V100/` and per-volume `.Spotlight-V100/` |
| Time Machine snapshots | Attempting to delete manually | Use `tmutil deletelocalsnapshots` or let macOS manage |
| Homebrew cache | Deleting `~/Library/Caches/Homebrew/` | Use `brew cleanup` (respects dependencies, safer) |
| npm/yarn cache | Deleting `~/.npm/` | Use `npm cache clean --force` (ensures consistency) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Recursive scanning without limits | Scan hangs forever, high CPU | Set max depth, skip symlinks, timeout per directory | Large directories (node_modules, photo libraries) |
| Loading entire file lists into memory | RAM explosion, OOM crashes | Stream/iterator pattern, process in batches | >10K files |
| Parallel deletion without throttling | System unresponsive, Finder freezes | Limit concurrent operations (max 10 parallel) | Deleting 1000+ files |
| Scanning network volumes | Extremely slow, timeouts | Skip network mounts by default (check mount point type) | Any network volume |
| Not handling APFS clones | Reporting incorrect space savings | Use APFS-aware APIs to detect clones | User expectations vs reality |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Following symlinks during deletion | Can delete files outside intended scope, potential privilege escalation | Use `lstat()` not `stat()`, never follow symlinks, check for `.` and `..` |
| Not validating user-provided paths | Path traversal attacks (user inputs `../../` to escape) | Canonicalize paths, verify within allowed directories |
| Storing file list in temp file | Information disclosure (other processes can read) | Use secure temp directory with restricted permissions |
| Running with elevated privileges unnecessarily | Malicious files could exploit elevated context | Run as user, only elevate for specific operations if needed |
| Not checking for hardlinks | Deleting shared data affects multiple files | Count hardlinks before deletion, warn if >1 |
| Trusting file extensions | Malicious files disguised as safe types | Check file type, not just extension |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No preview before deletion | User anxiety, distrust | Always show what will be deleted with sizes |
| Unclear space savings | Confusion (purgeable vs actual) | Separate report: "X GB will be freed, Y GB already purgeable" |
| No dry-run mode | Fear of running tool | Default to dry-run, require flag for actual deletion |
| Deleting immediately without confirmation | Accidental data loss | Multi-step: scan → review → confirm → delete |
| No progress indication | Appears hung, users force-quit | Real-time progress with file count and current operation |
| Technical jargon in output | Users don't understand what's being cleaned | Plain language: "Safari cache" not "com.apple.Safari.Cache" |
| No undo/recovery option | Permanent data loss panic | Keep deletion log for recovery guidance, suggest backups |
| Cleaning everything by default | Overly aggressive, breaks things | Categorize by risk, only auto-select "safe" items |

## "Looks Done But Isn't" Checklist

- [ ] **Dry-run mode:** Often missing actual prevention (still opens files/checks permissions) — verify no filesystem modifications occur
- [ ] **Permission handling:** Often missing TCC (Transparency, Consent, Control) checks — verify tool handles permission denials gracefully
- [ ] **Error handling:** Often missing handling for in-use files — verify deletion continues after errors
- [ ] **Space calculation:** Often missing APFS clones/hardlinks — verify reported space matches reality
- [ ] **Localization:** Often missing non-English system support — verify works on localized macOS
- [ ] **macOS version differences:** Often tested on single version — verify on macOS 11, 12, 13, 14, 15
- [ ] **Apple Silicon vs Intel:** Often missing architecture-specific paths — verify on both platforms
- [ ] **Edge cases:** Often missing handling of special characters in filenames — verify with unicode, spaces, special chars
- [ ] **Backup verification:** Often suggests backup but doesn't verify — verify Time Machine enabled before cleaning
- [ ] **Cleanup after failure:** Often leaves partial state — verify cleanup is atomic or resumable

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Deleted SIP-protected files | LOW (impossible without SIP disabled) | If somehow occurred: Boot Recovery Mode, reinstall macOS |
| Deleted swap files | LOW (auto-recreated) | Reboot system, swap files regenerate automatically |
| Deleted user caches | MEDIUM | Wait for apps to rebuild caches (minutes to hours), no data loss |
| Deleted Spotlight index | MEDIUM | Reindex via System Preferences (hours), or `sudo mdutil -E /` |
| Deleted Application Support | HIGH | Restore from Time Machine backup, or reinstall apps |
| Deleted authentication tokens | MEDIUM | Re-login to affected apps (might require MFA) |
| Deleted Xcode DerivedData | MEDIUM | Xcode rebuilds automatically (slow for large projects) |
| Deleted Time Machine snapshots | MEDIUM | Lost quick restore points, but backups still exist |
| Deleted wrong user files | HIGH | Time Machine restore or third-party recovery tool (Disk Drill) |
| Kernel panic from swap deletion | LOW | System auto-reboots, no data loss if apps saved work |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SIP-protected paths | Phase 1: Core Scanning | Run on SIP-enabled system, verify no "operation not permitted" errors |
| Swap file deletion | Phase 1: Core Scanning | Verify `/private/var/vm/` excluded from scan results |
| Cache type misclassification | Phase 1: Core Scanning + Phase 2: Smart Cleaning | Verify safe/risky/forbidden categorization, test on developer Mac |
| Purgeable space confusion | Phase 2: Smart Cleaning + Phase 3: Reporting | Compare reported savings to `diskutil apfs list` output |
| SSV modification attempts | Phase 1: Core Scanning | Test on macOS 11+, verify System volume not scanned |
| Excessive permission requests | Phase 1: Core Scanning | Verify basic cleaning works without Full Disk Access |
| False positive file deletion | Phase 1: Core Scanning + Phase 2: Smart Cleaning | Test with project directories, verify no false positives |
| Application Support breakage | Phase 2: Smart Cleaning + Phase 4: Safety | Test with common apps, verify no license/settings loss |

## Phase-Specific Warnings

### Phase 1: Core Scanning
**Critical:** This phase defines safety boundaries. Get exclusion lists right from the start:
- Hardcode SIP-protected paths
- Exclude VM directory
- Implement conservative file classification
- Test on multiple macOS versions (11, 12, 13, 14, 15)
- Test on both Intel and Apple Silicon

**Deeper research needed:**
- Per-app Application Support directory safety (requires app-specific knowledge)
- Complete list of authentication token locations
- APFS snapshot handling differences across macOS versions

### Phase 2: Smart Cleaning
**Critical:** Don't rely on heuristics alone:
- File age is not sufficient for safety determination
- File extension is not sufficient for type determination
- Size is not an indicator of importance
- Access time can be misleading (backup volumes, seasonal projects)

**Deeper research needed:**
- Which caches are safe vs risky for popular apps
- Developer tool cache implications
- Photo/music library cache regeneration costs

### Phase 3: Reporting
**Critical:** Clear communication prevents user confusion:
- Separate purgeable vs truly freed space
- Explain why space savings might not match Finder immediately
- Show what was skipped and why
- Provide plain language descriptions

### Phase 4: Safety Features
**Critical:** Multiple layers of protection:
- Dry-run must truly prevent modifications
- Preview must show actual file paths, not just categories
- Confirmation should summarize risk level
- Consider deletion log for recovery guidance

**Deeper research needed:**
- Recovery/undo mechanisms (feasibility, storage cost)
- Backup verification methods
- Integration with Time Machine

## Sources

**macOS Security & Architecture:**
- [Apple: About System Integrity Protection](https://support.apple.com/en-us/102149)
- [Apple: Signed system volume security](https://support.apple.com/guide/security/signed-system-volume-security-secd698747c9/web)
- [System Integrity Protection - Wikipedia](https://en.wikipedia.org/wiki/System_Integrity_Protection)
- [macOS SIP - HackTricks](https://book.hacktricks.wiki/en/macos-hardening/macos-security-and-privilege-escalation/macos-security-protections/macos-sip.html)

**APFS & Storage Management:**
- [DaisyDisk: Local APFS snapshots](https://daisydiskapp.com/guide/4/en/Snapshots/)
- [Der Flounder: Reclaiming drive space by thinning APFS snapshot backups](https://derflounder.wordpress.com/2018/04/07/reclaiming-drive-space-by-thinning-apple-file-system-snapshot-backups/)
- [MacPaw: How to clear purgeable space on Mac](https://macpaw.com/how-to/purgeable-space-on-macos)
- [Michael Tsai: Clearing Space on Your Mac](https://mjtsai.com/blog/2024/03/19/clearing-space-on-your-mac/)

**Cleaning Tool Best Practices:**
- [Macworld: Best Mac Cleaner software 2026](https://www.macworld.com/article/673271/best-mac-cleaner-vs-cleanmymac.html)
- [TheSweetBits: Best Mac Cleaner Apps 2026](https://thesweetbits.com/best-mac-cleaner-software/)
- [CleanMyMac: 10 safe free Mac cleaners](https://cleanmymac.com/blog/best-free-mac-cleaners)
- [MacPaw: How to clean up Mac](https://macpaw.com/how-to/clean-up-mac)

**Cache Management:**
- [iBoysoft: What Is ~/Library/Caches on Mac & Is It Safe to Delete It?](https://iboysoft.com/wiki/library-caches-mac.html)
- [MacPaw: Ways to clear cache on Mac: which files are safe to delete?](https://macpaw.com/how-to/clear-cache-on-mac)
- [MacPaw: How to clear Xcode cache](https://macpaw.com/how-to/clear-xcode-cache)

**System Files & Recovery:**
- [Apple Community: What files are safe to delete or move?](https://discussions.apple.com/docs/DOC-2112)
- [MacRumors: sleepimage/swap files safe to delete?](https://forums.macrumors.com/threads/sleepimage-swap-files-safe-to-delete.265475/)
- [MacPaw: How to recover deleted files on Mac](https://macpaw.com/how-to/recover-deleted-files-on-mac)
- [Remo: Recover Files Deleted by CleanMyMac](https://www.remosoftware.com/info/recover-files-deleted-by-cleanmymac)

**Permissions & Full Disk Access:**
- [MacPaw: Full Disk Access explained](https://macpaw.com/how-to/full-disk-access)
- [iBoysoft: What Is Full Disk Access on Mac & Is It Safe](https://iboysoft.com/wiki/full-disk-access-mac.html)
- [MacPaw: Full Disk Access permission for CleanMyMac](https://macpaw.com/support/cleanmymac/knowledgebase/full-disk-access)

**Spotlight & Indexing:**
- [Apple: Rebuild the Spotlight index](https://support.apple.com/en-us/102321)
- [MacPaw: How to rebuild Spotlight index](https://macpaw.com/how-to/reindex-spotlight)
- [MacRumors: How to Rebuild Spotlight Index](https://www.macrumors.com/how-to/rebuild-spotlight-search-index-on-mac/)

---
*Pitfalls research for: macOS Disk Cleaning CLI Tools*
*Researched: 2026-02-16*
