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

### App Leftovers
- **Orphaned Preferences** — `.plist` files in `~/Library/Preferences/` for uninstalled apps (risky)
- **iOS Device Backups** — `~/Library/Application Support/MobileSync/Backup/` (risky)
- **Old Downloads** — files in `~/Downloads/` older than 90 days (moderate)

## Safety

mac-cleaner is designed to protect your system:

- **SIP-protected paths are blocked** — `/System`, `/usr`, `/bin`, `/sbin` are never touched (`/usr/local` is allowed)
- **Swap/VM protection** — `/private/var/vm` is always blocked to prevent kernel panics
- **Symlink resolution** — all paths are resolved before deletion to prevent escaping intended directories
- **Three-tier risk levels** — every category is classified as **safe**, **moderate**, or **risky** so you know what you're getting into
- **Re-validation before deletion** — safety checks run again at deletion time, not just during scanning
- **Dry-run mode** — preview everything before committing with `--dry-run`
- **Interactive confirmation** — explicit user approval required before anything is deleted (unless `--force` is used)

## Installation

### Prerequisites

- **Go 1.25+**
- **macOS**

### Build from source

```bash
git clone https://github.com/gregor/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
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

## License

This project does not currently include a license file.
