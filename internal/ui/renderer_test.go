package ui

import (
	"fmt"
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

	for i := 0; i < 10; i++ {
		r.ActionOutput(0, fmt.Sprintf("line %d", i))
	}

	if len(r.actions[0].lines) != 3 {
		t.Errorf("lines = %d, want 3", len(r.actions[0].lines))
	}
	if r.actions[0].lines[0] != "line 7" {
		t.Errorf("oldest line = %q, want line 7", r.actions[0].lines[0])
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
			for i := 0; i < len(got); i++ {
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
