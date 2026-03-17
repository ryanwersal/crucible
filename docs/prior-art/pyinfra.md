# pyinfra: Behavioral Analysis

pyinfra is an agentless infrastructure automation tool written in Python. It connects to
remote (or local) hosts, inspects their current state, computes the delta to the desired
state, and executes only the commands needed to close that gap. Its architecture is built
around a clean separation between reading state (facts) and producing changes (operations).

## The Fact/Operation Split

This is pyinfra's defining architectural decision. Every piece of automation is divided into
two concerns:

- **Facts** are read-only queries that collect the current state of a host.
- **Operations** consume facts, compare current state to desired state, and yield only the
  commands needed to reconcile the difference.

This separation is what makes dry runs trustworthy, idempotency natural, and the entire
system composable.

## Facts

A fact is a Python class inheriting from `FactBase[T]` that defines two methods:

- **`command()`** returns the shell command to run on the target host (e.g., `stat`, `dpkg-query`, `systemctl is-active`).
- **`process(output)`** receives stdout as a list of lines and transforms it into structured Python data.

```python
class File(FactBase[Union[FileDict, Literal[False], None]]):
    type = "file"

    def command(self, path):
        return make_formatted_string_command(
            "! (test -e {0} || test -L {0} ) || "
            "( {linux_stat_command} {0} 2> /dev/null || "
            "{bsd_stat_command} {0} || {ls_command} {0} )",
            path,
        )

    def process(self, output):
        # Parses stat or ls output into a FileDict with user, group, mode, etc.
        # Returns None if path doesn't exist, False if it exists but is wrong type
```

### What Makes Facts Good

**They query real hosts.** Facts SSH into the target and run actual commands. There is no
local model of what the remote state "should" be -- facts report what it *is*. This is the
foundation for trustworthy dry runs: the diff between desired and actual state is computed
from real data.

**They are cross-platform by construction.** The `File` fact tries Linux `stat`, then BSD
`stat`, then falls back to `ls -ld`. Hash facts (`Sha1File`, `Md5File`, `Sha256File`) try
multiple hash binaries. This fallback cascade means the same deploy code works across
distributions without the user writing platform conditionals.

**They have well-defined failure semantics.** A fact can declare `requires_command` -- if the
command isn't present on the host, the fact returns its `default()` value rather than
failing. Facts also handle non-existent sudo/su users gracefully.

**They are collected in parallel.** pyinfra uses gevent greenlets to gather facts from all
hosts concurrently, so fact collection scales with host count.

### Fact Caching and Limitations

Facts collected during the prepare phase are not re-collected during execution. This is by
design -- the prepare phase is a snapshot. The documentation is explicit: only use immutable
facts (OS, architecture) in deploy conditionals unless you are certain they won't change
during the deploy. For mutable state, pyinfra provides the `_if` runtime argument (see
Operations below).

### Fact Coverage

pyinfra ships 50+ fact modules covering: package managers (apt, apk, brew, dnf, pip, yum,
pacman, zypper), services (systemd, openrc, launchd, runit, sysvinit, upstart), containers
(docker, podman, lxd), databases (mysql, postgres), files, git, hardware, iptables, gpg,
selinux, zfs, and more.

## Operations

An operation is a Python generator function decorated with `@operation()`. It:

1. Reads current state via `host.get_fact(FactClass, ...)`
2. Compares current to desired state
3. Yields commands only when changes are needed
4. Calls `host.noop()` when the system already matches desired state

### The Canonical Pattern

Every idempotent operation follows the same structure:

```python
@operation()
def file(path, present=True, user=None, group=None, mode=None, touch=False, ...):
    info = host.get_fact(File, path=path)

    if info is False:  # exists but wrong type
        yield from _raise_or_remove_invalid_path(...)
        info = None

    if not present:
        if info:
            yield StringCommand("rm", "-f", QuoteString(path))
        else:
            host.noop("file does not exist")
        return

    if info is None:  # doesn't exist, create it
        yield StringCommand("touch", QuoteString(path))
        if mode:
            yield file_utils.chmod(path, mode)
        if user or group:
            yield file_utils.chown(path, user, group)
    else:  # exists, check attributes
        if mode and info["mode"] != mode:
            yield file_utils.chmod(path, mode)
        if (user and info["user"] != user) or (group and info["group"] != group):
            yield file_utils.chown(path, user, group)
        else:
            host.noop("file already exists")
```

This pattern -- query fact, branch on current state, yield minimal commands -- is what makes
pyinfra operations naturally idempotent. Running the same deploy twice produces zero commands
on the second run.

### Command Types

Operations can yield four types of commands:

- **`StringCommand`**: Shell commands to execute on the host
- **`FileUploadCommand`**: Upload a local file to the remote host
- **`FileDownloadCommand`**: Download a remote file locally
- **`FunctionCommand`**: Execute a Python function during deployment

### Idempotency Strategies by Domain

Different operation categories achieve idempotency through different fact comparisons:

| Domain | Fact Used | Comparison |
|--------|-----------|------------|
| Files (content) | `Sha1File`, `Md5File`, `Sha256File` | Checksum local vs remote |
| Files (attributes) | `File` (stat output) | User, group, mode comparison |
| Packages | `DebPackages`, `RpmPackages`, etc. | Installed package set vs desired set |
| Services | Service status facts | Running/enabled state vs desired state |
| Users/Groups | User/group facts | Existence and attribute comparison |
| Directories | `Directory` fact | Existence and permission comparison |

The `put()` operation is a good example of the checksum approach:

```python
def _file_equal(local_path, remote_path):
    for fact, get_sum in [
        (Sha1File, get_file_sha1),
        (Md5File, get_file_md5),
        (Sha256File, get_file_sha256),
    ]:
        remote_sum = host.get_fact(fact, path=remote_path)
        if remote_sum:
            local_sum = get_sum(local_path)
            return local_sum == remote_sum
    return False
```

It tries SHA1 first, falls back to MD5, then SHA256 -- using whichever hash utility the
remote host has available.

### Explicitly Non-Idempotent Operations

Some operations are marked `@operation(is_idempotent=False)` because they have no meaningful
state to compare:

- `server.shell` -- executes arbitrary commands
- `server.script` -- uploads and runs scripts
- `server.reboot` -- reboots the host
- `files.rsync` -- delegates to the rsync binary

### Runtime Conditionals with `_if`

Since facts are snapshots from before execution, operations that depend on the outcome of
earlier operations need runtime evaluation:

```python
create_user = server.user(user="myuser")
server.shell(
    commands=["bootstrap-script.sh"],
    _if=create_user.did_change,  # Only runs if user was actually created
)
```

### Operation Composition

Operations compose via `yield from`:

```python
yield from files.file._inner(path="/some/file", mode="644")
```

The `template()` operation renders Jinja2 then delegates to `put._inner()`. The `sync()`
operation composes `directory._inner()`, `link._inner()`, `put._inner()`, and
`file._inner()`. This enables building higher-level operations from primitives without
duplicating logic.

## Dry Run

pyinfra's execution is a two-phase pipeline:

1. **Prepare**: All deploy code runs, facts are collected from real hosts, operations compare
   current vs desired state, commands are generated. Nothing is modified.
2. **Execute**: Generated commands are sent to hosts and run.

`pyinfra --dry` stops after phase 1. The user sees which operations would run, how many
commands each would execute, and (with `-v`) the actual command text.

### Why This Is Good

The dry run is trustworthy because it operates on **real facts from real hosts**. It doesn't
guess at state or use a local model -- it SSHes into targets, runs `stat`, `dpkg-query`,
`systemctl`, etc., and uses the actual responses to compute what would change.

Additional inspection tools:

- `--debug-facts`: Print all collected facts and exit
- `--debug-operations`: Print generated operations and exit
- `-v`: Print noop information (what's already in desired state)
- `DIFF` config: File content operations generate color diffs showing exact changes

### Known Limitation

Since all deploy code runs before any execution, conditionals based on facts that *would
change* during deployment evaluate against pre-deployment state. This is documented
explicitly. The `_if` runtime argument exists for cases where execution-time evaluation is
needed.

## Execution Model

### Five-Stage Pipeline

1. **Setup**: Read inventory files, parse data configuration
2. **Connect**: Establish connections to all target hosts
3. **Prepare**: Run deploy code, collect facts, generate command lists
4. **Execute**: Run operations in order, each across all hosts in parallel
5. **Disconnect**: Close connections, cleanup

### Parallelism Model

Operations are sequential but each operation executes on all hosts in parallel:

```
Operation 1 → [Host A, Host B, Host C] (concurrent)
     ↓ (barrier)
Operation 2 → [Host A, Host B, Host C] (concurrent)
     ↓ (barrier)
Operation 3 → ...
```

This is controllable per-operation:

- `_serial=True`: Force host-by-host execution
- `_parallel=N`: Execute in batches of N hosts
- `_run_once=True`: Execute only on the first host

### Connectors

Connectors abstract how pyinfra reaches hosts:

| Connector | How It Works |
|-----------|-------------|
| `@ssh` (default) | Paramiko SSH; key/password/agent auth; SFTP or SCP file transfer |
| `@local` | Subprocess execution on the local machine |
| `@docker` | Create/modify Docker containers; image or container mode |
| `@dockerssh` | Docker containers on remote hosts via SSH |
| `@podman` / `@podmanssh` | Podman equivalents |
| `@chroot` | Operate within a chroot environment |
| `@terraform` | Read SSH targets from Terraform output |
| `@vagrant` | Get targets from Vagrant |

### Retry Logic

Built into the operation runner:

- `_retries=N`: Retry failed operations up to N times
- `_retry_delay=5`: Seconds between retries
- `_retry_until=callable`: Custom function to determine retry success based on stdout/stderr

## Inventory and Data

### Structure

```python
# inventory.py
web_servers = ["web-1.net", "web-2.net"]
db_servers = [
    ("db-1.net", {"install_postgres": True}),
    ("db-2.net", {"install_postgres": True}),
]
```

Each list variable becomes a group. An `all` group is auto-generated. Host-specific data
attaches via tuples.

### Data Priority (Highest to Lowest)

1. CLI override data
2. Host-level data (from inventory tuples)
3. Group data (from `group_data/<group_name>.py`)
4. `all` group data (from `group_data/all.py`)

### Data as Configuration

SSH settings, sudo defaults, and other global arguments can be set as data:

```python
# group_data/web_servers.py
ssh_user = "ubuntu"
ssh_key = "~/.ssh/deploy_key"
_sudo = True
```

## Deploy Composition

### Minimal Deploy

```python
# deploy.py
from pyinfra.operations import apt, files, server

apt.packages(packages=["nginx"], update=True, _sudo=True)
files.template(src="templates/nginx.conf.j2", dest="/etc/nginx/nginx.conf")
server.service(service="nginx", running=True, enabled=True, _sudo=True)
```

### Reusable Deploys

The `@deploy` decorator creates composable, distributable units:

```python
from pyinfra.api import deploy

@deploy("Install MariaDB", data_defaults={"mariadb_version": "10.6"})
def install_mariadb():
    apt.packages(packages=[f"mariadb-server={host.data.mariadb_version}"])
```

These can be packaged as PyPI modules. Consumers use them like any operation:

```python
from my_deploys import install_mariadb
install_mariadb(_sudo=True)
```

### Includes

```python
from pyinfra import local
local.include("tasks/install_nginx.py")
```

### Templating

`files.template()` renders Jinja2 templates with automatic access to `host`, `state`, and
`inventory` objects plus arbitrary keyword arguments.

## What Makes pyinfra Objectively Good

**The fact/operation split creates correct-by-construction dry runs.** Most tools either
don't have dry run or simulate it with heuristics. pyinfra's dry run queries real host state
and generates real command lists -- the only thing it skips is execution. This makes the dry
run output trustworthy for planning and review.

**Pure Python means no DSL learning curve and full ecosystem access.** Deploy code is regular
Python with loops, conditionals, functions, classes, imports, type hints, IDE autocomplete,
linting, and debugging. You can use `requests`, `boto3`, or any library directly in deploy
code without shelling out or writing plugins.

**Agentless with minimal target requirements.** Targets need only a POSIX shell and SSH. No
Python, no agent daemon, no pre-configuration. This makes pyinfra usable against minimal
containers, embedded systems, and locked-down hosts.

**Performance.** pyinfra uses gevent for concurrent fact collection and operation execution
across hosts. Benchmarks show it running up to 10x faster than Ansible on equivalent
workloads, with overhead closer to raw Fabric SSH execution.

**Extensibility is first-class.** Custom facts, operations, connectors, and deploys all
follow simple Python patterns. Reusable deploys distribute via PyPI as standard packages.

**Cross-platform fact resilience.** Facts try multiple command variants (Linux stat → BSD stat
→ ls; sha256sum → shasum → sha256) so deploy code works across distributions without
platform conditionals.
