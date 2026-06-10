package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// GitRepoHandler plans actions for git repository declarations.
type GitRepoHandler struct{}

func (GitRepoHandler) DeclType() decl.Type { return decl.GitRepo }
func (GitRepoHandler) DeclName() string    { return "GitRepo" }

func (GitRepoHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	factKey := fmt.Sprintf("gitrepo:%s", d.Path)
	repoFact, err := fact.Get(ctx, store, factKey, fact.GitRepoCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	acts, obs := action.DiffGitRepo(action.DesiredGitRepo{
		Path:   d.Path,
		URL:    d.GitURL,
		Branch: d.GitBranch,
	}, repoFact)
	// DiffGitRepo always reports the in-sync case as an observation, so no
	// synthetic "up to date" fallback is needed here.
	return PlanOutput{Actions: acts, Observations: obs}, nil
}
