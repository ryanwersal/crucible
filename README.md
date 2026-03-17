# crucible

A local system configuration tool. Manages dotfiles, packages, directories, and symlinks by
inspecting current state, computing what should exist, and applying the difference.

## How it works

Write a `crucible.js` in your dotfiles directory:

```javascript
const c = require("crucible");
const facts = require("crucible/facts");

if (facts.os.name === "darwin") {
    c.brew("ripgrep");
    c.brew("firefox", { type: "cask" });
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
```

Source is always the current directory. Target is always `$HOME`. No script? The current
directory is mirrored into `$HOME` as-is (backward compatible).

## Install

```
go install github.com/ryanwersal/crucible/cmd/crucible@latest
```

## License

GPL-3.0. See [LICENSE](LICENSE).
