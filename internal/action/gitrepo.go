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
		desc := fmt.Sprintf("git clone %s → %s", desired.URL, desired.Path)
		if desired.Branch != "" {
			desc = fmt.Sprintf("git clone %s → %s (branch %s)", desired.URL, desired.Path, desired.Branch)
		}
		return []Action{{
			Type:        CloneRepo,
			Path:        desired.Path,
			GitURL:      desired.URL,
			GitBranch:   desired.Branch,
			Description: desc,
		}}, nil
	}

	// Repo exists — check remote URL
	if actual.RemoteURL != desired.URL {
		// Don't clobber — warn via observation
		return nil, []Observation{{
			Description: fmt.Sprintf("%s: remote URL mismatch (want %s, have %s)", desired.Path, desired.URL, actual.RemoteURL),
		}}
	}

	var obs []Observation
	// `git pull` fast-forwards the checked-out branch; it never switches
	// branches. If the declared branch differs from what's checked out, the
	// declaration silently won't take effect, so surface it rather than
	// pretending the branch is managed.
	if desired.Branch != "" && actual.CurrentBranch != "" && desired.Branch != actual.CurrentBranch {
		obs = append(obs, Observation{
			Description: fmt.Sprintf("%s: on branch %s but %s is declared; pull won't switch branches", desired.Path, actual.CurrentBranch, desired.Branch),
		})
	}

	// If we were able to read both local and remote HEAD SHAs and they
	// match, there's nothing to pull. Report the current branch and SHA so
	// the user can confirm what's checked out even when nothing changes.
	if actual.LocalSHA != "" && actual.RemoteSHA != "" && actual.LocalSHA == actual.RemoteSHA {
		obs = append(obs, Observation{
			Description: upToDateDesc(desired.Path, actual),
		})
		return nil, obs
	}

	// Otherwise (drift, or remote SHA unavailable) emit a pull so we stay in
	// sync, describing the branch and the local→remote SHA transition.
	return []Action{{
		Type:        PullRepo,
		Path:        desired.Path,
		GitURL:      desired.URL,
		GitBranch:   desired.Branch,
		Description: pullDesc(desired.Path, actual),
	}}, obs
}

// pullDesc builds the plan description for a pull, annotating it with the
// branch and the local→remote SHA transition when those are known. Examples:
//
//	git pull ~/src/foo (main: 1d7b548 → 11b6466)   both SHAs known
//	git pull ~/src/foo (main: 1d7b548 → remote unknown)   ls-remote failed
//	git pull ~/src/foo (main)                      SHAs unavailable
func pullDesc(path string, actual *fact.GitRepoInfo) string {
	base := fmt.Sprintf("git pull %s", path)
	branch := actual.CurrentBranch
	switch {
	case actual.LocalSHA != "" && actual.RemoteSHA != "":
		return fmt.Sprintf("%s (%s: %s → %s)", base, branch, shortSHA(actual.LocalSHA), shortSHA(actual.RemoteSHA))
	case actual.LocalSHA != "":
		return fmt.Sprintf("%s (%s: %s → remote unknown)", base, branch, shortSHA(actual.LocalSHA))
	case branch != "":
		return fmt.Sprintf("%s (%s)", base, branch)
	default:
		return base
	}
}

// upToDateDesc describes an in-sync repo, e.g. "~/src/foo (up to date, main@1d7b548)".
func upToDateDesc(path string, actual *fact.GitRepoInfo) string {
	switch {
	case actual.CurrentBranch != "" && actual.LocalSHA != "":
		return fmt.Sprintf("%s (up to date, %s@%s)", path, actual.CurrentBranch, shortSHA(actual.LocalSHA))
	case actual.CurrentBranch != "":
		return fmt.Sprintf("%s (up to date, %s)", path, actual.CurrentBranch)
	default:
		return fmt.Sprintf("%s (up to date)", path)
	}
}

// shortSHA abbreviates a 40-char git object name to its 7-char prefix, matching
// git's own default abbreviation. Shorter or empty inputs are returned as-is.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
