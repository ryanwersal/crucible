# Project Structure

## Layout

```
crucible/
  cmd/
    crucible/
      main.go              # Minimal: wire deps, load config, call run()
  internal/
    cli/                   # Command definitions, flag parsing
      root.go
      <subcommand>.go
    config/                # Configuration loading/validation
    runner/                # Core business logic
    output/                # Formatting (table, JSON, plain text)
    tui/                   # Future TUI layer (Bubble Tea)
  docs/
    implementation/        # These docs
  go.mod
  go.sum
  .goreleaser.yaml
```

## Key Principles

### `cmd/crucible/main.go` stays minimal

```go
package main

import (
    "context"
    "os"
    "os/signal"

    "crucible/internal/cli"
)

var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    os.Exit(cli.Run(ctx, os.Args[1:], cli.BuildInfo{
        Version: version,
        Commit:  commit,
        Date:    date,
    }))
}
```

All logic lives in packages that `main` imports. The `main` function is the composition root — it wires dependencies and starts execution.

### Use `internal/` for everything

The `internal/` directory is enforced by the Go compiler — nothing outside this module can import these packages. Since this is a CLI tool (not a library), all code belongs in `internal/`. There is no `pkg/` directory.

### Separate CLI from business logic

The `internal/cli/` package handles command definitions, flag parsing, and user-facing I/O. The `internal/runner/` package (or domain-specific packages) contains business logic with no knowledge of CLI frameworks.

This separation enables:
- Unit testing business logic without CLI wiring
- Adding a TUI later that calls the same core logic
- Swapping CLI frameworks without touching business code

### Factory functions for commands, not global state

```go
// Good: factory function
func NewRootCmd(cfg *config.Config) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "crucible",
        Short: "A fast and friendly CLI tool",
        // ...
    }
    cmd.AddCommand(NewSubCmd(cfg))
    return cmd
}

// Bad: global init
var rootCmd = &cobra.Command{...}
func init() {
    rootCmd.AddCommand(subCmd)
}
```

Factory functions make commands testable and avoid shared mutable state.

## CLI Framework Choice: Cobra

[Cobra](https://github.com/spf13/cobra) is the standard for Go CLI tools (used by kubectl, helm, Hugo, Docker). It provides:

- Nested subcommands with automatic help generation
- Shell completion (bash, zsh, fish, PowerShell) out of the box
- POSIX-compliant flag parsing via `pflag`
- Integration with Viper/Koanf for configuration

### Alternative: Kong

[Kong](https://github.com/alecthomas/kong) is a lighter struct-tag-based alternative worth considering. It defines CLI structure as Go structs with tags, which is more type-safe and produces more testable code. However, Cobra's ecosystem and tooling support is significantly larger.

## TUI Readiness

The project structure above keeps a clean boundary so a TUI can be added later:

```
internal/
  cli/       # Command-line interface (cobra commands)
  tui/       # Terminal UI (Bubble Tea) — added when needed
  runner/    # Shared business logic — both CLI and TUI call this
```

Both `cli` and `tui` packages import the same core logic. The TUI layer is purely presentation. A flag or subcommand (`--interactive` or `tui`) switches between modes.

See [07-tui-readiness.md](07-tui-readiness.md) for details on the Charm stack.
