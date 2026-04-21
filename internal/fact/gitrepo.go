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
	LocalSHA      string // HEAD commit on the current branch
	RemoteSHA     string // origin's HEAD for the current branch (via ls-remote); empty if unknown
}

// GitRepoCollector checks the state of a git repository at a given path.
type GitRepoCollector struct {
	Path string
}

// Collect checks whether a git repo exists at the configured path and
// reads its current branch, remote URL, and local/remote HEAD SHAs.
// The remote SHA is obtained via `git ls-remote`, which negotiates refs
// without downloading objects, so it is cheap compared to `git fetch`.
// If the remote is unreachable, RemoteSHA is left empty.
func (c GitRepoCollector) Collect(ctx context.Context) (*GitRepoInfo, error) {
	gitDir := filepath.Join(c.Path, ".git")
	if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
		return &GitRepoInfo{Exists: false}, nil
	} else if err != nil {
		return nil, err
	}

	info := &GitRepoInfo{Exists: true}

	urlCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "remote", "get-url", "origin")
	if out, err := urlCmd.Output(); err == nil {
		info.RemoteURL = strings.TrimSpace(string(out))
	}

	branchCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "branch", "--show-current")
	if out, err := branchCmd.Output(); err == nil {
		info.CurrentBranch = strings.TrimSpace(string(out))
	}

	headCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "rev-parse", "HEAD")
	if out, err := headCmd.Output(); err == nil {
		info.LocalSHA = strings.TrimSpace(string(out))
	}

	if info.RemoteURL != "" && info.CurrentBranch != "" {
		ref := "refs/heads/" + info.CurrentBranch
		lsCmd := exec.CommandContext(ctx, "git", "-C", c.Path, "ls-remote", "origin", ref)
		// Prevent git from prompting for credentials — we want a fast
		// "remote unknown" fallback rather than blocking on interactive input.
		lsCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := lsCmd.Output(); err == nil {
			if sha, _, ok := strings.Cut(strings.TrimSpace(string(out)), "\t"); ok {
				info.RemoteSHA = sha
			}
		}
	}

	return info, nil
}
