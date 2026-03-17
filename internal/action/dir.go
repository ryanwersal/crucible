package action

import (
	"fmt"
	"io/fs"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredDir describes the desired state of a directory.
type DesiredDir struct {
	Path string
	Mode fs.FileMode
}

// DiffDir compares the desired directory state against the actual state.
func DiffDir(desired DesiredDir, actual *fact.DirInfo) []Action {
	if actual == nil || !actual.Exists {
		return []Action{{
			Type:        CreateDir,
			Path:        desired.Path,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("create directory %s", desired.Path),
		}}
	}

	if actual.Mode != desired.Mode {
		return []Action{{
			Type:        SetPermissions,
			Path:        desired.Path,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("chmod %s %04o → %04o", desired.Path, actual.Mode, desired.Mode),
		}}
	}

	return nil
}
