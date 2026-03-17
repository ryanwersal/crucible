# Configuration and Dependency Injection

## Configuration Precedence

Standard order (highest to lowest priority):

1. **CLI flags** — explicit user intent for this invocation
2. **Environment variables** — deployment/session-level configuration
3. **Config file** — persistent user preferences
4. **Defaults** — sensible out-of-the-box behavior

## Configuration Library: Koanf

[Koanf](https://github.com/knadh/koanf) (v2) is recommended over Viper:

- **Modular provider architecture** — load from files, env vars, flags, remote sources
- **No forced key lowercasing** — respects original key casing (Viper forces lowercase)
- **Small dependency footprint** — Viper bloats binary size by ~3x
- **No global state** — explicitly passed instances

```go
package config

import (
    "github.com/knadh/koanf/v2"
    "github.com/knadh/koanf/providers/env"
    "github.com/knadh/koanf/providers/file"
    "github.com/knadh/koanf/providers/posflag"
    "github.com/knadh/koanf/parsers/yaml"
)

type Config struct {
    Verbose  bool   `koanf:"verbose"`
    LogLevel string `koanf:"log_level"`
    Output   string `koanf:"output"`
    Workers  int    `koanf:"workers"`
}

// Load applies configuration precedence: flags > env > file > defaults


func Load(flags *pflag.FlagSet) (*Config, error) {
    k := koanf.New(".")

    // 1. Defaults
    k.Load(confmap.Provider(map[string]interface{}{
        "log_level": "info",
        "output":    "text",
        "workers":   runtime.NumCPU(),
    }, "."), nil)

    // 2. Config file (optional)
    configPath := filepath.Join(xdgConfigHome(), "crucible", "config.yaml")
    if _, err := os.Stat(configPath); err == nil {
        if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
            return nil, fmt.Errorf("loading config file: %w", err)
        }
    }

    // 3. Environment variables (CRUCIBLE_ prefix)
    k.Load(env.Provider("CRUCIBLE_", ".", func(s string) string {
        return strings.ToLower(strings.TrimPrefix(s, "CRUCIBLE_"))
    }), nil)

    // 4. CLI flags (highest priority)
    if flags != nil {
        k.Load(posflag.Provider(flags, ".", k), nil)
    }

    var cfg Config
    if err := k.Unmarshal("", &cfg); err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    return &cfg, nil
}
```

## Dependency Injection

Go favors explicit constructor injection. No DI framework needed — the `main` function (or `cli.Run`) is your composition root.

```go
// Constructor injection — dependencies are explicit
type Runner struct {
    logger *slog.Logger
    client *http.Client
    config *Config
}

func NewRunner(cfg *Config, logger *slog.Logger, client *http.Client) *Runner {
    return &Runner{
        config: cfg,
        logger: logger,
        client: client,
    }
}

// Composition root (in cli.Run or main)
func Run(ctx context.Context, args []string) int {
    cfg, err := config.Load(flags)
    if err != nil { ... }

    logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: logLevel(cfg.LogLevel),
    }))

    client := &http.Client{Timeout: 30 * time.Second}
    runner := runner.NewRunner(cfg, logger, client)

    // ... wire up commands with runner
}
```

### Use interfaces at consumption site

Define interfaces where they're used, not where they're implemented:

```go
// internal/runner/runner.go
// Define the interface the runner needs, not what the implementation provides
type FileReader interface {
    ReadFile(path string) ([]byte, error)
}

type Runner struct {
    files FileReader
}

// In production: os-based implementation
// In tests: in-memory implementation
```

This is Go's implicit interface satisfaction at work — the implementation doesn't need to know about the interface.

## Config File Location

Follow the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/):

```go
func xdgConfigHome() string {
    if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
        return dir
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config")
}

// Config file: ~/.config/crucible/config.yaml
```

## Environment Variables

Use `CRUCIBLE_` prefix for all environment variables:

| Variable             | Description              | Default         |
|----------------------|--------------------------|-----------------|
| `CRUCIBLE_VERBOSE`   | Enable verbose output    | `false`         |
| `CRUCIBLE_LOG_LEVEL` | Log level                | `info`          |
| `CRUCIBLE_OUTPUT`    | Output format            | `text`          |
| `CRUCIBLE_WORKERS`   | Concurrent worker count  | `runtime.NumCPU()` |
