# Crucible

Go CLI tool targeting Go 1.26. See `docs/implementation/` for detailed coding standards.

## Key conventions

- All code lives in `internal/` — this is a CLI tool, not a library.
- Use `errgroup` with `SetLimit()` for concurrent work, not raw goroutines.
- Use `slog` for structured logging. No `fmt.Print` for operational output.
- Use meaningful generic constraints — avoid `any` as a lazy default.
- Use `errors.Is()` and `errors.AsType[E]()` for error checking, never `==` or type assertions.
- Factory functions for cobra commands, not global `init()`.
- Business logic in `internal/runner/` (or domain packages) must be independent of CLI and TUI layers.

## Adding or modifying resources

When adding or modifying resources, update `internal/reference/reference.go` with JS API docs, declaration type description, and action type descriptions. The `crucible reference` command must always reflect the full API surface.

## After sizeable changes

Run `/audit` after all sizeable changes (new features, refactors, concurrency work, security-sensitive code). It automatically reviews all staged/unstaged changes plus every commit on the current branch back to main. No arguments needed — just run `/audit`.
