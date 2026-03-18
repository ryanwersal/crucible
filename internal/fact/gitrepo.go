package fact

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRepoInfo holds the current state of a git repository on disk.
type GitRepoInfo struct {
	Exists        bool
	CurrentBranch string
	RemoteURL     string // origin URL
}

// GitRepoCollector checks the state of a git repository at a given path.
type GitRepoCollector struct {
	Path string
}

// Collect checks whether a git repo exists at the configured path and
// reads its current branch and remote URL.
func (c GitRepoCollector) Collect(ctx context.Context) (*GitRepoInfo, error) {
	gitDir := filepath.Join(c.Path, ".git")
	if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
		return &GitRepoInfo{Exists: false}, nil
	} else if err != nil {
		return nil, err
	}

	info := &GitRepoInfo{Exists: true}

	// Get remote URL
	urlCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "remote", "get-url", "origin")
	if out, err := urlCmd.Output(); err == nil {
		info.RemoteURL = strings.TrimSpace(string(out))
	}

	// Get current branch
	branchCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "branch", "--show-current")
	if out, err := branchCmd.Output(); err == nil {
		info.CurrentBranch = strings.TrimSpace(string(out))
	}

	return info, nil
}
