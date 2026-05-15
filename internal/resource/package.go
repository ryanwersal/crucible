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
			Latest: d.State == decl.Latest,
		}
	}

	brewFact, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	pkgActions, diffObs, noted, err := action.DiffHomebrew(packages, brewFact)
	if err != nil {
		return PlanOutput{}, err
	}

	out := PlanOutput{Actions: pkgActions, Observations: diffObs}
	for _, pkg := range packages {
		if noted[pkg.Name] {
			continue
		}
		msg := fmt.Sprintf("%s (installed)", pkg.Name)
		if pkg.Absent {
			msg = fmt.Sprintf("%s (already absent)", pkg.Name)
		}
		out.Observations = append(out.Observations, action.Observation{Description: msg})
	}
	return out, nil
}
