package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DefaultsHandler plans actions for macOS defaults declarations.
type DefaultsHandler struct{}

func (DefaultsHandler) DeclType() decl.Type { return decl.Defaults }
func (DefaultsHandler) DeclName() string     { return "Defaults" }

func (DefaultsHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	factKey := fmt.Sprintf("defaults:%s:%s", d.DefaultsDomain, d.DefaultsKey)
	defaultsFact, err := fact.Get(ctx, store, factKey, fact.DefaultsCollector{
		Domain: d.DefaultsDomain,
		Key:    d.DefaultsKey,
	})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffDefaults(action.DesiredDefault{
		Domain: d.DefaultsDomain,
		Key:    d.DefaultsKey,
		Value:  d.DefaultsValue,
		Absent: d.State == decl.Absent,
	}, defaultsFact)
	var out PlanOutput
	if len(acts) == 0 {
		msg := fmt.Sprintf("defaults %s %s (up to date)", d.DefaultsDomain, d.DefaultsKey)
		if d.State == decl.Absent {
			msg = fmt.Sprintf("defaults %s %s (already absent)", d.DefaultsDomain, d.DefaultsKey)
		}
		out.Observations = append(out.Observations, action.Observation{Description: msg})
	} else {
		out.Actions = acts
	}
	return out, nil
}
