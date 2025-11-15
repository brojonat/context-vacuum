# TUI User Guide

## Interactive Terminal UI

The TUI provides a complete interactive workflow for managing context sources.

### Launching the TUI

```bash
context-vacuum        # Default action
context-vacuum tui    # Explicit command
```

### Main View

```
🚀 context-vacuum - Interactive Source Manager

  [✓] API Documentation (file)
> [ ] Code Examples (url)
  [✓] Test Files (file)

a: add • ↑/k: up • ↓/j: down • space/enter: toggle • r: reload • q: quit
```

### Key Bindings

| Key | Action |
|-----|--------|
| `a` | Add new source (opens modal) |
| `↑` or `k` | Move cursor up |
| `↓` or `j` | Move cursor down |
| `space` or `enter` | Toggle source enabled/disabled |
| `r` | Reload sources from database |
| `q` or `ctrl+c` | Quit |

### Adding Sources (Press `a`)

When you press `a`, a modal form appears:

```
┌────────────────────────────────────────────────────┐
│                                                    │
│  Name:                                             │
│  API Documentation_                                │
│                                                    │
│  Path or URL:                                      │
│  /path/to/docs.md                                  │
│                                                    │
│  [Enter] Add  [Tab] Switch  [Esc] Cancel          │
│                                                    │
└────────────────────────────────────────────────────┘
```

**Modal Key Bindings:**
- `Tab` - Switch between Name and Path fields
- `Shift+Tab` - Switch backwards
- `Enter` - Submit and add source
- `Esc` - Cancel and return to main view
- Type normally to enter text

**Features:**
- Validates non-empty inputs
- Auto-detects file vs URL
- Fetches and caches content immediately
- Shows errors in red, success in green
- Newly added sources are enabled by default

### Examples

#### Add a Local File
1. Press `a`
2. Name: "My Config"
3. Tab to Path
4. Path: "/path/to/config.yaml"
5. Press Enter
6. ✓ Source added and cached!

#### Add a URL
1. Press `a`
2. Name: "Documentation"
3. Tab to Path
4. Path: "https://example.com/docs"
5. Press Enter
6. ✓ URL fetched and cached!

### Tips

- **Fast Navigation**: Use vim-style `k`/`j` keys
- **Quick Toggle**: Space bar is fastest for toggling
- **Error Recovery**: If add fails, the modal stays open with error message
- **Duplicate Names**: If name exists, content is updated instead
- **Relative Paths**: Automatically converted to absolute paths

### Workflow Example

```bash
# Launch TUI
context-vacuum

# Add sources interactively
Press 'a' → Enter name and path → Enter
Press 'a' → Enter another source → Enter

# Toggle sources for generation
↓ ↓ → Space (toggle on)
↑ → Space (toggle off)

# Quit and generate from CLI
Press 'q'
context-vacuum generate > context.md
```

### Complete Workflow in TUI

You can do everything from the TUI except:
- Removing sources (use CLI: `context-vacuum remove "name"`)
- Generating output (use CLI: `context-vacuum generate`)
- Importing bookmarks (use CLI: `context-vacuum import-bookmarks`)

For these operations, quit the TUI (`q`) and use CLI commands.
