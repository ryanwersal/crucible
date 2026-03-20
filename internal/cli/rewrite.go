package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// RewriteScriptArgs rewrites CLI arguments so that a bare script path
// (e.g. ./crucible.js from a shebang invocation) is expanded to
// "apply --file <script>". Known subcommand names and flags are left unchanged.
func RewriteScriptArgs(args []string, subcommands []string) []string {
	subs := make(map[string]struct{}, len(subcommands))
	for _, s := range subcommands {
		subs[s] = struct{}{}
	}

	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// First non-flag argument found.
		if _, ok := subs[arg]; ok {
			return args
		}
		if strings.HasSuffix(arg, ".js") {
			// Insert "apply --file" before the script path.
			rewritten := make([]string, 0, len(args)+2)
			rewritten = append(rewritten, args[:i]...)
			rewritten = append(rewritten, "apply", "--file")
			rewritten = append(rewritten, args[i:]...)
			return rewritten
		}
		// Unknown non-flag, non-.js argument — leave unchanged.
		return args
	}

	return args
}

// SubcommandNames returns the names of all direct subcommands of cmd.
func SubcommandNames(cmd *cobra.Command) []string {
	cmds := cmd.Commands()
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name()
	}
	return names
}
