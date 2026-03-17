package fact

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// DirInfo holds the observed state of a directory.
type DirInfo struct {
	Exists   bool
	Mode     fs.FileMode
	UID      uint32
	GID      uint32
	Children []string // names of immediate children
}

// DirCollector collects facts about a directory at Path.
type DirCollector struct {
	Path string
}

func (d DirCollector) Collect(_ context.Context) (*DirInfo, error) {
	info, err := os.Lstat(d.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &DirInfo{Exists: false}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return &DirInfo{Exists: false}, nil
	}

	di := &DirInfo{
		Exists: true,
		Mode:   info.Mode().Perm(),
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		di.UID = stat.Uid
		di.GID = stat.Gid
	}

	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, err
	}
	di.Children = make([]string, 0, len(entries))
	for _, e := range entries {
		di.Children = append(di.Children, e.Name())
	}

	return di, nil
}
