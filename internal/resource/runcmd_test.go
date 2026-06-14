package resource

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestRunCmd_Success(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := runCmd(context.Background(), action.Action{}, nil, &stdout, &stderr, "sh", "-c", "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Fatalf("stdout = %q, want %q", got, "hello")
	}
}

func TestRunCmd_FailureCapturesOutput(t *testing.T) {
	t.Parallel()
	var live bytes.Buffer
	err := runCmd(
		context.Background(),
		action.Action{},
		nil, &live, &live,
		"sh", "-c", "echo to-stdout; echo to-stderr 1>&2; exit 7",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	cmdErr, ok := errors.AsType[*CommandError](err)
	if !ok {
		t.Fatalf("expected *CommandError, got %T: %v", err, err)
	}
	if cmdErr.Command != "sh" {
		t.Errorf("Command = %q, want sh", cmdErr.Command)
	}
	if !strings.Contains(cmdErr.Output, "to-stdout") || !strings.Contains(cmdErr.Output, "to-stderr") {
		t.Errorf("captured Output missing streams; got %q", cmdErr.Output)
	}
	// The captured tail must also reach the live writers, so streaming UI
	// keeps showing output as the command runs.
	if !strings.Contains(live.String(), "to-stdout") {
		t.Errorf("live writer missing tee'd stdout; got %q", live.String())
	}

	// The wrapped exec error must remain reachable so callers can detect it.
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Errorf("expected wrapped *exec.ExitError; got %v", err)
	}
}

func TestRunCmd_ErrorIncludesOutputInString(t *testing.T) {
	t.Parallel()
	err := runCmd(
		context.Background(),
		action.Action{},
		nil, nil, nil,
		"sh", "-c", "echo boom 1>&2; exit 1",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom") {
		t.Errorf("error message missing captured output: %q", msg)
	}
	if !strings.Contains(msg, "exit status 1") {
		t.Errorf("error message missing exit code: %q", msg)
	}
}

func TestRunCmd_NilWriters(t *testing.T) {
	t.Parallel()
	// Both nil should still capture for error context.
	err := runCmd(context.Background(), action.Action{}, nil, nil, nil, "sh", "-c", "echo silent 1>&2; exit 2")
	if err == nil {
		t.Fatal("expected error")
	}
	cmdErr, ok := errors.AsType[*CommandError](err)
	if !ok {
		t.Fatalf("expected *CommandError, got %T", err)
	}
	if !strings.Contains(cmdErr.Output, "silent") {
		t.Errorf("Output missing data when live writers were nil: %q", cmdErr.Output)
	}
}

func TestRunCmd_ContextCancelledNotWrapped(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the process is killed immediately on start
	err := runCmd(ctx, action.Action{}, nil, nil, nil, "sh", "-c", "sleep 5")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if _, ok := errors.AsType[*CommandError](err); ok {
		t.Fatalf("context cancellation should not produce a *CommandError; got %v", err)
	}
}

func TestRingBuffer_TruncatesToMax(t *testing.T) {
	t.Parallel()
	rb := newRingBuffer(8)
	if _, err := rb.Write([]byte("abcdefghijklmnop")); err != nil {
		t.Fatal(err)
	}
	got := rb.String()
	if got != "ijklmnop" {
		t.Fatalf("expected tail %q, got %q", "ijklmnop", got)
	}
}

func TestRingBuffer_MultipleWrites(t *testing.T) {
	t.Parallel()
	rb := newRingBuffer(6)
	for _, s := range []string{"abc", "def", "ghi"} {
		if _, err := rb.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got := rb.String(); got != "defghi" {
		t.Fatalf("expected %q, got %q", "defghi", got)
	}
}

func TestCommandError_ErrorIncludesIndentedOutput(t *testing.T) {
	t.Parallel()
	ce := &CommandError{
		Command: "brew",
		Args:    []string{"install", "foo"},
		Err:     errors.New("exit status 1"),
		Output:  "Error: No formula\nsecond line",
	}
	got := ce.Error()
	if !strings.Contains(got, "exit status 1") {
		t.Errorf("missing exit info: %q", got)
	}
	if !strings.Contains(got, "    Error: No formula") || !strings.Contains(got, "    second line") {
		t.Errorf("expected output to be indented per line: %q", got)
	}
}

func TestCommandError_ErrorWithEmptyOutput(t *testing.T) {
	t.Parallel()
	ce := &CommandError{
		Command: "brew",
		Args:    []string{"install", "foo"},
		Err:     errors.New("exit status 1"),
	}
	if got := ce.Error(); got != "exit status 1" {
		t.Fatalf("Error() = %q, want %q", got, "exit status 1")
	}
}

func TestCommandError_OutputTailCarrier(t *testing.T) {
	t.Parallel()
	var ce error = &CommandError{Output: "tail data"}
	type outputCarrier interface{ OutputTail() string }
	c, ok := ce.(outputCarrier)
	if !ok {
		t.Fatal("CommandError should satisfy outputCarrier")
	}
	if c.OutputTail() != "tail data" {
		t.Fatalf("OutputTail = %q", c.OutputTail())
	}
}

func TestRunCmdPTY_AllocatesTTY(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	// `test -t 1` succeeds only when stdout is a terminal, which the PTY provides.
	err := runCmdPTY(context.Background(), action.Action{PTY: true}, &out,
		"sh", "-c", "test -t 1 && echo TTY || echo NOTTY")
	if err != nil {
		// A sandbox without /dev/ptmx can't allocate a PTY; skip rather than fail.
		if cmdErr, ok := errors.AsType[*CommandError](err); ok && cmdErr.Output == "" {
			t.Skipf("PTY unavailable in this environment: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "TTY") || strings.Contains(out.String(), "NOTTY") {
		t.Fatalf("expected child to see a TTY, got %q", out.String())
	}
}

func TestRunCmd_PipedIsNotTTY(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	// Without PTY, the child's stdout is a pipe, so `test -t 1` fails.
	err := runCmd(context.Background(), action.Action{}, nil, &out, &out,
		"sh", "-c", "test -t 1 && echo TTY || echo NOTTY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "NOTTY") {
		t.Fatalf("expected NOTTY for piped stdout, got %q", out.String())
	}
}

func TestRunCmdPTY_FailureCapturesOutput(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	err := runCmdPTY(context.Background(), action.Action{PTY: true}, &out,
		"sh", "-c", "echo boom; exit 3")
	if err == nil {
		t.Fatal("expected an error from a command that exits non-zero")
	}
	cmdErr, ok := errors.AsType[*CommandError](err)
	if !ok {
		t.Fatalf("expected *CommandError, got %T: %v", err, err)
	}
	// An empty capture means pty.Start failed and the command never ran
	// (no /dev/ptmx here) — skip rather than assert on output.
	if cmdErr.Output == "" {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	if !strings.Contains(cmdErr.Output, "boom") {
		t.Errorf("captured output = %q, want it to contain boom", cmdErr.Output)
	}
}
