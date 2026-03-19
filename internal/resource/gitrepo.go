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
	var out PlanOutput
	out.Actions = acts
	out.Observations = obs
	if len(acts) == 0 && len(obs) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: fmt.Sprintf("%s (up to date)", d.Path),
		})
	}
	return out, nil
}
