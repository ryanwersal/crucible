package action

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredFile describes the desired state of a file.
type DesiredFile struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
	Absent  bool // true = ensure the file does not exist
}

// DiffFile compares the desired file state against the actual state and returns
// the actions needed to reconcile them.
func DiffFile(desired DesiredFile, actual *fact.FileInfo) ([]Action, error) {
	if desired.Absent {
		if actual != nil && actual.Exists && !actual.IsDir {
			return []Action{{
				Type:        DeletePath,
				Path:        desired.Path,
				Description: fmt.Sprintf("remove file %s", desired.Path),
			}}, nil
		}
		return nil, nil
	}

	desiredHash := sha256Hex(desired.Content)

	if actual == nil || !actual.Exists {
		return []Action{{
			Type:        WriteFile,
			Path:        desired.Path,
			Content:     desired.Content,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("write %s (new file)", desired.Path),
		}}, nil
	}

	if actual.IsDir {
		return nil, fmt.Errorf("path conflict: %s exists as a directory", desired.Path)
	}

	var actions []Action

	if actual.IsLink {
		actions = append(actions, Action{
			Type:        DeletePath,
			Path:        desired.Path,
			Description: fmt.Sprintf("remove symlink %s (replacing with file)", desired.Path),
		})
		actions = append(actions, Action{
			Type:        WriteFile,
			Path:        desired.Path,
			Content:     desired.Content,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("write %s (was symlink)", desired.Path),
		})
		return actions, nil
	}

	if actual.Hash != desiredHash {
		actions = append(actions, Action{
			Type:        WriteFile,
			Path:        desired.Path,
			Content:     desired.Content,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("write %s (content changed)", desired.Path),
		})
		return actions, nil
	}

	if actual.Mode != desired.Mode {
		actions = append(actions, Action{
			Type:        SetPermissions,
			Path:        desired.Path,
			Mode:        desired.Mode,
			Description: fmt.Sprintf("chmod %s %04o → %04o", desired.Path, actual.Mode, desired.Mode),
		})
		return actions, nil
	}

	return nil, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
