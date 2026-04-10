package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// ScriptHandler plans actions for script-installed tool declarations.
type ScriptHandler struct{}

func (ScriptHandler) DeclType() decl.Type { return decl.Script }
func (ScriptHandler) DeclName() string    { return "Script" }

func (ScriptHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	scriptFact, err := fact.Get(ctx, store, "script:"+d.ScriptName, fact.ScriptCollector{Check: d.ScriptCheck})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffScript(action.DesiredScript{
		Name:    d.ScriptName,
		Install: d.ScriptInstall,
	}, scriptFact)
	var out PlanOutput
	if len(acts) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: fmt.Sprintf("%s (installed)", d.ScriptName),
		})
	} else {
		out.Actions = acts
	}
	return out, nil
}
