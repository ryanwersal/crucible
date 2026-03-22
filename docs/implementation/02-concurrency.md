# Concurrency Patterns

Go's concurrency primitives are a primary reason for choosing the language. This document covers patterns for writing correct, fast concurrent code in CLI tools.

## Core Rules

1. **Every long-running operation accepts a `context.Context`** — this is non-negotiable for cancellation and timeout propagation.
2. **Never spawn unbounded goroutines on unbounded work** — always limit concurrency.
3. **Prefer `errgroup` over raw goroutines** — it handles synchronization and error propagation.
4. **Use `signal.NotifyContext` for graceful shutdown** in CLI tools.

## errgroup (the workhorse)

`golang.org/x/sync/errgroup` is the standard tool for managing concurrent work with error handling.

### Bounded worker pool

```go
import "golang.org/x/sync/errgroup"

func processItems(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // max 10 concurrent goroutines

    for _, item := range items {
        g.Go(func() error {
            return process(ctx, item)
        })
    }

    return g.Wait()
}
```

Key details:
- `SetLimit(n)` caps active goroutines — no need to build your own worker pool.
- `errgroup.WithContext` creates a derived context that cancels when any goroutine returns an error.
- Loop variables are per-iteration (Go 1.22+) — no `item := item` capture needed.
- `g.Wait()` blocks until all goroutines complete and returns the first non-nil error.

### Fan-out / fan-in with results

When you need to collect results from concurrent work:

```go
func fetchAll(ctx context.Context, urls []string) ([]Result, error) {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(20)

    results := make([]Result, len(urls))

    for i, url := range urls {
        g.Go(func() error {
            r, err := fetch(ctx, url)
            if err != nil {
                return fmt.Errorf("fetching %s: %w", url, err)
            }
            results[i] = r // safe: each goroutine writes to its own index
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

Writing to distinct indices of a pre-allocated slice is safe without a mutex.

## Semaphore pattern

Use `golang.org/x/sync/semaphore` when you need weighted concurrency limiting (e.g., some operations cost more than others):

```go
import "golang.org/x/sync/semaphore"

sem := semaphore.NewWeighted(int64(maxConcurrency))

for _, item := range items {
    if err := sem.Acquire(ctx, 1); err != nil {
        return err // context cancelled
    }
    go func() {
        defer sem.Release(1)
        process(ctx, item)
    }()
}

// Wait for all in-flight work to finish
if err := sem.Acquire(ctx, int64(maxConcurrency)); err != nil {
    return err
}
```

## Channels

Use channels when goroutines need to communicate, not just synchronize.

### Pipeline pattern

```go
func pipeline(ctx context.Context, input <-chan Item) <-chan Result {
    out := make(chan Result)
    go func() {
        defer close(out)
        for item := range input {
            result, err := transform(ctx, item)
            if err != nil {
                continue // or send error on a separate channel
            }
            select {
            case out <- result:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}
```

Always include `case <-ctx.Done()` in `select` statements to prevent goroutine leaks.

### Buffered vs unbuffered

- **Unbuffered** (`make(chan T)`): Use when you need synchronization between sender and receiver.
- **Buffered** (`make(chan T, n)`): Use when the producer can outpace the consumer temporarily. Size the buffer based on known batching characteristics, not arbitrary numbers.

## Graceful Shutdown

CLI tools should handle interrupt signals cleanly:

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    if err := run(ctx); err != nil {
        if ctx.Err() != nil {
            fmt.Fprintln(os.Stderr, "interrupted")
            os.Exit(130) // standard exit code for SIGINT
        }
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

All downstream operations respect `ctx` cancellation, so an interrupt propagates naturally through the call tree.

## Common Mistakes

### Goroutine leaks

```go
// BAD: goroutine leaks if nobody reads from ch
ch := make(chan Result)
go func() {
    ch <- expensiveComputation()
}()
// ... early return without reading ch

// GOOD: use context or select with done channel
go func() {
    result := expensiveComputation()
    select {
    case ch <- result:
    case <-ctx.Done():
    }
}()
```

### Shared mutable state

If multiple goroutines must access shared state, prefer:
1. **Channel-based ownership** — only one goroutine owns the state, others communicate via channels.
2. **`sync.Mutex`** — when channel-based approaches add unnecessary complexity.
3. **`sync/atomic`** — for simple counters and flags.

```go
// Atomic counter for progress tracking
var completed atomic.Int64

g.Go(func() error {
    err := process(ctx, item)
    completed.Add(1)
    return err
})

fmt.Printf("Progress: %d/%d\n", completed.Load(), total)
```

### sync.WaitGroup

Prefer `errgroup` over raw `sync.WaitGroup` when you need error propagation. For fire-and-forget concurrent work, use `sync.WaitGroup.Go()` (Go 1.25+):

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() {
        process(item) // no error return needed
    })
}
wg.Wait()
```

`wg.Go()` handles `Add`/`Done` automatically — no manual counting.
