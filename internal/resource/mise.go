package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// MiseToolHandler batches all mise tool declarations into a single diff.
type MiseToolHandler struct{}

func (MiseToolHandler) DeclType() decl.Type { return decl.MiseTool }
func (MiseToolHandler) DeclName() string     { return "MiseTool" }

func (MiseToolHandler) PlanBatch(ctx context.Context, store *fact.Store, env Env, decls []decl.Declaration) (PlanOutput, error) {
	tools := make([]action.DesiredMiseTool, len(decls))
	for i, d := range decls {
		tools[i] = action.DesiredMiseTool{
			Name:    d.MiseToolName,
			Version: d.MiseToolVersion,
			Absent:  d.State == decl.Absent,
		}
	}

	miseFact, err := fact.Get(ctx, store, "mise", fact.MiseCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	miseActions, err := action.DiffMise(tools, miseFact)
	if err != nil {
		return PlanOutput{}, err
	}

	hasAction := make(map[string]bool, len(miseActions))
	for _, a := range miseActions {
		hasAction[a.MiseToolName] = true
	}

	var out PlanOutput
	for _, tool := range tools {
		if !hasAction[tool.Name] {
			msg := fmt.Sprintf("mise %s (installed)", tool.Name)
			if tool.Absent {
				msg = fmt.Sprintf("mise %s (already absent)", tool.Name)
			}
			out.Observations = append(out.Observations, action.Observation{Description: msg})
		}
	}
	out.Actions = miseActions
	return out, nil
}
