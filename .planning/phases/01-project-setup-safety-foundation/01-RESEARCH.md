# Phase 1: Project Setup & Safety Foundation - Research

**Researched:** 2026-02-16
**Domain:** Go project scaffolding, CLI framework setup, macOS safety layer
**Confidence:** HIGH

## Summary

Phase 1 requires scaffolding a Go project that compiles into a `mac-cleaner` binary with `--version` support, and implementing a hardcoded safety layer that rejects operations on SIP-protected paths and swap files. The technical domain is well-understood: Go project initialization is straightforward, Cobra provides built-in `--version` flag support, and macOS SIP-protected paths are well-documented and stable across versions.

The safety layer is the critical deliverable. It must use `filepath.Clean` plus path-prefix matching with separator boundaries (not naive `strings.HasPrefix`) to prevent both direct access and traversal attacks. Symlinks should be resolved via `filepath.EvalSymlinks` before validation, with any resolution failure treated as a rejection. The Go 1.24+ `os.Root` API is available but is not the right tool here -- it is designed for constraining operations to a directory, not for blocklist validation.

**Primary recommendation:** Use Cobra with a flags-only root command (no subcommands), hardcode SIP and swap exclusions as a `[]string` blocklist validated via cleaned-path prefix matching, and resolve symlinks before validation to prevent bypass.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Binary name: `mac-cleaner`
- `mac-cleaner --version` outputs just the version number (e.g., `0.1.0`)
- Tone: terse and technical -- "system-caches: 5.2G", not "Looks like you have 5.2 GB!"
- Help text follows the same terse style -- factual, no personality
- Blocked paths reported as warning lines to stderr: `SKIP: /System (SIP-protected)`
- Core protections hardcoded (SIP paths, swap files) -- no config can override
- User can add extra protected paths via config (allowlist for additional safety)
- Logging to stderr only -- no separate log file
- Go module path: `github.com/gregor/mac-cleaner`
- Go 1.22+ minimum
- One package per cleaning category: `pkg/system/`, `pkg/browser/`, `pkg/developer/`
- Tests alongside code (Go standard): `safety_test.go` next to `safety.go`
- Colors with TTY auto-detection -- colors when terminal, plain when piped
- File sizes: human-readable (5.2 GB) -- like `du -h`
- Unicode symbols for status indicators (checkmarks, arrows, warning triangles)
- Scan results in table format with aligned columns

### Claude's Discretion
- CLI structure (flags-only vs subcommands) -- pick what fits the roadmap best
- Symlink safety approach (resolve vs skip) -- pick the safest option
- Exact table formatting and column layout
- Color palette and specific Unicode symbols used

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24+ (go directive: `go 1.24`) | Language & runtime | Local system has go1.25.7. Go 1.24 introduced `os.Root` for traversal-resistant file ops. Use `go 1.24` in go.mod to set a reasonable floor that includes modern security APIs. User said 1.22+ minimum; 1.24 is compatible and gives us better stdlib. |
| Cobra | v1.10.2+ | CLI framework | Industry standard (kubectl, docker, hugo, gh). Built-in `--version` flag via `rootCmd.Version`. `SetVersionTemplate` for custom output format. Active maintenance (Dec 2024 release). |

### Supporting (Phase 1 only)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| path/filepath (stdlib) | -- | Path cleaning, symlink resolution | `filepath.Clean()`, `filepath.EvalSymlinks()` for safety layer path validation |
| os (stdlib) | -- | File stat, home directory | `os.UserHomeDir()` for `~` expansion, `os.Stat` for path existence checks |
| fmt (stdlib) | -- | Stderr output | `fmt.Fprintf(os.Stderr, ...)` for safety warnings |
| strings (stdlib) | -- | Path prefix matching | `strings.HasPrefix` for cleaned-path blocklist checks |

### NOT Needed in Phase 1

| Library | Why Deferred |
|---------|-------------|
| Viper | No config file support needed yet. User-added protected paths come in a later phase. |
| fatih/color | No colored output in Phase 1. The safety layer and version flag produce plain text. Color comes with Phase 2 scan output. |
| go-isatty | Comes with fatih/color. Not needed until output formatting phase. |
| encoding/json | No JSON output in Phase 1. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Cobra | urfave/cli | urfave/cli is simpler for single-command tools, but this project will grow into subcommand territory (scan, clean, etc.) in later phases. Cobra's persistent flags and command tree will pay off. |
| Cobra | flag (stdlib) | stdlib flag package has no `--version` built-in, no help formatting, no subcommand support. Too bare for this project. |

**Installation:**
```bash
go mod init github.com/gregor/mac-cleaner
go get github.com/spf13/cobra@latest
```

## Architecture Patterns

### Recommended Project Structure (Phase 1)

```
mac-cleaner/
├── main.go                    # Entry point: calls cmd.Execute()
├── cmd/
│   └── root.go                # Root command definition, --version flag
├── internal/
│   └── safety/
│       ├── safety.go          # IsPathBlocked(), blocklist, path validation
│       └── safety_test.go     # Table-driven tests for all blocked paths
├── pkg/
│   ├── system/                # (empty, placeholder for Phase 2)
│   ├── browser/               # (empty, placeholder for Phase 3)
│   └── developer/             # (empty, placeholder for Phase 3)
├── go.mod
└── go.sum
```

**Rationale:**
- `cmd/` -- Cobra convention. Root command lives here. Future subcommands go here too.
- `internal/safety/` -- Safety layer is internal (not for external import). The `internal/` boundary is compiler-enforced in Go.
- `pkg/system/`, `pkg/browser/`, `pkg/developer/` -- User specified these package paths. Created as empty placeholders so the structure is established. Content comes in Phases 2-3.
- `main.go` at root -- Standard Go convention. Minimal: imports `cmd` and calls `Execute()`.

### Pattern 1: Flags-Only Root Command (Discretion Decision)

**Decision: Use flags-only root command, no subcommands in Phase 1.**

**Rationale:** The roadmap shows `mac-cleaner --system-caches --dry-run`, `mac-cleaner --all`, `mac-cleaner` (interactive). All usage patterns work as flags on a single root command. Subcommands (`mac-cleaner scan`, `mac-cleaner clean`) would force a different UX from what the roadmap describes. Flags-only maps directly to the documented usage patterns.

Future phases can add subcommands if needed, but the roadmap's design works cleanly with flags.

**Implementation:**
```go
// cmd/root.go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

// Set via ldflags at build time: -ldflags "-X github.com/gregor/mac-cleaner/cmd.version=0.1.0"
var version = "dev"

var rootCmd = &cobra.Command{
    Use:   "mac-cleaner",
    Short: "scan and remove macOS junk files",
    Long:  "scan and remove system caches, browser data, developer caches, and app leftovers",
    Run: func(cmd *cobra.Command, args []string) {
        // Phase 1: just prints help. Interactive mode comes in Phase 5.
        cmd.Help()
    },
}

func init() {
    rootCmd.Version = version
    rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**Key details:**
- `SetVersionTemplate("{{.Version}}\n")` makes `--version` output just the version number (e.g., `0.1.0`), matching the user's requirement.
- `version` variable set via `ldflags` at build time. Defaults to `"dev"` for development builds.
- `Run` function is defined (making root executable), not omitted (which would require a subcommand).

### Pattern 2: Hardcoded Safety Blocklist with Path Normalization

**What:** All path operations pass through `safety.IsPathBlocked()` before any file system operation. Blocked paths are hardcoded constants that cannot be overridden by configuration.

**When to use:** Before any scan, stat, read, or delete operation on the filesystem.

**Implementation:**
```go
// internal/safety/safety.go
package safety

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// SIP-protected paths. These cannot be overridden.
var sipProtectedPrefixes = []string{
    "/System",
    "/usr",
    "/bin",
    "/sbin",
}

// Paths within SIP-protected areas that ARE writable.
var sipExceptions = []string{
    "/usr/local",
}

// Swap/VM paths. These cannot be overridden.
var swapProtectedPrefixes = []string{
    "/private/var/vm",
}

// IsPathBlocked checks whether a path is in the hardcoded blocklist.
// It resolves symlinks and cleans the path before checking.
// Returns (blocked bool, reason string).
func IsPathBlocked(path string) (bool, string) {
    cleaned := filepath.Clean(path)

    // Resolve symlinks to prevent bypass via symlink to protected path
    resolved, err := filepath.EvalSymlinks(cleaned)
    if err != nil {
        // If we can't resolve, check the literal path.
        // A non-existent path is not blocked (it will fail at the operation level).
        if os.IsNotExist(err) {
            resolved = cleaned
        } else {
            // Can't resolve and path exists -- treat as blocked for safety
            return true, fmt.Sprintf("cannot resolve path: %v", err)
        }
    }

    resolved = filepath.Clean(resolved)

    // Check swap/VM paths first (no exceptions)
    for _, prefix := range swapProtectedPrefixes {
        if pathHasPrefix(resolved, prefix) {
            return true, "swap/VM file"
        }
    }

    // Check SIP-protected paths (with exceptions)
    for _, prefix := range sipProtectedPrefixes {
        if pathHasPrefix(resolved, prefix) {
            // Check if it falls under an exception
            isException := false
            for _, exc := range sipExceptions {
                if pathHasPrefix(resolved, exc) {
                    isException = true
                    break
                }
            }
            if !isException {
                return true, "SIP-protected"
            }
        }
    }

    return false, ""
}

// pathHasPrefix checks if path starts with prefix, respecting path boundaries.
// "/System/Library" has prefix "/System" -> true
// "/SystemVolume" has prefix "/System" -> false (different path segment)
func pathHasPrefix(path, prefix string) bool {
    if path == prefix {
        return true
    }
    return strings.HasPrefix(path, prefix+string(filepath.Separator))
}

// WarnBlocked prints a warning to stderr for a blocked path.
func WarnBlocked(path, reason string) {
    fmt.Fprintf(os.Stderr, "SKIP: %s (%s)\n", path, reason)
}
```

### Pattern 3: Symlink Resolution Before Validation (Discretion Decision)

**Decision: Resolve symlinks before blocklist checking. If resolution fails on an existing path, block the operation.**

**Rationale for resolve over skip:**
- Skipping symlinks entirely would miss legitimate cleaning targets (many macOS paths use symlinks, e.g., `/var` -> `/private/var`).
- Resolving first, then checking, catches attacks where a symlink points into a protected directory.
- `filepath.EvalSymlinks` handles the resolution. If it fails on an existing path, we block for safety (conservative approach).
- This is a TOCTOU-safe-enough approach for a cleanup tool: the window between check and operation is small, and an attacker who can create symlinks in the cleanup targets already has write access to those directories.

**Go 1.24 `os.Root` is NOT the right tool here** because `os.Root` constrains operations *within* a directory. Our safety layer does the opposite: it blocks operations on specific directories regardless of where the operation originates. `os.Root` may be useful in later phases for confining scan operations, but the blocklist pattern is what Phase 1 needs.

### Anti-Patterns to Avoid

- **Naive `strings.HasPrefix` without separator boundary:** `strings.HasPrefix("/SystemVolume", "/System")` returns `true`, which is wrong. Always append `filepath.Separator` to the prefix before comparing, or check for exact match.
- **`filepath.HasPrefix` (deprecated):** This function is deprecated in Go and should not be used. It has the same boundary problem.
- **Checking path without cleaning first:** `/System/../etc/passwd` would bypass a naive `/System` prefix check. Always `filepath.Clean()` first.
- **Checking path without resolving symlinks:** A symlink at `/tmp/sneaky` -> `/System/Library` would bypass a check on the literal path. Always `filepath.EvalSymlinks()` before checking.
- **Making blocklist configurable:** The user explicitly decided core protections are hardcoded and no config can override them.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CLI flag parsing | Custom arg parsing | Cobra | Handles --version, --help, flag validation, error messages |
| Version template | Custom version output logic | `rootCmd.SetVersionTemplate("{{.Version}}\n")` | Cobra built-in, handles the exact format needed |
| Path normalization | Custom path cleaning | `filepath.Clean()` + `filepath.EvalSymlinks()` | Stdlib handles edge cases (double slashes, dot-dot, etc.) |
| TTY detection | Custom isatty check | `fatih/color` (later phases) | Handles NO_COLOR env var, pipe detection, cross-platform |

**Key insight:** Phase 1 is small enough that stdlib + Cobra cover everything. No custom solutions needed.

## Common Pitfalls

### Pitfall 1: Path Boundary Matching Errors
**What goes wrong:** `HasPrefix("/SystemVolume/data", "/System")` incorrectly matches, causing false blocks or (worse) false allows if the logic is inverted.
**Why it happens:** String prefix matching doesn't understand filesystem path boundaries.
**How to avoid:** Always check `path == prefix || strings.HasPrefix(path, prefix + "/")`.
**Warning signs:** Tests pass for `/System` but fail for `/SystemVolume` or `/usr/local`.

### Pitfall 2: Symlink Bypass of Blocklist
**What goes wrong:** An attacker or accidental symlink at `~/Library/Caches/evil` -> `/System/Library` causes the tool to operate on SIP-protected files.
**Why it happens:** Checking the literal path without resolving symlinks first.
**How to avoid:** Call `filepath.EvalSymlinks()` before blocklist validation. Block if resolution fails on an existing path.
**Warning signs:** Safety tests only use literal paths, never test symlink scenarios.

### Pitfall 3: `/var` vs `/private/var` Confusion
**What goes wrong:** macOS has `/var` as a symlink to `/private/var`. Blocking `/private/var/vm` doesn't catch `/var/vm` if you check before symlink resolution.
**Why it happens:** macOS path structure is unusual. `/var`, `/tmp`, `/etc` are all symlinks to their `/private/` counterparts.
**How to avoid:** Resolve symlinks first. After resolution, `/var/vm/swapfile0` becomes `/private/var/vm/swapfile0`, which matches the blocklist.
**Warning signs:** Swap file protection tests pass with `/private/var/vm/` but not `/var/vm/`.

### Pitfall 4: Missing `/usr/local` Exception
**What goes wrong:** Blocking `/usr` blocks `/usr/local`, which is writable and contains Homebrew installations.
**Why it happens:** SIP protects `/usr` but explicitly excludes `/usr/local`.
**How to avoid:** Maintain an exception list. Check exceptions before blocking.
**Warning signs:** Homebrew cache scanning fails in later phases because paths under `/usr/local` are blocked.

### Pitfall 5: go.mod Version Directive Format
**What goes wrong:** Using `go 1.24.0` (with patch) or `go 1.22.1` in go.mod causes issues with older toolchains.
**Why it happens:** The `go` directive in go.mod should use the language version format (`go 1.24`), not a specific patch version.
**How to avoid:** Use `go 1.24` (no patch version) in go.mod.
**Warning signs:** CI builds fail with "invalid go version" errors.

### Pitfall 6: Cobra Default Version Output
**What goes wrong:** Running `mac-cleaner --version` outputs `mac-cleaner version 0.1.0` instead of just `0.1.0`.
**Why it happens:** Cobra's default version template includes the command name.
**How to avoid:** Call `rootCmd.SetVersionTemplate("{{.Version}}\n")` to output only the version number.
**Warning signs:** Version output doesn't match the user requirement of "just the version number."

## Code Examples

### Complete main.go
```go
// Source: Go conventions + Cobra docs
package main

import "github.com/gregor/mac-cleaner/cmd"

func main() {
    cmd.Execute()
}
```

### Building with Version Injection
```bash
# Development build
go build -o mac-cleaner .

# Release build with version
go build -ldflags "-X github.com/gregor/mac-cleaner/cmd.version=0.1.0" -o mac-cleaner .

# Verify
./mac-cleaner --version
# Output: 0.1.0
```

### Safety Layer Test Pattern (Table-Driven)
```go
// Source: Go testing conventions
package safety

import "testing"

func TestIsPathBlocked(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        blocked bool
        reason  string
    }{
        // SIP-protected paths
        {"System root", "/System", true, "SIP-protected"},
        {"System subpath", "/System/Library/Caches", true, "SIP-protected"},
        {"usr root", "/usr", true, "SIP-protected"},
        {"usr bin", "/usr/bin", true, "SIP-protected"},
        {"bin root", "/bin", true, "SIP-protected"},
        {"sbin root", "/sbin", true, "SIP-protected"},

        // SIP exceptions
        {"usr local", "/usr/local", false, ""},
        {"usr local bin", "/usr/local/bin", false, ""},
        {"usr local Homebrew", "/usr/local/Cellar", false, ""},

        // Swap/VM files
        {"vm dir", "/private/var/vm", true, "swap/VM file"},
        {"swap file", "/private/var/vm/swapfile0", true, "swap/VM file"},
        {"sleep image", "/private/var/vm/sleepimage", true, "swap/VM file"},

        // Safe paths (should NOT be blocked)
        {"user caches", "/Users/test/Library/Caches", false, ""},
        {"library caches", "/Library/Caches", false, ""},
        {"tmp", "/tmp", false, ""},
        {"applications", "/Applications", false, ""},

        // Path boundary edge cases
        {"SystemVolume", "/SystemVolume", false, ""},
        {"usr-local-like", "/usrlocal", false, ""},
        {"sbin-like", "/sbinaries", false, ""},

        // Path traversal attempts
        {"traversal System", "/System/../System/Library", true, "SIP-protected"},
        {"traversal usr", "/usr/local/../../usr/bin", true, "SIP-protected"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            blocked, reason := IsPathBlocked(tt.path)
            if blocked != tt.blocked {
                t.Errorf("IsPathBlocked(%q) = %v, want %v", tt.path, blocked, tt.blocked)
            }
            if tt.blocked && reason == "" {
                t.Errorf("IsPathBlocked(%q) blocked but no reason given", tt.path)
            }
            if !tt.blocked && reason != "" {
                t.Errorf("IsPathBlocked(%q) not blocked but reason given: %q", tt.path, reason)
            }
        })
    }
}
```

### Stderr Warning Output
```go
// Usage in scanning code (future phases):
if blocked, reason := safety.IsPathBlocked(path); blocked {
    safety.WarnBlocked(path, reason)
    // Output to stderr: SKIP: /System (SIP-protected)
    continue
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `filepath.HasPrefix()` | `strings.HasPrefix(path, prefix+"/")` | Deprecated in Go (no removal date) | `filepath.HasPrefix` does not respect path boundaries; use manual prefix+separator check |
| `filepath.Walk()` | `io/fs.WalkDir()` | Go 1.16 (2021) | WalkDir is more efficient (avoids extra `os.Lstat` calls). Relevant for Phase 2 scanning. |
| Manual symlink handling | `os.Root` / `os.OpenInRoot` | Go 1.24 (Feb 2025) | Traversal-resistant file API. Not needed for blocklist validation, but useful for future scan confinement. |
| `flag` stdlib | Cobra | Cobra v1.0 (2020) | Cobra is the standard for Go CLIs with version support, help generation, subcommands. |

## Open Questions

1. **Config file format for user-added protected paths**
   - What we know: User wants config-based extra protected paths (CONTEXT.md decision).
   - What's unclear: File format (TOML, YAML, JSON), location (`~/.config/mac-cleaner/config.toml`?).
   - Recommendation: Defer to a later phase. Phase 1 only has hardcoded protections. When config comes, use Viper (pairs with Cobra, supports TOML/YAML/JSON).

2. **Pre-installed macOS apps in /Applications**
   - What we know: `/Applications` for pre-installed apps is SIP-protected; third-party apps in `/Applications` are not.
   - What's unclear: Whether to block `/Applications` scanning entirely.
   - Recommendation: Do NOT block `/Applications` in Phase 1. The safety layer only blocks the four SIP directories (`/System`, `/usr`, `/bin`, `/sbin`) and `/private/var/vm`. Application-level safety comes in later phases with risk categorization.

## Sources

### Primary (HIGH confidence)
- [Go Modules Reference](https://go.dev/ref/mod) - go.mod directives, version format
- [Cobra User Guide (GitHub)](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md) - Root command, Version field, flags patterns
- [Cobra pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) - SetVersionTemplate API
- [Go os.Root Blog Post](https://go.dev/blog/osroot) - Traversal-resistant file APIs in Go 1.24
- [Go filepath package](https://pkg.go.dev/path/filepath) - Clean, EvalSymlinks, HasPrefix deprecation
- [Apple SIP Documentation](https://support.apple.com/en-us/102149) - Official SIP protected path list
- [SIP Wikipedia](https://en.wikipedia.org/wiki/System_Integrity_Protection) - Protected directories: /System, /usr, /bin, /sbin

### Secondary (MEDIUM confidence)
- [Cobra Version Flag Pattern](https://www.jvt.me/posts/2023/05/27/go-cobra-version/) - SetVersionTemplate and ldflags pattern
- [Go Traversal-Resistant Security](https://icinga.com/blog/secure-file-operations-in-go-with-os-root-preventing-path-traversal/) - os.Root usage patterns
- [Go Path Traversal Guide](https://www.stackhawk.com/blog/golang-path-traversal-guide-examples-and-prevention/) - filepath.Clean + EvalSymlinks pattern
- [Go Project Layout](https://github.com/golang-standards/project-layout) - cmd/, internal/, pkg/ conventions
- [mac-cleanup-go](https://github.com/2ykwang/mac-cleanup-go) - Reference implementation of Go macOS cleaner with SIP protection
- [macOS VM/Swap Files](https://forums.macrumors.com/threads/what-is-this-file-private-var-vm-sleepimage.1710852/) - /private/var/vm contents: swapfile*, sleepimage, kernelcore

### Tertiary (LOW confidence)
- [SIP Path Checking via Terminal](https://www.alansiu.net/2020/09/09/terminal-command-to-tell-if-a-macos-directory-is-sip-protected/) - `ls -lO` shows `restricted` flag for SIP paths

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go, Cobra are well-documented with stable APIs. Local Go version (1.25.7) confirmed.
- Architecture: HIGH - Project structure follows Go conventions. Safety pattern uses well-understood stdlib functions.
- Pitfalls: HIGH - Path boundary issues, symlink bypass, /var symlink are all documented and verifiable.

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, unlikely to change)
