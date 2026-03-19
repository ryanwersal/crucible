package cli

import (
	"fmt"
	"io"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/engine"
	"github.com/spf13/cobra"
)

func newApplyCmd(opts *rootOpts) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration to the system",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(opts.verbose)
			eng := engine.New(opts.source, opts.target, logger)
			w := cmd.OutOrStdout()

			if dryRun {
				result, err := eng.Plan(cmd.Context())
				if err != nil {
					return err
				}
				printResult(w, result)
				_, _ = fmt.Fprintln(w)
				if len(result.Actions) == 0 {
					_, _ = fmt.Fprintln(w, "Everything up to date.")
				} else {
					_, _ = fmt.Fprintf(w, "%d action(s) would be taken.\n", len(result.Actions))
				}
				return nil
			}

			result, err := eng.Apply(cmd.Context())
			if err != nil {
				return err
			}
			printResult(w, result)
			_, _ = fmt.Fprintln(w)
			if len(result.Actions) == 0 {
				_, _ = fmt.Fprintln(w, "Everything up to date.")
			} else {
				_, _ = fmt.Fprintf(w, "%d action(s) applied.\n", len(result.Actions))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	return cmd
}

// printResult writes observations and actions to w using ✓/→ symbols.
func printResult(w io.Writer, result action.PlanResult) {
	for _, o := range result.Observations {
		_, _ = fmt.Fprintf(w, "  ✓ %s\n", o.Description)
	}
	for _, a := range result.Actions {
		if a.NeedsSudo {
			_, _ = fmt.Fprintf(w, "  → [sudo] %s\n", a.Description)
		} else {
			_, _ = fmt.Fprintf(w, "  → %s\n", a.Description)
		}
	}
}
