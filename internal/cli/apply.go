package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/engine"
	"github.com/spf13/cobra"
)

func newApplyCmd(opts *rootOpts) *cobra.Command {
	var (
		dryRun     bool
		scriptFile string
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration to the system",
		Long: `Apply configuration to the system.

Crucible looks for a crucible.js script in the current working directory.
Run this command from the directory containing your crucible.js, or use
--file to specify a script located elsewhere.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := newLogger(opts.verbose)

			sourceDir := opts.source
			if scriptFile != "" {
				absFile, err := filepath.Abs(scriptFile)
				if err != nil {
					return fmt.Errorf("resolve script path: %w", err)
				}
				scriptFile = absFile
				sourceDir = filepath.Dir(absFile)
			}

			eng := engine.New(sourceDir, opts.target, logger)
			if scriptFile != "" {
				eng.SetScriptFile(scriptFile)
			}
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
	cmd.Flags().StringVarP(&scriptFile, "file", "f", "", "path to a crucible.js script (default: ./crucible.js)")
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
