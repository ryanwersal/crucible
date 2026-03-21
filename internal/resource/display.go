package resource

import (
	"context"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DisplayHandler plans actions for display density declarations.
type DisplayHandler struct{}

func (DisplayHandler) DeclType() decl.Type { return decl.Display }
func (DisplayHandler) DeclName() string    { return "Display" }

func (DisplayHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	info, err := fact.Get(ctx, store, "display", fact.DisplayCollector{})
	if err != nil {
		return PlanOutput{}, err
	}

	acts := action.DiffDisplay(action.DesiredDisplay{
		SidebarIconSize: d.DisplaySidebarIconSize,
		MenuBarSpacing:  d.DisplayMenuBarSpacing,
		Resolution:      d.DisplayResolution,
		HZ:              d.DisplayHZ,
	}, info)

	var out PlanOutput
	if len(acts) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: "display density (up to date)",
		})
	} else {
		out.Actions = acts
	}
	return out, nil
}
