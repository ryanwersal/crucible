package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredScript describes a tool that should be installed via a shell command.
type DesiredScript struct {
	Name    string // human-readable name (e.g. "claude-code")
	Install string // shell command to install the tool
}

// DiffScript compares the desired script-installed tool against its current state.
func DiffScript(desired DesiredScript, actual *fact.ScriptInfo) []Action {
	if actual != nil && actual.Installed {
		return nil
	}

	return []Action{{
		Type:          RunScript,
		ScriptName:    desired.Name,
		ScriptInstall: desired.Install,
		Description:   fmt.Sprintf("run: %s", desired.Install),
	}}
}
