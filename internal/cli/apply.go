package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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

			errw := cmd.ErrOrStderr()

			if !yes {
				_, _ = fmt.Fprintf(errw, "%d action(s) will be taken:\n", len(result.Actions))
				printActions(errw, result.Actions)
				_, _ = fmt.Fprintf(errw, "Proceed? [y/N] ")
				ok, err := readConfirmation(cmd.Context(), cmd.InOrStdin(), errw, []string{"y", "yes"})
				if err != nil {
					return err
				}
				if !ok {
					_, _ = fmt.Fprintln(errw, "Aborted.")
					return nil
				}
			}

			// Destructive operations get a dedicated confirmation regardless
			// of --yes. The user must spell out "yes" — single "y" is not
			// enough — and the prompt enumerates exactly what will be lost.
			if destructive := result.Destructive(); len(destructive) > 0 {
				_, _ = fmt.Fprintln(errw)
				_, _ = fmt.Fprintf(errw, "⚠ %d destructive operation(s) detected — content will be permanently lost:\n", len(destructive))
				for _, a := range destructive {
					_, _ = fmt.Fprintf(errw, "  ⚠ %s\n", a.Description)
					if a.DestructiveReason != "" {
						_, _ = fmt.Fprintf(errw, "      %s\n", a.DestructiveReason)
					}
				}
				_, _ = fmt.Fprintf(errw, "Type \"yes\" to confirm destruction, anything else to abort: ")
				ok, err := readConfirmation(cmd.Context(), cmd.InOrStdin(), errw, []string{"yes"})
				if err != nil {
					return err
				}
				if !ok {
					_, _ = fmt.Fprintln(errw, "Aborted.")
					return nil
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

// readConfirmation reads a single line from stdin and reports whether it
// (case-insensitively, trimmed) matches one of the accepted strings. Returns
// (false, nil) on any other answer or on EOF; cancels cleanly if ctx fires
// while waiting for input. Errors from stdin (other than EOF) propagate up.
func readConfirmation(ctx context.Context, stdin io.Reader, stderr io.Writer, accept []string) (bool, error) {
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1) // buffered so the goroutine can exit if ctx wins the select
	go func() {
		reader := bufio.NewReader(stdin)
		line, err := reader.ReadString('\n')
		ch <- readResult{line, err}
	}()

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintln(stderr, "\nAborted.")
		return false, ctx.Err()
	case r := <-ch:
		if r.err != nil && !errors.Is(r.err, io.EOF) {
			return false, fmt.Errorf("reading confirmation: %w", r.err)
		}
		answer := strings.TrimSpace(strings.ToLower(r.line))
		return slices.Contains(accept, answer), nil
	}
}

// printResult writes observations and actions to w as a grouped tree.
// Items are grouped by their resource type (e.g. "brew", "file", "display").
func printResult(w io.Writer, result action.PlanResult) {
	writeGroupedTree(w, result.Observations, result.Actions)
}

// printActions writes only the actions portion as a grouped tree.
func printActions(w io.Writer, actions []action.Action) {
	writeGroupedTree(w, nil, actions)
}

type treeEntry struct {
	symbol string // "✓", "→", or "⚠"
	desc   string
	reason string // populated only for destructive entries
}

// writeGroupedTree renders observations and actions as a tree grouped by resource type.
// Destructive actions use the "⚠" symbol and a second indented line showing
// what content would be lost.
func writeGroupedTree(w io.Writer, observations []action.Observation, actions []action.Action) {
	// Collect entries per group, preserving declaration order.
	groups := make(map[string][]treeEntry)
	var groupOrder []string
	addGroup := func(g string) {
		if _, ok := groups[g]; !ok {
			groupOrder = append(groupOrder, g)
			groups[g] = nil
		}
	}

	for _, o := range observations {
		g := groupName(o.Group)
		addGroup(g)
		groups[g] = append(groups[g], treeEntry{symbol: "✓", desc: o.Description})
	}
	for _, a := range actions {
		g := groupName(a.Group)
		addGroup(g)
		desc := a.Description
		if a.NeedsSudo {
			desc = "[sudo] " + desc
		}
		entry := treeEntry{symbol: "→", desc: desc}
		if a.Destructive {
			entry.symbol = "⚠"
			entry.reason = a.DestructiveReason
		}
		groups[g] = append(groups[g], entry)
	}

	for _, g := range groupOrder {
		_, _ = fmt.Fprintf(w, "  %s\n", g)
		for _, e := range groups[g] {
			_, _ = fmt.Fprintf(w, "    %s %s\n", e.symbol, e.desc)
			if e.reason != "" {
				_, _ = fmt.Fprintf(w, "        destructive: %s\n", e.reason)
			}
		}
	}
}

// groupName normalises a group label for display.
func groupName(g string) string {
	g = strings.ToLower(g)
	if g == "" {
		return "other"
	}
	return g
}
