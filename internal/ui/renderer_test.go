package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestActionState_Transitions(t *testing.T) {
	t.Parallel()

	a := action.Action{Type: action.InstallPackage, Description: "install foo"}

	// Verify state transitions don't panic and set fields correctly.
	r := &Renderer{
		actions:   make([]actionState, 1),
		maxLines:  5,
		termWidth: 80,
	}

	r.ActionStarted(0, a)
	if r.actions[0].status != statusRunning {
		t.Errorf("status = %d, want statusRunning", r.actions[0].status)
	}

	r.ActionOutput(0, "Downloading...")
	r.ActionOutput(0, "Installing...")
	if len(r.actions[0].lines) != 2 {
		t.Errorf("lines = %d, want 2", len(r.actions[0].lines))
	}

	r.ActionCompleted(0, a, nil)
	if r.actions[0].status != statusDone {
		t.Errorf("status = %d, want statusDone", r.actions[0].status)
	}
}

func TestActionState_FailedTransition(t *testing.T) {
	t.Parallel()

	a := action.Action{Type: action.InstallPackage, Description: "install bar"}

	r := &Renderer{
		actions:   make([]actionState, 1),
		maxLines:  5,
		termWidth: 80,
	}

	r.ActionStarted(0, a)
	r.ActionCompleted(0, a, fmt.Errorf("not found"))
	if r.actions[0].status != statusFailed {
		t.Errorf("status = %d, want statusFailed", r.actions[0].status)
	}
	if r.actions[0].err == nil {
		t.Error("err should be set")
	}
}

func TestActionOutput_Overflow(t *testing.T) {
	t.Parallel()

	r := &Renderer{
		actions:   make([]actionState, 1),
		maxLines:  3,
		termWidth: 80,
	}
	a := action.Action{Description: "test"}
	r.ActionStarted(0, a)

	for i := range 10 {
		r.ActionOutput(0, fmt.Sprintf("line %d", i))
	}

	if len(r.actions[0].lines) != 3 {
		t.Errorf("lines = %d, want 3", len(r.actions[0].lines))
	}
	if r.actions[0].lines[0] != "line 7" {
		t.Errorf("oldest line = %q, want line 7", r.actions[0].lines[0])
	}
}

func TestFirstLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"one line", "one line"},
		{"first\nsecond", "first"},
		{"\nblank-first", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := firstLine(tt.in); got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRender_FailureKeepsOutputLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &Renderer{
		w:         &buf,
		actions:   make([]actionState, 1),
		maxLines:  5,
		termWidth: 200,
	}

	a := action.Action{Type: action.InstallPackage, Description: "brew install foo"}
	r.ActionStarted(0, a)
	r.ActionOutput(0, "==> Searching")
	r.ActionOutput(0, `Error: No available formula with the name "foo"`)
	r.ActionCompleted(0, a, errors.New("exit status 1\n    Error: No available formula"))

	r.render()

	got := buf.String()
	if !strings.Contains(got, "✗ brew install foo: exit status 1") {
		t.Errorf("headline missing or includes second line: %q", got)
	}
	// The headline is built from firstLine(err) so the second line of the
	// error body must not appear adjacent to "exit status 1" in the headline.
	if strings.Contains(got, "exit status 1\n    Error: No available formula") {
		t.Errorf("headline included multi-line error body: %q", got)
	}
	if !strings.Contains(got, "==> Searching") {
		t.Errorf("captured output missing first line: %q", got)
	}
	if !strings.Contains(got, `Error: No available formula with the name "foo"`) {
		t.Errorf("captured output missing failure line: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	r := &Renderer{termWidth: 10}

	tests := []struct {
		name  string
		input string
		want  int // max visible chars
	}{
		{"short", "hello", 5},
		{"exact", "0123456789", 10},
		{"over", "0123456789abc", 10},
		{"ansi", "\033[32mhello\033[0m", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.truncate(tt.input)
			// Count visible chars.
			visible := 0
			inEsc := false
			for i := range len(got) {
				if got[i] == '\033' {
					inEsc = true
					continue
				}
				if inEsc {
					if (got[i] >= 'A' && got[i] <= 'Z') || (got[i] >= 'a' && got[i] <= 'z') {
						inEsc = false
					}
					continue
				}
				visible++
			}
			if visible > r.termWidth {
				t.Errorf("visible = %d, want <= %d", visible, r.termWidth)
			}
		})
	}
}

func TestProgressKeepsLargePlansCompact(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &Renderer{w: &buf, termWidth: 80, actions: make([]actionState, 100), maxLines: 5}
	for i := range r.actions {
		r.actions[i] = actionState{action: action.Action{Description: "completed package"}, status: statusDone}
	}
	r.ActionStarted(99, action.Action{Description: "remaining package"})
	r.ActionOutput(99, "noisy download progress")
	r.renderProgress()
	got := buf.String()
	if strings.Contains(got, "completed package") || strings.Contains(got, "noisy") {
		t.Fatalf("noisy frame: %q", got)
	}
	if !strings.Contains(got, "remaining package") || !strings.Contains(got, "99/100 complete") {
		t.Fatalf("missing progress: %q", got)
	}
	if strings.Count(got, "\n") != 2 {
		t.Fatalf("expected compact frame: %q", got)
	}
}

func TestFinalFailurePreservesFullCapturedError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &Renderer{w: &buf, termWidth: 20, actions: make([]actionState, 1), maxLines: 5}
	a := action.Action{Description: "brew upgrade example"}
	r.ActionStarted(0, a)
	for range 10 {
		r.ActionOutput(0, "cleanup noise")
	}
	detail := "cannot load a required file: " + strings.Repeat("x", 100)
	r.ActionCompleted(0, a, &tailErr{msg: "exit status 1", tail: "\x1b[31m" + detail + "\x1b[0m\n\tfrom command\n"})
	r.renderProgress()
	buf.Reset()
	r.render()
	got := buf.String()
	if !strings.Contains(got, detail) {
		t.Fatalf("lost error: %q", got)
	}
	if strings.Contains(got, "\t") || strings.Contains(got, "\x1b[31m") {
		t.Fatalf("unsafe terminal output: %q", got)
	}
	if r.lastLineCount != 0 {
		t.Fatal("final report should never be redrawn")
	}
}

func TestProgressBoundsManyRunningActions(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := &Renderer{w: &buf, termWidth: 80, actions: make([]actionState, 100)}
	for i := range r.actions {
		r.ActionStarted(i, action.Action{Description: "running"})
	}
	r.renderProgress()
	if strings.Count(buf.String(), "\n") >= 24 {
		t.Fatal("frame exceeds terminal height")
	}
	if !strings.Contains(buf.String(), "100 running") {
		t.Fatal("missing running count")
	}
}
