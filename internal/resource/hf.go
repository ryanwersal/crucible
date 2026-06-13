package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// HFDownloadHandler plans actions for HuggingFace download declarations.
type HFDownloadHandler struct{}

func (HFDownloadHandler) DeclType() decl.Type { return decl.HFDownload }
func (HFDownloadHandler) DeclName() string    { return "HFDownload" }

func (HFDownloadHandler) Plan(ctx context.Context, store *fact.Store, _ Env, d decl.Declaration) (PlanOutput, error) {
	hfFact, err := fact.Get(ctx, store, "hf:"+d.HFDest, fact.HFCollector{Dest: d.HFDest})
	if err != nil {
		return PlanOutput{}, err
	}

	label := fmt.Sprintf("%s → %s", d.HFRepo, d.HFDest)

	// If hf isn't installed yet (e.g. first apply, before the package lands in
	// the same run), skip with a visible observation rather than erroring; a
	// later apply, once hf is on PATH, will download.
	if !hfFact.Available {
		return PlanOutput{Observations: []action.Observation{
			{Description: fmt.Sprintf("%s (hf not available — skipped)", label)},
		}}, nil
	}

	acts := action.DiffHF(action.DesiredHFDownload{
		Repo:     d.HFRepo,
		Dest:     d.HFDest,
		Include:  d.HFInclude,
		Exclude:  d.HFExclude,
		Revision: d.HFRevision,
		Absent:   d.State == decl.Absent,
	}, hfFact)

	if len(acts) == 0 {
		msg := fmt.Sprintf("%s (present)", label)
		if d.State == decl.Absent {
			msg = fmt.Sprintf("%s (already absent)", label)
		}
		return PlanOutput{Observations: []action.Observation{{Description: msg}}}, nil
	}
	return PlanOutput{Actions: acts}, nil
}
