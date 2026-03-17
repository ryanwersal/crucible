package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredSymlink describes the desired state of a symlink.
type DesiredSymlink struct {
	Path   string
	Target string
}

// DiffSymlink compares the desired symlink state against the actual state.
func DiffSymlink(desired DesiredSymlink, actual *fact.SymlinkInfo) []Action {
	if actual == nil || !actual.Exists {
		return []Action{{
			Type:        CreateSymlink,
			Path:        desired.Path,
			LinkTarget:  desired.Target,
			Description: fmt.Sprintf("create symlink %s → %s", desired.Path, desired.Target),
		}}
	}

	if actual.Target != desired.Target {
		return []Action{
			{
				Type:        DeletePath,
				Path:        desired.Path,
				Description: fmt.Sprintf("remove symlink %s (target changed)", desired.Path),
			},
			{
				Type:        CreateSymlink,
				Path:        desired.Path,
				LinkTarget:  desired.Target,
				Description: fmt.Sprintf("create symlink %s → %s", desired.Path, desired.Target),
			},
		}
	}

	return nil
}
