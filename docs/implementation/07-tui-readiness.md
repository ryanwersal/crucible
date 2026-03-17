# TUI Readiness

The project is designed CLI-first with a clean path to adding a TUI later. This document covers the Charm stack and architectural patterns for when that time comes.

## The Charm Stack

[Charmbracelet](https://github.com/charmbracelet) provides the dominant TUI ecosystem in Go:

- **Bubble Tea** — Framework based on The Elm Architecture (Model-Update-View). ~30k stars.
- **Lip Gloss** — Styling library for terminal output (colors, borders, padding, alignment). v2 is current.
- **Bubbles** — Pre-built components: text input, viewport (scrollable), list, progress bar, spinner, table, file picker, paginator.

## The Elm Architecture

Bubble Tea uses a unidirectional data flow:

```
User Input → Update(msg) → New Model → View() → Render to Terminal
                ↑                                       |
                └───────────────────────────────────────┘
```

```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "crucible/internal/runner"
)

type Model struct {
    runner  *runner.Runner  // shared business logic
    items   []runner.Item
    cursor  int
    err     error
}

func New(r *runner.Runner) Model {
    return Model{runner: r}
}

func (m Model) Init() tea.Cmd {
    // Return initial command (e.g., fetch data)
    return m.fetchItems
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.items)-1 {
                m.cursor++
            }
        case "enter":
            return m, m.processItem(m.items[m.cursor])
        }

    case itemsMsg:
        m.items = msg.items

    case errMsg:
        m.err = msg.err
    }

    return m, nil
}

func (m Model) View() string {
    if m.err != nil {
        return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
    }
    // Render items with cursor...
    return rendered
}
```

## Architecture: CLI and TUI Share Core Logic

```
cmd/crucible/main.go
    │
    ├── internal/cli/        ← CLI mode (cobra commands)
    │       │
    │       └── calls ──→ internal/runner/    ← Business logic
    │                    internal/config/
    │
    └── internal/tui/        ← TUI mode (bubble tea)
            │
            └── calls ──→ internal/runner/    ← Same business logic
                         internal/config/
```

The key rule: **`runner` knows nothing about CLI or TUI**. It operates on Go types, returns results and errors. The presentation layers (`cli` and `tui`) decide how to display them.

### Switching between modes

```go
// cmd/crucible/main.go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Parse top-level flags first
    if interactive {
        os.Exit(tui.Run(ctx))
    }
    os.Exit(cli.Run(ctx, os.Args[1:]))
}
```

## Pre-built Components (Bubbles)

When building the TUI, use existing Bubbles components rather than building from scratch:

| Component     | Use case                                    |
|---------------|---------------------------------------------|
| `list`        | Navigable item lists with filtering         |
| `table`       | Tabular data display                        |
| `textinput`   | Single-line text input                      |
| `textarea`    | Multi-line text input                       |
| `viewport`    | Scrollable content pane                     |
| `spinner`     | Loading indicators                          |
| `progress`    | Progress bars                               |
| `filepicker`  | File/directory selection                    |
| `paginator`   | Paginated content                           |

## Styling with Lip Gloss

```go
import "github.com/charmbracelet/lipgloss/v2"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(lipgloss.Color("#7D56F4")).
        Padding(0, 1)

    errorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FF0000")).
        Bold(true)
)

func (m Model) View() string {
    title := titleStyle.Render("Crucible")
    // ...
}
```

## Current Recommendations

Until the TUI is needed:

1. **Keep all business logic in `internal/runner/`** (or domain packages) with clean interfaces.
2. **Don't import any Charm packages yet** — avoid the dependency until needed.
3. **Design runner functions to return data, not print it** — this makes them usable from both CLI and TUI.
4. **Use `io.Writer` for output in the CLI layer** — this pattern works well for testing and later for TUI integration.
