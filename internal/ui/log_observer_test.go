package ui

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

// tailErr is a minimal error type that satisfies outputCarrier, used to verify
// LogObserver surfaces captured output as a dedicated attribute.
type tailErr struct {
	msg  string
	tail string
}

func (e *tailErr) Error() string      { return e.msg }
func (e *tailErr) OutputTail() string { return e.tail }

func TestLogObserver_FailureSurfacesOutputAttr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	o := NewLogObserver(logger)

	a := action.Action{Type: action.InstallPackage, Description: "brew install foo"}
	o.ActionCompleted(0, a, &tailErr{msg: "exit status 1", tail: "Error: No formula\nsecond"})

	got := buf.String()
	if !strings.Contains(got, `level=ERROR`) {
		t.Errorf("expected ERROR level entry, got: %q", got)
	}
	if !strings.Contains(got, `err="exit status 1"`) {
		t.Errorf("expected err attr, got: %q", got)
	}
	if !strings.Contains(got, "output=") {
		t.Errorf("expected output attr present, got: %q", got)
	}
	if !strings.Contains(got, "Error: No formula") {
		t.Errorf("expected captured tail content, got: %q", got)
	}
}

func TestLogObserver_FailureWithoutOutputCarrier(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	o := NewLogObserver(logger)

	a := action.Action{Type: action.WriteFile, Description: "write ~/.gitconfig"}
	o.ActionCompleted(0, a, errors.New("permission denied"))

	got := buf.String()
	if !strings.Contains(got, `level=ERROR`) {
		t.Errorf("expected ERROR level entry, got: %q", got)
	}
	if strings.Contains(got, "output=") {
		t.Errorf("plain errors should not produce an output attr, got: %q", got)
	}
}

func TestLogObserver_FailureWithEmptyTailOmitsAttr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	o := NewLogObserver(logger)

	a := action.Action{Type: action.InstallPackage, Description: "brew install foo"}
	o.ActionCompleted(0, a, &tailErr{msg: "exit status 1", tail: ""})

	got := buf.String()
	if strings.Contains(got, "output=") {
		t.Errorf("empty tail must not produce an output attr, got: %q", got)
	}
}

func TestLogObserver_SuccessLogsInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	o := NewLogObserver(logger)

	a := action.Action{Type: action.InstallPackage, Description: "brew install foo"}
	o.ActionCompleted(0, a, nil)

	got := buf.String()
	if !strings.Contains(got, `level=INFO`) {
		t.Errorf("expected INFO level entry, got: %q", got)
	}
	if !strings.Contains(got, "action completed") {
		t.Errorf("expected 'action completed' message, got: %q", got)
	}
}
