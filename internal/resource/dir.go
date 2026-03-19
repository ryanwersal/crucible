package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DirHandler plans actions for directory declarations.
type DirHandler struct{}

func (DirHandler) DeclType() decl.Type { return decl.Dir }

func (DirHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	dirFact, err := fact.Get(ctx, store, "dir:"+d.Path, fact.DirCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffDir(action.DesiredDir{
		Path:   d.Path,
		Mode:   d.Mode,
		Absent: d.State == decl.Absent,
	}, dirFact)
	var out PlanOutput
	if len(acts) == 0 {
		msg := fmt.Sprintf("%s (up to date)", d.Path)
		if d.State == decl.Absent {
			msg = fmt.Sprintf("%s (already absent)", d.Path)
		}
		out.Observations = append(out.Observations, action.Observation{Description: msg})
	} else {
		out.Actions = acts
	}
	return out, nil
}
