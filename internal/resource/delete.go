package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/ryanwersal/crucible/internal/action"
)

// DeletePathExecutor handles path deletion for file, dir, symlink, and font removal.
type DeletePathExecutor struct{}

func (DeletePathExecutor) ActionType() action.Type { return action.DeletePath }
func (DeletePathExecutor) ActionName() string      { return "DeletePath" }

func (DeletePathExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	if a.Seed {
		info, err := os.Lstat(a.Path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect seed migration: %w", err)
		}
		if info.Mode().IsRegular() {
			return nil
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return fmt.Errorf("path conflict: %s is not a symlink or regular file", a.Path)
		}
		return os.Remove(a.Path)
	}
	if a.Recursive {
		return os.RemoveAll(a.Path)
	}
	return os.Remove(a.Path)
}
