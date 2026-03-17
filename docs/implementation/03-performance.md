# Performance

Go compiles to native binaries with fast startup (~5-20ms). This document covers patterns for keeping things fast and techniques for finding bottlenecks.

## Allocation Patterns

Memory allocation is typically the largest performance lever in Go programs. The garbage collector is good, but generating less garbage is always better.

### Pre-allocate slices and maps

```go
// Bad: grows and reallocates multiple times
var results []Result
for _, item := range items {
    results = append(results, process(item))
}

// Good: single allocation
results := make([]Result, 0, len(items))
for _, item := range items {
    results = append(results, process(item))
}

// Maps too
m := make(map[string]int, expectedSize)
```

### Reuse with sync.Pool

For hot paths that create and discard temporary objects:

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func formatOutput(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()

    // use buf...
    return buf.String()
}
```

Use `sync.Pool` only when profiling shows allocation pressure. Don't use it speculatively.

### Avoid unnecessary string conversions

```go
// Bad: allocates a new string
if string(jsonBytes) == "{}" { ... }

// Good: compare bytes directly
if bytes.Equal(jsonBytes, []byte("{}")) { ... }
```

### Strings.Builder for concatenation

```go
// Bad for many concatenations
s := ""
for _, part := range parts {
    s += part
}

// Good
var b strings.Builder
b.Grow(estimatedSize) // optional: pre-allocate
for _, part := range parts {
    b.WriteString(part)
}
result := b.String()
```

## Go 1.25/1.26 Performance Wins (Free)

These improvements require no code changes:

- **Green Tea GC** (default in 1.26) — 10-40% reduction in GC overhead. Additional ~10% on newer amd64 CPUs (Ice Lake+, Zen 4+).
- **`io.ReadAll()` ~2x faster, ~50% less memory** (1.26).
- **cgo overhead reduced ~30%** (1.26).
- **Swiss Tables map implementation** (1.24) — faster map operations across the board.
- **Stack-allocated slice backing stores** in more situations (1.25+) — reduced heap allocations.
- **DWARF v5** debug info by default (1.25) — smaller binaries and faster linking.

## Profiling with pprof

### CPU profiling

```go
import "runtime/pprof"

f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

// ... code to profile
```

Or from tests:
```bash
go test -bench=. -cpuprofile=cpu.prof
go tool pprof -http=":8080" cpu.prof
```

### Memory profiling

```bash
go test -bench=. -benchmem -memprofile=mem.prof
go tool pprof -http=":8080" mem.prof
```

Key distinction in memory profiles:
- **`alloc_space`** — total allocations over program lifetime. Shows allocation-heavy code paths.
- **`inuse_space`** — currently retained memory. Shows where leaks are.

### Adding profile support to the CLI

For debugging production issues, consider a hidden `--profile` flag:

```go
if profilePath != "" {
    f, err := os.Create(profilePath)
    if err != nil {
        return err
    }
    defer f.Close()
    if err := pprof.StartCPUProfile(f); err != nil {
        return err
    }
    defer pprof.StopCPUProfile()
}
```

## Benchmarking

### Writing benchmarks

```go
func BenchmarkProcess(b *testing.B) {
    input := setupTestData()
    b.ResetTimer() // exclude setup from measurement

    for b.Loop() { // b.Loop() no longer prevents inlining (Go 1.26)
        process(input)
    }
}

func BenchmarkProcessWithAllocs(b *testing.B) {
    b.ReportAllocs()
    input := setupTestData()
    b.ResetTimer()

    for b.Loop() {
        process(input)
    }
}
```

### Running benchmarks

```bash
# Run benchmarks with statistical significance
go test -bench=BenchmarkProcess -benchmem -count=10 ./internal/runner/

# Compare before/after
go test -bench=. -benchmem -count=10 ./... > old.txt
# ... make changes ...
go test -bench=. -benchmem -count=10 ./... > new.txt
benchstat old.txt new.txt
```

Install benchstat: `go install golang.org/x/perf/cmd/benchstat@latest`

### What to benchmark

- Functions called in hot loops (processing many items)
- Serialization/deserialization (JSON, protocol buffers)
- String formatting and output rendering
- Any function where allocation count matters

### What NOT to benchmark

- One-time startup code
- Functions dominated by I/O (network, disk) — benchmark those as integration tests
- Code that's already fast enough — don't optimize without evidence

## Startup Time

Go binaries start fast. Keep it that way:

- **Avoid heavy `init()` functions** — lazy-load expensive resources.
- **Don't import unused packages** — each import adds to binary size and init time.
- **Avoid importing `net/http`** if you don't need it — it pulls in a large dependency tree.
- **Use `cobra`'s lazy command loading** for CLI tools with many subcommands.

## I/O Performance

### Buffered I/O

```go
// Bad: many small writes
for _, line := range lines {
    fmt.Fprintln(os.Stdout, line)
}

// Good: buffered output
w := bufio.NewWriter(os.Stdout)
defer w.Flush()
for _, line := range lines {
    fmt.Fprintln(w, line)
}
```

### Streaming vs loading into memory

For large data sets, prefer streaming (process items as they arrive) over loading everything into memory first.

```go
// Good: streaming with iterators (Go 1.23+)
func scanItems(r io.Reader) iter.Seq2[Item, error] {
    return func(yield func(Item, error) bool) {
        scanner := bufio.NewScanner(r)
        for scanner.Scan() {
            item, err := parseItem(scanner.Bytes())
            if !yield(item, err) {
                return
            }
        }
        if err := scanner.Err(); err != nil {
            yield(Item{}, err)
        }
    }
}
```
