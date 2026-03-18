package action

import (
	"slices"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredDock describes the desired macOS Dock layout.
type DesiredDock struct {
	Apps    []string
	Folders []DockFolder
}

// DiffDock compares the desired dock layout against the current state.
// The dock is managed as a whole unit — any difference emits a single SetDock action.
func DiffDock(desired DesiredDock, actual *fact.DockInfo) []Action {
	if actual != nil && dockMatches(desired, actual) {
		return nil
	}

	return []Action{{
		Type:        SetDock,
		DockApps:    desired.Apps,
		DockFolders: desired.Folders,
		Description: "set dock layout",
	}}
}

func dockMatches(desired DesiredDock, actual *fact.DockInfo) bool {
	if !slices.Equal(desired.Apps, actual.Apps) {
		return false
	}

	if len(desired.Folders) != len(actual.Folders) {
		return false
	}

	for i, df := range desired.Folders {
		af := actual.Folders[i]
		if df.Path != af.Path || df.View != af.View || df.Display != af.Display {
			return false
		}
	}

	return true
}
