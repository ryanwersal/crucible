# Build and Release

## GoReleaser

[GoReleaser](https://goreleaser.com) handles cross-compilation, packaging, and publishing.

### .goreleaser.yaml

```yaml
version: 2

builds:
  - main: ./cmd/crucible
    binary: crucible
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}
    mod_timestamp: "{{.CommitTimestamp}}"
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
```

### Version embedding

```go
// cmd/crucible/main.go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

Access via a `version` subcommand:

```go
func NewVersionCmd(info BuildInfo) *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print version information",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Fprintf(cmd.OutOrStdout(), "crucible %s (commit: %s, built: %s)\n",
                info.Version, info.Commit, info.Date)
        },
    }
}
```

## Reproducible Builds

Checklist:
1. **`-trimpath`** — removes local filesystem paths from the binary.
2. **`{{.CommitDate}}`** not `{{.Date}}` — build timestamp is deterministic from git history.
3. **`mod_timestamp: "{{.CommitTimestamp}}"`** — file modification times in archives are deterministic.
4. **`CGO_ENABLED=0`** — pure Go binary, no system C library dependency.
5. **Pin Go version** in `go.mod` (`go 1.26`) and CI.

## Development Build

```bash
# Quick local build
go build -o crucible ./cmd/crucible

# With version info during development
go build -ldflags "-X main.version=dev-$(git rev-parse --short HEAD)" -o crucible ./cmd/crucible

# Test GoReleaser config without publishing
goreleaser check
goreleaser build --snapshot --clean
```

## CI Pipeline

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test -race -coverprofile=cover.out ./...
      - run: go vet ./...

  release:
    needs: test
    if: startsWith(github.ref, 'refs/tags/')
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Tool Directives in go.mod

Track build/dev tools in `go.mod` (Go 1.24+):

```
tool (
    github.com/goreleaser/goreleaser/v2
    golang.org/x/vuln/cmd/govulncheck
)
```

Run with `go tool goreleaser`, `go tool govulncheck ./...`.

## `go fix` Modernizers

Go 1.26's rewritten `go fix` includes dozens of modernizers that safely refactor code to use newer features. Run periodically:

```bash
go fix ./...
```

## Install Methods

Support multiple install paths for ease of adoption:

```bash
# Go install (for Go developers)
go install github.com/user/crucible/cmd/crucible@latest

# Homebrew (via GoReleaser tap)
brew install user/tap/crucible

# Direct binary download
curl -sSL https://github.com/user/crucible/releases/latest/download/crucible_$(uname -s)_$(uname -m).tar.gz | tar xz
```
