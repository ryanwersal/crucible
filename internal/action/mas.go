package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredMasApp describes a Mac App Store app that should be installed.
type DesiredMasApp struct {
	ID   int64
	Name string
}

// DiffMas compares desired Mac App Store apps against installed state.
func DiffMas(desired []DesiredMasApp, actual *fact.MasInfo) ([]Action, error) {
	if !actual.Available {
		return nil, fmt.Errorf("mas is required but not installed")
	}

	var actions []Action
	for _, app := range desired {
		if _, ok := actual.Apps[app.ID]; !ok {
			desc := fmt.Sprintf("mas install %d", app.ID)
			if app.Name != "" {
				desc = fmt.Sprintf("mas install %d (%s)", app.ID, app.Name)
			}
			actions = append(actions, Action{
				Type:        InstallMasApp,
				MasAppID:    app.ID,
				MasAppName:  app.Name,
				Description: desc,
			})
		}
	}
	return actions, nil
}
