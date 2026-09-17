package resource

import (
	"context"
	"fmt"
	"io"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func homebrewDeclarations(decls []decl.Declaration) ([]action.DesiredHomebrewTap, []action.DesiredPackage) {
	var taps []action.DesiredHomebrewTap
	var packages []action.DesiredPackage
	for _, d := range decls {
		switch d.Type {
		case decl.HomebrewTap:
			taps = append(taps, action.DesiredHomebrewTap{Name: d.TapName, Absent: d.State == decl.Absent})
		case decl.Package:
			packages = append(packages, action.DesiredPackage{Name: d.PackageName, Kind: d.PackageKind, Absent: d.State == decl.Absent, Latest: d.State == decl.Latest})
		}
	}
	return taps, packages
}

func validateHomebrewTaps(ctx context.Context, store *fact.Store, decls []decl.Declaration, brew *fact.HomebrewInfo) (*fact.HomebrewTapInfo, error) {
	actual, err := fact.Get(ctx, store, "homebrew-taps", fact.HomebrewTapCollector{})
	if err != nil {
		return nil, err
	}
	taps, packages := homebrewDeclarations(decls)
	if err := action.ValidateHomebrewTaps(taps, packages, actual, brew); err != nil {
		return nil, err
	}
	return actual, nil
}

type HomebrewTapHandler struct{}

func (HomebrewTapHandler) DeclType() decl.Type { return decl.HomebrewTap }
func (HomebrewTapHandler) DeclName() string    { return "HomebrewTap" }
func (HomebrewTapHandler) PlanBatch(ctx context.Context, store *fact.Store, env Env, declarations []decl.Declaration) (PlanOutput, error) {
	brew, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{})
	if err != nil {
		return PlanOutput{}, err
	}
	all := env.Declarations
	if all == nil {
		all = declarations
	}
	actual, err := validateHomebrewTaps(ctx, store, all, brew)
	if err != nil {
		return PlanOutput{}, err
	}
	taps, _ := homebrewDeclarations(declarations)
	actions, observations, err := action.DiffHomebrewTaps(taps, actual)
	return PlanOutput{Actions: actions, Observations: observations}, err
}

type AddHomebrewTapExecutor struct{}

func (AddHomebrewTapExecutor) ActionType() action.Type { return action.AddHomebrewTap }
func (AddHomebrewTapExecutor) ActionName() string      { return "AddHomebrewTap" }
func (AddHomebrewTapExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return runCmd(ctx, a, stdin, stdout, stderr, "brew", "tap", a.TapName)
}

type RemoveHomebrewTapExecutor struct{}

func (RemoveHomebrewTapExecutor) ActionType() action.Type { return action.RemoveHomebrewTap }
func (RemoveHomebrewTapExecutor) ActionName() string      { return "RemoveHomebrewTap" }
func (RemoveHomebrewTapExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	// Recheck at execution time: failed uninstalls or out-of-band installations
	// must not turn untap into an interactive request to delete other packages.
	actual, err := (fact.HomebrewCollector{}).Collect(ctx)
	if err != nil {
		return err
	}
	for _, pkg := range actual.InstalledPackages {
		if pkg.Tap == a.TapName {
			return fmt.Errorf("cannot remove tap %s: %s %s is still installed", a.TapName, pkg.Kind, pkg.Name)
		}
	}
	return runCmd(ctx, a, stdin, stdout, stderr, "brew", "untap", a.TapName)
}
