# Security Architecture

This document describes the security architecture of mac-cleaner, a CLI tool that scans and deletes files on macOS. Given the destructive nature of file deletion, the tool implements multiple layers of defense to prevent accidental data loss or system damage.

## Threat Model

**What the tool does:** Scans known cache, log, and temporary file locations on macOS and optionally deletes them to reclaim disk space.

**What could go wrong:**
- Deletion of system files, causing macOS to become unbootable
- Deletion of user data outside intended cache directories
- Symlink attacks redirecting deletion to unintended targets
- Path traversal escaping intended directory boundaries

**Adversary assumptions:** The tool runs as the current user with no elevated privileges. The primary risk is bugs in path construction or validation, not external attackers. However, symlink-based attacks from other processes on the same system are considered.

## Safety Architecture

mac-cleaner uses a multi-layer defense strategy. Each layer is independent — a failure in one layer is caught by the next.

### Layer 1: Hardcoded Path Construction

All scan targets are hardcoded in scanner implementations (`pkg/*/scanner.go`). Paths are constructed using `filepath.Join()` from the user's home directory — never from user input, CLI arguments, or environment variables (except `$TMPDIR` for QuickLook, which is validated).

### Layer 2: Path Validation (`internal/safety/`)

Every path is validated by `safety.IsPathBlocked()` before any operation. This function:

1. **Normalizes** the path with `filepath.Clean()` to remove `..` components
2. **Resolves symlinks** with `filepath.EvalSymlinks()` to get the real filesystem path
3. **Checks critical paths** — exact matches on `/`, `/Users`, `/Library`, `/Applications`, `/private`, `/var`, `/etc`, `/Volumes`, `/opt`, `/cores` are always blocked
4. **Checks swap/VM paths** — `/private/var/vm` and children are always blocked to prevent kernel panics
5. **Checks SIP-protected paths** — `/System`, `/usr`, `/bin`, `/sbin` are blocked (with `/usr/local` as an exception)
6. **Enforces home containment** — all deletable paths must be under the user's home directory (`~/`) or under `/private/var/folders/` (for QuickLook caches). Everything else is blocked

### Layer 3: Re-validation at Deletion Time

`cleanup.Execute()` re-checks `safety.IsPathBlocked()` immediately before calling `os.RemoveAll()` on each path. This catches any issues that might arise between scan time and deletion time.

### Layer 4: User Confirmation

Before any deletion occurs, the user must explicitly confirm the operation. This can be done through:
- **Interactive mode** (default) — walks through each category for approval
- **Confirmation prompt** — explicit yes/no before bulk deletion
- **Dry-run mode** (`--dry-run`) — previews what would be deleted without actually deleting
- **Force mode** (`--force`) — bypasses confirmation (explicit opt-in)

### Layer 5: Risk Classification

Every scan category is assigned a risk level (`safe`, `moderate`, or `risky`) displayed to the user before confirmation. This helps users make informed decisions about what to delete.

## Path Validation Details

### Symlink Handling

- **Scanning** uses `os.Lstat()` and `filepath.WalkDir()`, which do NOT follow symlinks. Symlinked files are not counted in size calculations.
- **Safety checks** use `filepath.EvalSymlinks()` to resolve the real path before checking against blocklists. A symlink pointing from `~/Library/Caches/safe-dir` to `/System/Library` would be caught and blocked.
- If symlink resolution fails for a path that exists (not `IsNotExist`), the path is blocked for safety.

### Path Boundary Safety

`pathHasPrefix()` checks that a path is equal to or is a proper child of a prefix (separated by `/`). This prevents false positives like `/SystemVolume` matching `/System`.

### TMPDIR Validation

The QuickLook scanner derives a cache directory from `$TMPDIR`. Before using this path:
1. Validates that `$TMPDIR` contains `/var/folders/` (macOS convention)
2. Checks the derived cache directory against `safety.IsPathBlocked()`
3. Individual entries within the cache directory are also safety-checked

## External Commands

The tool executes two external commands:
- `docker system df` — to query Docker disk usage
- `/usr/libexec/PlistBuddy` — to read bundle identifiers from `.plist` files

Both use `exec.CommandContext()` with arguments passed as separate parameters (not through a shell). There is no risk of shell injection. Command binaries are validated with `exec.LookPath()` before execution.

## What We Don't Do

- **No network access** — the tool never makes network requests
- **No privilege escalation** — no `sudo`, no setuid, no entitlements
- **No file writing** — the tool only reads (scanning) and deletes (cleanup)
- **No system modification** — no preference changes, no daemon management
- **No user input in paths** — all paths are derived from hardcoded bases and filesystem enumeration

## CI Security Tooling

The project runs these security tools in CI:
- **gosec** — static security analysis for Go (catches path traversal, unchecked errors, file permission issues)
- **govulncheck** — dependency vulnerability scanning with reachability analysis
- **Race detector** — `go test -race` catches data races in concurrent code paths
- **Fuzz testing** — `FuzzIsPathBlocked` discovers edge cases in path validation

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT open a public issue**
2. Email the maintainer or use GitHub's private vulnerability reporting feature
3. Include a description of the vulnerability, steps to reproduce, and potential impact
4. Allow reasonable time for a fix before public disclosure
