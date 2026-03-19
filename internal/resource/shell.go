package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// ShellHandler plans actions for shell declarations.
type ShellHandler struct{}

func (ShellHandler) DeclType() decl.Type { return decl.Shell }
func (ShellHandler) DeclName() string    { return "Shell" }

func (ShellHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	shellFact, err := fact.Get(ctx, store, "shell:"+d.ShellUsername, fact.ShellCollector{Username: d.ShellUsername})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffShell(action.DesiredShell{
		Path:     d.ShellPath,
		Username: d.ShellUsername,
	}, shellFact)
	var out PlanOutput
	if len(acts) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: fmt.Sprintf("shell %s (up to date)", d.ShellPath),
		})
	} else {
		out.Actions = acts
	}
	return out, nil
}
