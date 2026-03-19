package resource

import (
	"context"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DockHandler plans actions for dock layout declarations.
type DockHandler struct{}

func (DockHandler) DeclType() decl.Type { return decl.Dock }

func (DockHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	dockFact, err := fact.Get(ctx, store, "dock", fact.DockCollector{HomeDir: env.TargetDir})
	if err != nil {
		return PlanOutput{}, err
	}
	desiredFolders := make([]action.DockFolder, len(d.DockFolders))
	for i, f := range d.DockFolders {
		desiredFolders[i] = action.DockFolder{
			Path:    f.Path,
			View:    f.View,
			Display: f.Display,
		}
	}
	acts := action.DiffDock(action.DesiredDock{
		Apps:    d.DockApps,
		Folders: desiredFolders,
	}, dockFact)
	var out PlanOutput
	if len(acts) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: "dock layout (up to date)",
		})
	} else {
		out.Actions = acts
	}
	return out, nil
}
