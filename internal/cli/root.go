package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// rootOpts holds persistent flags and injected defaults for the root command.
type rootOpts struct {
	verbose bool

	// source and target are the engine directories. They default to "." and
	// $HOME respectively. Tests set them directly to avoid touching the real
	// home directory.
	source string
	target string
}

// NewRootCmd creates the root crucible command with production defaults.
func NewRootCmd() *cobra.Command {
	opts := &rootOpts{
		source: ".",
		target: homeDir(),
	}
	return buildRootCmd(opts)
}

// buildRootCmd constructs the command tree from the given opts.
// Separated from NewRootCmd so tests can inject source/target.
func buildRootCmd(opts *rootOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "crucible",
		Short:        "A declarative dotfile and system configuration manager",
		Long:         "Crucible manages your dotfiles and system configuration declaratively. Use --dry-run to preview changes.",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable debug logging")

	cmd.AddCommand(newApplyCmd(opts))
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func newLogger(verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
