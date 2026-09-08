package action

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

type DesiredVSCodeExtension struct {
	ID     string
	Absent bool
}

func DiffVSCode(desired []DesiredVSCodeExtension, actual *fact.VSCodeInfo) ([]Action, error) {
	var actions []Action
	seen := make(map[string]bool, len(desired))
	for _, d := range desired {
		id := strings.ToLower(d.ID)
		if absent, ok := seen[id]; ok {
			if absent != d.Absent {
				return nil, fmt.Errorf("conflicting states for VS Code extension %s", id)
			}
			continue
		}
		seen[id] = d.Absent
		if actual.Bundled[id] {
			if d.Absent {
				return nil, fmt.Errorf("VS Code extension %s is bundled and cannot be uninstalled", id)
			}
			continue
		}
		if actual.Command == "" || actual.Extensions[id] != d.Absent {
			continue
		}
		typ, verb := InstallVSCodeExtension, "install"
		if d.Absent {
			typ, verb = UninstallVSCodeExtension, "uninstall"
		}
		actions = append(actions, Action{
			Type: typ, VSCodeExtension: id, VSCodeCommand: actual.Command,
			Description: fmt.Sprintf("code --%s-extension %s", verb, id),
			SerialGroup: "vscode",
		})
	}
	return actions, nil
}
