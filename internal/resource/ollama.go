package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// OllamaModelHandler batches all Ollama model declarations into a single diff
// against the locally installed model set.
type OllamaModelHandler struct{}

func (OllamaModelHandler) DeclType() decl.Type { return decl.OllamaModel }
func (OllamaModelHandler) DeclName() string    { return "OllamaModel" }

func (OllamaModelHandler) PlanBatch(ctx context.Context, store *fact.Store, _ Env, decls []decl.Declaration) (PlanOutput, error) {
	ollamaFact, err := fact.Get(ctx, store, "ollama", fact.OllamaCollector{})
	if err != nil {
		return PlanOutput{}, err
	}

	// If ollama isn't installed yet (e.g. first apply, before the package is
	// installed in the same run), we can't act on models. Skip with a visible
	// observation rather than erroring out the whole plan — a subsequent apply,
	// once ollama is on PATH, will pull them.
	if !ollamaFact.Available {
		var out PlanOutput
		for _, d := range decls {
			out.Observations = append(out.Observations, action.Observation{
				Description: fmt.Sprintf("%s (ollama not available — skipped)", d.OllamaModel),
			})
		}
		return out, nil
	}

	desired := make([]action.DesiredOllamaModel, len(decls))
	for i, d := range decls {
		desired[i] = action.DesiredOllamaModel{
			Ref:    d.OllamaModel,
			Absent: d.State == decl.Absent,
		}
	}

	ollamaActions := action.DiffOllama(desired, ollamaFact)

	hasAction := make(map[string]bool, len(ollamaActions))
	for _, a := range ollamaActions {
		hasAction[a.OllamaModel] = true
	}

	var out PlanOutput
	for _, d := range desired {
		if !hasAction[d.Ref] {
			msg := fmt.Sprintf("ollama %s (installed)", d.Ref)
			if d.Absent {
				msg = fmt.Sprintf("ollama %s (already absent)", d.Ref)
			}
			out.Observations = append(out.Observations, action.Observation{Description: msg})
		}
	}
	out.Actions = ollamaActions
	return out, nil
}
