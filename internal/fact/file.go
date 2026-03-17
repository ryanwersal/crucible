package fact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// FileInfo holds the observed state of a file path.
type FileInfo struct {
	Exists bool
	Hash   string // SHA256 hex digest; empty if !Exists or IsDir
	Mode   fs.FileMode
	UID    uint32
	GID    uint32
	Size   int64
	IsDir  bool
	IsLink bool
}

// FileCollector collects facts about a file at Path.
type FileCollector struct {
	Path string
}

func (f FileCollector) Collect(_ context.Context) (*FileInfo, error) {
	info, err := os.Lstat(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &FileInfo{Exists: false}, nil
	}
	if err != nil {
		return nil, err
	}

	fi := &FileInfo{
		Exists: true,
		Mode:   info.Mode().Perm(),
		Size:   info.Size(),
		IsDir:  info.IsDir(),
		IsLink: info.Mode()&fs.ModeSymlink != 0,
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		fi.UID = stat.Uid
		fi.GID = stat.Gid
	}

	if !fi.IsDir && !fi.IsLink {
		h, err := hashFile(f.Path)
		if err != nil {
			return nil, err
		}
		fi.Hash = h
	}

	return fi, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
