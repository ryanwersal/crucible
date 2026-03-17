# Testing

## Philosophy

- **Prefer the standard `testing` package** over third-party assertion libraries. It's sufficient for almost everything and adds zero dependencies.
- **Use `testify/require`** sparingly — only when you need to halt a test immediately on failure (avoiding cascading panics on nil dereferences, etc).
- **Test behavior, not implementation** — tests should survive refactoring.
- **Race detection is mandatory** — always run CI with `-race`.

## Table-Driven Tests

The idiomatic Go testing pattern. Use it by default.

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Config
        wantErr string // empty means no error expected
    }{
        {
            name:  "valid config",
            input: `{"verbose": true}`,
            want:  Config{Verbose: true},
        },
        {
            name:    "invalid json",
            input:   `{bad`,
            wantErr: "invalid character",
        },
        {
            name:  "empty input uses defaults",
            input: `{}`,
            want:  Config{LogLevel: "info"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig([]byte(tt.input))

            if tt.wantErr != "" {
                if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
                    t.Fatalf("got err=%v, want err containing %q", err, tt.wantErr)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if got != tt.want {
                t.Errorf("got %+v, want %+v", got, tt.want)
            }
        })
    }
}
```

### Guidelines for table-driven tests

- **Name every test case** — the `name` field appears in `-v` output and failure messages.
- **Use `t.Fatal`/`t.Fatalf` for precondition failures** (errors that make further assertions meaningless).
- **Use `t.Error`/`t.Errorf` for assertion failures** (the test can continue to report more problems).
- **Keep test cases independent** — no test should depend on the order or outcome of another.

## Testing CLI Commands

Cobra commands need factory functions to be testable:

```go
// internal/cli/root.go
func NewRootCmd(cfg *config.Config) *cobra.Command {
    return &cobra.Command{
        Use: "crucible",
        // ...
    }
}

// internal/cli/root_test.go
func TestRootCmd(t *testing.T) {
    var stdout bytes.Buffer
    cmd := NewRootCmd(&config.Config{})
    cmd.SetOut(&stdout)
    cmd.SetErr(&bytes.Buffer{})
    cmd.SetArgs([]string{"--verbose"})

    err := cmd.Execute()
    if err != nil {
        t.Fatalf("execute: %v", err)
    }

    if !strings.Contains(stdout.String(), "expected output") {
        t.Errorf("stdout = %q, want it to contain 'expected output'", stdout.String())
    }
}
```

Cobra commands hold internal state, so always create new instances per test via factory functions.

## Golden Files

For testing complex output (formatted tables, multi-line strings), use golden files stored in `testdata/`.

```go
var update = flag.Bool("update", false, "update golden files")

func TestFormatTable(t *testing.T) {
    got := FormatTable(testData)

    golden := filepath.Join("testdata", t.Name()+".golden")

    if *update {
        os.MkdirAll("testdata", 0o755)
        os.WriteFile(golden, []byte(got), 0o644)
        return
    }

    want, err := os.ReadFile(golden)
    if err != nil {
        t.Fatalf("reading golden file (run with -update to create): %v", err)
    }

    if got != string(want) {
        t.Errorf("output mismatch.\ngot:\n%s\nwant:\n%s", got, string(want))
    }
}
```

Update golden files with: `go test -run TestFormatTable -update ./...`

The `testdata/` directory is automatically ignored by `go build` and `go mod`.

## Fuzzing

Built into `go test` since Go 1.18. Excellent for parsing, deserialization, and input validation code.

```go
func FuzzParseInput(f *testing.F) {
    // Seed corpus
    f.Add("valid input")
    f.Add("")
    f.Add("{}")

    f.Fuzz(func(t *testing.T, input string) {
        result, err := ParseInput(input)
        if err != nil {
            return // errors are fine, panics are not
        }
        // Check invariants that should always hold
        if result.ID == "" {
            t.Error("parsed result has empty ID")
        }
    })
}
```

```bash
# Run fuzzing for 30 seconds
go test -fuzz=FuzzParseInput -fuzztime=30s ./internal/parser/

# Corpus files are stored in testdata/fuzz/FuzzParseInput/
```

## Race Detection

```bash
# Always in CI
go test -race ./...

# Locally during development
go test -race -count=1 ./internal/runner/
```

Zero tolerance for data races. A single race detector failure should block CI.

## Testing Concurrent Code

```go
func TestConcurrentAccess(t *testing.T) {
    store := NewStore()
    var wg sync.WaitGroup

    for i := range 100 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            store.Set(fmt.Sprintf("key-%d", i), i)
            store.Get(fmt.Sprintf("key-%d", i))
        }()
    }

    wg.Wait()
    // With -race flag, any data race will be caught
}
```

## Integration Tests

Use build tags to separate integration tests from unit tests:

```go
//go:build integration

package mypackage_test

func TestWithExternalService(t *testing.T) {
    // ...
}
```

```bash
# Unit tests only (default)
go test ./...

# Including integration tests
go test -tags=integration ./...
```

## Test Helpers

Use `t.Helper()` in test helper functions so failure messages point to the caller:

```go
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func tempDir(t *testing.T) string {
    t.Helper()
    dir := t.TempDir() // automatically cleaned up
    return dir
}
```

## Deterministic Concurrency Testing with synctest

`testing/synctest` (Go 1.25) provides virtualized time for deterministic testing of concurrent code. Time advances instantly when all goroutines are blocked.

```go
import "testing/synctest"

func TestRateLimiter(t *testing.T) {
    synctest.Run(func() {
        rl := NewRateLimiter(10, time.Second)

        // Use all 10 tokens
        for range 10 {
            if !rl.Allow() {
                t.Fatal("should allow first 10 requests")
            }
        }

        // 11th should be denied
        if rl.Allow() {
            t.Fatal("should deny 11th request")
        }

        // Advance past the window — time jumps instantly
        time.Sleep(2 * time.Second)

        if !rl.Allow() {
            t.Fatal("should allow after window reset")
        }
    })
}
```

Use `synctest` for testing timers, debounce, rate limiting, retries, and any time-dependent concurrent logic.

## Test Artifacts

Use `t.ArtifactDir()` (Go 1.26) for test outputs that should be inspectable after test runs:

```go
func TestGenerateReport(t *testing.T) {
    report := generateReport(testData)
    outPath := filepath.Join(t.ArtifactDir(), "report.html")
    os.WriteFile(outPath, report, 0o644)
}
```

Control artifact output location with the `-artifacts` flag.

## Coverage

```bash
# Generate coverage report
go test -coverprofile=cover.out ./...

# View in browser
go tool cover -html=cover.out

# Check coverage percentage
go tool cover -func=cover.out
```

Aim for meaningful coverage of business logic. Don't chase 100% — testing `main()` wiring and trivial getters adds noise, not value.
