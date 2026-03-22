package engine

import "github.com/ryanwersal/crucible/internal/action"

// ActionObserver receives lifecycle events during action execution.
// Implementations control how execution progress is displayed.
type ActionObserver interface {
	// ActionStarted is called when an action begins executing.
	ActionStarted(index int, a action.Action)
	// ActionOutput is called with each line of stdout/stderr output.
	ActionOutput(index int, line string)
	// ActionCompleted is called when an action finishes, with a nil error on success.
	ActionCompleted(index int, a action.Action, err error)
	// Wait blocks until all rendering is finished (e.g. final repaint).
	Wait()
}

// ApplyOptions configures concurrent action execution.
type ApplyOptions struct {
	Concurrency int            // max parallel actions; 0 or 1 means sequential
	Observer    ActionObserver // receives lifecycle events; nil disables callbacks
}

// ActionResult records the outcome of a single action.
type ActionResult struct {
	Action action.Action
	Err    error
}

// ApplyResult holds the outcome of applying all actions.
type ApplyResult struct {
	Results []ActionResult
}

// Succeeded returns the actions that completed without error.
func (r ApplyResult) Succeeded() []action.Action {
	out := make([]action.Action, 0, len(r.Results))
	for _, ar := range r.Results {
		if ar.Err == nil {
			out = append(out, ar.Action)
		}
	}
	return out
}

// Errors returns only the results that failed.
func (r ApplyResult) Errors() []ActionResult {
	var out []ActionResult
	for _, ar := range r.Results {
		if ar.Err != nil {
			out = append(out, ar)
		}
	}
	return out
}
