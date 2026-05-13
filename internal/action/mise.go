package action

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredMiseTool describes a tool that should be globally installed via mise (or removed).
type DesiredMiseTool struct {
	Name    string // tool name (e.g. "python", "node")
	Version string // version spec (e.g. "3.12", "latest")
	Absent  bool   // true = ensure the tool is not installed
}

// MiseVersionResolver resolves a mise version spec to a concrete version,
// so DiffMise can recognize that an installed concrete version (e.g. "2.92.0")
// already satisfies a non-concrete spec (e.g. "latest" or "2").
type MiseVersionResolver interface {
	Resolve(ctx context.Context, name, spec string) (string, error)
}

// DiffMise compares desired mise tools against the currently installed set
// and returns actions for any that need installation, upgrade, or removal.
//
// When the desired version spec doesn't string-match the installed concrete
// version, the resolver is consulted to determine the spec's current resolution.
// This avoids treating "latest" or prefix specs (e.g. "2") as perpetually
// out-of-date when the installed version already satisfies them.
func DiffMise(ctx context.Context, desired []DesiredMiseTool, actual *fact.MiseInfo, resolver MiseVersionResolver) ([]Action, error) {
	if actual == nil || !actual.Available {
		return nil, fmt.Errorf("mise is not available")
	}

	var actions []Action
	for _, d := range desired {
		installedVersion, installed := actual.Globals[d.Name]
		if d.Absent {
			if installed {
				actions = append(actions, Action{
					Type:         UninstallMiseTool,
					MiseToolName: d.Name,
					Description:  fmt.Sprintf("mise uninstall %s", d.Name),
				})
			}
			continue
		}
		if !installed {
			actions = append(actions, Action{
				Type:            InstallMiseTool,
				MiseToolName:    d.Name,
				MiseToolVersion: d.Version,
				Description:     fmt.Sprintf("mise use --global %s@%s", d.Name, d.Version),
			})
			continue
		}
		if installedVersion == d.Version {
			continue
		}
		// Spec and installed concrete version differ as strings. Ask the
		// resolver what the spec means right now — if it matches the
		// installed version, we're already up to date.
		target := d.Version
		if resolver != nil {
			if r, err := resolver.Resolve(ctx, d.Name, d.Version); err == nil && r != "" {
				target = r
			}
		}
		if installedVersion == target {
			continue
		}
		actions = append(actions, Action{
			Type:            InstallMiseTool,
			MiseToolName:    d.Name,
			MiseToolVersion: d.Version,
			Description:     fmt.Sprintf("mise use --global %s@%s (%s → %s)", d.Name, d.Version, installedVersion, target),
		})
	}

	return actions, nil
}
