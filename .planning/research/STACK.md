# Technology Stack

**Project:** mac-cleaner
**Researched:** 2026-02-16
**Confidence:** HIGH

## Executive Recommendation

**Use Go** for building this macOS CLI disk cleaning tool. Go offers the best balance of simplicity, ecosystem maturity, cross-compilation, and single binary distribution for CLI tools targeting macOS users.

## Recommended Stack

### Core Language & Runtime

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26+ | Primary language | Single binary distribution, excellent stdlib for file operations, fast compilation, simple cross-compilation, mature CLI ecosystem. Go 1.26 released February 2026 with latest improvements. |
| Cobra | v1.10.2+ | CLI framework | Industry standard used by kubectl, docker, hugo, GitHub CLI. Sophisticated command tree architecture, automatic help generation, persistent flags, pre/post hooks. Latest stable release December 2024. |
| Viper | Latest | Configuration management | Pairs perfectly with Cobra for handling config files, environment variables, and CLI flags in a unified way. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | JSON output | Built-in, zero dependencies. Use for --json flag output. encoding/json/v2 published Feb 2026 with improved semantic processing. |
| path/filepath | stdlib | File path operations | Cross-platform path handling. Use filepath.Walk or io/fs.WalkDir (more efficient, introduced Go 1.16+) for directory traversal. |
| os | stdlib | File operations | File stat, removal, size calculation. Native macOS path expansion for ~/Library paths. |
| fatih/color | v1.18.0+ | Colored terminal output | Automatic tty detection, NO_COLOR env var support, Windows compatibility. Use for interactive mode visual feedback. |
| go-isatty | Latest | Terminal detection | Auto-detect if output is terminal or pipe. Essential for disabling colors in --json mode. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| GoReleaser | Binary distribution | Automates cross-platform builds, GitHub releases, Homebrew tap updates, checksums. Set CGO_ENABLED=0 for fully static binaries. |
| go test | Unit testing | Built-in test framework, table-driven tests for file scanning logic. |
| golangci-lint | Code quality | Aggregates multiple linters. Run in CI/CD pipeline. |

## Installation

```bash
# Initialize Go module
go mod init github.com/yourusername/mac-cleaner

# Core dependencies
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/fatih/color@latest
go get github.com/mattn/go-isatty@latest

# Development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/goreleaser/goreleaser/v2@latest
```

## Build & Distribution

```bash
# Local development build
go build -o mac-cleaner

# Production release build (via GoReleaser)
goreleaser release --snapshot --clean  # Test locally first
goreleaser release  # Create GitHub release with multi-arch binaries
```

## Alternatives Considered

| Category | Recommended | Alternative | Why Not Alternative |
|----------|-------------|-------------|---------------------|
| Language | **Go** | Rust | Rust has steeper learning curve, slower compilation, more complex ecosystem. Binary sizes comparable (Go: ~2.5MB, Rust: ~2.1MB). Rust's safety guarantees less critical for file scanning tool. Go's simplicity wins for this use case. |
| Language | **Go** | Swift | Too Apple-centric, requires Xcode toolchain, more complex distribution (need lipo for universal binaries), smaller ecosystem for CLI tools. Native to macOS but worse for general CLI development. |
| CLI Framework | **Cobra** | urfave/cli | urfave/cli better for simple single-command tools. This project needs subcommands (scan, clean, list) making Cobra's command tree architecture superior. |
| CLI Framework | **Cobra** | bubbletea | bubbletea is for TUI (terminal user interface) apps with interactive menus/dashboards. Overkill for this tool which needs simple flag-based + interactive prompts. |
| JSON Library | **stdlib encoding/json** | github.com/goccy/go-json | Third-party JSON libraries offer marginal performance gains. Stdlib is sufficient, zero dependencies, well-tested. encoding/json/v2 (Feb 2026) improved semantic processing. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Python | Requires Python runtime on user's machine, slower execution, harder single-binary distribution | Go with single static binary |
| Node.js/JavaScript | Requires Node.js runtime, larger distribution size, slower startup time | Go with single static binary |
| Bash/Shell scripts | Limited error handling, hard to maintain, no structured JSON output, poor cross-platform support | Go for robust CLI with proper error handling |
| CGO_ENABLED=1 | Creates dynamic binaries requiring C libraries at runtime. Breaks portability. | CGO_ENABLED=0 for fully static binaries |
| os.Getwd() for home expansion | Doesn't expand ~ in paths like ~/Library/Caches | os.UserHomeDir() + filepath.Join() |
| filepath.Walk | Less efficient, calls os.Lstat on every file | io/fs.WalkDir (since Go 1.16) |

## Stack Patterns by Variant

**For basic MVP (Phase 1):**
- Use Cobra for CLI structure
- Use stdlib only (no external deps except Cobra/Viper)
- JSON output via encoding/json
- Basic colored output via fatih/color

**For production release (Phase 2+):**
- Add GoReleaser for distribution
- Homebrew tap for easy installation
- Code signing with Apple Developer ID (requires $99/year Apple Developer Program)
- Notarization via xcrun notarytool for distribution trust

**For AI agent integration (future):**
- Ensure --json flag outputs structured data
- Use consistent exit codes (0 = success, 1 = error, 2 = nothing to clean)
- Provide machine-readable error messages in JSON mode

## macOS-Specific Considerations

### Code Signing & Distribution

| Requirement | Solution | Notes |
|-------------|----------|-------|
| Code signing | Apple Developer ID certificate + codesign tool | Required for distribution outside App Store. Free with $99/year Apple Developer Program. |
| Notarization | xcrun notarytool | Required since macOS Catalina. Prevents "unidentified developer" warnings. |
| Architecture | Universal binary (arm64 + x86_64) | 2026 is last macOS version for Intel. Build both, combine with lipo or use GoReleaser multi-arch. |
| Distribution | Homebrew tap | Best practice for CLI tools. Users install via `brew install yourusername/tap/mac-cleaner`. |

### File System Paths

Common macOS cache locations to target:

```go
// Use os.UserHomeDir() to get home directory
home, _ := os.UserHomeDir()

paths := []string{
    filepath.Join(home, "Library", "Caches"),
    filepath.Join(home, "Library", "Logs"),
    "/Library/Caches",
    "/System/Library/Caches",
    filepath.Join(home, ".npm"),
    filepath.Join(home, ".cache"),
}
```

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| Go 1.26+ | Cobra v1.10.2+ | Cobra works with Go 1.15+, but use latest Go for performance |
| Cobra v1.10.2 | Viper latest | Designed to work together, same maintainer (spf13) |
| fatih/color v1.18.0 | Go 1.13+ | Supports NO_COLOR env var, Windows compatibility |
| GoReleaser v2 | Go 1.21+ | Requires recent Go version for build features |

## Performance Characteristics

| Aspect | Go Performance | Notes |
|--------|---------------|-------|
| Binary size | 2-8 MB (typical CLI) | Can strip debug symbols to reduce. Stripping reduces to ~8.5 MB for larger apps. |
| Compilation speed | Sub-second for incremental builds | Much faster than Rust (minutes). Enables rapid development iteration. |
| Runtime performance | Excellent for I/O-bound tasks | File scanning is I/O-bound. Go's goroutines ideal for concurrent file scanning. |
| Startup time | Near-instant | No JVM/runtime warmup. Critical for CLI responsiveness. |
| Memory usage | Minimal for file operations | Efficient garbage collection. Consider streaming for large file lists. |

## Confidence Assessment

| Technology Choice | Confidence | Source Quality |
|-------------------|-----------|----------------|
| Go as primary language | **HIGH** | Official Go docs, JetBrains ecosystem analysis 2025, multiple comparison articles, GitHub ecosystem data |
| Cobra framework | **HIGH** | Official docs, widespread adoption (kubectl, docker, hugo, GitHub CLI), active maintenance (Dec 2024 release) |
| stdlib for file operations | **HIGH** | Official Go docs, path/filepath and io/fs documented patterns |
| GoReleaser for distribution | **HIGH** | Official docs, widespread use in Go CLI ecosystem, Homebrew integration patterns |
| fatih/color for output | **MEDIUM** | GitHub documentation, community adoption, but verified against official Go color handling patterns |
| Rust/Swift alternatives | **MEDIUM** | Comparison articles from 2025-2026, JetBrains analysis, community discussions |

## Sources

### Language & Ecosystem
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26) - Official release information (Feb 2026)
- [Go Ecosystem in 2025: Key Trends](https://blog.jetbrains.com/go/2025/11/10/go-language-trends-ecosystem-2025/) - HIGH confidence
- [Rust vs Go in 2025](https://blog.jetbrains.com/rust/2025/06/12/rust-vs-go/) - Comparison analysis
- [Go Solutions: CLIs](https://go.dev/solutions/clis) - Official guidance

### CLI Frameworks
- [Cobra GitHub](https://github.com/spf13/cobra) - v1.10.2 release (Dec 2024)
- [Cobra Official Site](https://cobra.dev/) - Framework documentation
- [Clap (Rust) GitHub](https://github.com/clap-rs/clap) - v4.5.58 release (Feb 2026)
- [Swift ArgumentParser](https://github.com/apple/swift-argument-parser) - v1.7.0 release (Dec 2024)

### File Operations
- [path/filepath Go Package](https://pkg.go.dev/path/filepath) - Official stdlib docs
- [io/fs Go Package](https://pkg.go.dev/io/fs) - Modern filesystem interface
- [Walking with filesystems: Go's new fs.FS interface](https://bitfieldconsulting.com/posts/filesystems) - WalkDir efficiency analysis

### Distribution
- [GoReleaser Documentation](https://goreleaser.com/) - Official distribution tool docs
- [Creating Homebrew Taps Guide](https://kristoffer.dev/blog/guide-to-creating-your-first-homebrew-tap/) - Tap best practices
- [macOS Code Signing 2026](https://eclecticlight.co/2026/01/17/whats-happening-with-code-signing-and-future-macos/) - Current state of signing

### Supporting Libraries
- [fatih/color GitHub](https://github.com/fatih/color) - Terminal color library
- [encoding/json/v2](https://pkg.go.dev/encoding/json/v2) - Published Feb 2026

### Comparisons
- [Rust vs Go binary sizes](https://www.nicolas-hahn.com/python/go/rust/programming/2019/07/01/program-in-python-go-rust/) - Size comparison
- [Go Binary Optimization](https://oneuptime.com/blog/post/2026-01-07-go-reduce-binary-size/view) - Size reduction techniques
- [Swift CLI Distribution](https://www.swifttoolkit.dev/posts/distribute-swift-clis) - macOS distribution patterns

---
*Stack research for: macOS CLI disk cleaning tool*
*Researched: 2026-02-16*
*Recommendation: Go 1.26+ with Cobra framework for optimal CLI development experience*
