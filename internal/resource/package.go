package resource

import (
	"context"
	"fmt"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// PackageHandler batches all package declarations into a single Homebrew diff.
type PackageHandler struct{}

func (PackageHandler) DeclType() decl.Type { return decl.Package }
func (PackageHandler) DeclName() string    { return "Package" }

func (PackageHandler) PlanBatch(ctx context.Context, store *fact.Store, env Env, decls []decl.Declaration) (PlanOutput, error) {
	packages := make([]action.DesiredPackage, len(decls))
	for i, d := range decls {
		packages[i] = action.DesiredPackage{
			Name:   d.PackageName,
			Absent: d.State == decl.Absent,
		}
	}

	brewFact, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	pkgActions, err := action.DiffHomebrew(packages, brewFact)
	if err != nil {
		return PlanOutput{}, err
	}

	hasAction := make(map[string]bool, len(pkgActions))
	for _, a := range pkgActions {
		hasAction[a.PackageName] = true
	}

	var out PlanOutput
	for _, pkg := range packages {
		if !hasAction[pkg.Name] {
			msg := fmt.Sprintf("%s (installed)", pkg.Name)
			if pkg.Absent {
				msg = fmt.Sprintf("%s (already absent)", pkg.Name)
			}
			out.Observations = append(out.Observations, action.Observation{Description: msg})
		}
	}
	out.Actions = pkgActions
	return out, nil
}
