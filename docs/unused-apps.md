# Unused Applications Detection

## How It Works

macOS Spotlight records `kMDItemLastUsedDate` on every `.app` bundle each time a user opens it. mac-cleaner queries this metadata to identify applications that haven't been opened in a configurable time period (default: 180 days).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Returns: "2024-05-14 09:23:41 +0000" or "(null)" if never opened
```

No special permissions are needed. This works on all modern macOS versions (APFS/HFS+).

## What "Total Footprint" Includes

Each unused app's reported size includes:

1. **The `.app` bundle itself** — the application package in `/Applications` or `~/Applications`
2. **Associated `~/Library/` directories** — data stored by the app across these locations:

| Location | Match by |
|----------|----------|
| `~/Library/Application Support/<id or name>/` | Bundle ID or app name |
| `~/Library/Caches/<id>/` | Bundle ID |
| `~/Library/Containers/<id>/` | Bundle ID |
| `~/Library/Group Containers/*<id>*/` | Bundle ID (glob) |
| `~/Library/Preferences/<id>.plist` | Bundle ID |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Bundle ID (glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Bundle ID |
| `~/Library/HTTPStorages/<id>/` | Bundle ID |
| `~/Library/WebKit/<id>/` | Bundle ID |
| `~/Library/Logs/<id or name>/` | Bundle ID or app name |
| `~/Library/Cookies/<id>.binarycookies` | Bundle ID |
| `~/Library/LaunchAgents/<id>*.plist` | Bundle ID (glob) |

## Default Threshold

Applications not opened in **180 days** (approximately 6 months) are flagged as unused. Apps opened more recently are excluded from results.

## Directories Scanned

| Directory | Included |
|-----------|----------|
| `/Applications` | Yes |
| `/Applications/Utilities` | Yes |
| `~/Applications` | Yes |
| `/System/Applications` | No (system apps are never flagged) |

## Why Apps in /Applications Require Manual Removal

The existing `safety.IsPathBlocked()` mechanism blocks deletion of paths outside `~/`. Since `/Applications` is outside the user's home directory, the cleanup module will naturally skip these entries. Apps in `~/Applications` are under `~` and can be cleaned normally.

To remove an app from `/Applications`, drag it to the Trash manually or use:
```bash
sudo rm -rf /Applications/SomeApp.app
```

## CLI Usage

```bash
# Scan for unused apps only
mac-cleaner --unused-apps --dry-run

# Include unused apps in a full scan
mac-cleaner --all --dry-run

# Full scan but skip unused apps
mac-cleaner --all --skip-unused-apps

# JSON output
mac-cleaner --unused-apps --json
```

## Risk Level

All entries in the `unused-apps` category are classified as **risky** because removing applications is not easily reversible.
