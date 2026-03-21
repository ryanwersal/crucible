package engine

import (
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
		result.Actions = append(result.Actions, out.Actions...)
		result.Observations = append(result.Observations, out.Observations...)
	}
	for t, ds := range batched {
		out, err := e.registry.PlanBatch(ctx, store, env, t, ds)
		if err != nil {
			return action.PlanResult{}, err
		}
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

// ApplyResult executes a pre-computed PlanResult, applying all actions.
// Use this when you've already called Plan and want to apply without re-planning.
func (e *Engine) ApplyResult(ctx context.Context, result action.PlanResult) (action.PlanResult, error) {
	// Pre-acquire sudo credentials if any action needs privilege escalation.
	if needsSudo(result.Actions) {
		e.logger.Info("pre-acquiring sudo credentials")
		cmd := exec.CommandContext(ctx, "sudo", "-v")
		cmd.Stdin = e.stdin
		cmd.Stdout = e.stdout
		cmd.Stderr = e.stderr
		if err := cmd.Run(); err != nil {
			return action.PlanResult{}, fmt.Errorf("sudo credential acquisition failed: %w", err)
		}
	}

	for i, a := range result.Actions {
		e.logger.Info("executing", "action", a.Type.String(), "description", a.Description)
		if err := e.registry.Execute(ctx, a, e.stdin, e.stdout, e.stderr); err != nil {
			return action.PlanResult{}, fmt.Errorf("action %d/%d failed (%s): %w", i+1, len(result.Actions), a.Description, err)
		}
	}

	return result, nil
}

// needsSudo reports whether any action requires privilege escalation.
func needsSudo(actions []action.Action) bool {
	return slices.ContainsFunc(actions, func(a action.Action) bool {
		return a.NeedsSudo
	})
}
