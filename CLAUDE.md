# CLAUDE.md

## Build & Run

```bash
go build -o mac-cleaner .                    # build binary
go build -ldflags "-X github.com/sp3esu/mac-cleaner/cmd.version=0.1.0" -o mac-cleaner .  # build with version
go test ./...                                # run all tests
go test ./internal/safety/...                # run tests for one package
```

## Architecture

Go CLI (cobra) for scanning and cleaning macOS junk files. Entry point: `main.go` -> `cmd.Execute()`.

### Layout

- `cmd/` — CLI root command, flag definitions, output formatting (cobra)
- `internal/` — private packages:
  - `cleanup/` — file deletion execution
  - `confirm/` — interactive confirmation prompts
  - `interactive/` — walkthrough mode (category-by-category selection)
  - `safety/` — path blocking (SIP, swap/VM) and risk level classification
  - `scan/` — shared types (`ScanEntry`, `CategoryResult`, `ScanSummary`) and helpers (`DirSize`, `ScanTopLevel`, `FormatSize`)
- `pkg/` — scanner implementations per category:
  - `system/` — user caches, logs, QuickLook
  - `browser/` — Safari, Chrome, Firefox
  - `developer/` — Xcode, npm, yarn, Homebrew, Docker, pnpm, CocoaPods, Gradle, pip, simulators
  - `appleftovers/` — orphaned prefs, iOS backups, old downloads
  - `creative/` — Adobe, Sketch, Figma
  - `messaging/` — Slack, Discord, Teams, Zoom

### Key patterns

- Each `pkg/*/scanner.go` exports a `Scan() ([]scan.CategoryResult, error)` function
- Scanners resolve the home directory, scan filesystem paths, call `safety.IsPathBlocked` before deletion, and set risk levels via `CategoryResult.SetRiskLevels(safety.RiskForCategory)`
- Risk levels: `safe`, `moderate`, `risky` (constants in `internal/safety/risk.go`)
- Category IDs (e.g. `"dev-xcode"`, `"browser-safari"`) are used for skip-flag filtering and risk mapping
- Version is injected via ldflags: `-X github.com/sp3esu/mac-cleaner/cmd.version=...`

## Conventions

- Standard Go project layout: `internal/` for private, `pkg/` for scanner implementations
- Tests use stdlib `testing` with `t.TempDir()` for filesystem tests — no test frameworks
- Package-level doc comments on every package
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Permission errors collected as `PermissionIssue` rather than failing the scan
- Safety checks (SIP/swap blocking) resolve symlinks before checking path prefixes
- Every change, fix, or new feature must include tests covering all code paths

## Releasing

### Infrastructure

- `.goreleaser.yml` — GoReleaser v2 config: builds dual-arch darwin binaries (`amd64`/`arm64`), publishes to `sp3esu/homebrew-tap`, generates changelog
- `.github/workflows/release.yml` — triggers on `v*` tag push, runs GoReleaser
- Version is injected from the git tag via ldflags (no code changes needed for a release)

### Version bump rules

Commits use conventional prefixes: `feat:`, `fix:`, `chore:`, `docs:`, `test:`

- **Patch** (e.g. `v1.1.0` -> `v1.1.1`): only `fix:`, `docs:`, `test:`, `chore:` commits since last tag
- **Minor** (e.g. `v1.1.0` -> `v1.2.0`): at least one `feat:` commit since last tag
- **Major** (e.g. `v1.1.0` -> `v2.0.0`): breaking changes (indicated by `BREAKING CHANGE` in commit body or `!` after type, e.g. `feat!:`)

### Pre-release checklist

- Must be on `main` branch
- Working tree must be clean (no uncommitted changes)
- All tests must pass (`go test ./...`)

### Release procedure

```bash
# 1. Get the latest tag
git tag --sort=-v:refname | head -1

# 2. List commits since last tag to determine bump type
git log --oneline <last-tag>..HEAD

# 3. Ensure working tree is clean
git status

# 4. Ensure all tests pass
go test ./...

# 5. Create annotated tag and push
git tag -a v<X.Y.Z> -m "Release v<X.Y.Z>"
git push origin v<X.Y.Z>

# 6. Verify
gh run list --workflow=release.yml --limit=1
gh run watch <run-id>
gh release view v<X.Y.Z>
```
