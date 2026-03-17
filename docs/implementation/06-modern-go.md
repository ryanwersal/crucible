# Modern Go Features

Target **Go 1.26** (released Feb 2026). Use modern features where they improve clarity and correctness.

## Go 1.26: Latest Features

### `new(expr)` syntax

The `new()` built-in now accepts an expression as its operand, specifying the initial value. Eliminates the awkward helper-function pattern for pointer-to-value:

```go
// Before: needed a helper or temp variable
func ptr[T any](v T) *T { return &v }
Age: ptr(yearsSince(born))

// Go 1.26: direct
Age: new(yearsSince(born))
```

Particularly useful for serialization structs where pointers indicate optional fields.

### Generic type self-reference

Generic types can now refer to themselves in their own type parameter list:

```go
type Adder[A Adder[A]] interface {
    Add(A) A
}
```

### Green Tea GC (enabled by default)

10-40% reduction in GC overhead through better locality and CPU scalability. Additional ~10% on newer amd64 CPUs (Intel Ice Lake+, AMD Zen 4+) via vector instructions. No code changes needed — it's automatic.

Opt-out (temporary): `GOEXPERIMENT=nogreenteagc` (expected removal in Go 1.27).

### `errors.AsType[E]()` — type-safe error extraction

```go
// Before
var ve *ValidationError
if errors.As(err, &ve) {
    fmt.Println(ve.Field)
}

// Go 1.26: generic, no pointer variable needed
if ve, ok := errors.AsType[*ValidationError](err); ok {
    fmt.Println(ve.Field)
}
```

Use `errors.AsType` in new code. It's cleaner and harder to misuse.

### `go fix` modernizers

The rewritten `go fix` command includes dozens of "modernizers" that suggest safe refactors to adopt newer language features and stdlib APIs. Run it on the codebase periodically:

```bash
go fix ./...
```

### `slog.NewMultiHandler()`

Fan out log records to multiple handlers:

```go
logger := slog.New(slog.NewMultiHandler(
    slog.NewTextHandler(os.Stderr, nil),  // human-readable to stderr
    slog.NewJSONHandler(logFile, nil),     // structured to file
))
```

### Goroutine leak detection (experimental)

Enable with `GOEXPERIMENT=goroutineleakprofile`. Detects goroutines blocked on unreachable concurrency primitives. Available via `runtime/pprof` and `/debug/pprof/goroutineleak`. Expected default in Go 1.27.

### `T.ArtifactDir()` for test artifacts

```go
func TestGenerate(t *testing.T) {
    dir := t.ArtifactDir() // managed directory for test outputs
    outPath := filepath.Join(dir, "output.json")
    // write test artifacts here; controlled by -artifacts flag
}
```

### `B.Loop()` no longer prevents inlining

All benchmarks should now use `b.Loop()` instead of `for range b.N`:

```go
func BenchmarkFoo(b *testing.B) {
    for b.Loop() {
        foo() // can be inlined now
    }
}
```

### SIMD support (experimental)

`GOEXPERIMENT=simd` enables the `simd/archsimd` package with 128/256/512-bit vector types. amd64 only for now. Useful for data-intensive CLI operations.

### Performance wins (free, no code changes)

- `io.ReadAll()` ~2x faster, ~50% less memory
- cgo overhead reduced ~30%
- Heap base address randomization (security)
- Post-quantum TLS key exchanges enabled by default

## Go 1.25: Key Features

### `sync.WaitGroup.Go()`

Convenient goroutine creation with automatic Add/Done:

```go
var wg sync.WaitGroup
for _, item := range items {
    wg.Go(func() {
        process(item)
    })
}
wg.Wait()
```

Use `errgroup` when you need error propagation; use `wg.Go()` for fire-and-forget work.

### `testing/synctest` — deterministic concurrency testing

Test concurrent code with virtualized time:

```go
import "testing/synctest"

func TestDebounce(t *testing.T) {
    synctest.Run(func() {
        d := NewDebouncer(100 * time.Millisecond)
        d.Trigger()
        d.Trigger()
        d.Trigger()

        // Time advances instantly when all goroutines are blocked
        time.Sleep(200 * time.Millisecond)

        if d.Count() != 1 {
            t.Errorf("expected 1 trigger, got %d", d.Count())
        }
    })
}
```

This is a game-changer for testing timers, debounce, rate limiting, and any time-dependent concurrent logic.

### Container-aware GOMAXPROCS

On Linux, the runtime now respects cgroup CPU bandwidth limits automatically. GOMAXPROCS defaults to the lower of logical CPUs vs. cgroup limit. Relevant if the CLI runs in containers.

### Trace Flight Recorder

Continuously records execution traces to an in-memory ring buffer. Snapshot the last few seconds for debugging:

```go
fr := trace.NewFlightRecorder()
fr.Start()
defer fr.Stop()

// ... later, when something interesting happens
fr.WriteTo(traceFile)
```

### Experimental JSON v2

Enable with `GOEXPERIMENT=jsonv2`. Two new packages: `encoding/json/v2` and `encoding/json/jsontext`. Substantially faster decoding with better configuration options. Consider enabling once it graduates from experimental.

### `os.Root` expanded API

The sandboxed filesystem API (introduced in 1.24) now includes `Chmod`, `Chown`, `Link`, `MkdirAll`, `ReadFile`, `Rename`, `Symlink`, `WriteFile`, and more.

## Established Features (1.22-1.24)

### Loop variable fix (1.22)

Each loop iteration gets its own variable copy. The classic goroutine closure bug is gone:

```go
for _, item := range items {
    go func() {
        process(item) // safe — item is per-iteration
    }()
}
```

### Range over functions / iterators (1.23)

Functions matching `func(yield func(V) bool)` work in `for range` loops:

```go
func FilterItems(items []Item, pred func(Item) bool) iter.Seq[Item] {
    return func(yield func(Item) bool) {
        for _, item := range items {
            if pred(item) {
                if !yield(item) {
                    return
                }
            }
        }
    }
}

for item := range FilterItems(items, isValid) {
    fmt.Println(item)
}
```

Use `iter.Seq[V]` for single-value and `iter.Seq2[K, V]` for two-value iterators.

### Tool directives in go.mod (1.24)

```
// go.mod
module crucible

go 1.26

tool (
    github.com/goreleaser/goreleaser/v2
    golang.org/x/vuln/cmd/govulncheck
)
```

Run with `go tool <name>`.

### `os.Root` sandboxed filesystem (1.24, expanded in 1.25)

```go
root, err := os.OpenRoot("/allowed/directory")
if err != nil {
    return err
}
defer root.Close()

data, err := root.ReadFile("subdir/file.txt")  // OK
data, err := root.ReadFile("../../etc/passwd")  // error: path escapes root
```

## Structured Logging with slog

Use `log/slog` for all logging:

```go
import "log/slog"

// Setup
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)

// Usage
slog.Info("processing items", "count", len(items), "source", path)
slog.Error("failed to connect", "addr", addr, "err", err)

// Multi-handler (Go 1.26)
logger := slog.New(slog.NewMultiHandler(
    slog.NewTextHandler(os.Stderr, nil),
    slog.NewJSONHandler(logFile, nil),
))
```

**Guidelines**:
- `slog.LevelDebug` for verbose diagnostic info (behind `--verbose`/`--debug` flags)
- `slog.LevelInfo` for normal operational messages
- `slog.LevelError` for failures
- Pass loggers via dependency injection in library code
- Use `slog.DiscardHandler` in tests to suppress output

## Generics

Generics are a first-class tool in Go. Use them freely — but use meaningful type constraints, not `any`.

### The `any` problem

`any` as a constraint means "I accept everything but can do nothing with it." It's `interface{}` with generic syntax — you lose type safety and gain nothing:

```go
// Bad: any tells you nothing — what can you do with T? Nothing without assertions.
func Process[T any](items []T) []T { ... }
func Transform[T any, U any](input T, fn func(T) U) U { ... }

// Also bad: using any to avoid thinking about constraints
type Cache[K any, V any] struct { ... } // K isn't even comparable — can't use as map key
```

If you find yourself writing `any`, ask: "what operations does this function actually need from T?" The answer is your constraint.

### Meaningful constraints

Generics shine when the constraint communicates what the type must support:

```go
// Good: constraint tells you exactly what T must do
type Identifiable interface {
    ID() string
}

func Index[T Identifiable](items []T) map[string]T {
    m := make(map[string]T, len(items))
    for _, item := range items {
        m[item.ID()] = item
    }
    return m
}

// Good: comparable is a real constraint — enables map keys and equality checks
type Set[T comparable] map[T]struct{}

func NewSet[T comparable](items ...T) Set[T] {
    s := make(Set[T], len(items))
    for _, item := range items {
        s[item] = struct{}{}
    }
    return s
}

// Good: union constraint for numeric operations
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](values []T) T {
    var total T
    for _, v := range values {
        total += v
    }
    return total
}
```

### Generics in domain modeling

Well-constrained generics are appropriate for domain code:

```go
// Result type for operations that can fail with typed errors
type Result[T any, E error] struct {
    value T
    err   E
}

// Generic repository — the constraint ensures T is a domain entity
type Entity interface {
    Identifiable
    Validate() error
}

type Repository[T Entity] struct {
    store map[string]T
}

func (r *Repository[T]) Save(entity T) error {
    if err := entity.Validate(); err != nil {
        return fmt.Errorf("validation: %w", err)
    }
    r.store[entity.ID()] = entity
    return nil
}

// Pipeline stages with typed input/output
type Stage[In, Out any] func(context.Context, In) (Out, error)

func Chain[A, B, C any](first Stage[A, B], second Stage[B, C]) Stage[A, C] {
    return func(ctx context.Context, input A) (C, error) {
        mid, err := first(ctx, input)
        if err != nil {
            var zero C
            return zero, err
        }
        return second(ctx, mid)
    }
}
```

Note: `any` is acceptable when the generic function truly works with *all* types and the type parameter exists for type safety between caller and callee (like `Stage[In, Out]` above ensuring pipeline stages connect correctly). The problem is `any` as a lazy default, not `any` as a deliberate "this works for everything."

### Standard library generics

Use the stdlib generic packages — they replace tons of hand-rolled boilerplate:

```go
import (
    "slices"
    "maps"
)

slices.Sort(items)
slices.SortFunc(items, func(a, b Item) int {
    return strings.Compare(a.Name, b.Name)
})

idx, found := slices.BinarySearch(sorted, target)

keys := maps.Keys(m)

// errors.AsType (Go 1.26) — generics in the stdlib
if ve, ok := errors.AsType[*ValidationError](err); ok { ... }
```

### When NOT to use generics

- When a concrete type works fine and there's only one type involved
- When the generic version is harder to read than 2-3 concrete implementations
- When you're adding a type parameter just to avoid writing a second function that you don't actually need yet
