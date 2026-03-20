const c = require("crucible");

if (!c.facts.homebrew.available) {
    c.log("Homebrew is not installed — skipping packages");
} else {
    c.brew("ryanwersal/tools/helios");
    c.brew("alacritty");
}
