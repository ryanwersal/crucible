package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredGitRepo describes a git repository that should exist at a given path.
type DesiredGitRepo struct {
	Path   string
	URL    string
	Branch string
}

// DiffGitRepo compares the desired git repo against the current state.
func DiffGitRepo(desired DesiredGitRepo, actual *fact.GitRepoInfo) ([]Action, []Observation) {
	if actual == nil || !actual.Exists {
		return []Action{{
			Type:        CloneRepo,
			Path:        desired.Path,
			GitURL:      desired.URL,
			GitBranch:   desired.Branch,
			Description: fmt.Sprintf("git clone %s → %s", desired.URL, desired.Path),
		}}, nil
	}

	// Repo exists — check remote URL
	if actual.RemoteURL != desired.URL {
		// Don't clobber — warn via observation
		return nil, []Observation{{
			Description: fmt.Sprintf("%s: remote URL mismatch (want %s, have %s)", desired.Path, desired.URL, actual.RemoteURL),
		}}
	}

	// If we were able to read both local and remote HEAD SHAs and they
	// match, there's nothing to pull. Otherwise (drift, or remote SHA
	// unavailable) emit a pull so we stay in sync.
	if actual.LocalSHA != "" && actual.RemoteSHA != "" && actual.LocalSHA == actual.RemoteSHA {
		return nil, nil
	}

	return []Action{{
		Type:        PullRepo,
		Path:        desired.Path,
		GitURL:      desired.URL,
		GitBranch:   desired.Branch,
		Description: fmt.Sprintf("git pull %s", desired.Path),
	}}, nil
}
