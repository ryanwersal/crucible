const c = require("crucible");

// ---------------------------------------------------------------------------
// Directories
// ---------------------------------------------------------------------------
c.dir("~/.ssh", { mode: 0o700 });
c.dir("~/.vim", { mode: 0o755 });
c.dir("~/.local/bin", { mode: 0o755 });

// ---------------------------------------------------------------------------
// Homebrew packages (macOS only)
// ---------------------------------------------------------------------------
if (c.facts.os.name === "darwin") {
    if (!c.facts.homebrew.available) {
        c.log("Homebrew is not installed — skipping packages");
    } else {
        // System utilities
        c.brew(["vim", "tmux", "zsh", "curl", "wget"]);

        // Core CLI tools
        c.brew(["coreutils", "findutils", "gnu-sed", "grep", "htop", "tree", "jq"]);

        // Modern replacements
        c.brew([
            "ripgrep", "fd", "bat", "eza", "zoxide",
            "dust", "procs", "git-delta", "tokei", "bottom",
            "zellij",
        ]);

        // Development tools
        c.brew(["git", "neovim", "starship", "pyenv", "pipenv", "nvm", "make"]);

        // GUI applications (casks)
        c.brew(["visual-studio-code", "alacritty", "firefox", "chromium", "docker", "sublime-merge"]);

        // Custom tap
        c.brew("ryanwersal/tools/helios");
    }

    // -----------------------------------------------------------------------
    // macOS Defaults
    // -----------------------------------------------------------------------
    c.defaults("com.apple.dock", { autohide: true, tilesize: 16 });
    c.defaults("com.apple.finder", "AppleShowAllExtensions", true);

    // -----------------------------------------------------------------------
    // Dock Layout
    // -----------------------------------------------------------------------
    c.dock({
        apps: [
            "/System/Applications/Launchpad.app",
            "/Applications/Alacritty.app",
            "/Applications/Firefox.app",
            "/Applications/Chromium.app",
            "/System/Applications/System Settings.app",
        ],
        folders: [
            { path: "~/Downloads", view: "grid", display: "folder" },
        ],
    });
}

// ---------------------------------------------------------------------------
// Templated config files
// ---------------------------------------------------------------------------
// Templates automatically have access to .os and .homebrew facts,
// plus built-in functions like env, lookPath, default, etc.
c.file("~/.config/starship.toml", { template: "starship.toml.tmpl" });

// You can still pass extra data — user keys override auto-injected ones
c.file("~/.gitconfig", {
    template: "gitconfig.tmpl",
    data: { email: "ryan@example.com" },
});

// ---------------------------------------------------------------------------
// Development environment
// ---------------------------------------------------------------------------
c.git("~/.oh-my-zsh", {
    url: "https://github.com/ohmyzsh/ohmyzsh.git",
    branch: "master",
});
