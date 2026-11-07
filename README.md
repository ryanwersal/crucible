# crucible

A local system configuration tool. Manages dotfiles, packages, directories, and symlinks by
inspecting current state, computing what should exist, and applying the difference.

## How it works

Write a `crucible.js` in your dotfiles directory:

```javascript
const c = require("crucible");

if (c.facts.os.name === "darwin") {
    c.brew("ripgrep");
    c.brew.cask("firefox");
}

c.file("~/.gitconfig", {
    content: `[user]\n  name = Ryan\n  email = me@example.com`,
    mode: 0o644,
});

c.file("~/.config/fish/config.fish", { source: "fish/config.fish" });
c.symlink("~/.vimrc", { target: "~/.config/nvim/init.vim" });
c.dir("~/.config/fish", { mode: 0o755 });
```

Then run from your dotfiles directory:

```
crucible apply --dry-run   # show what would change
crucible apply             # apply changes
crucible apply --show-plan  # show the detailed plan, then apply changes
```

Applies and dry runs show pending actions by default, without listing unchanged resources.
Use `--show-plan` to include unchanged resources; `--dry-run` previews without applying.
Destructive operations still show their details and require explicit confirmation.

Source is always the current directory. Target is always `$HOME`. No script? The current
directory is mirrored into `$HOME` as-is (backward compatible).

## Apple Software Update

On macOS, manage individual Apple updates separately from Homebrew packages:

```javascript
if (c.facts.os.name === "darwin") {
    c.softwareUpdate.commandLineTools(); // Install if missing; preserve selected Xcode.
    c.softwareUpdate("Safari", { match: "prefix" }); // Latest compatible offer.
}
```

Use `state: "latest"` on the CLT helper to follow updates. Generic selectors can
match exact titles, title prefixes, or exact catalog labels. `state: "present"`
requires a `receipt` package ID. Dry runs scan the catalog without installing;
CLT discovery temporarily uses the same install-on-demand marker as Homebrew.
Only selected labels are installed. Restart-requiring updates need explicit
`restart: true`; run those in a dedicated apply. The existing sudo authentication,
confirmation, and progress handling apply. Apple may require additional owner
authentication. No GUI fallback, automatic Xcode switching, or bulk `--all` update
is performed. Run `crucible reference` for all options and failure behavior.

## Install

```
go install github.com/ryanwersal/crucible/cmd/crucible@latest
```

## License

GPL-3.0. See [LICENSE](LICENSE).
