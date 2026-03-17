package script

import (
	"fmt"

	"github.com/dop251/goja"
)

// ScriptError wraps errors originating from JavaScript execution with
// file location and stack trace information.
type ScriptError struct {
	File    string
	Message string
	Stack   string // JS stack trace
	Cause   error
}

func (e *ScriptError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: %s", e.File, e.Message)
	}
	return e.Message
}

func (e *ScriptError) Unwrap() error {
	return e.Cause
}

// wrapGojaError converts a goja exception or interrupt into a ScriptError.
// Non-goja errors are returned unchanged.
func wrapGojaError(err error, file string) error {
	if err == nil {
		return nil
	}

	if exc, ok := err.(*goja.Exception); ok {
		return &ScriptError{
			File:    file,
			Message: exc.Value().String(),
			Stack:   exc.String(),
			Cause:   exc,
		}
	}

	if intr, ok := err.(*goja.InterruptedError); ok {
		return &ScriptError{
			File:    file,
			Message: fmt.Sprint(intr.Value()),
			Stack:   intr.String(),
			Cause:   intr,
		}
	}

	return err
}
