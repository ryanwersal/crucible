package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredMiseTool describes a tool that should be globally installed via mise (or removed).
type DesiredMiseTool struct {
	Name    string // tool name (e.g. "python", "node")
	Version string // version spec (e.g. "3.12", "latest")
	Absent  bool   // true = ensure the tool is not installed
}

// DiffMise compares desired mise tools against the currently installed set
// and returns actions for any that are missing.
func DiffMise(desired []DesiredMiseTool, actual *fact.MiseInfo) ([]Action, error) {
	if actual == nil || !actual.Available {
		return nil, fmt.Errorf("mise is not available")
	}

	var actions []Action
	for _, d := range desired {
		installed := actual.Globals[d.Name]
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
		}
	}

	return actions, nil
}
