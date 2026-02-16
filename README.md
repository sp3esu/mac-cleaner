# mac-cleaner

A fast, safe CLI tool for reclaiming disk space on macOS.

English | [Polski](docs/README_PL.md) | [Deutsch](docs/README_DE.md) | [Українська](docs/README_UA.md) | [Русский](docs/README_RU.md) | [Français](docs/README_FR.md)

## Features

### System Caches
- **User App Caches** — `~/Library/Caches/` (safe)
- **User Logs** — `~/Library/Logs/` (safe)
- **QuickLook Thumbnails** — per-user QuickLook cache (safe)

### Browser Data
- **Safari Cache** — `~/Library/Caches/com.apple.Safari/` (moderate)
- **Chrome Cache** — `~/Library/Caches/Google/Chrome/` across all profiles (moderate)
- **Firefox Cache** — `~/Library/Caches/Firefox/` (moderate)

### Developer Caches
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (risky)
- **npm Cache** — `~/.npm/` (moderate)
- **Yarn Cache** — `~/Library/Caches/yarn/` (moderate)
- **Homebrew Cache** — `~/Library/Caches/Homebrew/` (moderate)
- **Docker Reclaimable** — containers, images, build cache, volumes (risky)
- **iOS Simulator Caches** — `~/Library/Developer/CoreSimulator/Caches/` (safe)
- **iOS Simulator Logs** — `~/Library/Logs/CoreSimulator/` (safe)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (moderate)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (risky)
- **pnpm Store** — `~/Library/pnpm/store/` (moderate)
- **CocoaPods Cache** — `~/Library/Caches/CocoaPods/` (moderate)
- **Gradle Cache** — `~/.gradle/caches/` (moderate)
- **pip Cache** — `~/Library/Caches/pip/` (safe)

### App Leftovers
- **Orphaned Preferences** — `.plist` files in `~/Library/Preferences/` for uninstalled apps (risky)
- **iOS Device Backups** — `~/Library/Application Support/MobileSync/Backup/` (risky)
- **Old Downloads** — files in `~/Downloads/` older than 90 days (moderate)

### Creative App Caches
- **Adobe Caches** — `~/Library/Caches/Adobe/` (safe)
- **Adobe Media Cache** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (moderate)
- **Sketch Cache** — `~/Library/Caches/com.bohemiancoding.sketch3/` (safe)
- **Figma Cache** — `~/Library/Application Support/Figma/` (safe)

### Messaging App Caches
- **Slack Cache** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (safe)
- **Discord Cache** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (safe)
- **Microsoft Teams Cache** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (safe)
- **Zoom Cache** — `~/Library/Application Support/zoom.us/data/` (safe)

## Safety

mac-cleaner is designed to protect your system:

- **SIP-protected paths are blocked** — `/System`, `/usr`, `/bin`, `/sbin` are never touched (`/usr/local` is allowed)
- **Swap/VM protection** — `/private/var/vm` is always blocked to prevent kernel panics
- **Symlink resolution** — all paths are resolved before deletion to prevent escaping intended directories
- **Three-tier risk levels** — every category is classified as **safe**, **moderate**, or **risky** so you know what you're getting into
- **Re-validation before deletion** — safety checks run again at deletion time, not just during scanning
- **Dry-run mode** — preview everything before committing with `--dry-run`
- **Interactive confirmation** — explicit user approval required before anything is deleted (unless `--force` is used)

For a detailed security analysis, see [Security Architecture](docs/SECURITY.md).

## Installation

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Build from source

**Prerequisites:** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Shell Completion

Generate shell completion scripts for tab-completing flags and subcommands.

**Bash:**
```bash
# Load in current session:
source <(mac-cleaner completion bash)

# Install permanently:
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh:**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Then restart your shell or run: compinit
```

**Fish:**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell:**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
```

## Usage

**Interactive mode** (default — walks you through each category):
```bash
./mac-cleaner
```

**Scan everything, preview only:**
```bash
./mac-cleaner --all --dry-run
```

**Clean system caches without confirmation:**
```bash
./mac-cleaner --system-caches --force
```

**Scan everything, JSON output:**
```bash
./mac-cleaner --all --json
```

**Scan all but skip Docker and iOS backups:**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

## CLI Flags

### Scan Categories

| Flag | Description |
|------|-------------|
| `--all` | Scan all categories |
| `--system-caches` | Scan user app caches, logs, and QuickLook thumbnails |
| `--browser-data` | Scan Safari, Chrome, and Firefox caches |
| `--dev-caches` | Scan Xcode, npm/yarn, Homebrew, and Docker caches |
| `--app-leftovers` | Scan orphaned preferences, iOS backups, and old Downloads |
| `--creative-caches` | Scan Adobe, Sketch, and Figma caches |
| `--messaging-caches` | Scan Slack, Discord, Teams, and Zoom caches |

### Output & Behavior

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview what would be removed without deleting |
| `--json` | Output results as JSON |
| `--verbose` | Show detailed file listing |
| `--force` | Bypass confirmation prompt |

### Category Skip Flags

| Flag | Description |
|------|-------------|
| `--skip-system-caches` | Skip system cache scanning |
| `--skip-browser-data` | Skip browser data scanning |
| `--skip-dev-caches` | Skip developer cache scanning |
| `--skip-app-leftovers` | Skip app leftover scanning |
| `--skip-creative-caches` | Skip creative app cache scanning |
| `--skip-messaging-caches` | Skip messaging app cache scanning |

### Item Skip Flags

| Flag | Description |
|------|-------------|
| `--skip-derived-data` | Skip Xcode DerivedData |
| `--skip-npm` | Skip npm cache |
| `--skip-yarn` | Skip Yarn cache |
| `--skip-homebrew` | Skip Homebrew cache |
| `--skip-docker` | Skip Docker reclaimable space |
| `--skip-safari` | Skip Safari cache |
| `--skip-chrome` | Skip Chrome cache |
| `--skip-firefox` | Skip Firefox cache |
| `--skip-quicklook` | Skip QuickLook thumbnails |
| `--skip-orphaned-prefs` | Skip orphaned preferences |
| `--skip-ios-backups` | Skip iOS device backups |
| `--skip-old-downloads` | Skip old Downloads files |
| `--skip-simulator-caches` | Skip iOS Simulator caches |
| `--skip-simulator-logs` | Skip iOS Simulator logs |
| `--skip-xcode-device-support` | Skip Xcode Device Support files |
| `--skip-xcode-archives` | Skip Xcode Archives |
| `--skip-pnpm` | Skip pnpm store |
| `--skip-cocoapods` | Skip CocoaPods cache |
| `--skip-gradle` | Skip Gradle cache |
| `--skip-pip` | Skip pip cache |
| `--skip-adobe` | Skip Adobe caches |
| `--skip-adobe-media` | Skip Adobe media caches |
| `--skip-sketch` | Skip Sketch cache |
| `--skip-figma` | Skip Figma cache |
| `--skip-slack` | Skip Slack cache |
| `--skip-discord` | Skip Discord cache |
| `--skip-teams` | Skip Microsoft Teams cache |
| `--skip-zoom` | Skip Zoom cache |

## License

MIT

## Built With

This project was built using [Claude Code](https://claude.com/product/claude-code) and the [Get Shit Done](https://github.com/gsd-build/get-shit-done) plugin.
