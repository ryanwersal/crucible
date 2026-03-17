# Crucible: Approach

Crucible is a local system configuration tool. It manages the state of a single machine --
dotfiles, packages, services, directories, permissions -- by inspecting what exists,
computing what should exist, and applying the difference.

The design draws from two tools: pyinfra's fact/operation architecture and chezmoi's
declarative dotfile management. It takes the parts that are objectively good from each and
leaves behind the parts that aren't.

## What We Take from pyinfra

### The Fact/Operation Split

This is the core architectural idea. Every piece of system management is divided into:

- **Facts**: Read-only queries that inspect current system state. A fact runs a command (or
  calls an API, reads a file, checks a path) and returns structured data. Facts never modify
  anything.
- **Operations**: Functions that consume facts, compare current state to desired state, and
  produce the commands or actions needed to close the gap. When current matches desired, an
  operation produces nothing.

This separation is what makes everything else work. Dry runs are trustworthy because they
query real state and generate real action lists -- the only thing skipped is execution.
Idempotency is natural because operations only produce actions when state diverges.
Composability follows because operations are just functions that yield actions.

### Facts as the Foundation for Dry Run

pyinfra's dry run works because facts query real system state. The dry run doesn't guess or
simulate -- it reads the machine, computes the delta, and reports what would change. This is
the standard crucible must meet. A dry run that lies is worse than no dry run at all.

### Operations as Pure Transforms

An operation is a function: `(current_state, desired_state) → []action`. It reads facts,
compares, and yields. This is a pure transform from state to actions. The operation doesn't
execute anything -- it produces a plan. Execution is a separate phase.

### Cross-Platform Fact Resilience

pyinfra facts try multiple command variants (Linux stat → BSD stat → ls fallback) so the
same operation works across platforms. Crucible should follow this pattern where applicable
-- particularly for macOS and Linux differences, since those are the likely target platforms.

## What We Leave Behind from pyinfra

### Deploys

pyinfra's `@deploy` decorator creates named, composable units of operations. This is an
abstraction layer over operations that adds naming, data defaults, and packaging concerns.
We don't need it. Operations compose naturally as function calls. If someone wants to group
operations, they write a function. Python's module system handles packaging. An extra
abstraction here adds ceremony without capability.

### Remote Execution

pyinfra's SSH/Docker/Podman connectors and parallel-across-hosts execution model are
powerful but irrelevant for now. Crucible operates on the local machine only. The
architecture should not preclude remote execution later, but we won't build abstractions for
it upfront. No connector interface, no host inventory, no parallel execution model. Just
local state, local facts, local operations.

### Inventory and Host Groups

With only local execution, there's no inventory to manage. Configuration data that would
live in pyinfra's `group_data/` lives in crucible's config file instead.

## What We Take from chezmoi

### Declarative State Model

chezmoi's core idea is right: declare what the home directory should look like, compute the
diff against reality, apply the minimum changes. Files, directories, symlinks, and
permissions are all expressed as desired state, not as imperative scripts.

Crucible adopts this model for dotfile management. The source directory holds config files
and a `crucible.js` script that programmatically declares desired system state. The tool
executes the script, collects declarations, diffs against actual state, and applies.

### Atomic Writes

chezmoi writes files to a temp location and renames them into place. This prevents partial
writes -- a crash during apply never leaves a managed file half-written. Crucible does the
same. This is especially important for files read on every shell invocation (`.bashrc`,
`.zshrc`) or by daemons that might reload mid-write.

### Template System for Machine Differences

A single source tree with templates that evaluate differently per machine is the right
approach. Branches-per-machine diverges. Separate directories-per-machine duplicates.

Crucible provides two template mechanisms:

1. **JavaScript template literals** (primary) -- users write inline content in `crucible.js`
   with `${}` interpolation and full access to system facts. This is just JavaScript.
2. **Go `text/template`** (secondary) -- for large config files stored in the source dir.
   The `template` option in `c.file()` renders a `.tmpl` file with user-provided data.

### Secret Management

chezmoi's integration with password managers and file encryption is a genuine differentiator
over other dotfile tools. Crucible should support this pattern: template functions that
resolve secrets at evaluation time, so the source repo contains references rather than
values. Encryption for files that must be stored encrypted. The specific integrations can
grow over time, but the architecture supports them from the start.

### Drift Detection

chezmoi tracks what it last wrote and detects external modifications. This is valuable --
applying blindly over user edits is a data loss vector. Crucible tracks managed file state
and warns (or prompts) when the destination has been modified outside of crucible.

### External Resources

chezmoi's `.chezmoiexternal.toml` pulls archives, binaries, and git repos from URLs with
caching. This handles plugin managers, font installations, and tool downloads without
scripts. Crucible supports a similar mechanism for declaring external dependencies.

## What We Leave Behind from chezmoi

### The CLI Edit Workflow

chezmoi's `chezmoi edit`, `chezmoi edit --apply`, `chezmoi edit --watch` workflow treats
the source directory as something mediated by the tool. You're supposed to go through
chezmoi to modify your files.

This is wrong. The source directory is a directory of files. You edit them with your editor,
your IDE, whatever you use for code. There is no special edit command. There is no wrapper.
`chezmoi add` to bring a file under management and `chezmoi edit` to modify it creates an
unnecessary indirection -- you should just edit the file.

Crucible's source directory is opened in your editor like any project. You modify files
directly. You run `crucible apply` (or dry-run, diff, etc.) when you want to push state to
the system. The tool never inserts itself between you and your files.

### Source Filename Encoding

chezmoi encodes file metadata in filenames: `private_dot_ssh/encrypted_private_id_rsa`
means `.ssh/id_rsa` with 0600 permissions, encrypted. This is clever and self-describing,
but it means your source tree doesn't look like your home directory. The mental mapping from
`dot_bashrc` to `.bashrc` is trivial; the mapping from
`exact_dot_config/private_dot_gnupg/encrypted_private_gpg-keys` is not.

Crucible keeps source files named as they are in the target. Metadata -- permissions,
encryption, template status, ownership -- is declared in the `crucible.js` script, not
encoded in filenames. The source tree is a readable mirror of the target structure.

### `chezmoi cd` / `chezmoi git`

chezmoi wraps git operations and provides a shell command to enter the source directory.
These exist because chezmoi's source directory is an implementation detail that the user
doesn't normally interact with directly. Since crucible's source directory *is* the project
directory -- a normal folder you open in your editor and commit with git -- these wrappers
are unnecessary.

## Crucible's Architecture

### Two-Phase Execution

Like pyinfra, crucible separates planning from execution:

1. **Plan**: Execute `crucible.js` to collect declarations, gather facts (real system state),
   compute the diff between declarations and reality, produce an action list.
2. **Execute**: Apply the action list to the system.

`crucible apply` runs both phases. `crucible plan` (or `--dry-run`) runs only phase 1 and
displays the action list. The plan phase never modifies the system.

### JavaScript Scripting Engine

Crucible uses a JavaScript scripting engine (Goja) to let users programmatically declare
desired system state. Users write `crucible.js` files that use CommonJS `require()` to load
crucible modules:

```javascript
const c = require("crucible");
const facts = require("crucible/facts");

// Conditional logic — the whole point of scripting
if (facts.os.name === "darwin") {
    c.brew("coreutils");
    c.brew("firefox", { type: "cask" });
}

// Files — inline content via JS template literals
c.file("~/.gitconfig", {
    content: `[user]\n  name = Ryan\n  email = ${facts.os.hostname === "work" ? "work@co.com" : "me@home.com"}`,
    mode: 0o644,
});

// Files — from source directory
c.file("~/.config/fish/config.fish", {
    source: "fish/config.fish",
    mode: 0o644,
});

// Files — Go templates for large configs
c.file("~/.config/starship.toml", {
    template: "starship.toml.tmpl",
    data: { prompt: ">" },
});

// Symlinks
c.symlink("~/.vimrc", { target: "~/.config/nvim/init.vim" });

// Directories
c.dir("~/.config/fish", { mode: 0o755 });

// Require other scripts for organization
require("./work-setup");
```

Five declaration functions: `file`, `dir`, `symlink`, `brew`, `log`. No `exec()` --
declarations only, keeping dry runs trustworthy.

Scripts declare desired state; they do not perform side effects. The engine diffs
declarations against reality and applies the difference -- the same plan/apply pipeline,
with a scripting front-end.

### Facts

Facts are Go interfaces that query local system state:

- File existence, content hash, permissions, ownership
- Package installation status (Homebrew, apt, etc.)
- Directory contents
- Symlink targets
- OS, architecture, hostname
- Running services
- Arbitrary command output

Facts are collected during the plan phase and cached for the duration of that phase. They
return structured data, not raw strings. A file fact returns a struct with hash, mode, uid,
gid, mtime -- not the output of `stat`.

Facts are exposed to scripts via the `crucible/facts` module. Scripts access facts like
`facts.os.name`, `facts.homebrew.formulae`, `facts.file(path)`, etc.

### Operations

Operations are functions that take desired state and current facts, and return a list of
actions. They don't execute anything. The action types are:

- Write file (with content, permissions, ownership)
- Create directory (with permissions, ownership)
- Create symlink
- Delete path
- Run command (package install, service restart, etc.)

Operations compose by calling each other and concatenating action lists. There is no special
composition abstraction -- they are functions that return data.

### Source Directory

The source directory is the current working directory when `crucible` is invoked. Run
crucible from your dotfiles repo. The `crucible.js` entry point drives what gets managed.
Source files referenced by scripts live alongside the script. The target is always `$HOME`.

```
~/dotfiles/           ← run `crucible apply` from here
├── crucible.js               # Script declaring desired state
├── work-setup.js             # Optional sub-module for organization
├── fish/
│   └── config.fish           # Referenced via { source: "fish/config.fish" }
├── starship.toml.tmpl        # Go template for large configs
└── .ssh/
    └── config.tmpl           # Go template referenced via { template: ".ssh/config.tmpl" }
```

If no `crucible.js` exists, the engine falls back to walk-based file sync -- mirroring the
current directory into `$HOME`. This preserves backward compatibility.

### What Gets Applied

The apply process:

1. Check for `crucible.js` in the source directory.
   - If absent, fall back to walk-based file sync (mirror source → target).
   - If present, execute the script-driven pipeline:

2. Pre-collect expensive facts concurrently (OS info, Homebrew state).

3. Execute `crucible.js`:
   - JS calls to `c.file()`, `c.brew()`, etc. accumulate declarations.
   - `require()` loads sub-modules for organization.

4. Resolve content:
   - Declarations with `source` references → read file from source dir.
   - Declarations with `template` references → render Go template.

5. Diff declarations against system state:
   - Each declaration maps to a Desired* struct.
   - Each Diff function compares desired vs actual (via fact collectors).
   - Actions are produced only for differences.

6. Display the action list (plan mode) or execute it (apply mode).

### CLI

```
crucible apply             # Apply configuration to the system
crucible apply --dry-run   # Show what would change without making changes
crucible diff              # Show unified diffs of file content changes
crucible status            # Quick overview of drift and pending changes
crucible verify            # Silent check, exit code 0 = clean, 1 = drift
```

Run from your dotfiles directory. There is no `--source` or `--target` — source is always
the current directory, target is always `$HOME`. There is no `crucible edit`, `crucible
add`, or `crucible cd`. You edit files in the source directory with your tools. You add
files by putting them in the source directory. You manage git by being in the git repo.

### Extensibility

The scripting engine provides the primary extensibility mechanism -- users write JavaScript
for custom logic, conditional configuration, and organizational structure. For system-level
extensions (new fact collectors, new action types), Go code is added directly to the
project. There is no plugin system, registration mechanism, or dynamic loading -- you add
code to the project and it's available.

This is appropriate for a tool where the user is the developer. If crucible grows to need
third-party extensibility, that can be designed then. Premature plugin architecture is
wasted abstraction.
