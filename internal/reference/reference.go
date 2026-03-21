package reference

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/resource"
	"github.com/ryanwersal/crucible/internal/script"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"github.com/spf13/cobra"
)

// Build assembles the complete reference documentation from the given root command.
// The registry must be provided so that type names and lists are populated.
func Build(root *cobra.Command, reg *resource.Registry) string {
	var b strings.Builder
	writeOverview(&b)
	writeCLICommands(&b, root)
	writeJSAPI(&b)
	writeFacts(&b)
	writeTemplateFuncs(&b)
	writeTemplateData(&b)
	writeDeclTypes(&b, reg)
	writeActionTypes(&b, reg)
	return b.String()
}

func writeOverview(b *strings.Builder) {
	b.WriteString("# Crucible Reference\n\n")
	b.WriteString("Crucible is a declarative dotfile and system configuration manager. ")
	b.WriteString("You write JavaScript configuration scripts that declare desired state, ")
	b.WriteString("and crucible converges your system to match.\n\n")
	b.WriteString("Crucible looks for a crucible.js script in the current working directory. ")
	b.WriteString("Run crucible from the directory containing your crucible.js, or use ")
	b.WriteString("`--file` to specify a script elsewhere.\n\n")
	b.WriteString("Run `crucible apply` to apply changes, or `crucible apply --dry-run` to preview them.\n\n")
}

func writeCLICommands(b *strings.Builder, root *cobra.Command) {
	b.WriteString("# CLI Commands\n\n")
	fmt.Fprintf(b, "## %s\n\n", root.Use)
	if root.Long != "" {
		fmt.Fprintf(b, "%s\n\n", root.Long)
	}
	writeFlags(b, root)

	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}
		fmt.Fprintf(b, "## %s %s\n\n", root.Use, cmd.Use)
		if cmd.Long != "" {
			fmt.Fprintf(b, "%s\n\n", cmd.Long)
		} else if cmd.Short != "" {
			fmt.Fprintf(b, "%s\n\n", cmd.Short)
		}
		writeFlags(b, cmd)
	}
}

func writeFlags(b *strings.Builder, cmd *cobra.Command) {
	flags := cmd.NonInheritedFlags()
	if usage := flags.FlagUsages(); usage != "" {
		b.WriteString("Flags:\n")
		b.WriteString(usage)
		b.WriteString("\n")
	}
	inherited := cmd.InheritedFlags()
	if usage := inherited.FlagUsages(); usage != "" {
		b.WriteString("Global flags:\n")
		b.WriteString(usage)
		b.WriteString("\n")
	}
}

const jsAPI = `# JavaScript API

Scripts import the crucible module:

  var c = require("crucible");

Each function on ` + "`c`" + ` declares desired state. System facts are available via ` + "`c.facts`" + `.
All path arguments accept ` + "`~`" + ` as a prefix for the target (home) directory.
Any resource function accepts ` + "`{ state: \"absent\" }`" + ` as its options to remove the resource.

## c.file(path, options?)

Declare a managed file.

Options (mutually exclusive content sources):
- content: string — inline file content
- source: string — relative path to a file in the source directory (copied verbatim)
- template: string, data: object — relative path to a Go template, rendered with data
- mode: number — file permissions (default: 0o644)

Examples:
  c.file("~/.gitconfig", { content: "[user]\n  name = Me" })
  c.file("~/.config/fish/config.fish", { source: "fish/config.fish" })
  c.file("~/.config/starship.toml", { template: "starship.toml.tmpl", data: { theme: "dark" } })

## c.dir(path, options?)

Declare a managed directory.

Options:
- mode: number — directory permissions (default: 0o755)

Example:
  c.dir("~/.config/fish", { mode: 0o755 })

## c.symlink(path, options)

Declare a managed symlink.

Options:
- target: string — the symlink target path (required unless state: "absent")

Example:
  c.symlink("~/.vimrc", { target: "~/.config/nvim/init.vim" })

## c.brew(packages, options?)

Declare Homebrew packages. Accepts a single string or array of strings.
Supports tap syntax for third-party formulae (e.g. "user/tap/formula").

Examples:
  c.brew("ripgrep")
  c.brew(["ripgrep", "fd", "bat"])
  c.brew("ripgrep", { state: "absent" })

## c.defaults(domain, key, value) / c.defaults(domain, object)

Declare macOS defaults. Two calling forms:
- 3-arg: domain, key, value (bool, number, or string)
- Object: domain, { key: value, ... } for multiple keys at once

Examples:
  c.defaults("com.apple.dock", "autohide", true)
  c.defaults("com.apple.dock", { autohide: true, tilesize: 36 })
  c.defaults("com.apple.dock", "autohide", { state: "absent" })

## c.dock(options)

Declare the macOS Dock layout.

Options:
- apps: string[] — ordered list of application paths
- folders: object[] — each with path, view ("grid"|"list"|"fan"|"auto"), display ("folder"|"stack")

Example:
  c.dock({
    apps: ["/Applications/Safari.app", "/Applications/Terminal.app"],
    folders: [{ path: "~/Downloads", view: "grid", display: "folder" }]
  })

## c.git(path, options)

Declare a git repository clone.

Options:
- url: string — the remote URL to clone
- branch: string — branch to check out (optional)

Example:
  c.git("~/src/project", { url: "https://github.com/user/repo.git", branch: "main" })

## c.font(source, options?)

Declare font files to install. Accepts a single string or array of strings (relative paths in source dir).

Options:
- dest: string — custom destination directory (default: ~/Library/Fonts)

Examples:
  c.font("fonts/Mono.ttf")
  c.font(["fonts/Mono.ttf", "fonts/Sans.otf"])
  c.font("fonts/Mono.ttf", { dest: "~/Library/Fonts" })

## c.mas(id, name?) / c.mas(array)

Declare Mac App Store apps to install. Two calling forms:
- Single: numeric App Store ID and optional name string
- Array: array of objects with id (number) and name (string) fields

Examples:
  c.mas(497799835, "Xcode")
  c.mas([{ id: 497799835, name: "Xcode" }, { id: 409183694, name: "Keynote" }])

## c.mise(tool, version) / c.mise(tool, options)

Declare globally installed mise tools.

Examples:
  c.mise("python", "3.12")
  c.mise("node", "22")
  c.mise("python", { state: "absent" })

## c.shell(path, options?)

Declare the desired login shell for the current user.

Options:
- user: string — username (defaults to current user)

Examples:
  c.shell("/opt/homebrew/bin/zsh")
  c.shell("/opt/homebrew/bin/zsh", { user: "ryan" })

## c.keyRemap(options)

Declare keyboard modifier key remappings for all keyboards via hidutil.
Object keys are "from" keys, values are "to" keys.
A LaunchAgent plist is written for persistence across reboots.

Supported key names: capsLock, control, leftControl, rightControl,
leftShift, rightShift, leftOption, rightOption, leftCommand, rightCommand, fn.

Note: "control" is an alias for "leftControl".

Examples:
  c.keyRemap({ capsLock: "control" })
  c.keyRemap({ capsLock: "control", control: "capsLock" })
  c.keyRemap({ state: "absent" })

## c.log(message)

Log a message during script evaluation. Does not create a declaration.

Example:
  c.log("configuring development tools...")
`

func writeJSAPI(b *strings.Builder) {
	b.WriteString(jsAPI)
	b.WriteString("\n")
}

const factsDoc = `# Facts

System state is available via ` + "`c.facts`" + ` (no separate import needed):

  var c = require("crucible");
  c.log(c.facts.os.name);               // "darwin"
  if (c.facts.file("~/.bashrc").exists) { ... }

Facts are collected lazily and cached for the duration of a plan phase.

## c.facts.os

Pre-collected OS information object:
- name: string — operating system (e.g. "darwin", "linux")
- arch: string — architecture (e.g. "arm64", "amd64")
- hostname: string — machine hostname

## c.facts.homebrew

Pre-collected Homebrew state object:
- available: boolean — whether brew is installed
- formulae: string[] — installed formula names
- casks: string[] — installed cask names

## c.facts.file(path)

Returns file information for the given path:
- exists: boolean
- hash: string — SHA256 hex digest (empty if directory or missing)
- mode: number — file permissions
- size: number — file size in bytes
- isDir: boolean
- isLink: boolean

## c.facts.dir(path)

Returns directory information for the given path:
- exists: boolean
- mode: number — directory permissions
- children: string[] — names of entries in the directory
`

func writeFacts(b *strings.Builder) {
	b.WriteString(factsDoc)
	b.WriteString("\n")
}

var templateFuncDescriptions = map[string]string{
	"env":       "env(name) — returns the value of the environment variable",
	"lookPath":  "lookPath(name) — returns the absolute path to an executable, or empty string",
	"default":   "default(fallback, value) — returns value if non-nil and non-empty, otherwise fallback",
	"hasPrefix": "hasPrefix(s, prefix) — reports whether s begins with prefix",
	"hasSuffix": "hasSuffix(s, suffix) — reports whether s ends with suffix",
	"contains":  "contains(s, substr) — reports whether substr is within s",
	"replace":   "replace(old, new, s) — replaces all occurrences of old with new in s",
	"lower":     "lower(s) — converts s to lowercase",
	"upper":     "upper(s) — converts s to uppercase",
	"trimSpace": "trimSpace(s) — removes leading and trailing whitespace from s",
	"join":      "join(sep, elems) — joins string slice elems with separator sep",
}

func writeTemplateFuncs(b *strings.Builder) {
	b.WriteString("# Template Functions\n\n")
	b.WriteString("Available in Go templates rendered via the `template` option on `c.file()`.\n\n")

	for _, name := range script.TemplateFuncNames() {
		desc, ok := templateFuncDescriptions[name]
		if !ok {
			desc = name
		}
		fmt.Fprintf(b, "- %s\n", desc)
	}
	b.WriteString("\n")
}

const templateDataDoc = `# Template Data

Templates rendered via ` + "`c.file()`" + ` with the ` + "`template`" + ` option receive these auto-injected variables:

- .os.name — operating system (e.g. "darwin")
- .os.arch — architecture (e.g. "arm64")
- .os.hostname — machine hostname
- .homebrew.available — whether Homebrew is installed (boolean)
- .homebrew.formulae — installed formula names (string slice)
- .homebrew.casks — installed cask names (string slice)

User-supplied data from the ` + "`data`" + ` option is merged at the top level.
For example, ` + "`{ data: { theme: \"dark\" } }`" + ` makes ` + "`.theme`" + ` available in the template.
`

func writeTemplateData(b *strings.Builder) {
	b.WriteString(templateDataDoc)
	b.WriteString("\n")
}

var declTypeDescriptions = map[decl.Type]string{
	decl.File:     "Managed file — created or updated with specified content, source, or template",
	decl.Dir:      "Managed directory — created with specified permissions",
	decl.Symlink:  "Managed symlink — points to a target path",
	decl.Package:  "Homebrew package — installed or uninstalled via brew",
	decl.Defaults: "macOS defaults key — set or deleted in a preference domain",
	decl.Dock:     "macOS Dock layout — apps and folders in the Dock",
	decl.GitRepo:  "Git repository — cloned or updated at a path",
	decl.Font:     "Font file — installed to the fonts directory",
	decl.MiseTool: "Mise tool — globally installed version manager tool",
	decl.Shell:    "Login shell — sets the user's default shell",
	decl.MasApp:   "Mac App Store app — installed via mas",
	decl.KeyRemap: "Keyboard modifier remap — applied globally via hidutil with LaunchAgent persistence",
}

func writeDeclTypes(b *strings.Builder, reg *resource.Registry) {
	b.WriteString("# Declaration Types\n\n")
	b.WriteString("These are the internal declaration types produced by the JavaScript API:\n\n")

	for _, t := range reg.AllDeclTypes() {
		desc, ok := declTypeDescriptions[t]
		if !ok {
			desc = t.String()
		}
		fmt.Fprintf(b, "- %s — %s\n", t.String(), desc)
	}
	b.WriteString("\n")
}

var actionTypeDescriptions = map[action.Type]string{
	action.WriteFile:         "Write or update a file's content",
	action.CreateDir:         "Create a directory",
	action.CreateSymlink:     "Create or update a symlink",
	action.SetPermissions:    "Set file or directory permissions",
	action.DeletePath:        "Remove a file, directory, or symlink",
	action.InstallPackage:    "Install a Homebrew package",
	action.SetDefaults:       "Write a macOS defaults key",
	action.SetDock:           "Set the macOS Dock layout",
	action.CloneRepo:         "Clone a git repository",
	action.PullRepo:          "Pull updates in an existing git repository",
	action.InstallFont:       "Install a font file",
	action.InstallMiseTool:   "Install a mise tool at a specific version",
	action.SetShell:          "Change the user's login shell",
	action.UninstallPackage:  "Uninstall a Homebrew package",
	action.UninstallMiseTool: "Uninstall a mise tool",
	action.DeleteDefaults:    "Delete a macOS defaults key",
	action.InstallMasApp:     "Install a Mac App Store app",
	action.SetKeyRemap:       "Apply keyboard modifier remappings via hidutil and write LaunchAgent",
	action.RemoveKeyRemap:    "Clear keyboard modifier remappings and remove LaunchAgent",
}

func writeActionTypes(b *strings.Builder, reg *resource.Registry) {
	b.WriteString("# Action Types\n\n")
	b.WriteString("These are the internal action types that the engine executes:\n\n")

	for _, t := range reg.AllActionTypes() {
		desc, ok := actionTypeDescriptions[t]
		if !ok {
			desc = t.String()
		}
		fmt.Fprintf(b, "- %s — %s\n", t.String(), desc)
	}
	b.WriteString("\n")
}
