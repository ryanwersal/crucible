package resource

import (
	"context"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

const keyRemapPlistName = "com.crucible.keyremap.plist"

// KeyRemapHandler plans actions for keyboard modifier remapping declarations.
type KeyRemapHandler struct{}

func (KeyRemapHandler) DeclType() decl.Type { return decl.KeyRemap }
func (KeyRemapHandler) DeclName() string    { return "KeyRemap" }

func (KeyRemapHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	info, err := fact.Get(ctx, store, "keyremap", fact.KeyRemapCollector{})
	if err != nil {
		return PlanOutput{}, err
	}

	plistPath := filepath.Join(env.TargetDir, "Library", "LaunchAgents", keyRemapPlistName)

	remaps := make([]action.KeyRemapEntry, len(d.KeyRemaps))
	for i, r := range d.KeyRemaps {
		remaps[i] = action.KeyRemapEntry{From: r.From, To: r.To}
	}

	acts := action.DiffKeyRemap(action.DesiredKeyRemap{
		Remaps: remaps,
		Absent: d.State == decl.Absent,
	}, info, plistPath)

	var out PlanOutput
	if len(acts) == 0 {
		desc := "key remappings (up to date)"
		if d.State == decl.Absent {
			desc = "no key remappings (up to date)"
		}
		out.Observations = append(out.Observations, action.Observation{
			Description: desc,
		})
	} else {
		out.Actions = acts
	}
	return out, nil
}
