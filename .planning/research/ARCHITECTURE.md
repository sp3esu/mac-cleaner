# Architecture Patterns: macOS Disk Cleaning CLI Tool

**Domain:** macOS disk cleaning CLI tools
**Researched:** 2026-02-16
**Confidence:** MEDIUM-HIGH

## Recommended Architecture

The standard architecture for macOS disk cleaning CLI tools follows a **layered, component-based design** with clear separation between scanning, analysis, user interaction, and cleanup operations.

```
┌───────────────────────────────────────────────────────────────┐
│                      CLI Entry Layer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │ Flag Parser  │  │ Interactive  │  │  JSON Output │        │
│  │  (--all)     │  │ Mode Handler │  │   Handler    │        │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘        │
└─────────┼──────────────────┼──────────────────┼───────────────┘
          │                  │                  │
          └──────────────────┴──────────────────┘
                             │
┌───────────────────────────────────────────────────────────────┐
│                     Orchestration Layer                        │
│  ┌────────────────────────────────────────────────────────┐   │
│  │               Execution Controller                      │   │
│  │  - Mode detection (interactive vs flag-based)          │   │
│  │  - Workflow coordination (scan → report → clean)       │   │
│  │  - Dry-run / Skip flag handling                        │   │
│  └──────┬──────────────────────────┬──────────────────────┘   │
└─────────┼──────────────────────────┼───────────────────────────┘
          │                          │
          ▼                          ▼
┌─────────────────────┐    ┌─────────────────────┐
│   Scanner Layer     │    │   Reporter Layer    │
│                     │    │                     │
│  ┌──────────────┐   │    │  ┌──────────────┐  │
│  │ Path Scanner │   │    │  │ Size Calc    │  │
│  │ - Parallel   │   │    │  │ - Formatter  │  │
│  │ - Async I/O  │   │    │  │ - Summary    │  │
│  └──────┬───────┘   │    │  └──────────────┘  │
│         │           │    │                     │
│  ┌──────▼───────┐   │    └─────────────────────┘
│  │ Rule Matcher │   │
│  │ - Categories │   │
│  │ - Filters    │   │
│  └──────┬───────┘   │
│         │           │
│  ┌──────▼───────┐   │
│  │ Safety Check │   │
│  │ - SIP paths  │   │
│  │ - Risk level │   │
│  └──────────────┘   │
└─────────┬───────────┘
          │
          │ (scan results)
          │
┌─────────▼─────────────────┐    ┌────────────────────────┐
│   Confirmation Layer      │    │    Cleaner Layer       │
│                           │    │                        │
│  ┌────────────────────┐   │    │  ┌─────────────────┐  │
│  │ Interactive Prompt │───┼───▶│  │ Trash Handler   │  │
│  │ - Category select  │   │    │  │ - System API    │  │
│  │ - Item select      │   │    │  └─────────────────┘  │
│  └────────────────────┘   │    │                        │
│                           │    │  ┌─────────────────┐  │
│  ┌────────────────────┐   │    │  │ Direct Delete   │  │
│  │ Auto-confirm       │───┼───▶│  │ - fs.unlink     │  │
│  │ - Flag-based       │   │    │  └─────────────────┘  │
│  └────────────────────┘   │    │                        │
└───────────────────────────┘    │  ┌─────────────────┐  │
                                 │  │ External Tools  │  │
                                 │  │ - brew cleanup  │  │
                                 │  │ - docker prune  │  │
                                 │  └─────────────────┘  │
                                 └────────────────────────┘
                                           │
                                           ▼
                                  ┌─────────────────┐
                                  │  Summary Layer  │
                                  │  - Stats        │
                                  │  - Errors       │
                                  └─────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|----------------|-------------------|
| **CLI Entry Layer** | Parse flags, detect mode, route to appropriate handler | Orchestration Layer |
| **Orchestration Controller** | Coordinate workflow, handle dry-run, manage skip flags | All layers below |
| **Scanner Layer** | Discover cleanable files, apply rules, perform safety checks | Orchestration, Reporter |
| **Rule Matcher** | Match files against cleaning categories, apply filters | Scanner, Config |
| **Safety Checker** | Validate paths against SIP, check risk levels | Scanner |
| **Reporter Layer** | Calculate sizes, format output, generate summaries | Orchestration, Output handlers |
| **Confirmation Layer** | Handle user prompts (interactive) or auto-confirm (flags) | Orchestration, Cleaner |
| **Cleaner Layer** | Execute actual deletions via Trash, direct delete, or external tools | Confirmation, Summary |
| **Summary Layer** | Aggregate results, report errors, show statistics | CLI Entry (output) |

## Recommended Project Structure

```
src/
├── cli/                    # CLI entry and mode handling
│   ├── index.ts           # Main entry point
│   ├── parser.ts          # Flag/argument parsing
│   └── modes/             # Mode handlers
│       ├── interactive.ts # Interactive mode flow
│       └── flags.ts       # Flag-based mode flow
├── core/                  # Core business logic
│   ├── orchestrator.ts    # Workflow coordination
│   ├── scanner.ts         # File scanning engine
│   ├── cleaner.ts         # Deletion operations
│   └── reporter.ts        # Size calculation and formatting
├── rules/                 # Cleaning rules and categories
│   ├── index.ts           # Rule registry
│   ├── system-caches.ts   # System cache rules
│   ├── app-leftovers.ts   # App leftover rules
│   ├── dev-caches.ts      # Developer cache rules
│   └── browser-data.ts    # Browser data rules
├── safety/                # Safety and validation
│   ├── sip-checker.ts     # SIP path validation
│   ├── risk-assessor.ts   # Risk level assessment
│   └── path-validator.ts  # Path existence/permission checks
├── ui/                    # User interaction
│   ├── prompts.ts         # Interactive prompts (inquirer)
│   ├── formatters.ts      # Output formatting
│   └── progress.ts        # Progress indicators
├── adapters/              # External integrations
│   ├── trash.ts           # macOS Trash API
│   ├── brew.ts            # Homebrew cleanup
│   └── docker.ts          # Docker cleanup
├── types/                 # TypeScript types
│   ├── scan-result.ts     # Scan result types
│   ├── clean-target.ts    # Cleaning target types
│   └── config.ts          # Configuration types
└── utils/                 # Utilities
    ├── fs-async.ts        # Async filesystem helpers
    ├── size-calculator.ts # Size calculation utilities
    └── logger.ts          # Logging
```

### Structure Rationale

- **cli/**: Entry point isolation allows easy testing of business logic without CLI concerns
- **core/**: Business logic separated from I/O concerns follows Clean Architecture principles
- **rules/**: Organizing rules by category makes them easy to find, modify, and test independently
- **safety/**: Critical safety logic isolated for thorough testing and auditing
- **ui/**: All user interaction in one place, swappable for different UIs (TUI, GUI)
- **adapters/**: External dependencies isolated for easy mocking and testing

## Data Flow

### Scan → Report → Confirm → Clean → Summary Flow

```
User Command
    │
    ▼
1. PARSE & ROUTE
   ├─ Flag-based? → Extract flags (--all, --system-caches, --skip)
   └─ Interactive? → Prepare prompt system
    │
    ▼
2. SCAN PHASE
   ├─ Load applicable rules based on flags/selection
   ├─ Scan paths in parallel (ThreadPool/Promise.all)
   ├─ Apply filters (--skip flags, SIP checks)
   ├─ Calculate sizes
   └─ Build ScanResult[]
    │
    ▼
3. REPORT PHASE
   ├─ Group by category
   ├─ Calculate totals
   ├─ Format for display/JSON
   └─ Output results
    │
    ▼
4. CONFIRMATION PHASE
   ├─ Interactive: Show prompt, get user selection
   ├─ Flag-based: Auto-confirm based on flags
   └─ --dry-run: Skip to summary
    │
    ▼
5. CLEAN PHASE
   ├─ For each confirmed item:
   │   ├─ Check permissions
   │   ├─ Move to Trash OR direct delete OR external tool
   │   └─ Record result (success/error)
   └─ Parallel execution with rate limiting
    │
    ▼
6. SUMMARY PHASE
   ├─ Aggregate results
   ├─ Calculate space freed
   ├─ Report errors
   └─ Exit with code (0 = success, 1 = errors)
```

### Interactive vs Flag-Based Mode Flow

```
Interactive Mode:
  Parse → Scan all → Report → Prompt by category → Prompt by item → Clean → Summary

Flag-Based Mode:
  Parse → Apply flag filters → Scan filtered → Report → Auto-confirm → Clean → Summary

Dry-Run Mode (both):
  Parse → Scan → Report → Summary (skip clean)
```

### Key Data Structures

```typescript
// Flows through the system
interface ScanResult {
  path: string;
  category: Category;
  size: number;
  riskLevel: 'safe' | 'moderate' | 'risky' | 'manual';
  reason: string;  // Why this is cleanable
}

interface CleanResult {
  path: string;
  success: boolean;
  bytesFreed?: number;
  error?: Error;
  method: 'trash' | 'delete' | 'external';
}

interface Summary {
  scanned: number;
  selected: number;
  cleaned: number;
  bytesFreed: number;
  errors: CleanResult[];
  duration: number;
}
```

## Architectural Patterns

### Pattern 1: Rule-Based Scanning with Config Files

**What:** Define cleaning targets in configuration files (JSON/TypeScript) rather than hardcoding paths in scanner logic.

**When to use:** When you need flexibility to add new cleaning targets without code changes, or when different users might want different rules.

**Trade-offs:**
- **Pros:** Easy to extend, user-customizable, testable without code changes
- **Cons:** Slightly more complex initial setup, config validation needed

**Example:**
```typescript
// rules/system-caches.ts
export const systemCacheRules: CleaningRule[] = [
  {
    category: 'system-caches',
    paths: [
      '~/Library/Caches/*',
      '~/Library/Logs/*',
    ],
    exclude: [
      '~/Library/Caches/com.apple.*',  // System-critical
    ],
    riskLevel: 'safe',
    reason: 'System caches regenerate automatically',
  },
];

// core/scanner.ts
async function scan(rules: CleaningRule[]): Promise<ScanResult[]> {
  const results: ScanResult[] = [];
  for (const rule of rules) {
    for (const pathPattern of rule.paths) {
      const matches = await glob(pathPattern);
      const filtered = matches.filter(p => !isExcluded(p, rule.exclude));
      const checked = await Promise.all(
        filtered.map(p => safetyCheck(p, rule.riskLevel))
      );
      results.push(...checked.filter(r => r.safe));
    }
  }
  return results;
}
```

**Recommendation:** Use config-based rules for mac-cleaner. Hardcoding is simpler initially but becomes maintenance burden. Plugin system is overkill for this scope.

### Pattern 2: Safety-First Architecture with Multiple Validation Layers

**What:** Apply multiple layers of safety checks before any deletion: SIP validation → Risk assessment → Permission check → User confirmation.

**When to use:** Always, for any disk cleaning tool. User trust depends on safety.

**Trade-offs:**
- **Pros:** Prevents catastrophic mistakes, builds user trust
- **Cons:** Adds some overhead, requires careful ordering

**Example:**
```typescript
// safety/pipeline.ts
async function validateForCleaning(path: string, riskLevel: RiskLevel): Promise<ValidationResult> {
  // Layer 1: SIP protection check
  if (isSIPProtected(path)) {
    return { safe: false, reason: 'SIP-protected path' };
  }

  // Layer 2: Risk assessment
  if (riskLevel === 'manual') {
    return { safe: false, reason: 'Manual intervention required' };
  }

  // Layer 3: Permission check
  if (!canAccess(path)) {
    return { safe: false, reason: 'Insufficient permissions' };
  }

  // Layer 4: Path existence
  if (!await exists(path)) {
    return { safe: false, reason: 'Path does not exist' };
  }

  return { safe: true };
}

// Hardcoded SIP paths from official macOS documentation
const SIP_PROTECTED_PATHS = [
  '/System',
  '/usr',      // except /usr/local
  '/bin',
  '/sbin',
  '/var',
];

function isSIPProtected(path: string): boolean {
  return SIP_PROTECTED_PATHS.some(p =>
    path.startsWith(p) && !path.startsWith('/usr/local')
  );
}
```

### Pattern 3: Parallel Scanning with Controlled Concurrency

**What:** Scan multiple paths concurrently using Promise.all or worker threads, but limit concurrency to avoid overwhelming the filesystem.

**When to use:** For CLI tools scanning many directories. Essential for good UX.

**Trade-offs:**
- **Pros:** Much faster than sequential scanning (3-10x speedup observed in existing tools)
- **Cons:** More complex error handling, need to manage resource limits

**Example:**
```typescript
// core/scanner.ts with controlled concurrency
import pLimit from 'p-limit';

async function scanPaths(paths: string[], concurrency = 8): Promise<ScanResult[]> {
  const limit = pLimit(concurrency);
  const results = await Promise.all(
    paths.map(path => limit(() => scanSinglePath(path)))
  );
  return results.flat();
}

async function scanSinglePath(path: string): Promise<ScanResult[]> {
  try {
    const stats = await fs.stat(path);
    if (stats.isDirectory()) {
      const entries = await fs.readdir(path);
      return scanPaths(entries.map(e => join(path, e)));
    }
    return [{
      path,
      size: stats.size,
      // ... other fields
    }];
  } catch (error) {
    // Log and continue - don't fail entire scan for one path
    logger.warn(`Failed to scan ${path}:`, error);
    return [];
  }
}
```

### Pattern 4: Mode Detection and Adapter Pattern for UI

**What:** Detect execution mode (interactive vs non-interactive) early and use different adapters for user interaction.

**When to use:** For CLI tools supporting both interactive prompts and scriptable flag-based execution.

**Trade-offs:**
- **Pros:** Clean separation, easy to test each mode independently
- **Cons:** Slight duplication of logic between modes

**Example:**
```typescript
// cli/modes/detector.ts
function detectMode(args: ParsedArgs): 'interactive' | 'flags' {
  // If any cleaning flags specified, use flag-based mode
  if (args.all || args.systemCaches || args.devCaches || args.browserData) {
    return 'flags';
  }
  // If --json or CI environment, force non-interactive
  if (args.json || process.env.CI) {
    return 'flags';
  }
  // Default to interactive if TTY available
  if (process.stdout.isTTY) {
    return 'interactive';
  }
  return 'flags';
}

// cli/modes/interactive.ts
async function runInteractive(orchestrator: Orchestrator) {
  const scanResults = await orchestrator.scan();

  // Group by category
  const byCategory = groupBy(scanResults, r => r.category);

  // Category selection
  const selectedCategories = await prompts.multiselect({
    message: 'Select categories to clean',
    choices: Object.keys(byCategory).map(cat => ({
      title: cat,
      value: cat,
      description: `${byCategory[cat].length} items, ${formatSize(totalSize(byCategory[cat]))}`,
    })),
  });

  // Item selection within categories
  for (const category of selectedCategories) {
    const items = byCategory[category];
    const selected = await prompts.multiselect({
      message: `Select items in ${category}`,
      choices: items.map(item => ({
        title: basename(item.path),
        value: item,
        description: `${formatSize(item.size)} - ${item.reason}`,
      })),
    });
    await orchestrator.clean(selected);
  }

  orchestrator.showSummary();
}

// cli/modes/flags.ts
async function runFlagBased(orchestrator: Orchestrator, args: ParsedArgs) {
  // Determine which categories to scan based on flags
  const categories = [];
  if (args.all || args.systemCaches) categories.push('system-caches');
  if (args.all || args.devCaches) categories.push('dev-caches');
  if (args.all || args.browserData) categories.push('browser-data');
  if (args.all || args.appLeftovers) categories.push('app-leftovers');

  const scanResults = await orchestrator.scanCategories(categories);

  // Apply --skip filters
  const filtered = applySkipFilters(scanResults, args.skip);

  orchestrator.report(filtered);

  if (!args.dryRun) {
    await orchestrator.clean(filtered);
  }

  orchestrator.showSummary();
}
```

### Pattern 5: Graceful Degradation for External Tools

**What:** For cleanup tasks requiring external tools (brew, docker), detect availability and gracefully skip if not installed.

**When to use:** When integrating with optional tools that might not be present on all systems.

**Trade-offs:**
- **Pros:** Broader compatibility, better UX
- **Cons:** Need to maintain detection logic

**Example:**
```typescript
// adapters/brew.ts
export class BrewAdapter {
  private available?: boolean;

  async isAvailable(): Promise<boolean> {
    if (this.available !== undefined) return this.available;
    try {
      await exec('which brew');
      this.available = true;
    } catch {
      this.available = false;
    }
    return this.available;
  }

  async cleanup(): Promise<CleanResult[]> {
    if (!await this.isAvailable()) {
      return [{
        path: 'brew',
        success: false,
        error: new Error('Homebrew not installed'),
        method: 'external',
      }];
    }

    const output = await exec('brew cleanup --prune=all -s --dry-run');
    // Parse output to estimate space savings
    const size = parseBrewOutput(output);

    if (!this.dryRun) {
      await exec('brew cleanup --prune=all -s');
    }

    return [{
      path: 'homebrew-cache',
      success: true,
      bytesFreed: size,
      method: 'external',
    }];
  }
}
```

## Build Order and Component Dependencies

### Recommended Build Order

1. **Phase 1: Types & Core Models** (No dependencies)
   - Define TypeScript types (ScanResult, CleanResult, Config)
   - Minimal, can work alongside other phases

2. **Phase 2: Safety Layer** (Depends on: Types)
   - SIP checker (hardcoded paths, pure logic)
   - Risk assessor
   - Path validator
   - Can be built and tested in isolation

3. **Phase 3: Rules System** (Depends on: Types, Safety)
   - Rule definitions for each category
   - Rule registry
   - Needs safety layer to validate paths

4. **Phase 4: Scanner** (Depends on: Types, Safety, Rules)
   - File system scanning
   - Rule matching
   - Size calculation
   - Core functionality, used by all modes

5. **Phase 5: Reporter** (Depends on: Types, Scanner results)
   - Formatting utilities
   - Summary generation
   - Can be built in parallel with Scanner if interfaces defined

6. **Phase 6: Cleaner** (Depends on: Types, Safety)
   - Trash integration
   - Direct delete
   - External tool adapters (brew, docker)
   - Needs safety layer for validation

7. **Phase 7: UI Components** (Depends on: Types, Reporter)
   - Prompts (interactive mode)
   - Progress indicators
   - Output formatters
   - Can be built in parallel with Cleaner

8. **Phase 8: Orchestrator** (Depends on: Scanner, Cleaner, Reporter, UI)
   - Workflow coordination
   - Mode handling
   - Dry-run logic
   - Integrates all other components

9. **Phase 9: CLI Entry** (Depends on: Orchestrator)
   - Argument parsing
   - Mode detection
   - Entry point
   - Final integration layer

### Dependency Graph

```
                         Types (1)
                           │
          ┌────────────────┼────────────────┐
          │                │                │
       Safety (2)       Rules (3)       Reporter (5)
          │                │                │
          └────────┬───────┴────────────────┘
                   │
              Scanner (4)
                   │
          ┌────────┴─────────┐
          │                  │
      Cleaner (6)        UI (7)
          │                  │
          └─────────┬────────┘
                    │
              Orchestrator (8)
                    │
              CLI Entry (9)
```

### Critical Path

**Scanner → Orchestrator → CLI Entry** is the critical path. These must work together for basic functionality. Other components can be stubbed or simplified initially:
- Safety checks can start with basic SIP validation
- Cleaner can use simple delete initially, add Trash later
- UI can be basic console.log initially, add prompts later

## Anti-Patterns to Avoid

### Anti-Pattern 1: Scanning Before Parsing User Intent

**What people do:** Scan all possible locations immediately on startup, then filter based on user selections.

**Why it's wrong:**
- Wastes time scanning paths the user didn't ask for
- In flag-based mode (--system-caches only), you scan everything unnecessarily
- Poor UX - user waits for full scan when they only wanted one category

**Do this instead:**
Parse flags/get user input first, then scan only requested categories. Load rules on-demand.

```typescript
// WRONG
const allResults = await scanner.scanEverything();
const filtered = allResults.filter(r => selectedCategories.includes(r.category));

// RIGHT
const rulesToApply = rules.filter(r => selectedCategories.includes(r.category));
const results = await scanner.scan(rulesToApply);
```

### Anti-Pattern 2: Direct Deletion Without Trash

**What people do:** Use `fs.unlink` or `rm -rf` for all deletions to avoid Trash API complexity.

**Why it's wrong:**
- No recovery path for users if something goes wrong
- Violates user expectations (macOS users expect deletions to go to Trash)
- High risk if safety checks fail

**Do this instead:**
Use macOS Trash API (via `trash` npm package) as default, direct delete only for specific cases with explicit warning.

```typescript
// Use trash for user-facing deletions
import trash from 'trash';
await trash(['/path/to/file']);

// Direct delete only for:
// - Explicitly requested (--force flag)
// - Empty directories
// - Known temp files that don't need recovery
```

### Anti-Pattern 3: Synchronous File Operations

**What people do:** Use sync fs operations (`fs.statSync`, `fs.readdirSync`) for simplicity.

**Why it's wrong:**
- Blocks event loop during scanning
- Prevents parallel execution
- Poor performance on large directory trees

**Do this instead:**
Use async operations with controlled concurrency.

### Anti-Pattern 4: Hardcoding All Paths in Scanner

**What people do:** Put all cache paths directly in scanner code:
```typescript
const pathsToScan = [
  '~/Library/Caches/Chrome',
  '~/Library/Caches/Firefox',
  // ... 100 more lines
];
```

**Why it's wrong:**
- Hard to maintain as apps change cache locations
- Can't add new targets without code changes
- Difficult to test individual categories
- No way for users to customize

**Do this instead:**
Separate rule definitions from scanning logic (Pattern 1).

### Anti-Pattern 5: Single-Pass Scanning and Cleaning

**What people do:** Scan and delete in one pass to "optimize performance":
```typescript
for (const path of paths) {
  if (shouldClean(path)) {
    await delete(path);  // Deleting while scanning
  }
}
```

**Why it's wrong:**
- Can't show "what would be cleaned" before cleaning
- --dry-run becomes complicated
- No opportunity for user review in interactive mode
- Partial failures leave unclear state

**Do this instead:**
Always separate scan → report → confirm → clean phases, even in flag-based mode.

## Integration Points

### System APIs

| API | Purpose | Implementation | Notes |
|-----|---------|----------------|-------|
| **macOS Trash** | Move files to Trash instead of permanent delete | Use `trash` npm package (wraps NSFileManager) | Requires Full Disk Access for some paths |
| **File System** | Scan, stat, delete operations | Node.js `fs/promises` | Use async operations, handle ENOENT gracefully |
| **Process Execution** | Run external commands (brew, docker) | Node.js `child_process.exec` with promisify | Parse stdout, handle non-zero exit codes |

### External Tools (Optional)

| Tool | Integration Method | Availability Check | Notes |
|------|-------------------|-------------------|-------|
| **Homebrew** | Execute `brew cleanup --prune=all -s` | `which brew` | Parse output for space savings estimate |
| **Docker** | Execute `docker system prune` | `which docker` | Multiple prune options (images, containers, volumes) |
| **npm/yarn/pnpm** | Execute `[tool] cache clean` | Check for lock files in scanned dirs | Project-specific, not global |

### Module Boundaries

```
CLI Layer ←→ Orchestrator    # Commands to actions
    ↕
Orchestrator ←→ Scanner      # Initiate scans
    ↕
Orchestrator ←→ Cleaner      # Execute cleaning
    ↕
Orchestrator ←→ Reporter     # Get summaries
    ↕
Scanner ←→ Rules             # Apply rules
    ↕
Scanner ←→ Safety            # Validate paths
    ↕
Cleaner ←→ Safety            # Pre-delete checks
    ↕
Cleaner ←→ Adapters          # External tool integration
```

**Key principle:** Orchestrator is the only component that coordinates between Scanner, Cleaner, and Reporter. They don't call each other directly.

## Sources

### macOS Cleaning Tools (HIGH confidence - direct repository analysis)
- [mac-cleanup-go](https://github.com/2ykwang/mac-cleanup-go) - TUI cleaner with parallel scanning, 107 targets, risk levels
- [MacCleanCLI](https://github.com/QDenka/MacCleanCLI) - Python tool with SOLID architecture, multi-threaded scanning
- [mac-cleanup-py](https://github.com/mac-cleanup/mac-cleanup-py) - Modular plugin-based architecture, 40+ modules

### CLI Framework Architecture (MEDIUM confidence - comparison sources)
- [Node.js CLI Frameworks Comparison](https://www.oreateai.com/blog/indepth-comparison-of-cli-frameworks-technical-features-and-application-scenarios-of-yargs-commander-and-oclif/24440ae03bfbae6c4916c403a728f6da)
- [Building CLI Applications with Node.js](https://ibrahim-haouari.medium.com/building-cli-applications-made-easy-with-these-nodejs-frameworks-2c06d1ff7a51)

### macOS System Architecture (HIGH confidence - official documentation and community)
- [macOS SIP Protection Paths](https://book.hacktricks.wiki/en/macos-hardening/macos-security-and-privilege-escalation/macos-security-protections/macos-sip)
- [System Integrity Protection Overview](https://www.cleverfiles.com/help/system-integrity-protection.html)
- [Safe macOS Cache Locations](https://macpaw.com/how-to/clear-cache-on-mac)
- [Library/Caches on Mac](https://iboysoft.com/wiki/library-caches-mac.html)

### CLI Design Patterns (MEDIUM confidence - design guidance)
- [Command-line design guidance - Microsoft](https://learn.microsoft.com/en-us/dotnet/standard/commandline/design-guidance)
- [Command Line Interface Guidelines](https://clig.dev/)
- [Understanding Terminal Architecture](https://medium.com/connected-things/understanding-terminal-architecture-from-ttys-to-modern-cli-tools-f42fe08652a3)

### Tool-Specific Implementation (MEDIUM confidence - official docs)
- [Homebrew cleanup documentation](https://docs.brew.sh/Manpage)
- [Homebrew Cleanup class](https://docs.brew.sh/rubydoc/Homebrew/Cleanup.html)
- [ncdu disk usage analyzer](https://dev.yorhel.nl/ncdu)

---

*Architecture research for: macOS disk cleaning CLI tool*
*Researched: 2026-02-16*
*Confidence: MEDIUM-HIGH (verified through multiple real-world implementations and official macOS documentation)*
