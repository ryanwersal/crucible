package script

import (
	"errors"
	"testing"

	"github.com/dop251/goja"
)

func TestScriptError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *ScriptError
		want string
	}{
		{
			name: "with file",
			err:  &ScriptError{File: "crucible.js", Message: "something broke"},
			want: "crucible.js: something broke",
		},
		{
			name: "without file",
			err:  &ScriptError{Message: "something broke"},
			want: "something broke",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScriptError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("root cause")
	se := &ScriptError{Message: "wrapped", Cause: cause}

	if !errors.Is(se, cause) {
		t.Error("expected errors.Is to find cause")
	}
}

func TestWrapGojaError_Nil(t *testing.T) {
	t.Parallel()

	if err := wrapGojaError(nil, "test.js"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWrapGojaError_GojaException(t *testing.T) {
	t.Parallel()

	vm := goja.New()
	_, err := vm.RunString("throw new Error('boom')")
	if err == nil {
		t.Fatal("expected error from throw")
	}

	wrapped := wrapGojaError(err, "test.js")
	var se *ScriptError
	if !errors.As(wrapped, &se) {
		t.Fatal("expected ScriptError")
	}
	if se.File != "test.js" {
		t.Errorf("file = %q, want test.js", se.File)
	}
	if se.Stack == "" {
		t.Error("expected non-empty stack")
	}
}

func TestWrapGojaError_NonGoja(t *testing.T) {
	t.Parallel()

	orig := errors.New("plain error")
	got := wrapGojaError(orig, "test.js")

	if got != orig {
		t.Error("expected original error to pass through")
	}
}
