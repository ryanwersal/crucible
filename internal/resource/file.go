package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// FileHandler plans actions for file declarations.
type FileHandler struct{}

func (FileHandler) DeclType() decl.Type { return decl.File }
func (FileHandler) DeclName() string    { return "File" }

func (FileHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	fileFact, err := fact.Get(ctx, store, "file:"+d.Path, fact.FileCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	acts, err := action.DiffFile(action.DesiredFile{
		Path:    d.Path,
		Content: d.Content,
		Mode:    d.Mode,
		Absent:  d.State == decl.Absent,
	}, fileFact)
	if err != nil {
		return PlanOutput{}, err
	}
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
