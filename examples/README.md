# Examples

## getting-started

Minimal example that installs a couple of Homebrew packages.

```
cd examples/getting-started
crucible apply --dry-run
crucible apply
```

## complete

Full macOS system configuration example demonstrating all supported action
types: Homebrew packages, macOS defaults, Dock layout, git clones, directories,
and symlinks.

```
cd examples/complete
crucible apply --dry-run
crucible apply
```
