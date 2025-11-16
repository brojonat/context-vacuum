# context-vacuum

> 🚀 Blazingly fast CLI tool to dynamically generate LLM context files
> (Claude.md, etc.) by curating and combining files from your project

## Problem & Solution

**The Problem:** LLM code assistants (Cursor, ChatGPT, Claude) often revert to
less ideal implementations despite having newer, better alternatives (e.g.,
ResponsesAPI vs. Completions API for OpenAI, Composition API vs. Options API for
Vue, etc.). This will continue to happen as long as API churn exists and LLMs
have training cutoff dates.

**The Solution:** `context-vacuum` lets you create curated context files that
override an LLM's default instincts by combining relevant code snippets,
documentation, and examples. Instead of messing with a jumble of CLAUDE.md file
or hoping Cursor's agent picks the right `md` file, you can explicity create a
fresh markdown file with content you want and pass it to the agent. Build custom
prompts in a fraction of a second instead of minutes.

## Features

- ⚡ **Lightning Fast** - Golang binary, optimized for speed and portability
- 🎯 **Intelligent Curation** - Toggle files on/off to dynamically generate
  context
- 📄 **Multi-source Support**:
  - Local files and directories
  - Web pages (automatic HTML conversion)
  - Collections of pages from browser bookmark lists
- 🎨 **Dual Interface**:
  - **TUI Mode** (default) - Interactive terminal UI for quick toggling
  - **CLI Mode** - Direct command-line invocation for scripting/automation
- 📦 **Single Binary** - No dependencies, just works
- 🔧 **Flexible Output** - Stdout for piping or files in multiple formats

## Installation

### Using Go (Recommended)

```bash
# Install latest version
go install github.com/brojonat/context-vacuum@latest

# Verify installation
context-vacuum --help
```

**Note:** Ensure `$GOPATH/bin` (usually `~/go/bin`) is in your PATH.

### Build from Source

```bash
git clone https://github.com/brojonat/context-vacuum.git
cd context-vacuum
make build
# Binary will be in bin/context-vacuum
```

## Quick Start

### TUI Mode (Interactive)

```bash
# Launch the interactive terminal UI
context-vacuum

# Key bindings:
# - 'a' to add sources
# - 'd' to delete sources (with confirmation)
# - arrow keys or j/k to navigate
# - space/enter to toggle enabled/disabled
# - 'r' to reload
# - 'q' to quit
```

### CLI Mode (Direct)

```bash
# Add a local file to the cache
context-vacuum add --name "API Handler" path/to/file.ts

# Add a web page (automatically cached)
context-vacuum add --name "Docs" https://example.com/docs

# Remove a source from the cache
context-vacuum remove "API Handler"

# Toggle files for inclusion
context-vacuum toggle-on "API Handler"
context-vacuum toggle-off "Docs"

# Generate to stdout (default - perfect for piping)
context-vacuum generate

# Generate to file in current directory
context-vacuum generate --output claude.md

# Or specify absolute path
context-vacuum generate --output /path/to/output/claude.md
```

## Usage Examples

### Example 1: API Documentation Context

```bash
# Build context around your API implementation
context-vacuum add --name "ResponsesAPI impl" src/api/responses.ts
context-vacuum add --name "API Patterns" docs/api-patterns.md
context-vacuum add --name "API Tests" tests/api.test.ts

# Remove a source if no longer needed
context-vacuum remove "API Tests"

# Preview context to stdout
context-vacuum generate | head -50

# Generate to file
context-vacuum generate --output claude.md

# Or pipe directly to clipboard (macOS)
context-vacuum generate | pbcopy
```

Then paste the generated context into Cursor, Claude, or any other agent's
context to ensure consistent API implementation.

### Example 2: Framework Migration Reference

```bash
# Create context for Vue Options→Composition migration
context-vacuum add --name "Composition Examples" examples/composition-api.vue
context-vacuum add --name "Migration Guide" migration-guide.md
context-vacuum add --name "Existing Composition" src/components/

# Generate in different formats
context-vacuum generate --format claude --output prefer-composition.md
context-vacuum generate --format cursor --output prefer-composition-cursor.md

# Preview before saving
context-vacuum generate --format default | less

# Pipe directly to LLM tools
context-vacuum generate | llm "summarize this context"

# Remove a source when done
context-vacuum remove "Migration Guide"
```

### Example 3: Import Bookmarks

```bash
# Add all bookmarks from exported Chrome/Firefox bookmark file
context-vacuum import-bookmarks ~/.config/bookmarks.html

# Now toggle them in the TUI to include relevant documentation
context-vacuum
```

### Example 4: Pipe to LLM Tools

```bash
# Pipe context directly to Claude CLI
context-vacuum generate | claude "refactor this following the patterns shown"

# Use with llm tool
context-vacuum generate --format default | llm -m gpt-4 "summarize key patterns"

# Save to file and view
context-vacuum generate --output context.md && cat context.md
```

## Configuration

All data is stored in `.context-vacuum/` directory:

```
$HOME/.context-vacuum/
├── config.yaml                # Global settings
├── cache.db                   # SQLite database with all cached sources
└── presets/                   # Saved configurations
    ├── api-context.yaml
    └── frontend-context.yaml
```

The SQLite database stores:

- **sources**: Cached files and URLs with metadata (name, path, enabled status,
  hash, last updated)
- **presets**: Named collections of enabled sources for quick context generation
- **history**: Track of generated contexts

### CLI Configuration Options and Defaults

```bash
--output=""                     # Output file path (default: stdout)
--format=claude                 # Output format (claude, cursor, default)
--cache-db=$HOME/.context-vacuum/cache.db
--max-file-size=10MB
--exclude-patterns=*.test.ts,*.spec.ts,node_modules/*
```

**Output Behavior:**

- No `--output` flag → writes to **stdout** (perfect for piping)
- `--output -` → explicit stdout
- `--output claude.md` → writes to file in **current directory**
- `--output .cursor/context.md` → creates subdirectory in current directory
- `--output /tmp/context.md` → writes to absolute path

## Performance

Designed for speed:

- **Smart Caching**: Content stored in SQLite with hash-based change detection
- **Incremental Refresh**: Only re-parses/re-fetches sources that changed
- **Single Pass**: Efficient file parsing with size limits
- **Fallback Strategy**: Uses cached content if refresh fails (graceful
  degradation)
- **Typical Generation**: < 100ms for 10-20 sources with cache hits
- **First Run**: Slightly slower as cache is populated, but amortized over
  subsequent runs
- **sllm Integration**: Can be invoked preflight to append context before
  queries are sent to the model

## Architecture

```
CLI Interface (TUI + Command)
    ↓
Generator (queries DB for enabled sources, detects cache misses)
    ↓
Parser (re-parses/re-fetches stale content only)
    ↓
SQLite Cache Manager (updates cache with fresh content)
    ↓
Output (stdout or files in multiple formats)
```

### How It Works

1. **Add**: User adds a file or URL → content parsed and cached in SQLite DB
   with hash
2. **Remove**: User deletes a source by name → removed from DB and cache
3. **Toggle**: User enables/disables sources in TUI or CLI → updates DB
4. **Generate**:
   - Query enabled sources from DB
   - Check each source for cache misses (file modified, URL changed)
   - Auto-refresh stale content and update cache
   - Combine fresh/cached content into output (stdout or file)

### Cache Refresh Strategy

- **Files**: Hash-based detection - compares current file hash with cached hash
- **URLs**: Always re-fetches to check for changes (hash comparison)
- **Smart Updates**: Only updates cache when content actually changed
- **Fallback**: If refresh fails, uses cached content with warning log

## Commands Reference

| Command                   | Description                                           | Example                                                                 |
| ------------------------- | ----------------------------------------------------- | ----------------------------------------------------------------------- |
| `add <source>`            | Add file/URL to cache DB (requires `--name`)          | `context-vacuum add --name "Docs" file.md`                              |
| `remove <name>`           | Remove source from cache by name                      | `context-vacuum remove "Docs"`                                          |
| `toggle-on <name>`        | Enable source for context generation                  | `context-vacuum toggle-on "Docs"`                                       |
| `toggle-off <name>`       | Disable source from context generation                | `context-vacuum toggle-off "Docs"`                                      |
| `list`                    | List all cached sources with status                   | `context-vacuum list`                                                   |
| `generate`                | Create context from enabled sources (default: stdout) | `context-vacuum generate` or `context-vacuum generate --output file.md` |
| `import-bookmarks <file>` | Import bookmarks into cache DB                        | `context-vacuum import-bookmarks bookmarks.html`                        |
| `tui`                     | Launch interactive terminal UI                        | `context-vacuum tui` or just `context-vacuum`                           |

## Development

```bash
# Set up development environment
go mod download

# Run tests
go test ./...

# Build for multiple platforms
make build-all
```

## Roadmap

- [ ] TUI enhancements (search, filtering, previews)
- [ ] Web UI alternative
- [ ] IDE plugin support (VSCode, JetBrains)
- [ ] Diff highlighting in TUI
- [ ] Analytics on which contexts work best
- [ ] Community context templates
- [x] Stdout output for piping
- [x] Smart cache refresh

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT - see LICENSE file for details

---

**Made with ❤️ for developers who want better LLM context**
