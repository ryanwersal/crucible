package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredSymlink describes the desired state of a symlink.
type DesiredSymlink struct {
	Path   string
	Target string
	Absent bool // true = ensure the symlink does not exist
}

// DiffSymlink compares the desired symlink state against the actual state.
//
// When something that ISN'T a symlink occupies the path (a regular file, a
// directory, …), removing it would destroy user content. The returned
// DeletePath action carries Destructive=true with a reason so the apply flow
// can gate behind an explicit confirmation.
func DiffSymlink(desired DesiredSymlink, actual *fact.SymlinkInfo) []Action {
	if desired.Absent {
		if actual.Exists() {
			return []Action{{
				Type:        DeletePath,
				Path:        desired.Path,
				Recursive:   actual.Kind == fact.PathDirectory,
				Description: fmt.Sprintf("remove %s %s", actual.Kind, desired.Path),
				// Removing a non-symlink under an "absent" declaration is also
				// destructive — the user said "this symlink shouldn't exist",
				// but what's actually there isn't a symlink.
				Destructive:       actual.Kind != fact.PathSymlink && actual.Kind != fact.PathMissing,
				DestructiveReason: destructiveReason(actual.Kind, desired.Path),
			}}
		}
		return nil
	}

	switch {
	case !actual.Exists():
		return []Action{{
			Type:        CreateSymlink,
			Path:        desired.Path,
			LinkTarget:  desired.Target,
			Description: fmt.Sprintf("create symlink %s → %s", desired.Path, desired.Target),
		}}

	case actual.IsSymlink():
		if actual.Target == desired.Target {
			return nil
		}
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

	default:
		// A regular file, directory, or other non-symlink occupies the path.
		// Plan a destructive removal followed by the symlink creation.
		return []Action{
			{
				Type:              DeletePath,
				Path:              desired.Path,
				Recursive:         actual.Kind == fact.PathDirectory,
				Description:       fmt.Sprintf("remove %s %s", actual.Kind, desired.Path),
				Destructive:       true,
				DestructiveReason: destructiveReason(actual.Kind, desired.Path),
			},
			{
				Type:        CreateSymlink,
				Path:        desired.Path,
				LinkTarget:  desired.Target,
				Description: fmt.Sprintf("create symlink %s → %s", desired.Path, desired.Target),
			},
		}
	}
}

func destructiveReason(kind fact.PathKind, path string) string {
	switch kind {
	case fact.PathRegularFile:
		return fmt.Sprintf("regular file at %s would be deleted", path)
	case fact.PathDirectory:
		return fmt.Sprintf("directory at %s would be deleted recursively", path)
	case fact.PathOther:
		return fmt.Sprintf("non-symlink entry at %s would be deleted", path)
	default:
		return ""
	}
}
