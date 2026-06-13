package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredOllamaModel describes a model that should be present locally (pulled)
// or absent (removed).
type DesiredOllamaModel struct {
	Ref    string // model reference as written by the user, e.g. "llama3.1:8b"
	Absent bool   // true = ensure the model is not installed
}

// DiffOllama compares desired Ollama models against the locally installed set
// and returns pull/remove actions for any that differ. The caller is expected
// to have already confirmed ollama is available; an installed model is matched
// in canonical form so equivalent spellings don't trigger a redundant pull.
//
// Pull and remove actions share the "ollama" serial group so they never run
// concurrently — model weights are large and the local registry takes per-blob
// locks, so serializing avoids contention and surprising bandwidth spikes.
func DiffOllama(desired []DesiredOllamaModel, actual *fact.OllamaInfo) []Action {
	var actions []Action
	for _, d := range desired {
		installed := actual.Has(d.Ref)
		switch {
		case d.Absent && installed:
			actions = append(actions, Action{
				Type:        RemoveOllamaModel,
				OllamaModel: d.Ref,
				SerialGroup: "ollama",
				Description: fmt.Sprintf("ollama rm %s", d.Ref),
			})
		case d.Absent:
			// Already absent — nothing to do.
		case installed:
			// Already present — nothing to do.
		default:
			actions = append(actions, Action{
				Type:        PullOllamaModel,
				OllamaModel: d.Ref,
				SerialGroup: "ollama",
				Description: fmt.Sprintf("ollama pull %s", d.Ref),
			})
		}
	}
	return actions
}
