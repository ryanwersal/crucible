package resource

import (
	"context"
	"io"
	"os"

	"github.com/ryanwersal/crucible/internal/action"
)

// DeletePathExecutor handles path deletion for file, dir, symlink, and font removal.
type DeletePathExecutor struct{}

func (DeletePathExecutor) ActionType() action.Type { return action.DeletePath }
func (DeletePathExecutor) ActionName() string      { return "DeletePath" }

func (DeletePathExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	if a.Recursive {
		return os.RemoveAll(a.Path)
	}
	return os.Remove(a.Path)
}
