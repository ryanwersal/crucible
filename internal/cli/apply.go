package cli

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/engine"
	"github.com/spf13/cobra"
)

func newApplyCmd(opts *rootOpts) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the planned actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(opts.verbose)
			eng := engine.New(opts.source, opts.target, logger)

			if dryRun {
				actions, err := eng.Plan(cmd.Context())
				if err != nil {
					return err
				}
				w := cmd.OutOrStdout()
				if len(actions) == 0 {
					fmt.Fprintln(w, "Everything up to date.")
					return nil
				}
				for _, a := range actions {
					fmt.Fprintf(w, "  %s: %s\n", a.Type, a.Description)
				}
				fmt.Fprintf(w, "\n%d action(s) would be taken (dry run).\n", len(actions))
				return nil
			}

			if err := eng.Apply(cmd.Context()); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Apply complete.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	return cmd
}
