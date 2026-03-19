package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// MasHandler batches all MasApp declarations into a single diff.
type MasHandler struct{}

func (MasHandler) DeclType() decl.Type { return decl.MasApp }

func (MasHandler) PlanBatch(ctx context.Context, store *fact.Store, env Env, decls []decl.Declaration) (PlanOutput, error) {
	apps := make([]action.DesiredMasApp, len(decls))
	for i, d := range decls {
		apps[i] = action.DesiredMasApp{
			ID:   d.MasAppID,
			Name: d.MasAppName,
		}
	}

	masFact, err := fact.Get(ctx, store, "mas", fact.MasCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	masActions, err := action.DiffMas(apps, masFact)
	if err != nil {
		return PlanOutput{}, err
	}

	hasAction := make(map[int64]bool, len(masActions))
	for _, a := range masActions {
		hasAction[a.MasAppID] = true
	}

	var out PlanOutput
	for _, app := range apps {
		if !hasAction[app.ID] {
			name := app.Name
			if name == "" {
				name = fmt.Sprintf("%d", app.ID)
			}
			out.Observations = append(out.Observations, action.Observation{
				Description: fmt.Sprintf("%s (installed)", name),
			})
		}
	}
	out.Actions = masActions
	return out, nil
}
