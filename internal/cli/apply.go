package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/engine"
	"github.com/ryanwersal/crucible/internal/ui"
	"github.com/spf13/cobra"
)

func newApplyCmd(opts *rootOpts) *cobra.Command {
	var (
		dryRun      bool
		yes         bool
		scriptFile  string
		concurrency int
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration to the system",
		Long: `Apply configuration to the system.

The preferred approach is to make your crucible.js executable with a shebang
line (#!/usr/bin/env crucible) and run it directly: ./crucible.js --dry-run.
This command is the explicit alternative. Run from the directory containing
your crucible.js, or use --file to specify a script located elsewhere.`,
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

			result, err := eng.Plan(cmd.Context())
			if err != nil {
				return err
			}
			printResult(w, result)
			_, _ = fmt.Fprintln(w)

			if len(result.Actions) == 0 {
				_, _ = fmt.Fprintln(w, "Everything up to date.")
				return nil
			}

			if dryRun {
				_, _ = fmt.Fprintf(w, "%d action(s) would be taken.\n", len(result.Actions))
				return nil
			}

			if !yes {
				errw := cmd.ErrOrStderr()
				_, _ = fmt.Fprintf(errw, "%d action(s) will be taken:\n", len(result.Actions))
				for _, a := range result.Actions {
					if a.NeedsSudo {
						_, _ = fmt.Fprintf(errw, "  → [sudo] %s\n", a.Description)
					} else {
						_, _ = fmt.Fprintf(errw, "  → %s\n", a.Description)
					}
				}
				_, _ = fmt.Fprintf(errw, "Proceed? [y/N] ")

				type readResult struct {
					line string
					err  error
				}
				ch := make(chan readResult, 1) // buffered so the goroutine can exit if ctx wins the select
				go func() {
					reader := bufio.NewReader(cmd.InOrStdin())
					line, err := reader.ReadString('\n')
					ch <- readResult{line, err}
				}()

				select {
				case <-cmd.Context().Done():
					_, _ = fmt.Fprintln(errw, "\nAborted.")
					return cmd.Context().Err()
				case r := <-ch:
					if r.err != nil && !errors.Is(r.err, io.EOF) {
						return fmt.Errorf("reading confirmation: %w", r.err)
					}
					answer := strings.TrimSpace(strings.ToLower(r.line))
					if answer != "y" && answer != "yes" {
						_, _ = fmt.Fprintln(errw, "Aborted.")
						return nil
					}
				}
			}

			// Build observer based on whether stdout is a terminal.
			var observer engine.ActionObserver
			if f, ok := w.(*os.File); ok && ui.IsTerminal(f) {
				r := ui.NewRenderer(f, len(result.Actions), 5)
				r.Start(cmd.Context())
				defer r.Wait() // ensure render loop stops and cursor is restored
				observer = r
			} else {
				observer = ui.NewLogObserver(logger)
			}

			applyResult, err := eng.ApplyResultWithOptions(cmd.Context(), result, engine.ApplyOptions{
				Concurrency: concurrency,
				Observer:    observer,
			})
			if err != nil {
				return err
			}

			// Print summary.
			succeeded := len(applyResult.Succeeded())
			errs := applyResult.Errors()
			if len(errs) == 0 {
				_, _ = fmt.Fprintf(w, "%d action(s) applied.\n", succeeded)
				return nil
			}
			_, _ = fmt.Fprintf(w, "%d action(s) applied, %d failed.\n", succeeded, len(errs))
			for _, e := range errs {
				_, _ = fmt.Fprintf(w, "  ✗ %s: %v\n", e.Action.Description, e.Err)
			}
			return fmt.Errorf("%d action(s) failed", len(errs))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().StringVarP(&scriptFile, "file", "f", "", "path to a crucible.js script (default: ./crucible.js)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "max parallel actions (1 for sequential)")
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
