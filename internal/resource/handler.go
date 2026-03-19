package resource

import (
	"context"
	"io"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// Env carries directory paths needed by handlers during planning.
type Env struct {
	SourceDir string
	TargetDir string
}

// PlanOutput holds the actions and observations produced by a handler.
type PlanOutput struct {
	Actions      []action.Action
	Observations []action.Observation
}

// Handler diffs a single declaration against collected facts.
type Handler interface {
	DeclType() decl.Type
	DeclName() string
	Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error)
}

// BatchHandler accumulates all declarations of its type, then diffs them together.
type BatchHandler interface {
	DeclType() decl.Type
	DeclName() string
	PlanBatch(ctx context.Context, store *fact.Store, env Env, decls []decl.Declaration) (PlanOutput, error)
}

// ActionExecutor applies a single action type.
type ActionExecutor interface {
	ActionType() action.Type
	ActionName() string
	Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error
}
