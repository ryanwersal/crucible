package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/ryanwersal/crucible/internal/action"
	"golang.org/x/sync/errgroup"
)

// The fake permits validation but refuses noninteractive execution, modeling
// a policy that does not cache credentials. The command itself must prompt.
func TestSudoApplyLifecycle(t *testing.T) {
	for _, mode := range []string{"cold", "cached", "denied", "uncached", "canceled", "unprivileged"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("SUDO_TEST_DIR", dir)
			t.Setenv("SUDO_TEST_MODE", mode)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			mustWriteFile(t, filepath.Join(dir, "sudo"), []byte(`#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$SUDO_TEST_DIR/calls"
[ -t 0 ] || exit 88
# Validation is not proof that a command can run without a password.
[ "$1" != '-v' ] || exit 0
if [ "$1" = '-n' ]; then
 echo 'sudo: a password is required' >&2
 exit 1
fi
[ "$1" = '-p' ] && [ "$3" = '--' ] || exit 90
[ ! -f "$SUDO_TEST_DIR/ready" ] || exit 91
[ "$SUDO_TEST_MODE" != denied ] || exit 1
shift 3
exec "$@"
`), 0o755)
			master, terminal, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = master.Close() }()
			defer func() { _ = terminal.Close() }()
			eng := New(dir, dir, slog.New(slog.DiscardHandler))
			eng.SetInput(terminal)
			eng.SetOutput(io.Discard, io.Discard)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			seedPlanningFacts(eng)
			planned, err := eng.planScript(t.Context(), eng.factStore, []byte(fmt.Sprintf(`
var c = require("crucible");
c.script("sudo-lifecycle", {
    check: "false",
    install: 'touch "$SUDO_TEST_DIR/executed"',
    sudo: %t,
});`, mode != "unprivileged")))
			if err != nil {
				t.Fatal(err)
			}
			if len(planned.Actions) != 1 {
				t.Fatalf("expected one action, got %v", planned.Actions)
			}
			planned.Actions[0].PTY = true
			if mode == "uncached" {
				planned.Actions = append(planned.Actions, planned.Actions[0])
			}
			result, err := eng.ApplyResultWithOptions(ctx, planned, ApplyOptions{Interactive: true, BeforeActions: func() {
				mustWriteFile(t, filepath.Join(dir, "ready"), nil, 0o644)
			}})
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			_, executed := os.Stat(filepath.Join(dir, "executed"))
			switch mode {
			case "canceled", "denied":
				if err == nil {
					t.Fatal("expected authentication failure")
				}
				if mode == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v", err)
				}
				if _, readyErr := os.Stat(filepath.Join(dir, "ready")); readyErr == nil {
					t.Fatal("renderer started on authentication failure")
				}
				if executed == nil {
					t.Fatal("action ran on authentication failure")
				}
			default:
				if err != nil || len(result.Errors()) != 0 || executed != nil {
					t.Fatalf("result = %+v, err = %v, executed = %v", result, err, executed)
				}
			}
			prompts := strings.Count(string(calls), "-p crucible")
			wantPrompts := 1
			if mode == "canceled" || mode == "unprivileged" {
				wantPrompts = 0
			}
			if mode == "uncached" {
				wantPrompts = 2
			}
			if prompts != wantPrompts {
				t.Fatalf("prompt count = %d, calls = %s", prompts, calls)
			}
			if (mode == "canceled" || mode == "unprivileged") && len(calls) != 0 {
				t.Fatalf("unexpected sudo: %s", calls)
			}
		})
	}
}

func TestSudoCanceledWhilePrompting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	mustWriteFile(t, filepath.Join(dir, "sudo"), []byte("#!/bin/sh\n[ \"$1\" != '-n' ] || exit 1\necho waiting >&2\nread answer\n"), 0o755)
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close() }()
	defer func() { _ = writer.Close() }()
	eng := New(dir, dir, slog.New(slog.DiscardHandler))
	eng.SetInput(input)
	prompted := make(chan struct{}, 1)
	eng.SetOutput(io.Discard, promptSignal{prompted})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ready := false
	var group errgroup.Group
	group.Go(func() error {
		_, err := eng.ApplyResultWithOptions(ctx, action.PlanResult{Actions: []action.Action{{Type: action.RunScript, NeedsSudo: true}}}, ApplyOptions{BeforeActions: func() { ready = true }})
		return err
	})
	select {
	case <-prompted:
		cancel()
	case <-ctx.Done():
		t.Error("fake sudo never reached password prompt")
	}
	if err := group.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if ready {
		t.Fatal("renderer started after canceled authentication")
	}
}

type promptSignal struct{ ready chan struct{} }

func (s promptSignal) Write(p []byte) (int, error) {
	select {
	case s.ready <- struct{}{}:
	default:
	}
	return len(p), nil
}

func TestSudoWithoutTerminalFailsBeforeActions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	mustWriteFile(t, filepath.Join(dir, "sudo"), []byte("#!/bin/sh\n[ \"$1\" != '-n' ] || exit 1\necho 'sudo: a terminal is required' >&2\nexit 1\n"), 0o755)
	eng := New(dir, dir, slog.New(slog.DiscardHandler))
	eng.SetInput(nil)
	var output strings.Builder
	eng.SetOutput(io.Discard, &output)
	_, err := eng.ApplyResultWithOptions(t.Context(), action.PlanResult{Actions: []action.Action{{Type: action.RunScript, NeedsSudo: true}}}, ApplyOptions{BeforeActions: func() { t.Error("renderer started without credentials") }})
	if err == nil || !strings.Contains(output.String(), "terminal is required") {
		t.Fatalf("err = %v, output = %s", err, output.String())
	}
}

func TestPrivilegedChainRunsBeforeRenderer(t *testing.T) {
	t.Parallel()
	var order []string
	executor := &testExecutor{execFn: func(_ context.Context, a action.Action, _, _ io.Writer) error {
		order = append(order, a.Description)
		return nil
	}}
	eng := newTestEngine(t, executor)
	result, err := eng.ApplyResultWithOptions(t.Context(), action.PlanResult{Actions: []action.Action{
		{Type: action.WriteFile, Description: "unrelated"},
		{Type: action.WriteFile, Path: "shared", Description: "prerequisite"},
		{Type: action.WriteFile, Path: "shared", Description: "privileged", NeedsSudo: true},
		{Type: action.WriteFile, Path: "shared", Description: "dependent"},
	}}, ApplyOptions{Concurrency: 4, BeforeActions: func() { order = append(order, "renderer") }})
	if err != nil || len(result.Errors()) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if got := strings.Join(order, ","); got != "prerequisite,privileged,dependent,renderer,unrelated" {
		t.Fatalf("order=%s", got)
	}
}
