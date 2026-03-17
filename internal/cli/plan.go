package cli

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/engine"
	"github.com/spf13/cobra"
)

func newPlanCmd(opts *rootOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Show what actions would be taken (dry run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(opts.verbose)
			eng := engine.New(opts.source, opts.target, logger)

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
			fmt.Fprintf(w, "\n%d action(s) would be taken.\n", len(actions))
			return nil
		},
	}
}
