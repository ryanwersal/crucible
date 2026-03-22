package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"golang.org/x/sync/errgroup"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/resource"
	"github.com/ryanwersal/crucible/internal/script"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// Engine implements the two-phase plan/apply pipeline.
type Engine struct {
	sourceDir  string
	targetDir  string
	scriptFile string // optional explicit script path; overrides crucible.js discovery
	logger     *slog.Logger
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	registry   *resource.Registry
}

// New creates an Engine that maps sourceDir files onto targetDir.
func New(sourceDir, targetDir string, logger *slog.Logger) *Engine {
	return &Engine{
		sourceDir: sourceDir,
		targetDir: targetDir,
		logger:    logger,
		stdin:     os.Stdin,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		registry:  resource.DefaultRegistry(),
	}
}

// SetInput configures the reader used for subprocess stdin during Apply.
func (e *Engine) SetInput(stdin io.Reader) {
	e.stdin = stdin
}

// SetOutput configures the writers used for subprocess output during Apply.
func (e *Engine) SetOutput(stdout, stderr io.Writer) {
	e.stdout = stdout
	e.stderr = stderr
}

// SetScriptFile overrides the default crucible.js entry point discovery
// with an explicit script file path.
func (e *Engine) SetScriptFile(path string) {
	e.scriptFile = path
}

// Plan loads the crucible.js script (from the source directory or an explicit
// script file), collects facts about the current system state, and returns the
// result of comparing desired vs actual state.
func (e *Engine) Plan(ctx context.Context) (action.PlanResult, error) {
	store := fact.NewStore()

	if e.scriptFile != "" {
		content, err := os.ReadFile(e.scriptFile)
		if err != nil {
			return action.PlanResult{}, fmt.Errorf("read script %s: %w", e.scriptFile, err)
		}
		return e.planScript(ctx, store, content)
	}

	loader := script.NewLoader(e.sourceDir)
	_, content, err := loader.EntryPoint()
	if err != nil {
		return action.PlanResult{}, fmt.Errorf("load script: %w", err)
	}
	return e.planScript(ctx, store, content)
}

// planScript executes a crucible.js script and converts the resulting
// declarations into a PlanResult by diffing against current system state.
func (e *Engine) planScript(ctx context.Context, store *fact.Store, scriptContent []byte) (action.PlanResult, error) {
	// Pre-collect expensive facts concurrently
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(3)

	g.Go(func() error {
		_, err := fact.Get(gctx, store, "os", fact.OSCollector{})
		return err
	})
	g.Go(func() error {
		_, err := fact.Get(gctx, store, "homebrew", fact.HomebrewCollector{})
		return err
	})
	g.Go(func() error {
		_, err := fact.Get(gctx, store, "mas", fact.MasCollector{})
		return err
	})

	if err := g.Wait(); err != nil {
		return action.PlanResult{}, fmt.Errorf("pre-collect facts: %w", err)
	}

	// Create and execute script runtime
	rt := script.NewRuntime(ctx, e.logger, e.sourceDir, e.targetDir, store)
	entryPath := filepath.Join(e.sourceDir, "crucible.js")
	_, err := rt.Execute(ctx, entryPath, scriptContent)
	if err != nil {
		return action.PlanResult{}, err
	}

	// Resolve source files and templates
	if err := rt.ResolveContent(ctx, store); err != nil {
		return action.PlanResult{}, err
	}

	return e.declarationsToResult(ctx, store, rt.Declarations())
}

// declarationsToResult converts script declarations into a PlanResult by
// dispatching each declaration to the appropriate registry handler.
func (e *Engine) declarationsToResult(ctx context.Context, store *fact.Store, decls []decl.Declaration) (action.PlanResult, error) {
	var result action.PlanResult
	env := resource.Env{SourceDir: e.sourceDir, TargetDir: e.targetDir}

	batched := make(map[decl.Type][]decl.Declaration)
	for _, d := range decls {
		if e.registry.IsBatched(d.Type) {
			batched[d.Type] = append(batched[d.Type], d)
			continue
		}
		out, err := e.registry.PlanOne(ctx, store, env, d)
		if err != nil {
			return action.PlanResult{}, err
		}
		group := d.Type.String()
		stampGroup(out.Actions, out.Observations, group)
		result.Actions = append(result.Actions, out.Actions...)
		result.Observations = append(result.Observations, out.Observations...)
	}
	for t, ds := range batched {
		out, err := e.registry.PlanBatch(ctx, store, env, t, ds)
		if err != nil {
			return action.PlanResult{}, err
		}
		group := t.String()
		stampGroup(out.Actions, out.Observations, group)
		result.Actions = append(result.Actions, out.Actions...)
		result.Observations = append(result.Observations, out.Observations...)
	}

	return result, nil
}

// Apply runs Plan and then executes all resulting actions, returning the full
// PlanResult so callers can report what was already current and what was applied.
func (e *Engine) Apply(ctx context.Context) (action.PlanResult, error) {
	result, err := e.Plan(ctx)
	if err != nil {
		return action.PlanResult{}, err
	}
	return e.ApplyResult(ctx, result)
}

// ApplyResult executes a pre-computed PlanResult sequentially, applying all actions.
// Use this when you've already called Plan and want to apply without re-planning.
func (e *Engine) ApplyResult(ctx context.Context, result action.PlanResult) (action.PlanResult, error) {
	ar, err := e.ApplyResultWithOptions(ctx, result, ApplyOptions{Concurrency: 1})
	if err != nil {
		return action.PlanResult{}, err
	}
	if errs := ar.Errors(); len(errs) > 0 {
		first := errs[0]
		return action.PlanResult{}, fmt.Errorf("action failed (%s): %w", first.Action.Description, first.Err)
	}
	return result, nil
}

// ApplyResultWithOptions executes a pre-computed PlanResult with configurable
// concurrency and an optional observer for live progress display.
func (e *Engine) ApplyResultWithOptions(ctx context.Context, result action.PlanResult, opts ApplyOptions) (ApplyResult, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}

	// Pre-acquire sudo credentials if any action needs privilege escalation.
	if needsSudo(result.Actions) {
		e.logger.Info("pre-acquiring sudo credentials")
		cmd := exec.CommandContext(ctx, "sudo", "-v")
		cmd.Stdin = e.stdin
		cmd.Stdout = e.stdout
		cmd.Stderr = e.stderr
		if err := cmd.Run(); err != nil {
			return ApplyResult{}, fmt.Errorf("sudo credential acquisition failed: %w", err)
		}
	}

	// Partition actions: SetShell needs stdin and must run sequentially.
	concurrent := make([]indexedAction, 0, len(result.Actions))
	var sequential []indexedAction
	for i, a := range result.Actions {
		if a.Type == action.SetShell {
			sequential = append(sequential, indexedAction{index: i, action: a})
		} else {
			concurrent = append(concurrent, indexedAction{index: i, action: a})
		}
	}

	results := make([]ActionResult, len(result.Actions))

	// Pre-populate actions so callers always have action metadata in results.
	for i, a := range result.Actions {
		results[i] = ActionResult{Action: a}
	}

	// Run concurrent actions.
	if len(concurrent) > 0 {
		e.runConcurrent(ctx, concurrent, results, opts)
	}

	// Run sequential (stdin-needing) actions.
	for _, ia := range sequential {
		if ctx.Err() != nil {
			break
		}
		if opts.Observer != nil {
			opts.Observer.ActionStarted(ia.index, ia.action)
		}
		err := e.registry.Execute(ctx, ia.action, e.stdin, e.stdout, e.stderr)
		results[ia.index] = ActionResult{Action: ia.action, Err: err}
		if opts.Observer != nil {
			opts.Observer.ActionCompleted(ia.index, ia.action, err)
		}
	}

	if opts.Observer != nil {
		opts.Observer.Wait()
	}

	return ApplyResult{Results: results}, nil
}

type indexedAction struct {
	index  int
	action action.Action
}

func (e *Engine) runConcurrent(ctx context.Context, actions []indexedAction, results []ActionResult, opts ApplyOptions) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	for _, ia := range actions {
		g.Go(func() error {
			// Short-circuit if context was cancelled while waiting for a slot.
			if gctx.Err() != nil {
				return nil
			}

			if opts.Observer != nil {
				opts.Observer.ActionStarted(ia.index, ia.action)
			}

			// Always use per-action writers for concurrent execution to avoid
			// interleaved output from multiple goroutines sharing a writer.
			buf := newObserverWriter(ia.index, opts.Observer)

			err := e.registry.Execute(gctx, ia.action, nil, buf, buf)
			// Safe: each goroutine writes to a distinct index; results is only
			// read after g.Wait() returns.
			results[ia.index] = ActionResult{Action: ia.action, Err: err}

			if opts.Observer != nil {
				opts.Observer.ActionCompleted(ia.index, ia.action, err)
			}
			return nil // never return error — we collect them in results
		})
	}

	_ = g.Wait()
}

// observerWriter is an io.Writer that splits output into lines and feeds
// them to an ActionObserver. It treats both \n and \r as line terminators
// (the latter for progress bars that use carriage return).
type observerWriter struct {
	index    int
	observer ActionObserver
	partial  []byte
}

// maxPartialLen caps the partial line buffer to prevent unbounded growth
// from output that never emits a newline (e.g. long progress bars).
const maxPartialLen = 64 * 1024

func newObserverWriter(index int, observer ActionObserver) *observerWriter {
	return &observerWriter{index: index, observer: observer, partial: make([]byte, 0, 512)}
}

func (w *observerWriter) Write(p []byte) (int, error) {
	if w.observer == nil {
		return len(p), nil
	}
	w.partial = append(w.partial, p...)
	for {
		nl := bytes.IndexAny(w.partial, "\n\r")
		if nl < 0 {
			break
		}
		line := string(w.partial[:nl])
		// Skip empty lines produced by \r\n (the \n after a \r).
		skip := nl + 1
		if w.partial[nl] == '\r' && skip < len(w.partial) && w.partial[skip] == '\n' {
			skip++
		}
		w.partial = w.partial[skip:]
		if len(line) > 0 {
			w.observer.ActionOutput(w.index, line)
		}
	}
	// Cap partial buffer to prevent unbounded growth from output that never
	// emits a newline. Truncation may discard the start of a partial line,
	// so the next emitted line after truncation could be incomplete.
	if len(w.partial) > maxPartialLen {
		w.partial = w.partial[len(w.partial)-maxPartialLen:]
	}
	return len(p), nil
}

// stampGroup sets the Group field on actions and observations that lack one.
func stampGroup(acts []action.Action, obs []action.Observation, group string) {
	for i := range acts {
		if acts[i].Group == "" {
			acts[i].Group = group
		}
	}
	for i := range obs {
		if obs[i].Group == "" {
			obs[i].Group = group
		}
	}
}

// needsSudo reports whether any action requires privilege escalation.
func needsSudo(actions []action.Action) bool {
	return slices.ContainsFunc(actions, func(a action.Action) bool {
		return a.NeedsSudo
	})
}
