const c = require("crucible");
const facts = require("crucible/facts");

if (!facts.homebrew.available) {
    c.log("Homebrew is not installed — skipping Firefox");
} else {
    c.brew("firefox", { type: "cask" });
}
