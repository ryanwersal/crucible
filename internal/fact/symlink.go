package fact

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// PathKind classifies what (if anything) is at a path on disk. It's coarser
// than fs.FileMode — handlers only need to know "is there content that would
// be destroyed if I overwrite this".
type PathKind int

const (
	// PathMissing: nothing exists at the path.
	PathMissing PathKind = iota
	// PathSymlink: a symbolic link (regardless of whether its target resolves).
	PathSymlink
	// PathRegularFile: a regular file.
	PathRegularFile
	// PathDirectory: a directory.
	PathDirectory
	// PathOther: exists but is none of the above (device, socket, fifo, etc).
	PathOther
)

func (k PathKind) String() string {
	switch k {
	case PathMissing:
		return "missing"
	case PathSymlink:
		return "symlink"
	case PathRegularFile:
		return "regular file"
	case PathDirectory:
		return "directory"
	case PathOther:
		return "other"
	default:
		return "unknown"
	}
}

// SymlinkInfo holds the observed state of a path that a symlink declaration
// wants to manage. Kind reports what's actually on disk; Target is the link's
// pointed-at path when Kind == PathSymlink, otherwise empty.
type SymlinkInfo struct {
	Kind   PathKind
	Target string
}

// Exists reports whether *anything* is at the path (symlink, file, dir, …).
// Retained for callers that don't care about the specific kind.
func (s *SymlinkInfo) Exists() bool {
	return s != nil && s.Kind != PathMissing
}

// IsSymlink reports whether the path is specifically a symlink.
func (s *SymlinkInfo) IsSymlink() bool {
	return s != nil && s.Kind == PathSymlink
}

// SymlinkCollector collects facts about the path at Path. It deliberately uses
// Lstat (not Stat) so a symlink pointing at a regular file still reports as a
// symlink — we care about the link itself, not its target.
type SymlinkCollector struct {
	Path string
}

func (s SymlinkCollector) Collect(_ context.Context) (*SymlinkInfo, error) {
	info, err := os.Lstat(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &SymlinkInfo{Kind: PathMissing}, nil
	}
	if err != nil {
		return nil, err
	}

	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(s.Path)
		if err != nil {
			// The path is a symlink (Lstat said so) but Readlink failed —
			// surface what we know and let the caller decide.
			return &SymlinkInfo{Kind: PathSymlink}, nil
		}
		return &SymlinkInfo{Kind: PathSymlink, Target: target}, nil
	case mode.IsRegular():
		return &SymlinkInfo{Kind: PathRegularFile}, nil
	case mode.IsDir():
		return &SymlinkInfo{Kind: PathDirectory}, nil
	default:
		return &SymlinkInfo{Kind: PathOther}, nil
	}
}
