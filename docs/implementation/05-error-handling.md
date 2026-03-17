# Error Handling

## Three Patterns

### 1. Sentinel errors

Pre-declared error values for well-known conditions that callers need to match on.

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrTimeout      = errors.New("operation timed out")
)
```

Check with `errors.Is()`:

```go
if errors.Is(err, ErrNotFound) {
    // handle not-found case
}
```

**When to use**: When the caller needs to branch on a specific error condition and no additional context is needed. Define sparingly — each sentinel error is part of your package's API.

### 2. Custom error types

Structured errors that carry additional information.

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}
```

Check with `errors.AsType` (Go 1.26, preferred) or `errors.As()`:

```go
// Go 1.26: type-safe, no pointer variable needed
if ve, ok := errors.AsType[*ValidationError](err); ok {
    fmt.Printf("field %s: %s\n", ve.Field, ve.Message)
}

// Pre-1.26 style (still works)
var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Printf("field %s: %s\n", ve.Field, ve.Message)
}
```

**When to use**: When callers need structured information from the error (field name, status code, retry-after duration, etc).

### 3. Error wrapping

Add context as errors propagate up the call stack.

```go
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("loading config %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parsing config %s: %w", path, err)
    }

    return &cfg, nil
}
```

**When to use**: Always, as errors propagate up. But don't wrap mechanically — wrap when you add valuable context (what operation failed, what input caused it).

## Rules

### Always use `errors.Is()` and `errors.AsType()`

Never use `==` or type assertions for error checking. The `Is`/`AsType`/`As` functions handle wrapped errors correctly.

```go
// Bad
if err == ErrNotFound { ... }
if _, ok := err.(*ValidationError); ok { ... }

// Good
if errors.Is(err, ErrNotFound) { ... }
if ve, ok := errors.AsType[*ValidationError](err); ok { ... }
```

### Format errors for humans at the top level

Library/internal code returns structured errors. The CLI layer formats them for users.

```go
// internal/runner/process.go — returns structured error
func Process(ctx context.Context, items []Item) error {
    // ...
    return fmt.Errorf("processing item %s: %w", item.ID, err)
}

// internal/cli/run.go — formats for user
func runCmd(cmd *cobra.Command, args []string) error {
    if err := runner.Process(ctx, items); err != nil {
        // User-friendly message, not raw error chain
        return fmt.Errorf("failed to process: %w", err)
    }
    return nil
}
```

### Don't wrap errors that add no context

```go
// Bad: wrapper adds nothing
if err != nil {
    return fmt.Errorf("error: %w", err)
}

// Good: just propagate
if err != nil {
    return err
}

// Good: adds useful context
if err != nil {
    return fmt.Errorf("connecting to %s: %w", addr, err)
}
```

### Use `%w` for wrappable errors, `%v` for opaque errors

- `%w` — callers can unwrap and inspect the underlying error.
- `%v` — the error message is included but the underlying error is hidden from `errors.Is`/`errors.As`.

Use `%v` when you intentionally don't want callers to depend on the underlying error type (implementation detail).

## Error Handling in Concurrent Code

With `errgroup`, the first error cancels the group's context:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10)

for _, item := range items {
    g.Go(func() error {
        if err := process(ctx, item); err != nil {
            return fmt.Errorf("item %s: %w", item.ID, err)
        }
        return nil
    })
}

if err := g.Wait(); err != nil {
    // This is the first error that occurred.
    // All other goroutines were cancelled via ctx.
    return err
}
```

If you need to collect all errors (not just the first), use a mutex-protected slice or a channel — but this is rare in CLI tools where failing fast is usually correct.
