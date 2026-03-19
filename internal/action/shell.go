package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredShell describes the login shell that should be set for a user.
type DesiredShell struct {
	Path     string // e.g. "/opt/homebrew/bin/zsh"
	Username string
}

// DiffShell compares the desired login shell against the current one.
func DiffShell(desired DesiredShell, actual *fact.ShellInfo) []Action {
	if actual != nil && actual.Path == desired.Path {
		return nil
	}

	return []Action{{
		Type:          SetShell,
		ShellPath:     desired.Path,
		ShellUsername: desired.Username,
		Description:   fmt.Sprintf("chsh -s %s %s", desired.Path, desired.Username),
	}}
}
