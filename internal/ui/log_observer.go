package ui

import (
	"log/slog"

	"github.com/ryanwersal/crucible/internal/action"
)

// LogObserver is a non-TTY ActionObserver that writes structured log lines.
type LogObserver struct {
	logger *slog.Logger
}

// NewLogObserver creates an observer that logs action lifecycle events.
func NewLogObserver(logger *slog.Logger) *LogObserver {
	return &LogObserver{logger: logger}
}

func (o *LogObserver) ActionStarted(index int, a action.Action) {
	o.logger.Info("executing", "action", a.Type.String(), "description", a.Description)
}

func (o *LogObserver) ActionOutput(_ int, _ string) {
	// Non-TTY mode does not display per-action output lines.
}

func (o *LogObserver) ActionCompleted(_ int, a action.Action, err error) {
	if err != nil {
		o.logger.Error("action failed", "action", a.Type.String(), "description", a.Description, "err", err)
	} else {
		o.logger.Info("action completed", "action", a.Type.String(), "description", a.Description)
	}
}

func (o *LogObserver) Wait() {}
