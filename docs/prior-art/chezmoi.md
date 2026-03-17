# chezmoi: Behavioral Analysis

chezmoi is a dotfile manager that maintains a source directory as the single source of truth
for your home directory configuration. It computes the desired state from source files and
templates, compares it against the actual state of the home directory, and applies the
minimal changes needed. It ships as a single static binary with no runtime dependencies.

## The Source-Target Model

chezmoi operates on three distinct states:

- **Source state**: The desired configuration stored as files in the source directory
  (default: `~/.local/share/chezmoi`). This is version-controlled with git.
- **Destination state**: The actual current condition of managed files in `$HOME`.
- **Target state**: The computed desired state -- source state after template evaluation,
  decryption, and attribute application. This is what chezmoi makes the destination match.

The operational flow: read source state → evaluate templates with config/data → compute
target state → compare to destination state → apply minimal changes.

Updates are **atomic** -- files are written to a temporary location and renamed into place,
preventing partial or corrupt states. This is a meaningful safety property: a crash or
interruption during apply never leaves a managed file in a half-written state.

### Source Directory Naming Convention

Source filenames encode metadata through prefixes and suffixes:

| Source Name | Target | Effect |
|-------------|--------|--------|
| `dot_bashrc` | `.bashrc` | Leading dot |
| `private_dot_ssh/` | `.ssh/` | 0700 permissions |
| `executable_dot_local/bin/my-script` | `.local/bin/my-script` | Executable bit |
| `encrypted_private_dot_ssh/private_id_rsa` | `.ssh/id_rsa` | Decrypted, 0600 |
| `dot_gitconfig.tmpl` | `.gitconfig` | Template-evaluated |
| `exact_dot_config/fontconfig/` | `.config/fontconfig/` | Unmanaged children removed |

This encoding is the key design choice that makes the source directory both human-readable
and self-describing. The mapping between source and target is deterministic and inspectable
without running any commands.

## Template System

chezmoi uses Go's `text/template` syntax, augmented with the sprig function library and
chezmoi-specific functions. Files become templates when they have a `.tmpl` suffix.

### Template Data Sources (Later Overrides Earlier)

1. Built-in variables under `.chezmoi.*` (OS, hostname, arch, username, etc.)
2. `.chezmoidata.$FORMAT` files (JSON, TOML, YAML)
3. `[data]` section of the config file (`~/.config/chezmoi/chezmoi.toml`)

### Built-in Variables

The `.chezmoi` namespace provides system introspection without running commands:

- **System**: `.os`, `.arch`, `.hostname`, `.fqdnHostname`, `.kernel`
- **User**: `.username`, `.uid`, `.gid`, `.group`, `.homeDir`
- **Paths**: `.sourceDir`, `.destDir`, `.cacheDir`
- **Platform**: `.osRelease` (Linux distro info), `.windowsVersion`

### What Makes the Template System Good

**Machine-specific configuration from a single branch.** Instead of maintaining separate
branches per machine (fragile, diverges over time), chezmoi uses conditionals:

```
{{- if eq .chezmoi.os "darwin" }}
export HOMEBREW_PREFIX="/opt/homebrew"
{{- else if eq .chezmoi.os "linux" }}
export PATH="$HOME/.local/bin:$PATH"
{{- end }}
```

One repository, one branch, all machines. Differences are expressed where they occur, not in
separate copies of entire files.

**Password manager functions retrieve secrets at template evaluation time.** The source repo
contains references, not values:

```
export API_TOKEN='{{ onepasswordRead "op://Personal/api-token/password" }}'
```

Supported managers: 1Password, Bitwarden, LastPass, pass, Vault, gopass, KeePassXC, Keeper,
AWS Secrets Manager, Azure Key Vault, Doppler, Dashlane, system keyring, and more. This is
unique among dotfile managers -- no other tool has comparable secret management breadth.

**`output` and `outputList` execute arbitrary commands** and capture their output for use in
templates, enabling integration with any external system.

**Reusable template fragments** live in `.chezmoitemplates/` and are includable across
files, eliminating duplication.

**`chezmoi execute-template`** lets you test templates interactively without applying
anything.

## Special File Types

### Scripts

Scripts are source files with `run_` prefixes. They execute during `chezmoi apply` and
provide escape hatches for actions that can't be expressed declaratively.

| Prefix | Behavior |
|--------|----------|
| `run_` | Execute on every apply |
| `run_once_` | Execute only if this exact content hasn't run before (tracked by SHA256) |
| `run_onchange_` | Execute only when content differs from last successful run |
| `run_before_` / `run_after_` | Control timing relative to file updates |

`run_once_` and `run_onchange_` state is tracked in chezmoi's persistent BoltDB database.
`run_once_` scripts are identified by content hash -- if you change the script, it runs
again. `run_onchange_` scripts track content changes specifically.

Scripts can be templates (`.tmpl` suffix). A template script that evaluates to
empty/whitespace is skipped entirely, enabling conditional execution:

```
{{- if eq .chezmoi.os "linux" }}
#!/bin/bash
sudo apt-get update && sudo apt-get install -y ripgrep
{{- end }}
```

### Control Prefixes

| Prefix | Behavior |
|--------|----------|
| `create_` | Write contents only if target doesn't exist; never overwrite |
| `modify_` | Script that receives current file on stdin, outputs replacement on stdout |
| `remove_` | Remove the target |
| `exact_` | (Directories) Remove unmanaged children |
| `private_` | 0700/0600 permissions |
| `readonly_` | Remove write permissions |
| `empty_` | Manage even if contents are empty |
| `executable_` | Add executable permission |
| `encrypted_` | Stored encrypted in source |
| `symlink_` | Create symlink (contents = target path) |
| `literal_` | Stop prefix parsing (for files with names that look like prefixes) |

### `exact_` Directories

Normal managed directories only add/update files that chezmoi knows about. Unmanaged files
are left alone. `exact_` directories additionally **remove** anything not in the source
state. This is full directory state management -- useful for directories where stale files
cause problems.

### `create_` Files

`create_` files write initial contents but never overwrite. This is the right primitive for
files that the user will customize after initial setup -- chezmoi provides the starting
point, then steps back.

### `modify_` Scripts

`modify_` scripts receive the current file contents on stdin and output the desired contents
on stdout. This enables surgical modifications to files that chezmoi doesn't fully own --
adding a block to a system config, for instance, without overwriting the rest.

## Dry Run and Diff

chezmoi provides multiple levels of preview before making changes:

### `chezmoi diff`

Shows unified diffs between target state (computed desired) and destination state (actual).
Supports custom diff tools, pager integration, `--reverse` to flip direction, and
`--exclude`/`--include` filters by entry type.

### `chezmoi apply --dry-run --verbose`

The full "what would happen" command. `--dry-run` prevents all changes; `--verbose` shows
what would be done. Scripts are displayed but not executed.

### `chezmoi status`

Quick two-column overview using git-status-style characters:

- First column: differences between last chezmoi state and current actual state (drift)
- Second column: differences between actual state and target state (pending changes)
- Characters: space (none), A (added), D (deleted), M (modified), R (run script)

### `chezmoi verify`

Silent drift check. Exit code 0 = everything matches; exit code 1 = drift detected. Useful
for CI/automation.

### What Makes This Good

The diff operates on computed target state vs actual destination state. Templates are
evaluated, secrets are resolved, encryption is handled -- the diff shows the real content
that would be written, not the template source. This makes the preview accurate and
actionable.

## State Management

chezmoi maintains a persistent BoltDB database tracking:

- **Script execution state**: SHA256 hashes of `run_once_` and `run_onchange_` scripts that
  have completed successfully.
- **Entry state**: The last-written state of managed entries, enabling drift detection.

### Drift Detection

When `chezmoi apply` detects that a target has been modified since chezmoi last wrote it, it
**prompts the user** rather than silently overwriting. Resolution options:

- `chezmoi merge $FILE`: Three-way merge (destination vs source vs target)
- `chezmoi add $FILE`: Accept destination version back into source
- Confirm overwrite in the apply prompt

`chezmoi merge-all` resolves all conflicts in one pass.

### State Commands

- `chezmoi state dump`: Export complete state database
- `chezmoi state delete-bucket --bucket=scriptState`: Reset run_once tracking
- `chezmoi state reset`: Clear all persistent state

## Secret Management

chezmoi handles secrets through two complementary mechanisms:

### Password Manager Integration

Template functions retrieve secrets at evaluation time. The source repo stores only
references:

```toml
# .chezmoi.toml.tmpl (evaluated during chezmoi init)
[data]
    github_token = {{ onepasswordRead "op://Dev/github-token/password" | quote }}
```

This means secrets never appear in the git repository. They're resolved on each machine from
that machine's password manager session.

### File Encryption

For files that must be stored encrypted (SSH keys, credentials files):

- `chezmoi add --encrypt ~/.ssh/id_rsa` stores the file encrypted with `encrypted_` prefix
- Decryption is automatic during apply, diff, edit
- `chezmoi edit` decrypts to a temp file, opens editor, re-encrypts on save

Supported backends: **age** (recommended), **gpg**, **git-crypt**, **transcrypt**.

age configuration:
```toml
encryption = "age"
[age]
    identity = "~/.config/chezmoi/key.txt"
    recipient = "age1..."
```

### What Makes This Good

No other dotfile manager has comparable secret management. Most force you to either commit
secrets in plaintext, maintain a separate secret management workflow, or use `.gitignore` to
exclude sensitive files entirely (losing the benefit of managing them). chezmoi makes secrets
a first-class concept with multiple integration paths.

## Multi-Machine Management

chezmoi's approach is **single branch, single repository, templates for differences**. This
is architecturally superior to the multi-branch approach (one branch per machine) because:

- Branches diverge over time and are painful to keep in sync
- Common changes must be cherry-picked to every branch
- It's easy to push the wrong branch or merge incorrectly

### Mechanisms

**Template conditionals** for inline differences:
```
{{- if eq .chezmoi.hostname "work-laptop" }}
# work-specific config
{{- end }}
```

**`.chezmoi.toml.tmpl`** for machine-specific data, evaluated during `chezmoi init`:
```
{{- $email := promptStringOnce . "email" "Email address" -}}
[data]
    email = {{ $email | quote }}
```

**`.chezmoiignore`** with template support for conditional file exclusion:
```
{{ if ne .chezmoi.os "darwin" }}
.Brewfile
Library/
{{ end }}
```

**`.chezmoiexternal.toml`** with templated URLs for platform-specific downloads:
```toml
[".local/bin/age"]
    type = "archive-file"
    url = "https://github.com/.../age-v1.0.0-{{ .chezmoi.os }}-{{ .chezmoi.arch }}.tar.gz"
```

## Version Control Integration

The source directory is a standard git repository. chezmoi wraps common git operations but
does not require its wrapper -- you can operate on the repo directly.

### Auto-Commit and Auto-Push

```toml
[git]
    autoCommit = true
    autoPush = true
```

When enabled, `chezmoi add`, `chezmoi edit`, `chezmoi remove`, etc. automatically commit and
push. Custom commit message templates are supported.

### `chezmoi update`

Single command: `git pull --autostash --rebase` in the source directory followed by
`chezmoi apply`. Pulls remote changes and applies them in one step.

### Fresh Machine Bootstrap

```sh
sh -c "$(curl -fsLS get.chezmoi.io)" -- init --apply $GITHUB_USERNAME
```

Installs chezmoi, clones the dotfiles repo, evaluates `.chezmoi.toml.tmpl` (prompting for
machine-specific values), and applies everything. One command from bare machine to fully
configured environment.

### `--one-shot` Mode

For ephemeral environments (CI, containers, VMs): installs, applies, then removes chezmoi
and all source state. No residue.

## Edit Workflow

Three modes for modifying dotfiles:

1. **`chezmoi edit $FILE`**: Opens the source state file in your editor. Changes stay in
   source until you run `chezmoi apply`.
2. **`chezmoi edit --apply $FILE`**: Opens editor, applies automatically on exit.
3. **`chezmoi edit --watch $FILE`**: Applies on every save (fsnotify).

For encrypted files, `chezmoi edit` transparently decrypts to a temp directory, opens the
editor, and re-encrypts on save.

Alternative workflows:
- Edit directly in source directory, then `chezmoi apply`
- Edit the destination file directly, then `chezmoi re-add`
- `chezmoi cd` opens a shell in the source directory for direct git operations

## What Makes chezmoi Objectively Good

### Real Files, Not Symlinks

GNU Stow and similar tools use symlinks, meaning your actual dotfiles live in the tool's
directory. If you stop using the tool, every managed file is a dangling symlink. chezmoi
writes real files. Abandon chezmoi and your home directory continues to work unchanged. This
is a meaningful operational safety property.

### Declarative State Model

chezmoi declares what files should exist with what contents and permissions. It computes the
diff and applies the minimum changes. This makes it idempotent -- running `chezmoi apply`
twice produces the same result. It also means the source directory is a complete,
inspectable specification of your desired home directory state.

### Atomic Writes

Files are written to a temp location and renamed. No partial writes, no corrupt intermediate
states. This matters when the file being updated is something like `.bashrc` that's
read on every shell invocation.

### Single Binary, No Dependencies

chezmoi is a statically-linked Go binary. It runs on Linux, macOS, Windows, FreeBSD,
OpenBSD, and Termux. No Python, Ruby, Perl, or Bash required. This makes bootstrap
trivially reliable -- curl a binary and you're running.

### Comprehensive Secret Management

No other dotfile manager integrates with 15+ password managers and supports file encryption
via age/gpg. Most alternatives force secrets into plaintext, out of version control, or into
a separate manual workflow. chezmoi makes secret management native.

### Transparent Source Directory

The source directory structure maps directly to the home directory via a deterministic naming
convention. You can read the source directory and understand exactly what will be applied
without running any commands. If chezmoi disappears tomorrow, the source directory remains a
readable reference for your configuration.

### Drift Detection and Safe Conflict Resolution

chezmoi tracks what it last wrote and detects when external changes have been made. It
prompts rather than overwrites, and offers three-way merge for conflict resolution. Tools
that don't track state either overwrite silently or refuse to apply.

### Externals for Third-Party Resources

`.chezmoiexternal.toml` can pull archives, files, and git repos from URLs -- with caching
and configurable refresh periods. This handles oh-my-zsh, vim plugins, font installations,
and binary tool downloads without scripts or submodules.

### vs. Alternatives

| Concern | Bare Git | GNU Stow | yadm | chezmoi |
|---------|----------|----------|------|---------|
| Secret management | None | None | Limited | 15+ password managers + encryption |
| Templating | None | None | External deps | Built-in (Go templates) |
| Machine differences | Branches | Separate packages | Branches/alt files | Templates + data |
| File type | Real | Symlinks | Real | Real |
| Drift detection | None | None | Limited | Full (state DB) |
| Dry run | `git diff` | None | `git diff` | `diff`, `status`, `verify`, `--dry-run` |
| Portability | Git required | Perl required | Git + Bash | Single binary |
| Atomic writes | No | No (symlinks) | No | Yes |
| Stop using it | Manual cleanup | Replace all symlinks | Manual cleanup | Nothing -- files are real |
