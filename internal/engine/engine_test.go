package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/resource"
	"github.com/ryanwersal/crucible/internal/script"
)

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirAll(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatal(err)
	}
}

func TestPlan_NoScript_Fails(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	_, err := eng.Plan(context.Background())
	if err == nil {
		t.Fatal("expected error when crucible.js is missing")
	}
	if !errors.Is(err, script.ErrNoScript) {
		t.Errorf("expected ErrNoScript, got: %v", err)
	}
}

func TestPlan_ExplicitScriptFile_NotFound(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	eng.SetScriptFile(filepath.Join(src, "nonexistent.js"))
	_, err := eng.Plan(context.Background())
	if err == nil {
		t.Fatal("expected error for missing explicit script file")
	}
}

// TestPlan_Script verifies script-driven planning with crucible.js.
func TestPlan_Script(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "# managed by crucible" });
		c.dir("~/.config", { mode: 493 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hasWrite := false
	hasDir := false
	for _, a := range result.Actions {
		switch a.Type {
		case action.WriteFile:
			hasWrite = true
		case action.CreateDir:
			hasDir = true
		}
	}
	if !hasWrite {
		t.Fatal("expected WriteFile action from script")
	}
	if !hasDir {
		t.Fatal("expected CreateDir action from script")
	}
}

// TestApply_Script verifies end-to-end script-driven apply.
func TestApply_Script(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "# managed by crucible", mode: 420 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# managed by crucible" {
		t.Fatalf("expected '# managed by crucible', got %q", content)
	}
}

// TestPlan_Script_SourceFile verifies source file references in scripts.
func TestPlan_Script_SourceFile(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustMkdirAll(t, filepath.Join(src, "fish"), 0o755)
	mustWriteFile(t, filepath.Join(src, "fish", "config.fish"), []byte("set PATH /usr/local/bin"), 0o644)

	scriptContent := `
		var c = require("crucible");
		c.file("~/.config/fish/config.fish", { source: "fish/config.fish" });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, ".config", "fish", "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "set PATH /usr/local/bin" {
		t.Fatalf("got %q", content)
	}
}

// TestPlan_Script_Idempotent verifies that a second plan produces no actions
// and produces observations for each managed item.
func TestPlan_Script_Idempotent(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "hello", mode: 420 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions on second plan, got %d", len(result.Actions))
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation on second plan, got %d", len(result.Observations))
	}
}

// --- ApplyResultWithOptions tests ---

// testObserver records lifecycle events for test assertions.
type testObserver struct {
	mu        sync.Mutex
	started   []int
	outputs   []observerOutput
	completed []observerCompleted
}

type observerOutput struct {
	index int
	line  string
}

type observerCompleted struct {
	index int
	err   error
}

func (o *testObserver) ActionStarted(index int, _ action.Action) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.started = append(o.started, index)
}

func (o *testObserver) ActionOutput(index int, line string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outputs = append(o.outputs, observerOutput{index: index, line: line})
}

func (o *testObserver) ActionCompleted(index int, _ action.Action, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completed = append(o.completed, observerCompleted{index: index, err: err})
}

func (o *testObserver) Wait() {}

// testExecutor is a configurable executor for testing.
type testExecutor struct {
	execFn func(ctx context.Context, a action.Action, stdout, stderr io.Writer) error
}

func (e *testExecutor) ActionType() action.Type { return action.WriteFile }
func (e *testExecutor) ActionName() string      { return "TestExecutor" }
func (e *testExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, stdout, stderr io.Writer) error {
	if e.execFn != nil {
		return e.execFn(ctx, a, stdout, stderr)
	}
	return nil
}

func newTestEngine(t *testing.T, exec *testExecutor) *Engine {
	t.Helper()
	reg := resource.NewRegistry()
	reg.RegisterExecutor(exec)
	eng := &Engine{
		sourceDir: t.TempDir(),
		targetDir: t.TempDir(),
		logger:    slog.New(slog.DiscardHandler),
		stdin:     os.Stdin,
		stdout:    io.Discard,
		stderr:    io.Discard,
		registry:  reg,
	}
	return eng
}

func TestApplyResultWithOptions_Concurrent(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var order []string
	exec := &testExecutor{
		execFn: func(_ context.Context, a action.Action, stdout, _ io.Writer) error {
			_, _ = fmt.Fprintf(stdout, "running %s\n", a.Description)
			mu.Lock()
			order = append(order, a.Description)
			mu.Unlock()
			return nil
		},
	}
	eng := newTestEngine(t, exec)
	obs := &testObserver{}

	plan := action.PlanResult{
		Actions: []action.Action{
			{Type: action.WriteFile, Description: "action-a"},
			{Type: action.WriteFile, Description: "action-b"},
			{Type: action.WriteFile, Description: "action-c"},
		},
	}

	result, err := eng.ApplyResultWithOptions(context.Background(), plan, ApplyOptions{
		Concurrency: 4,
		Observer:    obs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors()) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors())
	}
	if len(result.Succeeded()) != 3 {
		t.Fatalf("succeeded = %d, want 3", len(result.Succeeded()))
	}

	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.started) != 3 {
		t.Errorf("started events = %d, want 3", len(obs.started))
	}
	if len(obs.completed) != 3 {
		t.Errorf("completed events = %d, want 3", len(obs.completed))
	}
}

func TestApplyResultWithOptions_CollectsErrors(t *testing.T) {
	t.Parallel()

	exec := &testExecutor{
		execFn: func(_ context.Context, a action.Action, _, _ io.Writer) error {
			if a.Description == "fail" {
				return fmt.Errorf("intentional failure")
			}
			return nil
		},
	}
	eng := newTestEngine(t, exec)

	plan := action.PlanResult{
		Actions: []action.Action{
			{Type: action.WriteFile, Description: "ok"},
			{Type: action.WriteFile, Description: "fail"},
			{Type: action.WriteFile, Description: "also-ok"},
		},
	}

	result, err := eng.ApplyResultWithOptions(context.Background(), plan, ApplyOptions{Concurrency: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors()) != 1 {
		t.Fatalf("errors = %d, want 1", len(result.Errors()))
	}
	if len(result.Succeeded()) != 2 {
		t.Fatalf("succeeded = %d, want 2", len(result.Succeeded()))
	}
}

func TestApplyResultWithOptions_SetShellRunsSequentially(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var order []string
	exec := &testExecutor{
		execFn: func(_ context.Context, a action.Action, _, _ io.Writer) error {
			mu.Lock()
			order = append(order, a.Description)
			mu.Unlock()
			return nil
		},
	}
	// Register for SetShell type too.
	shellExec := &shellTestExecutor{execFn: exec.execFn}
	reg := resource.NewRegistry()
	reg.RegisterExecutor(exec)
	reg.RegisterExecutor(shellExec)
	eng := &Engine{
		sourceDir: t.TempDir(),
		targetDir: t.TempDir(),
		logger:    slog.New(slog.DiscardHandler),
		stdin:     os.Stdin,
		stdout:    io.Discard,
		stderr:    io.Discard,
		registry:  reg,
	}

	plan := action.PlanResult{
		Actions: []action.Action{
			{Type: action.WriteFile, Description: "concurrent-1"},
			{Type: action.SetShell, Description: "set-shell"},
			{Type: action.WriteFile, Description: "concurrent-2"},
		},
	}

	result, err := eng.ApplyResultWithOptions(context.Background(), plan, ApplyOptions{Concurrency: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors()) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors())
	}

	// All 3 should have completed.
	if len(result.Succeeded()) != 3 {
		t.Fatalf("succeeded = %d, want 3", len(result.Succeeded()))
	}

	// set-shell should be last (sequential runs after concurrent batch).
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("order = %v, want 3 entries", order)
	}
	if order[len(order)-1] != "set-shell" {
		t.Errorf("last action = %q, want set-shell", order[len(order)-1])
	}
}

type shellTestExecutor struct {
	execFn func(ctx context.Context, a action.Action, stdout, stderr io.Writer) error
}

func (e *shellTestExecutor) ActionType() action.Type { return action.SetShell }
func (e *shellTestExecutor) ActionName() string      { return "TestSetShell" }
func (e *shellTestExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, stdout, stderr io.Writer) error {
	if e.execFn != nil {
		return e.execFn(ctx, a, stdout, stderr)
	}
	return nil
}

func TestApplyResultWithOptions_ObserverOutput(t *testing.T) {
	t.Parallel()

	exec := &testExecutor{
		execFn: func(_ context.Context, _ action.Action, stdout, _ io.Writer) error {
			_, _ = fmt.Fprintln(stdout, "line 1")
			_, _ = fmt.Fprintln(stdout, "line 2")
			return nil
		},
	}
	eng := newTestEngine(t, exec)
	obs := &testObserver{}

	plan := action.PlanResult{
		Actions: []action.Action{
			{Type: action.WriteFile, Description: "test"},
		},
	}

	_, err := eng.ApplyResultWithOptions(context.Background(), plan, ApplyOptions{
		Concurrency: 1,
		Observer:    obs,
	})
	if err != nil {
		t.Fatal(err)
	}

	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(obs.outputs))
	}
	if obs.outputs[0].line != "line 1" {
		t.Errorf("output[0] = %q, want line 1", obs.outputs[0].line)
	}
}

func TestApplyResultWithOptions_ContextCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	exec := &testExecutor{
		execFn: func(ctx context.Context, _ action.Action, _, _ io.Writer) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}
	eng := newTestEngine(t, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	plan := action.PlanResult{
		Actions: []action.Action{
			{Type: action.WriteFile, Description: "blocking"},
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = eng.ApplyResultWithOptions(ctx, plan, ApplyOptions{Concurrency: 1})
	}()

	<-started
	cancel()
	<-done
}

func TestObserverWriter_LineSplitting(t *testing.T) {
	t.Parallel()

	obs := &testObserver{}
	w := newObserverWriter(0, obs)

	// Partial write followed by completion.
	_, _ = w.Write([]byte("hel"))
	_, _ = w.Write([]byte("lo\nwor"))
	_, _ = w.Write([]byte("ld\r\n"))

	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(obs.outputs))
	}
	if obs.outputs[0].line != "hello" {
		t.Errorf("output[0] = %q, want hello", obs.outputs[0].line)
	}
	if obs.outputs[1].line != "world" {
		t.Errorf("output[1] = %q, want world", obs.outputs[1].line)
	}
}

func TestObserverWriter_NilObserver(t *testing.T) {
	t.Parallel()

	w := newObserverWriter(0, nil)
	n, err := w.Write([]byte("hello\nworld\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 12 {
		t.Errorf("n = %d, want 12", n)
	}
}

// TestPlan_ExplicitScriptFile verifies that SetScriptFile overrides discovery.
func TestPlan_ExplicitScriptFile(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	// Put the script in a non-standard location
	altDir := t.TempDir()
	scriptContent := `
		var c = require("crucible");
		c.file("~/.vimrc", { content: "set nocompatible" });
	`
	mustWriteFile(t, filepath.Join(altDir, "my-config.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	eng.SetScriptFile(filepath.Join(altDir, "my-config.js"))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Actions) == 0 {
		t.Fatal("expected actions from explicit script file")
	}
}
