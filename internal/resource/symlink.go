package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// SymlinkHandler plans actions for symlink declarations.
type SymlinkHandler struct{}

func (SymlinkHandler) DeclType() decl.Type { return decl.Symlink }

func (SymlinkHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	symlinkFact, err := fact.Get(ctx, store, "symlink:"+d.Path, fact.SymlinkCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffSymlink(action.DesiredSymlink{
		Path:   d.Path,
		Target: d.LinkTarget,
		Absent: d.State == decl.Absent,
	}, symlinkFact)
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
