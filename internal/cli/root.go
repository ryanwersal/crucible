package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// rootOpts holds persistent flags for the root command.
type rootOpts struct {
	source  string
	target  string
	verbose bool
}

// NewRootCmd creates the root crucible command.
func NewRootCmd() *cobra.Command {
	opts := &rootOpts{}

	cmd := &cobra.Command{
		Use:          "crucible",
		Short:        "A declarative dotfile and system configuration manager",
		Long:         "Crucible manages your dotfiles and system configuration using a two-phase plan/apply pipeline.",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.source, "source", ".", "source directory containing desired state")
	cmd.PersistentFlags().StringVar(&opts.target, "target", homeDir(), "target directory to apply state to")
	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "enable debug logging")

	cmd.AddCommand(newPlanCmd(opts))
	cmd.AddCommand(newApplyCmd(opts))

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
