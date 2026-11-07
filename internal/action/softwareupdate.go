package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

func DiffSoftwareUpdates(requests []softwareupdate.Request, actual *fact.SoftwareUpdateInfo) ([]Action, []Observation, error) {
	var actions []Action
	var observations []Observation
	selected := make(map[string]softwareupdate.Request)
	for _, request := range requests {
		if err := request.Validate(); err != nil {
			return nil, nil, err
		}
		installed := actual.Receipts[request.Receipt] && (!request.CommandLineTools || actual.CLTUsable)
		if !request.Latest && installed {
			observations = append(observations, Observation{Description: request.Name + " (installed)"})
			continue
		}
		update, err := request.Select(actual.Updates)
		if err != nil {
			return nil, nil, err
		}
		if update == nil {
			if request.Receipt != "" && !installed {
				return nil, nil, fmt.Errorf("%s is missing or incomplete, but Apple offers no matching update; install a compatible package from Apple Developer Downloads", request.Name)
			}
			observations = append(observations, Observation{Description: request.Name + " (no matching update offered)"})
			continue
		}
		if update.Restart && !request.Restart {
			return nil, nil, fmt.Errorf("%s requires a restart; declare restart: true to authorize it", update.Label)
		}
		if previous, exists := selected[update.Label]; exists {
			if previous != request {
				return nil, nil, fmt.Errorf("overlapping software update declarations select %s", update.Label)
			}
			continue
		}
		selected[update.Label] = request
		description := "install Apple update " + update.Label
		note := "Only this catalog label will be installed; the selected developer directory is preserved."
		if update.Restart {
			description += " and restart"
			note = "This update can restart or shut down the machine. Apple may require volume-owner authentication."
		}
		actions = append(actions, Action{Type: InstallSoftwareUpdate, SoftwareUpdate: *update, SoftwareUpdateRequest: request, NeedsSudo: true, PTY: true, SerialGroup: "apple-softwareupdate", Description: description, Note: note})
	}
	return actions, observations, nil
}
