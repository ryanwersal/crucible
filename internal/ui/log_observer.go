package ui

import (
	"errors"
	"log/slog"

	"github.com/ryanwersal/crucible/internal/action"
)

// outputCarrier is satisfied by errors that carry a captured-output tail
// (e.g. resource.CommandError). Defined here to avoid a ui → resource import
// just for type-checking.
type outputCarrier interface {
	error
	OutputTail() string
}

// LogObserver is a non-TTY ActionObserver that writes structured log lines.
type LogObserver struct {
	logger *slog.Logger
}

// NewLogObserver creates an observer that logs action lifecycle events.
func NewLogObserver(logger *slog.Logger) *LogObserver {
	return &LogObserver{logger: logger}
}

func (o *LogObserver) ActionStarted(_ int, a action.Action) {
	o.logger.Info("executing", "action", a.Type.String(), "description", a.Description)
}

func (o *LogObserver) ActionOutput(_ int, _ string) {
	// Non-TTY mode does not display per-action output lines.
}

func (o *LogObserver) ActionCompleted(_ int, a action.Action, err error) {
	if err == nil {
		o.logger.Info("action completed", "action", a.Type.String(), "description", a.Description)
		return
	}
	// Surface captured subprocess output as a dedicated attribute so it isn't
	// folded into the err string as escaped newlines.
	attrs := make([]any, 0, 8)
	attrs = append(attrs, "action", a.Type.String(), "description", a.Description, "err", err)
	if carrier, ok := errors.AsType[outputCarrier](err); ok {
		if tail := carrier.OutputTail(); tail != "" {
			attrs = append(attrs, "output", tail)
		}
	}
	o.logger.Error("action failed", attrs...)
}

func (o *LogObserver) Wait() {}
