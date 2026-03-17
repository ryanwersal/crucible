package fact

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// SymlinkInfo holds the observed state of a symlink.
type SymlinkInfo struct {
	Exists bool
	Target string // what the symlink points to; empty if !Exists
}

// SymlinkCollector collects facts about a symlink at Path.
type SymlinkCollector struct {
	Path string
}

func (s SymlinkCollector) Collect(_ context.Context) (*SymlinkInfo, error) {
	target, err := os.Readlink(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &SymlinkInfo{Exists: false}, nil
	}
	if err != nil {
		// Path exists but isn't a symlink
		return &SymlinkInfo{Exists: false}, nil
	}
	return &SymlinkInfo{Exists: true, Target: target}, nil
}
