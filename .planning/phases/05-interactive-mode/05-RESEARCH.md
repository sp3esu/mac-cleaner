# Phase 5: Interactive Mode - Research

**Researched:** 2026-02-16
**Domain:** Interactive CLI walkthrough with per-item keep/remove prompting, testable stdin/stdout, Cobra command wiring
**Confidence:** HIGH

## Summary

Phase 5 replaces the current no-args `cmd.Help()` behavior with an interactive walkthrough mode. When the user runs `mac-cleaner` with no flags, the tool scans all four categories (system caches, browser data, dev caches, app leftovers), then presents each scan entry one-by-one with its description and size, prompting the user to respond "keep" or "remove" for each item. After all items are presented, the tool summarizes everything marked for removal, asks a final "yes" confirmation (reusing the established confirmation pattern), and then executes cleanup using the existing `cleanup.Execute` function.

The implementation requires a new `internal/interactive` package containing the walkthrough logic. This package accepts `io.Reader` and `io.Writer` (following the established `confirm` package pattern) so all interactive prompting is fully testable without stdin/stdout coupling. The core function iterates over all `[]scan.CategoryResult` entries, displays each with its category context and formatted size, reads a line of input, and classifies the response as "keep" or "remove." Items marked for removal are collected into a new `[]scan.CategoryResult` slice that feeds into the existing `confirm.PromptConfirmation` and `cleanup.Execute` pipeline.

The scanning side is already complete -- all four scanner packages export `Scan()` returning `[]scan.CategoryResult`. The deletion side is already complete -- `cleanup.Execute` takes `[]scan.CategoryResult` and handles safety re-checks, pseudo-path filtering, and error continuation. The only new work is: (1) the interactive prompt loop that filters entries by user choice, (2) wiring the no-args path in `cmd/root.go` to call this instead of `cmd.Help()`, and (3) formatting output to make the walkthrough clear and navigable.

**Primary recommendation:** Build `internal/interactive/interactive.go` with a `RunWalkthrough(in io.Reader, out io.Writer, results []scan.CategoryResult) []scan.CategoryResult` function that returns only the items marked for removal. Wire in `cmd/root.go` by replacing `cmd.Help()` with the full interactive flow: scan all -> walkthrough -> confirm -> execute -> summarize.

## Standard Stack

### Core (no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| bufio (stdlib) | -- | `bufio.NewReader` for reading per-item keep/remove input | Already used in `internal/confirm` for the same pattern |
| io (stdlib) | -- | `io.Reader`/`io.Writer` interfaces for testable prompting | Established pattern in `confirm.PromptConfirmation` |
| fmt (stdlib) | -- | `fmt.Fprintf` for writing prompts to the output writer | Already used throughout codebase |
| strings (stdlib) | -- | `strings.TrimSpace` and `strings.ToLower` for input normalization | Already used throughout codebase |
| fatih/color v1.18.0 | already in go.mod | Bold headers, cyan sizes, color-coded prompts | Established output pattern from Phases 2-4 |

### Not Needed

| Library | Why Not |
|---------|---------|
| charmbracelet/huh | Full-featured TUI form library. Overkill for simple line-by-line keep/remove. Adds dependency, complexity, and a different interaction model (cursor-based) that conflicts with the simple text flow. |
| manifoldco/promptui | Interactive select/confirm library. Adds unnecessary dependency when `bufio.NewReader` + `ReadString('\n')` is sufficient for the simple keep/remove choice. |
| charmbracelet/bubbletea | Full TUI framework. Massive overkill for sequential text prompts. Would change the entire interaction model. |
| survey/v2 (AlecAivazis) | Archived. Was popular but no longer maintained. |

**Installation:** No new dependencies. Phase 5 uses only stdlib plus the already-imported `fatih/color`.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| bufio.NewReader line reads | bufio.Scanner | Scanner is slightly cleaner for loop-based reading, but NewReader matches the existing `confirm` package pattern exactly. Consistency wins. |
| fatih/color for prompt highlighting | Plain fmt | Color makes the interactive flow much more navigable. Already in go.mod. No reason to skip it. |

## Architecture Patterns

### Recommended Project Structure (Phase 5 additions)

```
mac-cleaner/
├── cmd/
│   └── root.go                    # Replace Help() with interactive mode when no flags
├── internal/
│   ├── interactive/
│   │   ├── interactive.go         # RunWalkthrough(in, out, results) []CategoryResult
│   │   └── interactive_test.go    # Tests with strings.NewReader for input injection
│   ├── confirm/                   # (Phase 4 - unchanged)
│   ├── cleanup/                   # (Phase 4 - unchanged)
│   ├── safety/                    # (Phase 1 - unchanged)
│   └── scan/                      # (Phase 2 - unchanged)
```

### Pattern 1: Interactive Walkthrough Loop

**What:** Iterates over all scan entries, displays each with category context and size, reads user input ("keep" or "remove"), and collects marked-for-removal entries into a filtered result slice.

**When to use:** When the user runs `mac-cleaner` with no flags.

**Key design decisions:**

1. **Accept `io.Reader` and `io.Writer`** -- Same testability pattern as `confirm.PromptConfirmation`. Tests inject `strings.NewReader("remove\nkeep\nremove\n")` and `bytes.Buffer`.

2. **Input normalization:** `strings.TrimSpace` then `strings.ToLower`. Accept "remove", "r", "keep", "k" as valid responses. On unrecognized input, re-prompt (don't assume keep or remove).

3. **Return filtered `[]scan.CategoryResult`** -- The function returns a new slice containing only entries the user marked for removal. Categories with no removed entries are excluded entirely. This feeds directly into the existing `confirm.PromptConfirmation` and `cleanup.Execute` pipeline.

4. **Display format per item:** Show category name as a header when entering a new category, then for each entry show description + formatted size + prompt. Use fatih/color for the size (cyan, matching existing output) and bold for category headers.

5. **Progress indicator:** Show item number and total count (e.g., "[3/47]") so the user knows how far through the walkthrough they are.

6. **Empty results handling:** If scan returns zero items across all categories, print a message and return nil (no walkthrough needed).

7. **All-keep handling:** If the user keeps everything, print a message and return nil (nothing to delete, skip confirmation).

**Example approach:**
```go
func RunWalkthrough(in io.Reader, out io.Writer, results []scan.CategoryResult) []scan.CategoryResult {
    reader := bufio.NewReader(in)

    // Count total items for progress display
    totalItems := 0
    for _, cat := range results {
        totalItems += len(cat.Entries)
    }

    if totalItems == 0 {
        fmt.Fprintln(out, "Nothing to clean.")
        return nil
    }

    currentItem := 0
    var removeResults []scan.CategoryResult

    for _, cat := range results {
        var removedEntries []scan.ScanEntry
        var removedSize int64

        // Print category header
        bold.Fprintf(out, "\n%s\n", cat.Description)

        for _, entry := range cat.Entries {
            currentItem++
            // Display entry and prompt
            fmt.Fprintf(out, "  [%d/%d] %s  %s\n", currentItem, totalItems,
                entry.Description, cyan.Sprint(scan.FormatSize(entry.Size)))
            fmt.Fprint(out, "  keep or remove? [k/r]: ")

            response := readChoice(reader)
            if response == "remove" || response == "r" {
                removedEntries = append(removedEntries, entry)
                removedSize += entry.Size
            }
        }

        if len(removedEntries) > 0 {
            removeResults = append(removeResults, scan.CategoryResult{
                Category:    cat.Category,
                Description: cat.Description,
                Entries:     removedEntries,
                TotalSize:   removedSize,
            })
        }
    }

    return removeResults
}
```

### Pattern 2: Choice Reading with Re-prompt

**What:** Reads a line from the reader, normalizes it, and validates it as a keep/remove choice. Re-prompts on invalid input.

**When to use:** For each item in the walkthrough.

**Key design decisions:**

1. **Accept shorthand:** "k" for keep, "r" for remove, plus full words "keep" and "remove". Case-insensitive after TrimSpace + ToLower.
2. **Re-prompt on invalid input:** Don't silently default. Print a hint and ask again. This prevents accidental selections.
3. **EOF handling:** If the reader reaches EOF (e.g., piped input runs out), treat remaining items as "keep" (safe default).

**Example approach:**
```go
func readChoice(reader *bufio.Reader, out io.Writer) string {
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            return "keep" // EOF or error = safe default
        }
        choice := strings.ToLower(strings.TrimSpace(line))
        switch choice {
        case "r", "remove":
            return "remove"
        case "k", "keep":
            return "keep"
        default:
            fmt.Fprint(out, "  Please enter 'k' to keep or 'r' to remove: ")
        }
    }
}
```

### Pattern 3: Root Command Interactive Flow

**What:** When no scan flags are set, run the full interactive flow instead of displaying help.

**When to use:** The `!ran` branch in `cmd/root.go`.

**Key design decisions:**

1. **Replace `cmd.Help()` with interactive mode** -- The `!ran` block currently calls `cmd.Help()`. Replace with: scan all categories -> run walkthrough -> if items marked, confirm -> execute -> summarize.
2. **Respect `--dry-run`** -- In interactive mode with `--dry-run`, run the walkthrough but skip the confirmation+deletion step. Show what would be removed.
3. **Aggregate all scan results** -- Call all four `run*Scan` helpers (or the scanners directly) to get combined results before starting the walkthrough.

**Example wiring in root.go:**
```go
if !ran {
    // Interactive mode: scan all, walkthrough, confirm, execute
    allResults := scanAll()
    if len(allResults) == 0 {
        fmt.Println("Nothing to clean.")
        return
    }

    marked := interactive.RunWalkthrough(os.Stdin, os.Stdout, allResults)
    if len(marked) == 0 {
        fmt.Println("Nothing marked for removal.")
        return
    }

    if !flagDryRun {
        if !confirm.PromptConfirmation(os.Stdin, os.Stdout, marked) {
            fmt.Println("Aborted.")
            return
        }
        result := cleanup.Execute(marked)
        printCleanupSummary(result)
    }
}
```

### Pattern 4: scanAll Helper

**What:** Extracts the scan-all-categories logic into a reusable helper to avoid duplicating scanner calls.

**When to use:** Interactive mode needs to scan all categories, which currently is spread across four separate flag-gated blocks.

**Key design decisions:**

1. **Private helper function in cmd/root.go** -- `func scanAll() []scan.CategoryResult` that calls all four scanners and aggregates results.
2. **Print results per category during scan** -- Use `printResults` for each category as the scan runs, so the user sees what's being found before the walkthrough starts.
3. **Error handling** -- Same pattern as flag-gated scan: stderr warnings, continue on error, skip nil results.

### Anti-Patterns to Avoid

- **Hardcoding os.Stdin/os.Stdout in the interactive package:** Makes testing impossible. Always accept io.Reader/io.Writer. This is the established pattern from `internal/confirm`.
- **Defaulting to "remove" on invalid input:** Dangerous. Invalid input should re-prompt, and EOF should default to "keep" (safe default).
- **Building a new confirmation flow:** The confirmation prompt already exists in `internal/confirm`. Reuse it. The walkthrough just filters the results; the existing confirmation handles the final gate.
- **Modifying scan.CategoryResult or ScanEntry types:** No type changes needed. The interactive package works entirely with the existing types, just filtering which entries to pass downstream.
- **Using bufio.Scanner instead of bufio.NewReader:** Scanner is fine, but NewReader with `ReadString('\n')` matches the established pattern in `confirm.go`. Consistency within the codebase is more valuable than marginal API preference.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Final confirmation before deletion | A new confirmation dialog | Existing `confirm.PromptConfirmation` | Already built, tested, and uses the exact io.Reader/io.Writer pattern needed |
| File deletion with safety | Custom deletion logic | Existing `cleanup.Execute` | Already built with safety re-checks, pseudo-path filtering, error continuation |
| Scan result types | New types for interactive results | Existing `scan.CategoryResult`, `scan.ScanEntry` | The walkthrough just filters existing results. No new types needed. |
| Size formatting | Custom formatter | Existing `scan.FormatSize` | Already handles SI units matching macOS Finder convention |
| Color output | Direct ANSI codes | Existing `fatih/color` usage patterns | Already imported and used consistently across the codebase |

**Key insight:** Phase 5's new code is almost entirely the interactive prompt loop. Everything upstream (scanning) and downstream (confirmation, deletion, summary) already exists and is tested. The interactive package is a thin filtering layer between scan results and the existing deletion pipeline.

## Common Pitfalls

### Pitfall 1: Shared bufio.Reader Between Walkthrough and Confirmation

**What goes wrong:** The walkthrough uses `bufio.NewReader(os.Stdin)` and the confirmation prompt also creates `bufio.NewReader(os.Stdin)`. The first reader may buffer data that the second reader never sees, causing the confirmation to read stale or missing data.
**Why it happens:** `bufio.NewReader` reads ahead into its buffer. If the walkthrough's reader reads more bytes than the current line, those bytes are consumed from stdin and never available to a new reader.
**How to avoid:** Either (a) pass the same `bufio.Reader` to both the walkthrough and confirmation functions, or (b) have the walkthrough function handle the entire flow including confirmation (passing `io.Reader` through, not creating a new `bufio.Reader` per stage). The simplest approach: the interactive flow in `cmd/root.go` passes `os.Stdin` to the walkthrough, and the walkthrough creates one `bufio.Reader` internally. For the final confirmation, either integrate it into the walkthrough function or refactor `confirm.PromptConfirmation` to accept a `*bufio.Reader` instead of `io.Reader`. **Recommendation:** Have the `cmd/root.go` interactive flow create a single `bufio.Reader` from `os.Stdin` and pass it to both `interactive.RunWalkthrough` and `confirm.PromptConfirmation`. This requires `confirm.PromptConfirmation` to accept `io.Reader` (which `*bufio.Reader` satisfies since it implements `io.Reader`). The existing `confirm.PromptConfirmation` already accepts `io.Reader` and creates its own `bufio.NewReader` internally -- wrapping a `bufio.Reader` in another `bufio.Reader` is safe (no double-buffering issue) but wasteful. Better: have the interactive flow call confirmation directly using the same reader, or accept that the data flow is sequential (walkthrough finishes before confirmation starts) and the bufio.Reader created by confirm will inherit the same underlying os.Stdin. **Actual resolution:** Since `RunWalkthrough` returns before `PromptConfirmation` is called, and both create their own `bufio.NewReader` from the same `io.Reader` (os.Stdin), there IS a problem: the walkthrough's `bufio.NewReader` may have buffered extra bytes beyond the last `\n` it processed. However, in practice with line-oriented terminal input, each `ReadString('\n')` consumes exactly one line and the terminal sends lines one at a time, so no extra data will be buffered. **But in tests**, `strings.NewReader("r\nk\nr\nyes\n")` will be fully buffered by the first `bufio.NewReader`, leaving nothing for the second. **Solution:** Create one `bufio.Reader` in the calling code and pass it through. Or: the interactive function handles the entire flow (walkthrough + confirm) internally.
**Warning signs:** Tests pass individually but fail when walkthrough and confirmation are composed together. Confirmation always returns false despite correct input.

### Pitfall 2: Not Handling Empty Scan Results

**What goes wrong:** Running the walkthrough when no scannable items exist on the system, leading to a confusing empty prompt session.
**Why it happens:** Some systems have very clean caches, or all scanners return nil.
**How to avoid:** Check if the aggregated scan results have any entries before starting the walkthrough. Print "Nothing to clean." and return early.
**Warning signs:** User sees category headers with no items underneath them.

### Pitfall 3: Input Validation Too Strict or Too Loose

**What goes wrong:** Either accepting too many inputs as "remove" (e.g., treating typos as remove) or requiring exact full-word input (frustrating for 47+ items).
**Why it happens:** Not defining a clear accepted-input spec.
**How to avoid:** Accept exactly these inputs: "r", "remove" -> remove; "k", "keep" -> keep. Case-insensitive after ToLower. Everything else re-prompts. Empty input (just Enter) re-prompts. This is strict enough to prevent accidents but flexible enough for rapid walkthrough.
**Warning signs:** Users frustrated by having to type full words repeatedly, or accidentally removing items due to loose matching.

### Pitfall 4: Dry-Run Mode Shows Walkthrough Then Confusingly Stops

**What goes wrong:** User walks through 50 items marking some for removal, then the tool says "dry run, nothing deleted" and exits. The user wasted time on the walkthrough.
**Why it happens:** Not adjusting the interactive flow for dry-run mode.
**How to avoid:** In dry-run mode, still run the walkthrough and show the summary of what would be removed, but clearly indicate from the start that this is a dry run. Print "DRY RUN: No files will be deleted." at the top. After the walkthrough, show the removal summary without the final confirmation (since nothing will be deleted). Alternatively, skip the walkthrough entirely in dry-run and just show all scan results (the current flag-based behavior). **Recommendation:** In dry-run + interactive mode, run the full walkthrough so the user can practice the flow, but print a dry-run banner and skip the final confirmation/deletion.
**Warning signs:** User confusion about whether anything was deleted.

### Pitfall 5: Category With All Entries Kept Leaves Orphaned Header

**What goes wrong:** Printing a category header before iterating entries, then the user keeps all entries in that category, resulting in a category header with no removal items in the final summary.
**Why it happens:** Category header is printed eagerly before user makes choices.
**How to avoid:** The walkthrough itself prints headers eagerly (fine -- the user needs context). The final removal summary (shown by `confirm.PromptConfirmation`) only shows categories with entries, so this is handled automatically by filtering empty categories out of the returned results.
**Warning signs:** Orphaned bold headers with no entries under them in the confirmation summary.

## Code Examples

### Example 1: RunWalkthrough Function Signature

```go
// Source: Designed to match existing confirm.PromptConfirmation pattern

// RunWalkthrough presents each scan entry to the user one-by-one and asks
// whether to keep or remove it. Returns a filtered []CategoryResult containing
// only entries marked for removal. Returns nil if no items are marked.
func RunWalkthrough(in io.Reader, out io.Writer, results []scan.CategoryResult) []scan.CategoryResult
```

### Example 2: Per-Item Display Format

```
System Caches

  [1/47] com.apple.Safari            42.1 MB
  keep or remove? [k/r]: r

  [2/47] com.apple.dt.Xcode          1.3 GB
  keep or remove? [k/r]: k

Browser Data

  [3/47] Chrome (Default)            256.8 MB
  keep or remove? [k/r]: r
```

### Example 3: Post-Walkthrough Summary Before Confirmation

After the walkthrough, `confirm.PromptConfirmation` displays the standard deletion summary:

```
The following items will be permanently deleted:

  System Caches
    ~/Library/Caches/com.apple.Safari  (42.1 MB)

  Browser Data
    ~/Library/Caches/Google/Chrome/Default  (256.8 MB)

Total: 298.9 MB will be permanently deleted.
Type 'yes' to proceed:
```

### Example 4: Test with Injected Input

```go
func TestWalkthroughRemovesMarked(t *testing.T) {
    results := []scan.CategoryResult{
        {
            Category:    "test",
            Description: "Test Items",
            Entries: []scan.ScanEntry{
                {Path: "/tmp/a", Description: "item-a", Size: 1000},
                {Path: "/tmp/b", Description: "item-b", Size: 2000},
                {Path: "/tmp/c", Description: "item-c", Size: 3000},
            },
            TotalSize: 6000,
        },
    }

    // User removes first and third, keeps second
    in := strings.NewReader("r\nk\nr\n")
    out := &bytes.Buffer{}

    marked := RunWalkthrough(in, out, results)

    if len(marked) != 1 {
        t.Fatalf("expected 1 category, got %d", len(marked))
    }
    if len(marked[0].Entries) != 2 {
        t.Fatalf("expected 2 entries marked, got %d", len(marked[0].Entries))
    }
    if marked[0].Entries[0].Path != "/tmp/a" {
        t.Errorf("first marked entry should be /tmp/a, got %s", marked[0].Entries[0].Path)
    }
    if marked[0].TotalSize != 4000 {
        t.Errorf("total size should be 4000, got %d", marked[0].TotalSize)
    }
}
```

### Example 5: Root Command Interactive Wiring

```go
// In cmd/root.go, replacing the cmd.Help() call:
if !ran {
    allResults := scanAll()
    if len(allResults) == 0 {
        fmt.Println("Nothing to clean.")
        return
    }

    marked := interactive.RunWalkthrough(os.Stdin, os.Stdout, allResults)
    if len(marked) == 0 {
        fmt.Println("Nothing marked for removal.")
        return
    }

    if flagDryRun {
        // Dry-run: show what would be removed, don't confirm or delete
        return
    }

    if !confirm.PromptConfirmation(os.Stdin, os.Stdout, marked) {
        fmt.Println("Aborted.")
        return
    }
    result := cleanup.Execute(marked)
    printCleanupSummary(result)
}
```

### Example 6: scanAll Helper

```go
func scanAll() []scan.CategoryResult {
    var allResults []scan.CategoryResult

    if results, err := system.Scan(); err == nil {
        allResults = append(allResults, results...)
    }
    if results, err := browser.Scan(); err == nil {
        allResults = append(allResults, results...)
    }
    if results, err := developer.Scan(); err == nil {
        allResults = append(allResults, results...)
    }
    if results, err := appleftovers.Scan(); err == nil {
        allResults = append(allResults, results...)
    }

    return allResults
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Full TUI frameworks (bubbletea, termui) for interactive CLIs | Simple line-oriented prompts for sequential choices | N/A (design choice) | Line-oriented is simpler, more testable, works over SSH, doesn't require terminal capabilities |
| Separate libraries for each prompt type (survey, promptui) | stdlib bufio + io interfaces | N/A (design choice) | Zero dependencies, matches existing codebase patterns |
| readline-style history/completion for prompts | Simple ReadString('\n') | N/A (design choice) | Interactive mode is sequential, not exploratory -- history/completion not needed |

**Deprecated/outdated:**
- `survey/v2` (AlecAivazis/survey): Archived as of 2023. Was the most popular Go prompt library. Successor is `huh` by Charmbracelet.
- `promptui` (manifoldco/promptui): Still maintained but not actively developed. Works but adds unnecessary dependency.

## Open Questions

1. **Should the walkthrough show the scan results table BEFORE starting per-item prompting?**
   - What we know: The PROJECT.md interactive flow says "Scan all categories" then "Present each cleaning item one-by-one." It doesn't specify whether to show an overview first.
   - What's unclear: Whether users benefit from seeing the full scan summary (as in flag-based mode) before starting the item-by-item walkthrough.
   - Recommendation: Show a brief scan summary header ("Found 47 items across 4 categories, 12.3 GB total") before starting the walkthrough. This gives the user context about the scope. Do NOT show the full per-item table (that would duplicate the walkthrough).

2. **Should interactive mode support "remove all in category" or "skip category"?**
   - What we know: The success criteria say "User can respond 'keep' or 'remove' for each item." No mention of category-level shortcuts.
   - What's unclear: Whether walking through 100+ items one-by-one is practical without shortcuts.
   - Recommendation: For Phase 5, implement strict per-item only (matching the success criteria exactly). Category-level shortcuts can be a Phase 6 enhancement. The per-item approach is simpler to implement and test, and users with many items can use flag-based mode instead.

3. **How should the shared bufio.Reader concern be handled?**
   - What we know: Both `RunWalkthrough` and `PromptConfirmation` need to read from stdin. Creating separate `bufio.NewReader` instances on the same underlying `os.Stdin` can cause buffered data loss.
   - What's unclear: Whether this is actually a problem in practice with terminal line-buffered input.
   - Recommendation: Have `RunWalkthrough` accept an `io.Reader` and create a `bufio.NewReader` internally. Have the function also handle the final confirmation internally (calling `confirm.PromptConfirmation` with the same reader or re-implementing the yes/no check). Alternatively, create one `bufio.Reader` in `cmd/root.go` and pass it through. **Best approach:** The interactive flow function should handle everything from walkthrough through confirmation, accepting `io.Reader`/`io.Writer` and returning a `cleanup.CleanupResult` or similar. This avoids the shared-reader problem entirely.

## Sources

### Primary (HIGH confidence)
- **Codebase analysis** -- Direct reading of all source files in `/Users/gregor/projects/mac-clarner/`. All architectural patterns, type definitions, test patterns, and CLI wiring verified from source. This is the primary source for all recommendations.
- [Go bufio package docs](https://pkg.go.dev/bufio) -- `NewReader`, `ReadString` API confirmed
- [Go io package docs](https://pkg.go.dev/io) -- `io.Reader`/`io.Writer` interface definitions
- [Cobra docs - SetIn/SetOut](https://pkg.go.dev/github.com/spf13/cobra) -- Cobra's support for injectable IO streams

### Secondary (MEDIUM confidence)
- [Testing and mocking stdin in Golang](https://petersouter.xyz/testing-and-mocking-stdin-in-golang/) -- Pattern for injecting io.Reader for stdin testing
- [Making a testable Cobra CLI app](https://qua.name/antolius/making-a-testable-cobra-cli-app) -- Dependency injection patterns for Cobra commands
- [How to test CLI commands with Cobra](https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra) -- Testing patterns for Cobra CLI apps
- [Go test and os.Stdin](https://echorand.me/posts/go-test-stdin/) -- bufio.NewReader stdin testing considerations

### Tertiary (LOW confidence)
- None -- all findings verified through codebase analysis or official Go documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies. All stdlib patterns already proven in existing `internal/confirm` package.
- Architecture: HIGH -- Direct extension of established patterns (`io.Reader`/`io.Writer` injection, `[]scan.CategoryResult` pipeline, `fatih/color` output). Verified by reading all existing source.
- Interactive prompt logic: HIGH -- `bufio.NewReader` + `ReadString('\n')` + `strings.TrimSpace` + `strings.ToLower` is a well-known Go pattern already used in this codebase.
- bufio.Reader sharing pitfall: MEDIUM -- Identified through reasoning about buffered IO. The practical impact depends on terminal line buffering behavior. Recommended mitigation (single reader passed through) is safe regardless.
- Pitfalls: HIGH -- Based on codebase analysis and known Go IO patterns.

**Research date:** 2026-02-16
**Valid until:** 2026-03-16 (stable domain, stdlib APIs don't change)
