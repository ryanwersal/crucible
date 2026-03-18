const c = require("crucible");
const facts = require("crucible/facts");

if (!facts.homebrew.available) {
    c.log("Homebrew is not installed — skipping packages");
} else {
    c.brew("ryanwersal/tools/helios");
    c.brew("alacritty");
}
